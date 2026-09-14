package ctrl

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	entSql "entgo.io/ent/dialect/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/go-github/v53/github"
	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/kataras/golog"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"modernc.org/sqlite"

	"lousentry/ent"
	"lousentry/ent/migrate"
	"lousentry/ent/vulninformation"
	"lousentry/grab"
	"lousentry/push"
	"lousentry/store"
)

func init() {
	sql.Register("sqlite3", &sqlite.Driver{})
	sql.Register("postgres", &stdlib.Driver{})
}

const (
	// InitPageLimit must >= UpdatePageLimit !
	InitPageLimit   = 3
	UpdatePageLimit = 1
)

type App struct {
	config     *AppConfig
	textPusher push.TextPusher
	rawPusher  push.RawPusher

	log            *golog.Logger
	db             *ent.Client
	meta           *store.Store
	githubClient   *github.Client
	grabbers       []grab.Grabber
	prs            []*github.PullRequest
	mu             sync.RWMutex
	checking       atomic.Bool
	intervalUpdate chan time.Duration
	pushSettings   PushSettings
	lastCheckAt    time.Time
	nextCheckAt    time.Time
}

func NewApp(config *AppConfig) (*App, error) {
	config.Init()
	drvName, connStr, err := config.DBConnForEnt()
	if err != nil {
		return nil, err
	}
	drv, err := entSql.Open(drvName, connStr)
	if err != nil {
		return nil, errors.Wrap(err, "failed opening connection to db")
	}
	db := drv.DB()
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(time.Minute * 1)
	db.SetMaxIdleConns(1)
	dbClient := ent.NewClient(ent.Driver(drv))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := dbClient.Schema.Create(ctx, migrate.WithDropIndex(true), migrate.WithDropColumn(true)); err != nil {
		return nil, errors.Wrap(err, "failed creating schema resources")
	}

	meta, err := store.New(db, drvName)
	if err != nil {
		return nil, errors.Wrap(err, "failed creating sqlite meta tables")
	}
	if err := applyPersistedConfig(config, meta); err != nil {
		return nil, errors.Wrap(err, "failed loading sqlite config")
	}
	pushSettings, err := loadOrInitPushSettings(config, meta)
	if err != nil {
		return nil, errors.Wrap(err, "failed loading sqlite pushers")
	}
	applyPushSettingsToConfig(config, pushSettings)
	if config.Proxy != "" {
		_ = os.Setenv("HTTP_PROXY", config.Proxy)
		_ = os.Setenv("HTTPS_PROXY", config.Proxy)
	}

	textPusher, rawPusher, err := config.GetPusher()
	if err != nil {
		return nil, err
	}

	grabs, err := BuildGrabbers(config.Sources)
	if err != nil {
		return nil, err
	}

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = http.ProxyFromEnvironment
	githubClient := github.NewClient(&http.Client{
		Timeout:   time.Second * 10,
		Transport: tr,
	})

	return &App{
		config:         config,
		textPusher:     textPusher,
		rawPusher:      rawPusher,
		log:            golog.Child("[ctrl]"),
		db:             dbClient,
		meta:           meta,
		githubClient:   githubClient,
		grabbers:       grabs,
		intervalUpdate: make(chan time.Duration, 1),
		pushSettings:   pushSettings,
	}, nil
}

func (w *App) Run(ctx context.Context) error {
	if *w.config.DiffMode {
		w.log.Info("running in diff mode, skip init vuln database")
		w.collectAndPush(ctx)
		w.log.Info("diff finished")
		return nil
	}

	if w.config.Test {
		w.log.Info("running in test mode, the mocked message will be sent")
		if err := w.testPushMessage(); err != nil {
			return err
		}
		w.log.Infof("test finished")
		return nil
	}

	w.log.Infof("initialize local database..")
	success, fail := w.initData(ctx)
	w.mu.Lock()
	w.grabbers = success
	w.mu.Unlock()
	localCount, err := w.db.VulnInformation.Query().Count(ctx)
	if err != nil {
		return err
	}
	w.log.Infof("system init finished, local database has %d vulns", localCount)
	if !*w.config.NoStartMessage {
		providers := make([]*grab.Provider, 0, 10)
		failed := make([]*grab.Provider, 0, 10)
		for _, p := range w.grabbers {
			providers = append(providers, p.ProviderInfo())
		}
		for _, p := range fail {
			failed = append(failed, p.ProviderInfo())
		}
		msg := &push.InitialMessage{
			Version:        w.config.Version,
			VulnCount:      localCount,
			Interval:       w.config.IntervalParsed.String(),
			Provider:       providers,
			FailedProvider: failed,
		}
		if err := w.textPusher.PushMarkdown("漏哨 初始化完成", push.RenderInitialMsg(msg)); err != nil {
			return err
		}
		if err := w.rawPusher.PushRaw(push.NewRawInitialMessage(msg)); err != nil {
			return err
		}
	}

	w.log.Infof("ticking every %s", w.config.Interval)

	// 初始化只入库、不推送历史漏洞。完成后立刻做一轮增量：
	// 只有新出现或等级/标签升级、且符合策略的漏洞，才会自动推到 webhook。
	if w.checking.CompareAndSwap(false, true) {
		w.log.Infof("初始化完成，开始首次增量检查并自动推送")
		w.collectAndPush(ctx)
		w.markLastCheck()
		w.checking.Store(false)
	}

	defer func() {
		msg := "注意: 漏哨 进程退出"
		w.mu.RLock()
		textPusher := w.textPusher
		rawPusher := w.rawPusher
		w.mu.RUnlock()
		if err = textPusher.PushText(msg); err != nil {
			w.log.Error(err)
		}
		if err = rawPusher.PushRaw(push.NewRawTextMessage(msg)); err != nil {
			w.log.Error(err)
		}
		time.Sleep(time.Second)
	}()

	ticker := time.NewTicker(w.config.IntervalParsed)
	defer ticker.Stop()
	for {
		w.prs = nil
		w.mu.RLock()
		interval := w.config.IntervalParsed
		noSleep := boolVal(w.config.NoSleep, false)
		w.mu.RUnlock()
		w.markNextCheck(time.Now().Add(interval))
		w.log.Infof("next checking at %s\n", w.NextCheckAt().Format("2006-01-02 15:04:05"))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case next := <-w.intervalUpdate:
			ticker.Reset(next)
			w.markNextCheck(time.Now().Add(next))
			w.log.Infof("检查周期已更新为 %s，下次检查 %s", next, w.NextCheckAt().Format("2006-01-02 15:04:05"))
		case <-ticker.C:
			hour := time.Now().Hour()
			if hour >= 0 && hour < 7 && !noSleep {
				w.log.Infof("夜间休眠中（00:00-07:00），跳过本轮检查")
				continue
			}
			if !w.checking.CompareAndSwap(false, true) {
				w.log.Infof("上一次检查尚未结束，跳过本轮")
				continue
			}
			w.collectAndPush(ctx)
			w.markLastCheck()
			w.checking.Store(false)
		}
	}
}

func (w *App) collectAndPush(ctx context.Context) {
	vulns, err := w.collectUpdate(ctx)
	if err != nil {
		w.log.Errorf("failed to get updates, %s", err)
	}
	w.log.Infof("found %d new vulns in this ticking", len(vulns))
	for _, v := range vulns {
		if w.config.NoFilter || v.Creator.IsValuable(v) {
			dbVuln, err := w.db.VulnInformation.Query().Where(vulninformation.Key(v.UniqueKey)).First(ctx)
			if err != nil {
				w.log.Errorf("failed to query %s from db %s", v.UniqueKey, err)
				continue
			}
			if dbVuln.Pushed {
				w.log.Infof("%s has been pushed, skipped", v)
				continue
			}
			if v.CVE != "" && *w.config.EnableCVEFilter {
				// 同一个 cve 已经有其它源推送过了
				others, err := w.db.VulnInformation.Query().
					Where(vulninformation.And(vulninformation.Cve(v.CVE), vulninformation.Pushed(true))).All(ctx)
				if err != nil {
					w.log.Errorf("failed to query %s from db %s", v.UniqueKey, err)
					continue
				}
				if len(others) != 0 {
					ids := make([]string, 0, len(others))
					for _, o := range others {
						ids = append(ids, o.Key)
					}
					w.log.Infof("found new cve but other source has already pushed, others: %v", ids)
					continue
				}
			}

			// 如果配置了软件列表黑名单，不推送在黑名单内的漏洞
			if len(w.config.BlackKeywords) != 0 {
				shouldContinue := false
				for _, p := range w.config.BlackKeywords {
					if strings.Contains(strings.ToLower(v.Title), strings.ToLower(p)) {
						w.log.Infof("skipped %s as in product filter list", v)
						shouldContinue = true
					}
					// 黑名单就不检查 description 里的内容了, 容易漏推送
					// if strings.Contains(strings.ToLower(v.Description), strings.ToLower(p)) {
					// 	w.log.Infof("skipped %s as in product filter list", v)
					// 	continue
					// }
				}
				if shouldContinue {
					continue
				}
			}

			// 如果配置了软件列表白名单，仅推送在白名单内的漏洞
			if len(w.config.WhiteKeywords) != 0 {
				found := false
				for _, p := range w.config.WhiteKeywords {
					if strings.Contains(strings.ToLower(v.Title), strings.ToLower(p)) {
						found = true
						break
					}
					if strings.Contains(strings.ToLower(v.Description), strings.ToLower(p)) {
						found = true
						break
					}
				}
				if !found {
					w.log.Infof("skipped %s as not in product filter list", v)
					continue
				}
			}

			// 仅推送披露日期在1年以内的
			if v.Disclosure != "" {
				disclosureTime, err := time.Parse("2006-01-02", v.Disclosure)
				if err == nil {
					if time.Since(disclosureTime) > time.Hour*24*365 {
						w.log.Infof("skipped %s as disclosure date %s older than 1 year", v, v.Disclosure)
						continue
					}
				}
			}

			// find cve pr in nuclei repo
			if v.CVE != "" && !*w.config.NoGithubSearch {
				links, err := w.FindGithubPoc(ctx, v.CVE)
				if err != nil {
					w.log.Warn(err)
				}
				w.log.Infof("%s found %d links from github, %v", v.CVE, len(links), links)
				if len(links) != 0 {
					v.GithubSearch = grab.MergeUniqueString(v.GithubSearch, links)
					_, err = dbVuln.Update().SetGithubSearch(v.GithubSearch).Save(ctx)
					if err != nil {
						w.log.Warnf("failed to save %s references,  %s", v.UniqueKey, err)
					}
				}
			}
			w.log.Infof("Pushing %s", v)

			// 默认重试3次，如果有多种推送方式，禁用重置机制，避免出现一个成功一个失败，重试的话，成功那个会被重复推送
			i := 0
			for {
				if err := w.pushVuln(v); err == nil {
					// 如果两种推送都成功，才标记为已推送
					_, err = dbVuln.Update().SetPushed(true).Save(ctx)
					if err != nil {
						w.log.Errorf("failed to save pushed %s status, %s", v.UniqueKey, err)
					}
					w.log.Infof("pushed %s successfully", v)
					break
				} else {
					w.log.Errorf("failed to push %s, %s", v.UniqueKey, err)
				}
				i++
				if i > w.config.PushRetryCount {
					break
				}
				w.log.Infof("retry to push %s after 30s", v.UniqueKey)
				time.Sleep(time.Second * 30)
			}
		} else {
			w.log.Infof("skipped %s as not valuable", v)
		}
	}
}

func (w *App) pushVuln(vul *grab.VulnInfo) error {
	var pushErr *multierror.Error

	if err := w.textPusher.PushMarkdown(vul.Title, push.RenderVulnInfo(vul)); err != nil {
		pushErr = multierror.Append(pushErr, err)
	}

	if err := w.rawPusher.PushRaw(push.NewRawVulnInfoMessage(vul)); err != nil {
		pushErr = multierror.Append(pushErr, err)
	}

	return pushErr.ErrorOrNil()
}

func (w *App) Close() {
	_ = w.db.Close()
}

func (w *App) MetaStore() *store.Store {
	return w.meta
}

func (w *App) markLastCheck() {
	w.mu.Lock()
	w.lastCheckAt = time.Now()
	w.mu.Unlock()
}

func (w *App) markNextCheck(at time.Time) {
	w.mu.Lock()
	w.nextCheckAt = at
	w.mu.Unlock()
}

func (w *App) NextCheckAt() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.nextCheckAt
}

func (w *App) IsChecking() bool {
	return w.checking.Load()
}

func (w *App) initData(ctx context.Context) ([]grab.Grabber, []grab.Grabber) {
	var eg errgroup.Group
	eg.SetLimit(len(w.grabbers))
	var success []grab.Grabber
	var fail []grab.Grabber
	for _, grabber := range w.grabbers {
		gb := grabber
		eg.Go(func() error {
			source := gb.ProviderInfo()
			w.log.Infof("start to init data from %s", source.Name)
			initVulns, err := gb.GetUpdate(ctx, InitPageLimit)
			if err != nil {
				fail = append(fail, gb)
				return errors.Wrap(err, source.Name)
			}

			for _, data := range initVulns {
				if _, err = w.createOrUpdate(ctx, source, data); err != nil {
					fail = append(fail, gb)
					return errors.Wrap(errors.Wrap(err, data.String()), source.Name)
				}
			}
			success = append(success, gb)
			return nil
		})
	}
	err := eg.Wait()
	if err != nil {
		w.log.Error(errors.Wrap(err, "init data"))
	}
	return success, fail
}

func (w *App) collectUpdate(ctx context.Context) ([]*grab.VulnInfo, error) {
	w.mu.RLock()
	grabbers := append([]grab.Grabber{}, w.grabbers...)
	w.mu.RUnlock()

	var eg errgroup.Group
	eg.SetLimit(len(grabbers))

	var mu sync.Mutex
	var newVulns []*grab.VulnInfo

	for _, grabber := range grabbers {
		gb := grabber
		eg.Go(func() error {
			source := gb.ProviderInfo()
			dataChan, err := gb.GetUpdate(ctx, UpdatePageLimit)
			if err != nil {
				return errors.Wrap(err, gb.ProviderInfo().Name)
			}
			hasNewVuln := false
			w.log.Infof("collected %d vulns from %s", len(dataChan), source.Name)

			for _, data := range dataChan {
				isNewVuln, err := w.createOrUpdate(ctx, source, data)
				if err != nil {
					return errors.Wrap(err, gb.ProviderInfo().Name)
				}
				if isNewVuln {
					w.log.Infof("found new vuln: %s", data)
					mu.Lock()
					newVulns = append(newVulns, data)
					mu.Unlock()
					hasNewVuln = true
				}
			}

			// 如果一整页漏洞都是旧的，说明没有更新，不必再继续下一页了
			if !hasNewVuln {
				return nil
			}
			return nil
		})
	}
	err := eg.Wait()
	return newVulns, err
}

func (w *App) createOrUpdate(ctx context.Context, source *grab.Provider, data *grab.VulnInfo) (bool, error) {
	vuln, err := w.db.VulnInformation.Query().
		Where(vulninformation.Key(data.UniqueKey)).
		First(ctx)
	// not exist
	if err != nil {
		data.Reason = append(data.Reason, grab.ReasonNewCreated)
		newVuln, err := w.db.VulnInformation.
			Create().
			SetKey(data.UniqueKey).
			SetTitle(data.Title).
			SetDescription(data.Description).
			SetSeverity(string(data.Severity)).
			SetCve(data.CVE).
			SetDisclosure(data.Disclosure).
			SetSolutions(data.Solutions).
			SetReferences(data.References).
			SetPushed(false).
			SetTags(data.Tags).
			SetFrom(data.From).
			Save(ctx)
		if err != nil {
			return false, err
		}
		w.log.Infof("vuln %s(%s) created from %s", newVuln.Title, newVuln.Key, source.Name)
		return true, nil
	}

	// 如果一个漏洞之前是低危，后来改成了严重，这种可能也需要推送, 走一下高价值的判断逻辑
	asNewVuln := false
	if string(data.Severity) != vuln.Severity {
		w.log.Infof("%s from %s change severity from %s to %s", data.Title, data.From, vuln.Severity, data.Severity)
		data.Reason = append(data.Reason, fmt.Sprintf("%s: %s => %s", grab.ReasonSeverityUpdated, vuln.Severity, data.Severity))
		asNewVuln = true
	}
	for _, newTag := range data.Tags {
		found := false
		for _, dbTag := range vuln.Tags {
			if newTag == dbTag {
				found = true
				break
			}
		}
		// tag 有更新
		if !found {
			w.log.Infof("%s from %s add new tag %s", data.Title, data.From, newTag)
			data.Reason = append(data.Reason, fmt.Sprintf("%s: %v => %v", grab.ReasonTagUpdated, vuln.Tags, data.Tags))
			asNewVuln = true
			break
		}
	}

	if !vulnContentChanged(vuln, data) {
		w.log.Debugf("vuln %d unchanged, skip update_time %s", vuln.ID, vuln.Key)
		return asNewVuln, nil
	}

	// 只有源站内容真有变化时才写库，避免每次巡检把 update_time 刷成同一时刻
	newVuln, err := vuln.Update().SetKey(data.UniqueKey).
		SetTitle(data.Title).
		SetDescription(data.Description).
		SetSeverity(string(data.Severity)).
		SetCve(data.CVE).
		SetDisclosure(data.Disclosure).
		SetSolutions(data.Solutions).
		SetReferences(data.References).
		SetTags(data.Tags).
		SetFrom(data.From).
		Save(ctx)
	if err != nil {
		return false, err
	}
	w.log.Debugf("vuln %d updated from %s %s", newVuln.ID, newVuln.Key, source.Name)
	return asNewVuln, nil
}

func vulnContentChanged(vuln *ent.VulnInformation, data *grab.VulnInfo) bool {
	if vuln.Title != data.Title ||
		vuln.Description != data.Description ||
		vuln.Severity != string(data.Severity) ||
		vuln.Cve != data.CVE ||
		vuln.Disclosure != data.Disclosure ||
		vuln.Solutions != data.Solutions ||
		vuln.From != data.From {
		return true
	}
	return !sameStringSet(vuln.References, data.References) || !sameStringSet(vuln.Tags, data.Tags)
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, item := range a {
		seen[item]++
	}
	for _, item := range b {
		if seen[item] == 0 {
			return false
		}
		seen[item]--
	}
	return true
}

func (w *App) FindGithubPoc(ctx context.Context, cveId string) ([]string, error) {
	var eg errgroup.Group
	var results []string
	var mu sync.Mutex

	eg.Go(func() error {
		links, err := w.findGithubRepo(ctx, cveId)
		if err != nil {
			return errors.Wrap(err, "find github repo")
		}
		mu.Lock()
		defer mu.Unlock()
		results = append(results, links...)
		return nil
	})
	eg.Go(func() error {
		links, err := w.findNucleiPR(ctx, cveId)
		if err != nil {
			return errors.Wrap(err, "find nuclei PR")
		}
		mu.Lock()
		defer mu.Unlock()
		results = append(results, links...)
		return nil
	})
	err := eg.Wait()
	return results, err
}

func (w *App) findGithubRepo(ctx context.Context, cveId string) ([]string, error) {
	w.log.Infof("finding github repo of %s", cveId)
	re, err := regexp.Compile(fmt.Sprintf("(?i)[\b/_]%s[\b/_]", cveId))
	if err != nil {
		return nil, err
	}
	lastYear := time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
	query := fmt.Sprintf(`language:Python language:JavaScript language:C language:C++ language:Java language:PHP language:Ruby language:Rust language:C# created:>%s %s`, lastYear, cveId)
	result, _, err := w.githubClient.Search.Repositories(ctx, query, &github.SearchOptions{
		ListOptions: github.ListOptions{Page: 1, PerPage: 100},
	})
	if err != nil {
		return nil, err
	}
	var links []string
	for _, repo := range result.Repositories {
		if re.MatchString(repo.GetHTMLURL()) {
			links = append(links, repo.GetHTMLURL())
		}
	}
	return links, nil
}

func (w *App) findNucleiPR(ctx context.Context, cveId string) ([]string, error) {
	w.log.Infof("finding nuclei PR of %s", cveId)
	if w.prs == nil {
		// 检查200个pr
		for page := 1; page < 2; page++ {
			prs, _, err := w.githubClient.PullRequests.List(ctx, "projectdiscovery", "nuclei-templates", &github.PullRequestListOptions{
				State:       "all",
				ListOptions: github.ListOptions{Page: page, PerPage: 100},
			})
			if err != nil {
				if len(w.prs) == 0 {
					return nil, err
				} else {
					w.log.Warnf("list nuclei pr failed: %v", err)
					continue
				}
			}
			w.prs = append(w.prs, prs...)
		}
	}

	var links []string
	re, err := regexp.Compile(fmt.Sprintf("(?i)[\b/_]%s[\b/_]", cveId))
	if err != nil {
		return nil, err
	}
	for _, pr := range w.prs {
		if re.MatchString(pr.GetTitle()) || re.MatchString(pr.GetBody()) {
			links = append(links, pr.GetHTMLURL())
		}
	}
	return links, nil
}

func (w *App) testPushMessage() error {
	// push start message
	providers := make([]*grab.Provider, 0, 10)
	failed := make([]*grab.Provider, 0, 10)
	for _, p := range w.grabbers {
		providers = append(providers, p.ProviderInfo())
	}
	msg := &push.InitialMessage{
		Version:        w.config.Version,
		VulnCount:      1024,
		Interval:       w.config.IntervalParsed.String(),
		Provider:       providers,
		FailedProvider: failed,
	}
	if err := w.textPusher.PushMarkdown("漏哨 初始化完成", push.RenderInitialMsg(msg)); err != nil {
		return err
	}
	if err := w.rawPusher.PushRaw(push.NewRawInitialMessage(msg)); err != nil {
		return err
	}
	w.log.Infof("start message pushed")

	// push a mocked vuln
	v := &grab.VulnInfo{
		Title:        "漏哨 代码执行漏洞",
		CVE:          "CVE-2033-9096",
		Severity:     "严重",
		Tags:         []string{"POC公开", "源码公开", "技术细节公开"},
		Disclosure:   "2033-06-30",
		From:         "https://example.com/lousentry",
		Reason:       []string{"created"},
		Description:  "漏哨 存在代码执行漏洞，只要你想二开，那么就一定需要执行它原本的代码",
		GithubSearch: []string{"https://github.com/search?q=lousentry&ref=opensearch&type=repositories"},
		References:   []string{"https://example.com/lousentry"},
		Solutions:    "1. 升级到最新版本\n2. 赞助作者",
	}
	if err := w.pushVuln(v); err != nil {
		return err
	}
	w.log.Infof("mocked vuln message pushed")

	// push stop message
	info := "漏哨 测试结束，进程即将退出"
	if err := w.textPusher.PushText(info); err != nil {
		return err
	}
	if err := w.rawPusher.PushRaw(push.NewRawTextMessage(info)); err != nil {
		return err
	}
	w.log.Infof("stop message pushed")
	return nil
}
