package push

import "testing"

func TestNormalizeWechatKey(t *testing.T) {
	full := "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=abc-123"
	if got := normalizeWechatKey(full); got != "abc-123" {
		t.Fatalf("url key = %q", got)
	}
	if got := normalizeWechatKey("  abc-123  "); got != "abc-123" {
		t.Fatalf("plain key = %q", got)
	}
}

func TestNormalizeDingToken(t *testing.T) {
	full := "https://oapi.dingtalk.com/robot/send?access_token=dt-1"
	if got := normalizeDingToken(full); got != "dt-1" {
		t.Fatalf("url token = %q", got)
	}
}
