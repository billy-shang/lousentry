package push

import (
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/kataras/golog"
)

// TextPusher is a type that can push text and markdown messages.
type TextPusher interface {
	PushText(s string) error
	PushMarkdown(title, content string) error
}

// RawPusher is a type that can push raw messages.
type RawPusher interface {
	PushRaw(r *RawMessage) error
}

type multiPusher struct {
	textPusher []TextPusher
	rawPusher  []RawPusher

	interval time.Duration
}

// NewMultiTextPusherWithInterval returns a TextPusher that pushes to all the given pushers with interval.
func NewMultiTextPusherWithInterval(interval time.Duration, pushers ...TextPusher) TextPusher {
	return &multiPusher{textPusher: pushers, interval: interval}
}

// NewMultiRawPusherWithInterval returns a RawPusher that pushes to all the given pushers with interval.
func NewMultiRawPusherWithInterval(interval time.Duration, pushers ...RawPusher) RawPusher {
	return &multiPusher{rawPusher: pushers, interval: interval}
}

func (m *multiPusher) PushText(s string) error {
	return m.pushAll(func(p TextPusher) error { return p.PushText(s) })
}

func (m *multiPusher) PushMarkdown(title, content string) error {
	return m.pushAll(func(p TextPusher) error { return p.PushMarkdown(title, content) })
}

func (m *multiPusher) pushAll(fn func(TextPusher) error) error {
	var pushErr *multierror.Error
	ok := 0
	for _, item := range m.textPusher {
		if err := fn(item); err != nil {
			pushErr = multierror.Append(pushErr, err)
		} else {
			ok++
		}
		if m.interval != 0 {
			time.Sleep(m.interval)
		}
	}
	if ok > 0 {
		if pushErr != nil {
			golog.Errorf("部分渠道推送失败，已成功 %d 个: %v", ok, pushErr)
		}
		return nil
	}
	return pushErr.ErrorOrNil()
}

func (m *multiPusher) PushRaw(r *RawMessage) error {
	for _, push := range m.rawPusher {
		if err := push.PushRaw(r); err != nil {
			return err
		}
		if m.interval != 0 {
			time.Sleep(m.interval)
		}
	}
	return nil
}
