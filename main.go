package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"strings"
	"time"

	"lousentry/push"
	"lousentry/web"
	"gopkg.in/yaml.v3"

	"lousentry/ctrl"

	"github.com/kataras/golog"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

//go:embed all:webui/dist
var webDist embed.FS

var log = golog.Child("[main]")
var Version = "v2.8.0"

func main() {
	golog.Default.SetLevel("info")
	cli.VersionFlag = &cli.BoolFlag{
		Name:     "version",
		Aliases:  []string{"v"},
		Usage:    "print the version",
		Category: "[Other Options]",
	}
	cli.HelpFlag = &cli.BoolFlag{
		Name:     "help",
		Aliases:  []string{"h"},
		Usage:    "show help",
		Category: "[Other Options]",
	}

	app := cli.NewApp()
	app.Name = "lousentry"
	app.Usage = "漏哨：采集高价值漏洞并推送到钉钉 / 飞书 / 企业微信"
	app.Version = Version

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "config file path, support json or yaml",
		},
		&cli.StringFlag{
			Name:     "dingding-access-token",
			Aliases:  []string{"dt"},
			Usage:    "webhook access token of dingding bot",
			Category: "[\x00Push Options]",
		},
		&cli.StringFlag{
			Name:     "dingding-sign-secret",
			Aliases:  []string{"ds"},
			Usage:    "sign secret of dingding bot",
			Category: "[\x00Push Options]",
		},
		&cli.StringFlag{
			Name:     "wechatwork-key",
			Aliases:  []string{"wk"},
			Usage:    "webhook key of wechat work",
			Category: "[\x00Push Options]",
		},
		&cli.StringFlag{
			Name:     "lark-access-token",
			Aliases:  []string{"lt"},
			Usage:    "webhook access token/url of lark",
			Category: "[\x00Push Options]",
		},
		&cli.StringFlag{
			Name:     "lark-sign-secret",
			Aliases:  []string{"ls"},
			Usage:    "sign secret of lark",
			Category: "[\x00Push Options]",
		},
		&cli.StringFlag{
			Name:     "whitelist-file",
			Aliases:  []string{"wf"},
			Usage:    "specify a file that contains some keywords, vulns with these keywords will be pushed",
			Category: "[\x00Push Options]",
		},
		&cli.StringFlag{
			Name:     "blacklist-file",
			Aliases:  []string{"bf"},
			Usage:    "specify a file that contains some keywords, vulns with these products will NOT be pushed",
			Category: "[\x00Push Options]",
		},
		&cli.StringFlag{
			Name:     "db-conn",
			Aliases:  []string{"db"},
			Usage:    "database connection string",
			Value:    "sqlite3://vuln_v3.sqlite3",
			Category: "[Launch Options]",
		},
		&cli.StringFlag{
			Name:     "sources",
			Aliases:  []string{"s"},
			Usage:    "set vuln sources",
			Value:    "avd,chaitin,ti,oscs,threatbook,seebug,struts2,kev,venustech",
			Category: "[Launch Options]",
		},
		&cli.StringFlag{
			Name:     "interval",
			Aliases:  []string{"i"},
			Usage:    "checking every [interval], supported format like 30s, 30m, 1h",
			Value:    "30m",
			Category: "[Launch Options]",
		},
		&cli.StringFlag{
			Name:     "proxy",
			Aliases:  []string{"x"},
			Usage:    "set request proxy, support socks5://xxx or http(s)://",
			Category: "[Launch Options]",
		},
		&cli.StringFlag{
			Name:     "listen",
			Aliases:  []string{"l"},
			Usage:    "web console listen address, set off to disable",
			Value:    ":8080",
			Category: "[Launch Options]",
		},
		&cli.StringFlag{
			Name:     "admin-user",
			Usage:    "web console username",
			Value:    "admin",
			Category: "[Launch Options]",
		},
		&cli.StringFlag{
			Name:     "admin-pass",
			Usage:    "web console password, only used when initializing or resetting",
			Category: "[Launch Options]",
		},
		&cli.BoolFlag{
			Name:     "enable-cve-filter",
			Usage:    "enable a filter that vulns from multiple sources with same cve id will be sent only once",
			Value:    true,
			Category: "[Launch Options]",
		},
		&cli.BoolFlag{
			Name:     "no-start-message",
			Aliases:  []string{"nm"},
			Usage:    "disable the hello message when server starts",
			Category: "[Launch Options]",
		},
		&cli.BoolFlag{
			Name:     "no-filter",
			Aliases:  []string{"nf"},
			Usage:    "ignore the valuable filter and push all discovered vulns",
			Category: "[Launch Options]",
		},
		&cli.BoolFlag{
			Name:     "no-github-search",
			Aliases:  []string{"ng"},
			Usage:    "don't search github repos and pull requests for every cve vuln",
			Category: "[Launch Options]",
		},
		&cli.BoolFlag{
			Name:     "no-sleep",
			Aliases:  []string{"ns"},
			Usage:    "don't sleep in night, run every interval",
			Category: "[Launch Options]",
		},
		&cli.BoolFlag{
			Name:     "diff",
			Usage:    "skip init vuln db, push new vulns then exit",
			Category: "[Launch Options]",
		},
		&cli.BoolFlag{
			Name:     "insecure",
			Aliases:  []string{"k"},
			Usage:    "allow insecure server connections when using SSL/TLS",
			Category: "[Other Options]",
		},
		&cli.BoolFlag{
			Name:     "test",
			Aliases:  []string{"T"},
			Usage:    "use to test message pusher, three mocked messages will be pushed",
			Category: "[Other Options]",
		},
		&cli.BoolFlag{
			Name:     "debug",
			Aliases:  []string{"d"},
			Usage:    "set log level to debug, print more details",
			Value:    false,
			Category: "[Other Options]",
		},
	}
	app.Before = func(c *cli.Context) error {
		if c.Bool("debug") {
			golog.Default.SetLevel("debug")
		}
		return nil
	}
	app.Action = Action

	err := app.Run(os.Args)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Fatal("user canceled")
		} else {
			log.Fatal(err)
		}
	}
}

func Action(c *cli.Context) error {
	ctx, cancel := signalCtx()
	defer cancel()

	var config *ctrl.AppConfig
	var err error
	configFile := c.String("config")
	if configFile != "" {
		config, err = initConfigFromFile(c, configFile)
	} else {
		config, err = initConfigFromCli(c)
	}
	if err != nil {
		return errors.Wrap(err, "failed to init config")
	}

	listen := firstNonEmpty(os.Getenv("LISTEN"), os.Getenv("WEB_LISTEN"))
	if listen == "" && c.IsSet("listen") {
		listen = c.String("listen")
	}
	if listen == "" {
		listen = firstNonEmpty(config.Listen, ":8080")
	}
	config.Listen = listen
	if !isListenDisabled(listen) {
		config.AllowEmptyPusher = true
	}

	logBuf := web.NewLogBuffer(2000)
	logBuf.Attach()

	app, err := ctrl.NewApp(config)
	if err != nil {
		return errors.Wrap(err, "failed to create app")
	}
	defer app.Close()

	if !isListenDisabled(listen) {
		adminUser := firstNonEmpty(os.Getenv("ADMIN_USER"), c.String("admin-user"), "admin")
		adminPass := firstNonEmpty(os.Getenv("ADMIN_PASSWORD"), os.Getenv("ADMIN_PASS"), c.String("admin-pass"))
		auth, err := web.NewAuthStore(app.MetaStore(), adminUser, adminPass)
		if err != nil {
			return errors.Wrap(err, "failed to init web auth")
		}
		static, err := fs.Sub(webDist, "webui/dist")
		if err != nil {
			return errors.Wrap(err, "failed to load web ui")
		}
		srv := web.New(app, auth, logBuf, static)
		go func() {
			if err := web.ListenAndServe(ctx, listen, srv.Handler()); err != nil {
				log.Errorf("web server stopped: %v", err)
			}
		}()
	} else {
		log.Infof("web console disabled")
	}

	if err = app.Run(ctx); err != nil {
		return errors.Wrap(err, "failed to run app")
	}
	return nil
}

func initConfigFromFile(c *cli.Context, configFile string) (*ctrl.AppConfig, error) {
	if configFile == "" {
		return nil, fmt.Errorf("config file is required")
	}
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	var config ctrl.AppConfig
	if strings.HasSuffix(configFile, ".json") {
		err = json.Unmarshal(data, &config)
	}
	if strings.HasSuffix(configFile, ".yaml") || strings.HasSuffix(configFile, ".yml") {
		err = yaml.Unmarshal(data, &config)
	}
	if err != nil {
		return nil, err
	}
	config.IntervalParsed, err = time.ParseDuration(config.Interval)
	if err != nil {
		return nil, err
	}
	if config.IntervalParsed.Minutes() < 1 && !c.Bool("debug") {
		return nil, fmt.Errorf("interval is too small, at least 1m")
	}
	return &config, nil
}

func initConfigFromCli(c *cli.Context) (*ctrl.AppConfig, error) {
	pusher, err := initPusher(c)
	if err != nil {
		return nil, err
	}

	sources := c.String("sources")
	if os.Getenv("SOURCES") != "" {
		sources = os.Getenv("SOURCES")
	}
	sourcesParts := strings.Split(sources, ",")

	noStartMessage := c.Bool("no-start-message")
	noFilter := c.Bool("no-filter")
	noGithubSearch := c.Bool("no-github-search")
	noSleep := c.Bool("no-sleep")
	cveFilter := c.Bool("enable-cve-filter")
	debug := c.Bool("debug")
	iv := c.String("interval")
	db := c.String("db")
	diff := c.Bool("diff")
	whitelistFile := c.String("whitelist-file")
	blacklistFile := c.String("blacklist-file")
	proxy := c.String("proxy")
	insecure := c.Bool("insecure")
	test := c.Bool("test")

	if os.Getenv("INTERVAL") != "" {
		iv = os.Getenv("INTERVAL")
	}
	if os.Getenv("NO_FILTER") != "" {
		noFilter = true
	}
	if os.Getenv("NO_START_MESSAGE") != "" {
		noStartMessage = true
	}
	if os.Getenv("NO_GITHUB_SEARCH") != "" {
		noGithubSearch = true
	}
	if os.Getenv("NO_SLEEP") != "" {
		noSleep = true
	}

	if os.Getenv("ENABLE_CVE_FILTER") == "false" {
		cveFilter = false
	}
	if os.Getenv("DIFF") != "" {
		diff = true
	}
	if os.Getenv("DB_CONN") != "" {
		db = os.Getenv("DB_CONN")
	}

	log.Infof("config: INTERVAL=%s, NO_FILTER=%v, NO_START_MESSAGE=%v, NO_GITHUB_SEARCH=%v, ENABLE_CVE_FILTER=%v",
		iv, noFilter, noStartMessage, noGithubSearch, cveFilter)

	interval, err := time.ParseDuration(iv)
	if err != nil {
		return nil, err
	}
	if interval.Minutes() < 1 && !debug {
		return nil, fmt.Errorf("interval is too small, at least 1m")
	}

	// 白名单关键字
	if os.Getenv("WHITELIST_FILE") != "" {
		whitelistFile = os.Getenv("WHITELIST_FILE")
	}
	whiteKeywords, err := splitLines(whitelistFile)
	if err != nil {
		return nil, err
	}
	if len(whiteKeywords) != 0 {
		log.Infof("using whitelist keywords: %v", whiteKeywords)
	}

	// 黑名单关键字
	if os.Getenv("BLACKLIST_FILE") != "" {
		blacklistFile = os.Getenv("BLACKLIST_FILE")
	}
	blackKeywords, err := splitLines(blacklistFile)
	if err != nil {
		return nil, err
	}
	if len(blackKeywords) != 0 {
		log.Infof("using blacklist keywords: %v", blackKeywords)
	}

	config := &ctrl.AppConfig{
		DBConn:          db,
		Sources:         sourcesParts,
		Interval:        iv,
		IntervalParsed:  interval,
		EnableCVEFilter: &cveFilter,
		NoStartMessage:  &noStartMessage,
		NoGithubSearch:  &noGithubSearch,
		NoSleep:         &noSleep,
		NoFilter:        noFilter,
		DiffMode:        &diff,
		Version:         Version,
		WhiteKeywords:   whiteKeywords,
		BlackKeywords:   blackKeywords,
		Pusher:          pusher,
		Proxy:           proxy,
		SkipTLSVerify:   insecure,
		Test:            test,
	}
	return config, nil
}

func initPusher(c *cli.Context) ([]map[string]string, error) {
	dingToken := firstNonEmpty(os.Getenv("DINGDING_ACCESS_TOKEN"), c.String("dingding-access-token"))
	dingSecret := firstNonEmpty(os.Getenv("DINGDING_SECRET"), c.String("dingding-sign-secret"))
	wxWorkKey := firstNonEmpty(os.Getenv("WECHATWORK_KEY"), c.String("wechatwork-key"))
	larkToken := firstNonEmpty(os.Getenv("LARK_ACCESS_TOKEN"), c.String("lark-access-token"))
	larkSecret := firstNonEmpty(os.Getenv("LARK_SECRET"), c.String("lark-sign-secret"))

	var pusherConfig []any
	if dingToken != "" {
		pusherConfig = append(pusherConfig, &push.DingDingConfig{
			Type:        push.TypeDingDing,
			AccessToken: dingToken,
			SignSecret:  dingSecret,
		})
	}
	if wxWorkKey != "" {
		pusherConfig = append(pusherConfig, &push.WechatWorkConfig{
			Type: push.TypeWechatWork,
			Key:  wxWorkKey,
		})
	}
	if larkToken != "" {
		pusherConfig = append(pusherConfig, &push.LarkConfig{
			Type:        push.TypeLark,
			AccessToken: larkToken,
			SignSecret:  larkSecret,
		})
	}
	data, err := json.Marshal(pusherConfig)
	if err != nil {
		return nil, err
	}
	var pusher []map[string]string
	err = json.Unmarshal(data, &pusher)
	if err != nil {
		return nil, err
	}
	return pusher, nil
}

func signalCtx() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		<-ch
		cancel()
	}()
	return ctx, cancel
}

func splitLines(path string) ([]string, error) {
	var products []string
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		for _, p := range strings.Split(string(data), "\n") {
			p = strings.TrimSpace(p)
			if p != "" {
				products = append(products, p)
			}
		}
	}
	return products, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func isListenDisabled(listen string) bool {
	switch strings.ToLower(strings.TrimSpace(listen)) {
	case "", "off", "none", "false", "disable", "disabled":
		return true
	default:
		return false
	}
}
