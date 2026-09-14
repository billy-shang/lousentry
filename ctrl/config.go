package ctrl

import (
	"encoding/json"
	"entgo.io/ent/dialect"
	"fmt"
	"github.com/kataras/golog"
	"lousentry/push"
	"net/url"
	"os"
	"time"
)

type AppConfig struct {
	DBConn          string              `yaml:"db_conn" json:"db_conn"`
	Sources         []string            `yaml:"sources" json:"sources"`
	Interval        string              `yaml:"interval" json:"interval"`
	EnableCVEFilter *bool               `yaml:"enable_cve_filter" json:"enable_cve_filter"`
	NoGithubSearch  *bool               `yaml:"no_github_search" json:"no_github_search"`
	NoStartMessage  *bool               `yaml:"no_start_message" json:"no_start_message"`
	NoSleep         *bool               `yaml:"no_sleep" json:"no_sleep"`
	DiffMode        *bool               `yaml:"diff_mode" json:"diff_mode"`
	WhiteKeywords   []string            `yaml:"white_keywords" json:"white_keywords"`
	BlackKeywords   []string            `yaml:"black_keywords" json:"black_keywords"`
	Pusher          []map[string]string `yaml:"pusher" json:"pusher"`
	Proxy           string              `yaml:"proxy" json:"proxy"`
	SkipTLSVerify   bool                `yaml:"skip_tls_verify" json:"skip_tls_verify"`
	Test            bool                `yaml:"test" json:"test"`
	NoFilter        bool                `yaml:"no_filter" json:"no_filter"`
	Listen          string              `yaml:"listen" json:"listen"`

	AllowEmptyPusher  bool          `yaml:"-" json:"-"`
	Version           string        `yaml:"-" json:"-"`
	IntervalParsed    time.Duration `json:"-" yaml:"-"`
	PushRetryCount    int           `yaml:"-" json:"-"`
}

const dbExample = `
sqlite3://vuln_v3.sqlite3
mysql://user:pass@host:port/dbname
postgres://user:pass@host:port/dbname
`

func (c *AppConfig) Init() {
	if c.EnableCVEFilter == nil {
		t := true
		c.EnableCVEFilter = &t
	}
	if c.NoGithubSearch == nil {
		c.NoGithubSearch = new(bool)
	}
	if c.NoStartMessage == nil {
		c.NoStartMessage = new(bool)
	}
	if c.NoSleep == nil {
		c.NoSleep = new(bool)
	}
	if c.DiffMode == nil {
		c.DiffMode = new(bool)
	}
	if c.Interval == "" {
		c.Interval = "30m"
	}
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if len(c.Sources) == 0 {
		c.Sources = []string{"avd", "chaitin", "ti", "oscs", "threatbook", "seebug", "struts2", "kev", "venustech"}
	}
	normalized := make([]string, 0, len(c.Sources))
	for _, s := range c.Sources {
		if id := NormalizeSource(s); id != "" {
			normalized = append(normalized, id)
		}
	}
	c.Sources = normalized

	if c.Proxy != "" {
		must(os.Setenv("HTTP_PROXY", c.Proxy))
		must(os.Setenv("HTTPS_PROXY", c.Proxy))
	}
	if os.Getenv("HTTPS_PROXY") != "" {
		must(os.Setenv("HTTP_PROXY", os.Getenv("HTTPS_PROXY")))
	}

	if c.SkipTLSVerify {
		// 这个环境变量仅内部使用，go 本身并不支持
		must(os.Setenv("GO_SKIP_TLS_CHECK", "1"))
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func (c *AppConfig) DBConnForEnt() (string, string, error) {
	u, err := url.Parse(c.DBConn)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse db_conn: %w, expected:%s", err, dbExample)
	}
	switch u.Scheme {
	case dialect.SQLite:
		query := `cache=shared&_pragma=foreign_keys(1)`
		if u.RawQuery != "" {
			query = u.RawQuery
		}
		return dialect.SQLite, fmt.Sprintf("file:%s%s?%s", u.Host, u.Path, query), nil
	case dialect.MySQL:
		path := ""
		if u.Path != "" {
			path = u.Path[1:]
		}
		query := `charset=utf8mb4&parseTime=True&loc=Local`
		if u.RawQuery != "" {
			query = u.RawQuery
		}
		return dialect.MySQL, fmt.Sprintf("%s@tcp(%s)/%s?%s", u.User.String(), u.Host, path, query), nil
	case dialect.Postgres:
		path := ""
		if u.Path != "" {
			path = u.Path[1:]
		}
		query := `sslmode=disable`
		if u.RawQuery != "" {
			query = u.RawQuery
		}
		return dialect.Postgres, fmt.Sprintf("postgresql://%s@%s/%s?%s", u.User.String(), u.Host, path, query), nil
	default:
		return "", "", fmt.Errorf("unsupported db_conn: %s, expected:%s", c.DBConn, dbExample)
	}
}

func (c *AppConfig) GetPusher() (push.TextPusher, push.RawPusher, error) {
	var textPusher []push.TextPusher
	var rawPusher []push.RawPusher

	for _, config := range c.Pusher {
		pushType := config["type"]

		switch pushType {
		case push.TypeDingDing:
			dingConfig := unmarshal[push.DingDingConfig](config)
			if dingConfig.AccessToken == "" {
				continue
			}
			textPusher = append(textPusher, push.NewDingDing(&dingConfig))
		case push.TypeLark:
			larkConfig := unmarshal[push.LarkConfig](config)
			if larkConfig.AccessToken == "" {
				continue
			}
			textPusher = append(textPusher, push.NewLark(&larkConfig))
		case push.TypeWechatWork:
			wechatConfig := unmarshal[push.WechatWorkConfig](config)
			if wechatConfig.Key == "" {
				continue
			}
			textPusher = append(textPusher, push.NewWechatWork(&wechatConfig))
		default:
			golog.Warnf("已忽略不支持的推送类型: %s（仅支持钉钉 / 飞书 / 企业微信）", pushType)
			continue
		}
		golog.Infof("add pusher: %s", pushType)
	}
	if len(textPusher) == 0 && len(rawPusher) == 0 {
		if c.AllowEmptyPusher {
			golog.Warnf("未配置推送渠道，漏洞只会入库，可在控制台补充钉钉/飞书/企业微信")
			return push.NewNoopTextPusher(), push.NewNoopRawPusher(), nil
		}
		msg := `
you must setup at least one pusher, eg: 
use dingding: %s --dt DINGDING_ACCESS_TOKEN --ds DINGDING_SECRET
use wechat:   %s --wk WECHATWORK_KEY
use lark:     %s --lt LARK_ACCESS_TOKEN`
		return nil, nil, fmt.Errorf(msg, os.Args[0], os.Args[0], os.Args[0])
	}
	pusherCount := len(textPusher) + len(rawPusher)
	if pusherCount > 1 {
		golog.Infof("multi pusher detected, push retry will be disabled")
		c.PushRetryCount = 0
	} else {
		c.PushRetryCount = 2
	}
	// 固定一个推送的间隔 1s，避免 dingding 等推送过快的问题
	interval := time.Second
	return push.NewMultiTextPusherWithInterval(interval, textPusher...), push.NewMultiRawPusherWithInterval(interval, rawPusher...), nil
}

func unmarshal[T any](config map[string]string) T {
	data, _ := json.Marshal(config)
	var res T
	_ = json.Unmarshal(data, &res)
	return res
}
