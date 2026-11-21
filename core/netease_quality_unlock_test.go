package core

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
)

// TestNeteaseVIPCookieUnlocksPremiumQualities reproduces the reported bug:
// with a valid high-tier NetEase cookie, premium tiers used to stay greyed out
// because the bare cookie made the server downgrade every request to lossless.
func TestNeteaseVIPCookieUnlocksPremiumQualities(t *testing.T) {
	cookie := strings.TrimSpace(os.Getenv("NETEASE_COOKIE"))
	if cookie == "" {
		t.Skip("NETEASE_COOKIE not set")
	}

	oldCookies := CM.GetAll()
	t.Cleanup(func() {
		CM.mu.Lock()
		CM.cookies = oldCookies
		CM.mu.Unlock()
		resetAudioQualityCache()
	})
	CM.mu.Lock()
	CM.cookies = map[string]string{"netease": cookie}
	CM.mu.Unlock()
	resetAudioQualityCache()

	options, err := GetAudioQualityOptions("netease")
	if err != nil {
		t.Fatalf("GetAudioQualityOptions(netease) returned error: %v", err)
	}
	if len(options) == 0 {
		t.Fatal("GetAudioQualityOptions(netease) returned no options")
	}
	if options[0].Value != model.AudioQualityMaster {
		t.Fatalf("highest available quality = %q, want master; options=%#v", options[0].Value, options)
	}
	for _, want := range []model.AudioQuality{
		model.AudioQualityMaster,
		model.AudioQualitySpatial,
		model.AudioQualityVivid,
		model.AudioQualityAtmos,
		model.AudioQualityHiRes,
		model.AudioQualityLossless,
	} {
		if !containsAudioQuality(options, want) {
			t.Fatalf("available options %#v missing %q", options, want)
		}
	}

	provider := netease.New(CM.Get("netease"))
	song := &model.Song{ID: "496869422", Source: "netease"}
	urlStr, err := provider.GetDownloadURLWithQuality(song, string(model.AudioQualityMaster))
	if err != nil {
		t.Fatalf("master download url returned error: %v", err)
	}
	if song.Ext != "flac" {
		t.Fatalf("master download ext = %q, want flac", song.Ext)
	}

	req, err := BuildSourceRequest(http.MethodGet, urlStr, "netease", "bytes=0-3")
	if err != nil {
		t.Fatalf("BuildSourceRequest returned error: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("range request returned error: %v", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("range read returned error: %v", err)
	}
	if len(data) != 4 || string(data) != "fLaC" {
		t.Fatalf("master stream is not FLAC: first bytes=%q", string(data))
	}
}

func resetAudioQualityCache() {
	audioQualityOptionsCache.Lock()
	audioQualityOptionsCache.entries = make(map[string]audioQualityCacheEntry)
	audioQualityOptionsCache.inflight = make(map[string]*audioQualityInflight)
	audioQualityOptionsCache.Unlock()
}
