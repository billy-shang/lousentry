package ctrl

import (
	"context"
	"strings"
	"time"

	"lousentry/ent"
	"lousentry/ent/vulninformation"
	"lousentry/grab"
)

// VulnDTO 控制台漏洞列表/详情返回结构。
type VulnDTO struct {
	ID           int      `json:"id"`
	Key          string   `json:"key"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Severity     string   `json:"severity"`
	CVE          string   `json:"cve"`
	Disclosure   string   `json:"disclosure"`
	Solutions    string   `json:"solutions"`
	References   []string `json:"references"`
	Tags         []string `json:"tags"`
	GithubSearch []string `json:"github_search"`
	From         string   `json:"from"`
	Source       string   `json:"source"`
	SourceName   string   `json:"source_name"`
	Pushed       bool     `json:"pushed"`
	CreateTime   string   `json:"create_time"`
	UpdateTime   string   `json:"update_time"`
}

// VulnQuery 漏洞列表筛选条件。
type VulnQuery struct {
	Keyword  string
	Source   string
	Severity string
	Pushed   *bool
	Page     int
	Size     int
}

// StatsDTO 控制台顶部统计。
type StatsDTO struct {
	TodayNew    int      `json:"today_new"`
	SourceCount int      `json:"source_count"`
	Pushed      int      `json:"pushed"`
	Unpushed    int      `json:"unpushed"`
	Interval    string   `json:"interval"`
	Total       int      `json:"total"`
	LastCheckAt string   `json:"last_check_at"`
	NextCheckAt string   `json:"next_check_at"`
	Checking    bool     `json:"checking"`
	AutoPush    bool     `json:"auto_push"`
	Pushers     []string `json:"pushers"`
}

func toVulnDTO(v *ent.VulnInformation) *VulnDTO {
	src := SourceFromURL(v.From)
	return &VulnDTO{
		ID:           v.ID,
		Key:          v.Key,
		Title:        v.Title,
		Description:  v.Description,
		Severity:     v.Severity,
		CVE:          v.Cve,
		Disclosure:   v.Disclosure,
		Solutions:    v.Solutions,
		References:   v.References,
		Tags:         v.Tags,
		GithubSearch: v.GithubSearch,
		From:         v.From,
		Source:       src.ID,
		SourceName:   src.DisplayName,
		Pushed:       v.Pushed,
		CreateTime:   v.CreateTime.Local().Format("2006-01-02 15:04:05"),
		UpdateTime:   v.UpdateTime.Local().Format("2006-01-02 15:04:05"),
	}
}

func (w *App) ListVulns(ctx context.Context, q VulnQuery) ([]*VulnDTO, int, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Size < 1 || q.Size > 100 {
		q.Size = 20
	}

	query := w.db.VulnInformation.Query()
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		query = query.Where(vulninformation.Or(
			vulninformation.TitleContainsFold(kw),
			vulninformation.CveContainsFold(kw),
			vulninformation.KeyContainsFold(kw),
			vulninformation.DescriptionContainsFold(kw),
		))
	}
	if q.Severity != "" {
		query = query.Where(vulninformation.SeverityEQ(q.Severity))
	}
	if q.Pushed != nil {
		query = query.Where(vulninformation.PushedEQ(*q.Pushed))
	}

	items, err := query.Order(
		ent.Desc(vulninformation.FieldCreateTime),
		ent.Desc(vulninformation.FieldDisclosure),
		ent.Desc(vulninformation.FieldUpdateTime),
	).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	var filtered []*ent.VulnInformation
	for _, item := range items {
		if !sourceMatchKeyword(item.From, q.Source) {
			continue
		}
		filtered = append(filtered, item)
	}
	total := len(filtered)
	start := (q.Page - 1) * q.Size
	if start > total {
		start = total
	}
	end := start + q.Size
	if end > total {
		end = total
	}

	out := make([]*VulnDTO, 0, end-start)
	for _, item := range filtered[start:end] {
		out = append(out, toVulnDTO(item))
	}
	return out, total, nil
}

func (w *App) GetVuln(ctx context.Context, id int) (*VulnDTO, error) {
	item, err := w.db.VulnInformation.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return toVulnDTO(item), nil
}

func (w *App) GetStats(ctx context.Context) (*StatsDTO, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	total, err := w.db.VulnInformation.Query().Count(ctx)
	if err != nil {
		return nil, err
	}
	pushed, err := w.db.VulnInformation.Query().Where(vulninformation.PushedEQ(true)).Count(ctx)
	if err != nil {
		return nil, err
	}
	todayNew, err := w.db.VulnInformation.Query().Where(vulninformation.CreateTimeGTE(today)).Count(ctx)
	if err != nil {
		return nil, err
	}

	w.mu.RLock()
	interval := w.config.Interval
	sourceCount := len(w.config.Sources)
	lastCheck := w.lastCheckAt
	nextCheck := w.nextCheckAt
	pushers := enabledPusherNames(w.pushSettings)
	w.mu.RUnlock()

	return &StatsDTO{
		TodayNew:    todayNew,
		SourceCount: sourceCount,
		Pushed:      pushed,
		Unpushed:    total - pushed,
		Interval:    interval,
		Total:       total,
		LastCheckAt: formatCheckTime(lastCheck),
		NextCheckAt: formatCheckTime(nextCheck),
		Checking:    w.checking.Load(),
		AutoPush:    len(pushers) > 0,
		Pushers:     pushers,
	}, nil
}

func formatCheckTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func enabledPusherNames(s PushSettings) []string {
	var out []string
	if s.DingDing.Enabled && strings.TrimSpace(s.DingDing.AccessToken) != "" {
		out = append(out, "钉钉")
	}
	if s.Lark.Enabled && strings.TrimSpace(s.Lark.AccessToken) != "" {
		out = append(out, "飞书")
	}
	if s.WechatWork.Enabled && strings.TrimSpace(s.WechatWork.Key) != "" {
		out = append(out, "企业微信")
	}
	return out
}

func (w *App) dbToVulnInfo(v *ent.VulnInformation) *grab.VulnInfo {
	return &grab.VulnInfo{
		UniqueKey:    v.Key,
		Title:        v.Title,
		Description:  v.Description,
		Severity:     grab.SeverityLevel(v.Severity),
		CVE:          v.Cve,
		Disclosure:   v.Disclosure,
		Solutions:    v.Solutions,
		References:   v.References,
		Tags:         v.Tags,
		GithubSearch: v.GithubSearch,
		From:         v.From,
		Reason:       []string{"手动推送"},
	}
}
