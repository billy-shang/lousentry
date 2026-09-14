package ctrl

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kataras/golog"
	"github.com/google/go-github/v53/github"
	"github.com/pkg/errors"
	"lousentry/ent"
	"lousentry/ent/vulninformation"
	"lousentry/push"
	"lousentry/store"
)

// MonitorSettings 控制台「监控设置」可写字段。
type MonitorSettings struct {
	Sources         []string `json:"sources"`
	Interval        string   `json:"interval"`
	EnableCVEFilter bool     `json:"enable_cve_filter"`
	NoGithubSearch  bool     `json:"no_github_search"`
	NoStartMessage  bool     `json:"no_start_message"`
	NoSleep         bool     `json:"no_sleep"`
	NoFilter        bool     `json:"no_filter"`
	WhiteKeywords   []string `json:"white_keywords"`
	BlackKeywords   []string `json:"black_keywords"`
	Proxy           string   `json:"proxy"`
	SkipTLSVerify   bool     `json:"skip_tls_verify"`
}

// PushChannel 控制台仅管理钉钉 / 飞书 / 企业微信。
type PushChannel struct {
	Enabled     bool   `json:"enabled"`
	AccessToken string `json:"access_token"`
	SignSecret  string `json:"sign_secret"`
	Key         string `json:"key"`
}

// PushSettings 推送渠道配置。
type PushSettings struct {
	DingDing   PushChannel `json:"dingding"`
	Lark       PushChannel `json:"lark"`
	WechatWork PushChannel `json:"wechatwork"`
}

func (w *App) GetMonitorSettings() MonitorSettings {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c := w.config
	return MonitorSettings{
		Sources:         append([]string{}, c.Sources...),
		Interval:        c.Interval,
		EnableCVEFilter: boolVal(c.EnableCVEFilter, true),
		NoGithubSearch:  boolVal(c.NoGithubSearch, false),
		NoStartMessage:  boolVal(c.NoStartMessage, false),
		NoSleep:         boolVal(c.NoSleep, false),
		NoFilter:        c.NoFilter,
		WhiteKeywords:   append([]string{}, c.WhiteKeywords...),
		BlackKeywords:   append([]string{}, c.BlackKeywords...),
		Proxy:           c.Proxy,
		SkipTLSVerify:   c.SkipTLSVerify,
	}
}

func (w *App) GetPushSettings() PushSettings {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.pushSettings
}

func (w *App) ApplyMonitor(settings MonitorSettings) error {
	if len(settings.Sources) == 0 {
		return fmt.Errorf("至少选择一个数据源")
	}
	interval, err := time.ParseDuration(settings.Interval)
	if err != nil {
		return fmt.Errorf("检查周期格式错误，例如 30m 或 1h")
	}
	if interval < time.Minute {
		return fmt.Errorf("检查周期至少 1 分钟")
	}
	grabs, err := BuildGrabbers(settings.Sources)
	if err != nil {
		return err
	}

	normalized := make([]string, 0, len(settings.Sources))
	for _, s := range settings.Sources {
		id := NormalizeSource(s)
		if id != "" {
			normalized = append(normalized, id)
		}
	}

	w.mu.Lock()
	w.config.Sources = normalized
	w.config.Interval = settings.Interval
	w.config.IntervalParsed = interval
	w.config.EnableCVEFilter = boolPtr(settings.EnableCVEFilter)
	w.config.NoGithubSearch = boolPtr(settings.NoGithubSearch)
	w.config.NoStartMessage = boolPtr(settings.NoStartMessage)
	w.config.NoSleep = boolPtr(settings.NoSleep)
	w.config.NoFilter = settings.NoFilter
	w.config.WhiteKeywords = cleanKeywords(settings.WhiteKeywords)
	w.config.BlackKeywords = cleanKeywords(settings.BlackKeywords)
	w.config.Proxy = strings.TrimSpace(settings.Proxy)
	w.config.SkipTLSVerify = settings.SkipTLSVerify
	w.grabbers = grabs
	w.applyProxyLocked()
	w.mu.Unlock()

	select {
	case w.intervalUpdate <- interval:
	default:
	}
	w.log.Infof("监控设置已更新: sources=%v interval=%s", normalized, settings.Interval)
	return w.persistConfig()
}

func (w *App) ApplyPush(settings PushSettings) error {
	settings = normalizePushSettings(settings)
	if err := validatePushSettings(settings); err != nil {
		return err
	}

	w.mu.Lock()
	w.pushSettings = settings
	applyPushSettingsToConfig(w.config, settings)
	w.config.AllowEmptyPusher = true
	textPusher, rawPusher, err := w.config.GetPusher()
	if err != nil {
		w.mu.Unlock()
		return err
	}
	w.textPusher = textPusher
	w.rawPusher = rawPusher
	w.mu.Unlock()

	w.log.Infof("推送设置已更新并写入 sqlite")
	if err := w.persistPushChannels(); err != nil {
		return err
	}
	return w.persistConfig()
}

func (w *App) PushVulnByID(ctx context.Context, id int) error {
	item, err := w.db.VulnInformation.Get(ctx, id)
	if err != nil {
		return err
	}
	info := w.dbToVulnInfo(item)
	w.mu.RLock()
	err = w.pushVuln(info)
	w.mu.RUnlock()
	if err != nil {
		return err
	}
	_, err = item.Update().SetPushed(true).Save(ctx)
	if err != nil {
		return errors.Wrap(err, "标记已推送失败")
	}
	w.log.Infof("手动推送漏洞成功: %s", item.Title)
	return nil
}

func (w *App) PushUnpushed(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	items, err := w.db.VulnInformation.Query().
		Where(vulninformation.PushedEQ(false)).
		Order(ent.Desc(vulninformation.FieldUpdateTime)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return 0, err
	}
	ok := 0
	for _, item := range items {
		info := w.dbToVulnInfo(item)
		w.mu.RLock()
		err = w.pushVuln(info)
		w.mu.RUnlock()
		if err != nil {
			w.log.Errorf("手动推送失败 %s: %v", item.Title, err)
			continue
		}
		if _, err = item.Update().SetPushed(true).Save(ctx); err != nil {
			w.log.Errorf("标记已推送失败 %s: %v", item.Title, err)
			continue
		}
		ok++
	}
	w.log.Infof("批量手动推送完成: success=%d total=%d", ok, len(items))
	return ok, nil
}

func (w *App) SendDailyReport(ctx context.Context) error {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	items, err := w.db.VulnInformation.Query().
		Where(vulninformation.CreateTimeGTE(today)).
		Order(ent.Desc(vulninformation.FieldUpdateTime)).
		Limit(30).
		All(ctx)
	if err != nil {
		return err
	}
	stats, err := w.GetStats(ctx)
	if err != nil {
		return err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "漏哨 每日报告\n")
	fmt.Fprintf(&b, "日期: %s\n", now.Format("2006-01-02"))
	fmt.Fprintf(&b, "今日新增: %d\n", stats.TodayNew)
	fmt.Fprintf(&b, "已推送: %d\n", stats.Pushed)
	fmt.Fprintf(&b, "未推送: %d\n", stats.Unpushed)
	fmt.Fprintf(&b, "数据源: %d\n\n", stats.SourceCount)
	if len(items) == 0 {
		b.WriteString("今日暂无新增漏洞。")
	} else {
		b.WriteString("今日新增漏洞:\n")
		for i, item := range items {
			cve := item.Cve
			if cve == "" {
				cve = "-"
			}
			fmt.Fprintf(&b, "%d. [%s] %s (%s)\n", i+1, item.Severity, item.Title, cve)
		}
	}

	w.mu.RLock()
	err = w.textPusher.PushMarkdown("漏哨 每日报告", b.String())
	if rawErr := w.rawPusher.PushRaw(push.NewRawTextMessage(b.String())); rawErr != nil && err == nil {
		err = rawErr
	}
	w.mu.RUnlock()
	if err != nil {
		return err
	}
	w.log.Infof("每日报告已推送, 今日新增=%d", stats.TodayNew)
	return nil
}

func (w *App) TestPush() error {
	msg := fmt.Sprintf("漏哨 测试推送成功\n时间: %s", time.Now().Format("2006-01-02 15:04:05"))
	w.mu.RLock()
	err := w.textPusher.PushMarkdown("漏哨 测试推送", msg)
	if rawErr := w.rawPusher.PushRaw(push.NewRawTextMessage(msg)); rawErr != nil && err == nil {
		err = rawErr
	}
	w.mu.RUnlock()
	if err != nil {
		return err
	}
	w.log.Infof("测试推送已发送")
	return nil
}

func (w *App) TriggerCheck() error {
	if !w.checking.CompareAndSwap(false, true) {
		return fmt.Errorf("检查任务正在进行中，请稍后再试")
	}
	go func() {
		defer w.checking.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		w.log.Infof("收到立即检查请求")
		w.collectAndPush(ctx)
		w.markLastCheck()
	}()
	return nil
}

func (w *App) persistConfig() error {
	if w.meta == nil {
		return nil
	}
	w.mu.RLock()
	cfg := snapshotAppConfig(w.config)
	w.mu.RUnlock()
	return w.meta.SaveAppConfig(cfg)
}

func (w *App) persistPushChannels() error {
	if w.meta == nil {
		return nil
	}
	w.mu.RLock()
	items := pushSettingsToRows(w.pushSettings)
	w.mu.RUnlock()
	return w.meta.SavePushChannels(items)
}

func loadOrInitPushSettings(config *AppConfig, meta *store.Store) (PushSettings, error) {
	saved, ok, err := meta.LoadPushChannels()
	if err != nil {
		return PushSettings{}, err
	}
	if ok {
		settings := pushRowsToSettings(saved)
		golog.Child("[store]").Infof("已从 sqlite 加载钉钉/飞书/企业微信 webhook")
		return settings, nil
	}
	settings := pushSettingsFromConfig(config.Pusher)
	if err = meta.SavePushChannels(pushSettingsToRows(settings)); err != nil {
		return PushSettings{}, err
	}
	golog.Child("[store]").Infof("已将当前 webhook 配置迁入 sqlite web_pushers")
	return settings, nil
}

func applyPushSettingsToConfig(config *AppConfig, settings PushSettings) {
	kept := make([]map[string]string, 0, 3)
	if settings.DingDing.Enabled && strings.TrimSpace(settings.DingDing.AccessToken) != "" {
		kept = append(kept, map[string]string{
			"type":         push.TypeDingDing,
			"access_token": strings.TrimSpace(settings.DingDing.AccessToken),
			"sign_secret":  strings.TrimSpace(settings.DingDing.SignSecret),
		})
	}
	if settings.Lark.Enabled && strings.TrimSpace(settings.Lark.AccessToken) != "" {
		kept = append(kept, map[string]string{
			"type":         push.TypeLark,
			"access_token": strings.TrimSpace(settings.Lark.AccessToken),
			"sign_secret":  strings.TrimSpace(settings.Lark.SignSecret),
		})
	}
	if settings.WechatWork.Enabled && strings.TrimSpace(settings.WechatWork.Key) != "" {
		kept = append(kept, map[string]string{
			"type": push.TypeWechatWork,
			"key":  strings.TrimSpace(settings.WechatWork.Key),
		})
	}
	config.Pusher = kept
}

func pushSettingsFromConfig(pushers []map[string]string) PushSettings {
	out := PushSettings{}
	for _, item := range pushers {
		switch item["type"] {
		case push.TypeDingDing:
			out.DingDing = PushChannel{
				Enabled:     strings.TrimSpace(item["access_token"]) != "",
				AccessToken: item["access_token"],
				SignSecret:  item["sign_secret"],
			}
		case push.TypeLark:
			out.Lark = PushChannel{
				Enabled:     strings.TrimSpace(item["access_token"]) != "",
				AccessToken: item["access_token"],
				SignSecret:  item["sign_secret"],
			}
		case push.TypeWechatWork:
			out.WechatWork = PushChannel{
				Enabled: strings.TrimSpace(item["key"]) != "",
				Key:     item["key"],
			}
		}
	}
	return out
}

func pushSettingsToRows(settings PushSettings) []store.PushChannel {
	return []store.PushChannel{
		{
			Channel:     push.TypeDingDing,
			Enabled:     settings.DingDing.Enabled,
			AccessToken: settings.DingDing.AccessToken,
			SignSecret:  settings.DingDing.SignSecret,
		},
		{
			Channel:     push.TypeLark,
			Enabled:     settings.Lark.Enabled,
			AccessToken: settings.Lark.AccessToken,
			SignSecret:  settings.Lark.SignSecret,
		},
		{
			Channel:     push.TypeWechatWork,
			Enabled:     settings.WechatWork.Enabled,
			ExtraKey:    settings.WechatWork.Key,
		},
	}
}

func pushRowsToSettings(items []store.PushChannel) PushSettings {
	out := PushSettings{}
	for _, item := range items {
		switch item.Channel {
		case push.TypeDingDing:
			out.DingDing = PushChannel{
				Enabled:     item.Enabled,
				AccessToken: item.AccessToken,
				SignSecret:  item.SignSecret,
			}
		case push.TypeLark:
			out.Lark = PushChannel{
				Enabled:     item.Enabled,
				AccessToken: item.AccessToken,
				SignSecret:  item.SignSecret,
			}
		case push.TypeWechatWork:
			out.WechatWork = PushChannel{
				Enabled: item.Enabled,
				Key:     item.ExtraKey,
			}
		}
	}
	return out
}

func normalizePushSettings(in PushSettings) PushSettings {
	in.DingDing.AccessToken = strings.TrimSpace(in.DingDing.AccessToken)
	in.DingDing.SignSecret = strings.TrimSpace(in.DingDing.SignSecret)
	in.Lark.AccessToken = strings.TrimSpace(in.Lark.AccessToken)
	in.Lark.SignSecret = strings.TrimSpace(in.Lark.SignSecret)
	in.WechatWork.Key = strings.TrimSpace(in.WechatWork.Key)
	if !in.DingDing.Enabled {
		in.DingDing.AccessToken = ""
		in.DingDing.SignSecret = ""
	}
	if !in.Lark.Enabled {
		in.Lark.AccessToken = ""
		in.Lark.SignSecret = ""
	}
	if !in.WechatWork.Enabled {
		in.WechatWork.Key = ""
	}
	return in
}

func validatePushSettings(settings PushSettings) error {
	if settings.DingDing.Enabled && settings.DingDing.AccessToken == "" {
		return fmt.Errorf("已启用钉钉，请填写 Access Token / Webhook")
	}
	if settings.Lark.Enabled && settings.Lark.AccessToken == "" {
		return fmt.Errorf("已启用飞书，请填写 Access Token / Webhook")
	}
	if settings.WechatWork.Enabled && settings.WechatWork.Key == "" {
		return fmt.Errorf("已启用企业微信，请填写机器人 Key")
	}
	return nil
}

func applyPersistedConfig(config *AppConfig, meta *store.Store) error {
	saved, ok, err := meta.LoadAppConfig()
	if err != nil {
		return err
	}
	if !ok {
		// 首次启动：把当前 CLI/配置文件写入 sqlite，之后都以数据库为准
		return meta.SaveAppConfig(snapshotAppConfig(config))
	}
	if len(saved.Sources) > 0 {
		config.Sources = saved.Sources
	}
	if saved.Interval != "" {
		parsed, err := time.ParseDuration(saved.Interval)
		if err != nil {
			return err
		}
		config.Interval = saved.Interval
		config.IntervalParsed = parsed
	}
	config.EnableCVEFilter = boolPtr(saved.EnableCVEFilter)
	config.NoGithubSearch = boolPtr(saved.NoGithubSearch)
	config.NoStartMessage = boolPtr(saved.NoStartMessage)
	config.NoSleep = boolPtr(saved.NoSleep)
	config.NoFilter = saved.NoFilter
	config.WhiteKeywords = saved.WhiteKeywords
	config.BlackKeywords = saved.BlackKeywords
	config.Proxy = saved.Proxy
	config.SkipTLSVerify = saved.SkipTLSVerify
	if saved.Pusher != nil {
		config.Pusher = saved.Pusher
	}
	golog.Child("[store]").Infof("已从 sqlite 加载监控与推送配置")
	return nil
}

func snapshotAppConfig(c *AppConfig) *store.AppConfig {
	return &store.AppConfig{
		Sources:         append([]string{}, c.Sources...),
		Interval:        c.Interval,
		EnableCVEFilter: boolVal(c.EnableCVEFilter, true),
		NoGithubSearch:  boolVal(c.NoGithubSearch, false),
		NoStartMessage:  boolVal(c.NoStartMessage, false),
		NoSleep:         boolVal(c.NoSleep, false),
		NoFilter:        c.NoFilter,
		WhiteKeywords:   append([]string{}, c.WhiteKeywords...),
		BlackKeywords:   append([]string{}, c.BlackKeywords...),
		Proxy:           c.Proxy,
		SkipTLSVerify:   c.SkipTLSVerify,
		Pusher:          c.Pusher,
	}
}

func (w *App) applyProxyLocked() {
	if w.config.Proxy != "" {
		_ = os.Setenv("HTTP_PROXY", w.config.Proxy)
		_ = os.Setenv("HTTPS_PROXY", w.config.Proxy)
	} else {
		_ = os.Unsetenv("HTTP_PROXY")
		_ = os.Unsetenv("HTTPS_PROXY")
	}
	if w.config.SkipTLSVerify {
		_ = os.Setenv("GO_SKIP_TLS_CHECK", "1")
	} else {
		_ = os.Unsetenv("GO_SKIP_TLS_CHECK")
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = http.ProxyFromEnvironment
	w.githubClient = github.NewClient(&http.Client{
		Timeout:   time.Second * 10,
		Transport: tr,
	})
}

func cleanKeywords(in []string) []string {
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func boolVal(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

func boolPtr(v bool) *bool {
	return &v
}

