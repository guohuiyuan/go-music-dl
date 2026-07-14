package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestPlaybackArgsInjectsRefererAndCookie(t *testing.T) {
	CM.SetAll(map[string]string{"netease": "MUSIC_U=token"})
	t.Cleanup(func() { CM.SetAll(map[string]string{"netease": ""}) })

	song := &model.Song{ID: "1", Source: "netease"}
	args := PlaybackArgs(song, "http://example.com/a.mp3")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-nodisp") || !strings.Contains(joined, "-autoexit") {
		t.Fatalf("missing base flags: %v", args)
	}
	if args[len(args)-1] != "http://example.com/a.mp3" {
		t.Fatalf("url must be last arg, got %v", args)
	}

	uaIdx := indexOf(args, "-user_agent")
	if uaIdx < 0 || args[uaIdx+1] != UA_Common {
		t.Fatalf("expected common UA, got %v", args)
	}

	hdrIdx := indexOf(args, "-headers")
	if hdrIdx < 0 {
		t.Fatalf("expected -headers, got %v", args)
	}
	headers := args[hdrIdx+1]
	if !strings.Contains(headers, "Referer: "+Ref_Netease) {
		t.Fatalf("expected netease referer in headers: %q", headers)
	}
	if !strings.Contains(headers, "Cookie: MUSIC_U=token") {
		t.Fatalf("expected cookie in headers: %q", headers)
	}
}

func TestPlaybackArgsMiguUsesMobileUA(t *testing.T) {
	song := &model.Song{ID: "9", Source: "migu"}
	args := PlaybackArgs(song, "http://example.com/b.mp3")

	uaIdx := indexOf(args, "-user_agent")
	if uaIdx < 0 || args[uaIdx+1] != UA_Mobile {
		t.Fatalf("expected mobile UA for migu, got %v", args)
	}
}

func TestResolveFFplayPathUsesEnv(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "ffplay")
	if err := os.WriteFile(tool, []byte("test"), 0755); err != nil {
		t.Fatalf("write tool: %v", err)
	}

	t.Setenv(ffplayEnvName, tool)
	got, err := ResolveFFplayPath()
	if err != nil {
		t.Fatalf("ResolveFFplayPath error: %v", err)
	}
	if got != tool {
		t.Fatalf("ResolveFFplayPath = %q, want %q", got, tool)
	}
}

func indexOf(args []string, target string) int {
	for i, a := range args {
		if a == target {
			return i
		}
	}
	return -1
}
