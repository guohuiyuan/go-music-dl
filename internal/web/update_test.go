package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestGitHubRepoFromURLAndProxyURL(t *testing.T) {
	owner, repo, err := githubRepoFromURL("https://github.com/guohuiyuan/go-music-dl.git")
	if err != nil {
		t.Fatalf("githubRepoFromURL returned error: %v", err)
	}
	if owner != "guohuiyuan" || repo != "go-music-dl" {
		t.Fatalf("repo parse mismatch: owner=%q repo=%q", owner, repo)
	}

	got := proxiedGitHubURL("https://github.com/guohuiyuan/go-music-dl/releases", "https://gh-proxy.com/", true)
	want := "https://gh-proxy.com/https://github.com/guohuiyuan/go-music-dl/releases"
	if got != want {
		t.Fatalf("proxiedGitHubURL = %q, want %q", got, want)
	}
	if compareVersions("1.4.0", "1.3.1") <= 0 {
		t.Fatal("compareVersions should detect newer version")
	}
}

func TestUpdateCheckRouteUsesGitHubReleaseResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/guohuiyuan/go-music-dl/releases/latest" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "v9.9.9",
			"name": "v9.9.9",
			"html_url": "https://github.com/guohuiyuan/go-music-dl/releases/tag/v9.9.9",
			"body": "release notes",
			"assets": [
				{"name": "music-dl-windows-amd64.zip", "browser_download_url": "https://github.com/guohuiyuan/go-music-dl/releases/download/v9.9.9/music-dl-windows-amd64.zip", "size": 1024, "content_type": "application/zip"}
			]
		}`))
	}))
	defer server.Close()

	originalBase := githubAPIBaseURL
	githubAPIBaseURL = server.URL
	t.Cleanup(func() { githubAPIBaseURL = originalBase })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterUpdateRoutes(router.Group(RoutePrefix))

	req := httptest.NewRequest(http.MethodGet, RoutePrefix+"/app_update/check?repo=https://github.com/guohuiyuan/go-music-dl", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"latest_version":"9.9.9"`,
		`"update_available":true`,
		`music-dl-windows-amd64.zip`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("update response missing %q: %s", want, body)
		}
	}
}

// TestFetchLatestGitHubReleaseRacesProxyAndOrigin 验证当代理 URL 非空时，
// 后端会并发竞速代理候选与原始 GitHub API，谁先成功用谁，整体延迟约等于更快那一侧。
func TestFetchLatestGitHubReleaseRacesProxyAndOrigin(t *testing.T) {
	var fastHits, slowHits atomic.Int32
	fastServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fastHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v2.0.0","name":"v2.0.0","html_url":"https://github.com/guohuiyuan/go-music-dl/releases/tag/v2.0.0"}`))
	}))
	defer fastServer.Close()

	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slowHits.Add(1)
		time.Sleep(1500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.0.0","name":"v1.0.0","html_url":"https://github.com/guohuiyuan/go-music-dl/releases/tag/v1.0.0"}`))
	}))
	defer slowServer.Close()

	// 把原始 GitHub API 指向慢服务，代理候选指向快服务（proxiedGitHubURL 通过前缀拼接）。
	originalBase := githubAPIBaseURL
	githubAPIBaseURL = slowServer.URL
	t.Cleanup(func() { githubAPIBaseURL = originalBase })

	proxyURL := fastServer.URL + "/"

	startedAt := time.Now()
	release, err := fetchLatestGitHubRelease(context.Background(), "guohuiyuan", "go-music-dl", true, proxyURL)
	elapsed := time.Since(startedAt)
	if err != nil {
		t.Fatalf("fetchLatestGitHubRelease error: %v", err)
	}
	if release.TagName != "v2.0.0" {
		t.Fatalf("expected fast (proxy) candidate to win, got tag %q", release.TagName)
	}
	if elapsed >= 1500*time.Millisecond {
		t.Fatalf("race should return as soon as fast candidate succeeds, elapsed=%s", elapsed)
	}
	if fastHits.Load() == 0 {
		t.Fatal("proxy candidate should have been requested")
	}
}

// TestFetchLatestGitHubReleaseFallsBackWhenProxyFails 验证代理失败时仍然可以拿到原始 API 的结果。
func TestFetchLatestGitHubReleaseFallsBackWhenProxyFails(t *testing.T) {
	originServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v3.0.0","name":"v3.0.0","html_url":"https://github.com/guohuiyuan/go-music-dl/releases/tag/v3.0.0"}`))
	}))
	defer originServer.Close()

	brokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer brokenServer.Close()

	originalBase := githubAPIBaseURL
	githubAPIBaseURL = originServer.URL
	t.Cleanup(func() { githubAPIBaseURL = originalBase })

	release, err := fetchLatestGitHubRelease(context.Background(), "guohuiyuan", "go-music-dl", true, brokenServer.URL+"/")
	if err != nil {
		t.Fatalf("fetchLatestGitHubRelease error: %v", err)
	}
	if release.TagName != "v3.0.0" {
		t.Fatalf("expected origin to win when proxy fails, got tag %q", release.TagName)
	}
}
