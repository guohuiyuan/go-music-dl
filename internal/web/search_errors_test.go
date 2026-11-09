package web

import (
	"strings"
	"testing"
)

func TestFormatSearchSourceErrorsUsesRequestedOrderAndSkipsLocal(t *testing.T) {
	got := formatSearchSourceErrors(
		[]string{"spotify", "local", "qq", "spotify"},
		map[string]string{
			"qq":      "qq timeout",
			"spotify": "anonymous web player unavailable",
			"local":   "local scan failed",
		},
	)
	want := "部分平台搜索失败: Spotify: anonymous web player unavailable；QQ音乐: qq timeout"
	if got != want {
		t.Fatalf("formatSearchSourceErrors() = %q, want %q", got, want)
	}
}

func TestFormatSearchSourceErrorsSingleAndTruncated(t *testing.T) {
	got := formatSearchSourceErrors(
		[]string{"spotify"},
		map[string]string{"spotify": strings.Repeat("x", 300)},
	)
	if !strings.HasPrefix(got, "搜索失败: Spotify: ") {
		t.Fatalf("formatSearchSourceErrors() = %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("formatSearchSourceErrors() did not truncate: %q", got)
	}
	if len([]rune(got)) > 240 {
		t.Fatalf("formatSearchSourceErrors() result is too long: %d", len([]rune(got)))
	}
}

func TestFormatSearchSourceErrorsEmpty(t *testing.T) {
	if got := formatSearchSourceErrors([]string{"spotify"}, nil); got != "" {
		t.Fatalf("formatSearchSourceErrors() = %q, want empty", got)
	}
}
