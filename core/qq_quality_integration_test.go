package core

import (
	"os"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/qq"
)

func TestQQConfiguredQualityOptionsIntegration(t *testing.T) {
	if os.Getenv("MUSIC_DL_RUN_QQ_QUALITY_INTEGRATION") != "1" {
		t.Skip("set MUSIC_DL_RUN_QQ_QUALITY_INTEGRATION=1 to probe the configured QQ account")
	}

	originalCookies := CM.GetAll()
	t.Cleanup(func() { CM.SetAll(originalCookies) })
	cookie := strings.TrimSpace(os.Getenv("QQ_TEST_COOKIE"))
	if cookie == "" {
		CM.Load()
		cookie = CM.Get("qq")
	}
	if strings.TrimSpace(cookie) == "" {
		t.Skip("no QQ cookie is configured")
	}

	client := qq.New(cookie)
	options, err := client.AvailableQualityOptions()
	if err != nil {
		t.Fatalf("probe QQ quality options: %v", err)
	}
	if len(options) < 2 {
		t.Fatalf("expected at least standard and high quality, got %#v", options)
	}

	var highAvailable bool
	for _, option := range options {
		if option.Value == model.AudioQualityHigh {
			highAvailable = true
		}
		t.Logf("QQ quality available: %s (%s)", option.Value, option.Label)
	}
	if !highAvailable {
		t.Fatalf("high quality is not available: %#v", options)
	}

	url, err := client.GetDownloadURLWithQuality(
		&model.Song{ID: "0039MnYb0qxYhV", Source: "qq"},
		string(model.AudioQualityHigh),
	)
	if err != nil {
		t.Fatalf("resolve high-quality URL: %v", err)
	}
	if !strings.HasPrefix(url, "https://ws.stream.qqmusic.qq.com/") {
		t.Fatalf("unexpected QQ download URL host: %q", url)
	}
}

func TestQQPlaylistQualityFallbackIntegration(t *testing.T) {
	if os.Getenv("MUSIC_DL_RUN_QQ_QUALITY_INTEGRATION") != "1" {
		t.Skip("set MUSIC_DL_RUN_QQ_QUALITY_INTEGRATION=1 to probe the configured QQ account")
	}

	cookie := strings.TrimSpace(os.Getenv("QQ_TEST_COOKIE"))
	if cookie == "" {
		CM.Load()
		cookie = CM.Get("qq")
	}
	if cookie == "" {
		t.Skip("no QQ cookie is configured")
	}

	originalCookies := CM.GetAll()
	CM.SetAll(map[string]string{"qq": cookie})
	t.Cleanup(func() { CM.SetAll(originalCookies) })

	client := qq.New(cookie)
	songs, err := client.GetPlaylistSongs("7078064623")
	if err != nil {
		t.Fatalf("load QQ playlist: %v", err)
	}
	if len(songs) == 0 {
		t.Fatal("QQ playlist returned no songs")
	}

	download := GetDownloadFuncWithQuality("qq", string(model.AudioQualityMaster))
	for index := 0; index < len(songs) && index < 12; index++ {
		song := &songs[index]
		_, exactErr := client.GetDownloadURLWithQuality(
			song,
			string(model.AudioQualityMaster),
		)
		if exactErr == nil {
			continue
		}

		lowerQuality := model.AudioQuality("")
		var lowerURL string
		for _, option := range qq.ListQualityOptions()[1:] {
			url, lowerErr := client.GetDownloadURLWithQuality(song, string(option.Value))
			if lowerErr == nil && strings.TrimSpace(url) != "" {
				lowerQuality = option.Value
				lowerURL = url
				break
			}
		}
		if lowerQuality == "" {
			continue
		}

		url, fallbackErr := download(song)
		if fallbackErr != nil {
			t.Fatalf("fallback for %q: %v", song.Name, fallbackErr)
		}
		if !strings.HasPrefix(url, "https://ws.stream.qqmusic.qq.com/") {
			t.Fatalf("unexpected fallback URL host for %q: %q", song.Name, url)
		}
		if strings.SplitN(url, "?", 2)[0] != strings.SplitN(lowerURL, "?", 2)[0] {
			t.Fatalf(
				"fallback path for %q = %q, want first lower tier %s path %q",
				song.Name,
				url,
				lowerQuality,
				lowerURL,
			)
		}
		t.Logf(
			"verified fallback for %q (%s): master unavailable, resolved %s",
			song.Name,
			song.ID,
			lowerQuality,
		)
		return
	}
	t.Skip("no song in the first 12 playlist entries required a quality fallback")
}
