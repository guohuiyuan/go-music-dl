package core

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/guohuiyuan/music-lib/model"
)

func TestDownloadNeteaseFLACWithCookieRegression(t *testing.T) {
	cookie := strings.TrimSpace(os.Getenv("NETEASE_COOKIE"))
	if cookie == "" {
		t.Skip("NETEASE_COOKIE not set")
	}

	oldCookies := CM.GetAll()
	CM.mu.Lock()
	CM.cookies = map[string]string{"netease": cookie}
	CM.mu.Unlock()
	t.Cleanup(func() {
		CM.mu.Lock()
		CM.cookies = oldCookies
		CM.mu.Unlock()
	})

	result, err := DownloadSongData(&model.Song{
		ID:     "496869422",
		Source: "netease",
		Name:   "Netease FLAC Regression",
		Artist: "Netease",
	}, false, false)
	if err != nil {
		t.Fatalf("DownloadSongData returned error: %v", err)
	}
	if result.Ext != "flac" {
		t.Fatalf("download ext = %q, want flac; content-type=%q len=%d", result.Ext, result.ContentType, len(result.Data))
	}
	if len(result.Data) < 4 || string(result.Data[:4]) != "fLaC" {
		t.Fatalf("downloaded data is not FLAC; first bytes=%q len=%d", string(result.Data[:min(4, len(result.Data))]), len(result.Data))
	}
}

func TestProbeNeteaseFLACRangeRegression(t *testing.T) {
	cookie := strings.TrimSpace(os.Getenv("NETEASE_COOKIE"))
	if cookie == "" {
		t.Skip("NETEASE_COOKIE not set")
	}

	oldCookies := CM.GetAll()
	CM.mu.Lock()
	CM.cookies = map[string]string{"netease": cookie}
	CM.mu.Unlock()
	t.Cleanup(func() {
		CM.mu.Lock()
		CM.cookies = oldCookies
		CM.mu.Unlock()
	})

	urlStr, err := GetDownloadFunc("netease")(&model.Song{ID: "496869422", Source: "netease"})
	if err != nil {
		t.Fatalf("GetDownloadFunc returned error: %v", err)
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
	t.Logf("status=%d content-type=%q content-length=%d content-range=%q accept-ranges=%q first=%q",
		resp.StatusCode,
		resp.Header.Get("Content-Type"),
		resp.ContentLength,
		resp.Header.Get("Content-Range"),
		resp.Header.Get("Accept-Ranges"),
		string(data),
	)
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		t.Fatalf("range status = %d", resp.StatusCode)
	}
}

func TestNeteaseFLACTimingDiagnostic(t *testing.T) {
	cookie := strings.TrimSpace(os.Getenv("NETEASE_COOKIE"))
	if cookie == "" {
		t.Skip("NETEASE_COOKIE not set")
	}

	oldCookies := CM.GetAll()
	CM.mu.Lock()
	CM.cookies = map[string]string{"netease": cookie}
	CM.mu.Unlock()
	t.Cleanup(func() {
		CM.mu.Lock()
		CM.cookies = oldCookies
		CM.mu.Unlock()
	})

	song := &model.Song{ID: "496869422", Source: "netease"}
	fn := GetDownloadFunc("netease")

	start := time.Now()
	urlStr, err := fn(song)
	if err != nil {
		t.Fatalf("cold GetDownloadURL returned error: %v", err)
	}
	t.Logf("cold GetDownloadURL=%s ext=%q", time.Since(start).Round(time.Millisecond), song.Ext)

	start = time.Now()
	cachedURL, err := fn(&model.Song{ID: "496869422", Source: "netease"})
	if err != nil {
		t.Fatalf("cached GetDownloadURL returned error: %v", err)
	}
	if cachedURL != urlStr {
		t.Log("cached URL differs from cold URL")
	}
	t.Logf("cached GetDownloadURL=%s", time.Since(start).Round(time.Millisecond))

	for _, tc := range []struct {
		name        string
		rangeHeader string
	}{
		{name: "first-4-bytes", rangeHeader: "bytes=0-3"},
		{name: "first-1MiB", rangeHeader: "bytes=0-1048575"},
		{name: "next-4MiB", rangeHeader: "bytes=1048576-5242879"},
	} {
		status, n, elapsed, err := measureNeteaseRange(urlStr, tc.rangeHeader)
		if err != nil {
			t.Fatalf("%s range failed: %v", tc.name, err)
		}
		mbps := float64(n) / elapsed.Seconds() / 1024 / 1024
		t.Logf("%s status=%d bytes=%d time=%s throughput=%.2f MiB/s", tc.name, status, n, elapsed.Round(time.Millisecond), mbps)
	}
}

func measureNeteaseRange(urlStr string, rangeHeader string) (int, int, time.Duration, error) {
	req, err := BuildSourceRequest(http.MethodGet, urlStr, "netease", rangeHeader)
	if err != nil {
		return 0, 0, 0, err
	}
	client := &http.Client{Timeout: 2 * time.Minute}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, len(data), time.Since(start), err
	}
	return resp.StatusCode, len(data), time.Since(start), nil
}
