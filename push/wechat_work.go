package push

import (
	"net/url"
	"strings"
	"time"

	"github.com/kataras/golog"
	"github.com/pkg/errors"
	wxworkbot "github.com/vimsucks/wxwork-bot-go"
)

var _ = TextPusher(&WechatWork{})

const TypeWechatWork = "wechatwork"

type WechatWorkConfig struct {
	Type string `json:"type" yaml:"type"`
	Key  string `yaml:"key" json:"key"`
}

type WechatWork struct {
	client *wxworkbot.WxWorkBot
	log    *golog.Logger
}

func NewWechatWork(config *WechatWorkConfig) TextPusher {
	key := normalizeWechatKey(config.Key)
	bot := wxworkbot.New(key)
	bot.Client.Timeout = 15 * time.Second
	return &WechatWork{
		client: bot,
		log:    golog.Child("[pusher-wechat-work]"),
	}
}

func normalizeWechatKey(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "key=") {
		if u, err := url.Parse(raw); err == nil {
			if key := strings.TrimSpace(u.Query().Get("key")); key != "" {
				return key
			}
		}
		if i := strings.LastIndex(raw, "key="); i >= 0 {
			return strings.TrimSpace(raw[i+4:])
		}
	}
	return raw
}

func clipWechatMarkdown(s string) string {
	const maxBytes = 4000
	if len(s) <= maxBytes {
		return s
	}
	rs := []rune(s)
	for len(string(rs)) > maxBytes-3 && len(rs) > 0 {
		rs = rs[:len(rs)-1]
	}
	return string(rs) + "..."
}

func (d *WechatWork) PushText(s string) error {
	// fixme: wxworkbot 不支持 text 类型
	d.log.Infof("sending text %s", s)
	msg := wxworkbot.Markdown{Content: clipWechatMarkdown(s)}
	err := d.client.Send(msg)
	if err != nil {
		return errors.Wrap(err, "wechat-work")
	}
	return nil
}

func (d *WechatWork) PushMarkdown(title, content string) error {
	d.log.Infof("sending markdown %s", title)
	msg := wxworkbot.Markdown{Content: clipWechatMarkdown(content)}
	err := d.client.Send(msg)
	if err != nil {
		return errors.Wrap(err, "wechat-work")
	}
	return nil
}
