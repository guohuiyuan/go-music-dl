package web

import (
	"strings"
	"testing"
)

func TestClassifyLyricFormat(t *testing.T) {
	rich := "[00:01.00]你[00:01.50]好[00:02.00]\n[00:01.00]hello[00:02.00]\n[00:01.00]ni hao[00:02.00]"
	if got := classifyLyricFormat(rich); got != lyricFormatKaraoke {
		t.Fatalf("rich format = %q, want %q", got, lyricFormatKaraoke)
	}

	line := "[00:01.00]你好\n[00:02.00]世界"
	if got := classifyLyricFormat(line); got != lyricFormatLine {
		t.Fatalf("line format = %q, want %q", got, lyricFormatLine)
	}
}

func TestLyricOriginalLineOnly(t *testing.T) {
	raw := "[ti:test]\n[00:01.00]你[00:01.50]好[00:02.00]\n[00:01.00]hello[00:02.00]\n[00:03.00]世界[00:04.00]"
	got := lyricOriginalLineOnly(raw)
	for _, want := range []string{"[ti:test]", "[00:01.00]你好", "[00:03.00]世界"} {
		if !strings.Contains(got, want) {
			t.Fatalf("line-only lyric missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "hello") || strings.Contains(got, "[00:01.50]") {
		t.Fatalf("line-only lyric still contains translation or word timestamp:\n%s", got)
	}
}
