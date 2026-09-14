package push

import "github.com/kataras/golog"

// NoopTextPusher 在尚未配置推送渠道时占位，避免进程无法启动。
type NoopTextPusher struct {
	log *golog.Logger
}

func NewNoopTextPusher() TextPusher {
	return &NoopTextPusher{log: golog.Child("[pusher-noop]")}
}

func (n *NoopTextPusher) PushText(s string) error {
	n.log.Warnf("未配置推送渠道，已跳过文本推送: %s", truncateForLog(s, 80))
	return nil
}

func (n *NoopTextPusher) PushMarkdown(title, content string) error {
	n.log.Warnf("未配置推送渠道，已跳过 Markdown 推送: %s", title)
	return nil
}

// NoopRawPusher 在尚未配置推送渠道时占位。
type NoopRawPusher struct {
	log *golog.Logger
}

func NewNoopRawPusher() RawPusher {
	return &NoopRawPusher{log: golog.Child("[pusher-noop]")}
}

func (n *NoopRawPusher) PushRaw(r *RawMessage) error {
	n.log.Warnf("未配置推送渠道，已跳过 Raw 推送: %s", r.Type)
	return nil
}

func truncateForLog(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n]) + "..."
}
