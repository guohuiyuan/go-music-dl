package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
)

const githubRequestTimeout = 5 * time.Second

var githubAPIBaseURL = "https://api.github.com"

type githubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
		ContentType        string `json:"content_type"`
	} `json:"assets"`
}

type updateAsset struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	ProxiedURL  string `json:"proxied_url"`
	Size        int64  `json:"size"`
	SizeText    string `json:"size_text"`
	ContentType string `json:"content_type"`
	Preferred   bool   `json:"preferred"`
}

type updateCheckResponse struct {
	CurrentVersion  string        `json:"current_version"`
	LatestVersion   string        `json:"latest_version"`
	UpdateAvailable bool          `json:"update_available"`
	ReleaseName     string        `json:"release_name"`
	ReleaseURL      string        `json:"release_url"`
	DownloadURL     string        `json:"download_url"`
	ProxiedURL      string        `json:"proxied_url"`
	RepoURL         string        `json:"repo_url"`
	ProxyEnabled    bool          `json:"proxy_enabled"`
	ProxyURL        string        `json:"proxy_url"`
	Assets          []updateAsset `json:"assets"`
	Body            string        `json:"body"`
	CheckedAt       time.Time     `json:"checked_at"`
}

func RegisterUpdateRoutes(api *gin.RouterGroup) {
	api.GET("/app_update/check", func(c *gin.Context) {
		settings := core.GetWebSettings()
		repoURL := strings.TrimSpace(c.DefaultQuery("repo", settings.UpdateRepoURL))
		proxyURL := strings.TrimSpace(c.DefaultQuery("proxy", settings.GithubProxyURL))
		proxyEnabled := settings.GithubProxyEnabled
		if raw := strings.TrimSpace(c.Query("use_proxy")); raw != "" {
			proxyEnabled = raw == "1" || strings.EqualFold(raw, "true")
		}

		result, err := checkLatestRelease(c.Request.Context(), repoURL, proxyEnabled, proxyURL)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	})

	api.GET("/github_proxy/test", func(c *gin.Context) {
		proxyURL := strings.TrimSpace(c.Query("proxy"))
		if proxyURL == "" {
			proxyURL = core.DefaultGithubProxyURL
		}
		target := proxiedGitHubURL("https://github.com/guohuiyuan/go-music-dl", proxyURL, true)
		startedAt := time.Now()
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, target, nil)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		req.Header.Set("User-Agent", "go-music-dl/"+core.AppVersion)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"ok":         false,
				"proxy_url":  proxyURL,
				"target_url": target,
				"latency_ms": time.Since(startedAt).Milliseconds(),
				"error":      err.Error(),
			})
			return
		}
		defer resp.Body.Close()

		c.JSON(http.StatusOK, gin.H{
			"ok":         resp.StatusCode >= 200 && resp.StatusCode < 500,
			"proxy_url":  proxyURL,
			"target_url": target,
			"status":     resp.Status,
			"latency_ms": time.Since(startedAt).Milliseconds(),
		})
	})
}

func checkLatestRelease(ctx context.Context, repoURL string, proxyEnabled bool, proxyURL string) (updateCheckResponse, error) {
	owner, repo, err := githubRepoFromURL(repoURL)
	if err != nil {
		return updateCheckResponse{}, err
	}

	release, err := fetchLatestGitHubRelease(ctx, owner, repo, proxyEnabled, proxyURL)
	if err != nil {
		return updateCheckResponse{}, err
	}

	latest := normalizeVersion(release.TagName)
	if latest == "" {
		latest = normalizeVersion(release.Name)
	}
	current := normalizeVersion(core.AppVersion)
	updateAvailable := compareVersions(latest, current) > 0

	assets := make([]updateAsset, 0, len(release.Assets))
	preferredIndex := preferredUpdateAssetIndex(release.Assets)
	for index, asset := range release.Assets {
		rawURL := strings.TrimSpace(asset.BrowserDownloadURL)
		assets = append(assets, updateAsset{
			Name:        asset.Name,
			URL:         rawURL,
			ProxiedURL:  proxiedGitHubURL(rawURL, proxyURL, proxyEnabled),
			Size:        asset.Size,
			SizeText:    core.FormatSize(asset.Size),
			ContentType: asset.ContentType,
			Preferred:   index == preferredIndex,
		})
	}

	downloadURL := strings.TrimSpace(release.HTMLURL)
	if preferredIndex >= 0 && preferredIndex < len(assets) && assets[preferredIndex].URL != "" {
		downloadURL = assets[preferredIndex].URL
	}
	proxiedURL := proxiedGitHubURL(downloadURL, proxyURL, proxyEnabled)

	return updateCheckResponse{
		CurrentVersion:  current,
		LatestVersion:   latest,
		UpdateAvailable: updateAvailable,
		ReleaseName:     strings.TrimSpace(release.Name),
		ReleaseURL:      strings.TrimSpace(release.HTMLURL),
		DownloadURL:     downloadURL,
		ProxiedURL:      proxiedURL,
		RepoURL:         fmt.Sprintf("https://github.com/%s/%s", owner, repo),
		ProxyEnabled:    proxyEnabled,
		ProxyURL:        proxyURL,
		Assets:          assets,
		Body:            strings.TrimSpace(release.Body),
		CheckedAt:       time.Now(),
	}, nil
}

func fetchLatestGitHubRelease(ctx context.Context, owner string, repo string, proxyEnabled bool, proxyURL string) (githubRelease, error) {
	apiURL := strings.TrimRight(githubAPIBaseURL, "/") + fmt.Sprintf("/repos/%s/%s/releases/latest", owner, repo)
	candidates := []string{apiURL}
	if proxyEnabled && strings.TrimSpace(proxyURL) != "" {
		if proxied := proxiedGitHubURL(apiURL, proxyURL, true); proxied != "" && proxied != apiURL {
			candidates = []string{proxied, apiURL}
		}
	}

	if len(candidates) == 1 {
		return requestGitHubRelease(ctx, candidates[0])
	}
	return raceGitHubReleaseCandidates(ctx, candidates)
}

func raceGitHubReleaseCandidates(ctx context.Context, urls []string) (githubRelease, error) {
	type fetchResult struct {
		release githubRelease
		err     error
	}

	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan fetchResult, len(urls))
	for _, candidate := range urls {
		candidate := candidate
		go func() {
			release, err := requestGitHubRelease(raceCtx, candidate)
			results <- fetchResult{release: release, err: err}
		}()
	}

	var lastErr error
	for i := 0; i < len(urls); i++ {
		select {
		case r := <-results:
			if r.err == nil {
				return r.release, nil
			}
			lastErr = r.err
		case <-ctx.Done():
			return githubRelease{}, ctx.Err()
		}
	}
	if lastErr == nil {
		lastErr = errors.New("GitHub release request failed")
	}
	return githubRelease{}, lastErr
}

func requestGitHubRelease(ctx context.Context, apiURL string) (githubRelease, error) {
	reqCtx, cancel := context.WithTimeout(ctx, githubRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, apiURL, nil)
	if err != nil {
		return githubRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "go-music-dl/"+core.AppVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return githubRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return githubRelease{}, fmt.Errorf("GitHub returned %s", resp.Status)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return githubRelease{}, err
	}
	if strings.TrimSpace(release.TagName) == "" && strings.TrimSpace(release.Name) == "" {
		return githubRelease{}, errors.New("GitHub release response missing version")
	}
	return release, nil
}

func githubRepoFromURL(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = core.DefaultUpdateRepoURL
	}
	if !strings.Contains(raw, "://") {
		raw = "https://github.com/" + strings.TrimPrefix(raw, "github.com/")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", err
	}
	host := strings.ToLower(strings.TrimPrefix(parsed.Host, "www."))
	if host != "github.com" {
		return "", "", fmt.Errorf("only github.com repository links are supported: %s", raw)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid GitHub repository link: %s", raw)
	}
	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSuffix(strings.TrimSpace(parts[1]), ".git")
	if owner == "" || repo == "" {
		return "", "", fmt.Errorf("invalid GitHub repository link: %s", raw)
	}
	return owner, repo, nil
}

func proxiedGitHubURL(rawURL string, proxyURL string, enabled bool) string {
	rawURL = strings.TrimSpace(rawURL)
	proxyURL = strings.TrimSpace(proxyURL)
	if !enabled || rawURL == "" || proxyURL == "" {
		return rawURL
	}
	return strings.TrimRight(proxyURL, "/") + "/" + rawURL
}

func normalizeVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "refs/tags/")
	raw = strings.TrimPrefix(strings.TrimPrefix(raw, "v"), "V")
	var b strings.Builder
	for _, r := range raw {
		if (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		break
	}
	return strings.Trim(b.String(), ".-_")
}

func compareVersions(a string, b string) int {
	ap := versionNumberParts(a)
	bp := versionNumberParts(b)
	maxLen := len(ap)
	if len(bp) > maxLen {
		maxLen = len(bp)
	}
	for i := 0; i < maxLen; i++ {
		av, bv := 0, 0
		if i < len(ap) {
			av = ap[i]
		}
		if i < len(bp) {
			bv = bp[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func versionNumberParts(version string) []int {
	version = normalizeVersion(version)
	if version == "" {
		return []int{0}
	}
	fields := strings.FieldsFunc(version, func(r rune) bool {
		return r == '.' || r == '-' || r == '_'
	})
	parts := make([]int, 0, len(fields))
	for _, field := range fields {
		n, err := strconv.Atoi(field)
		if err != nil {
			parts = append(parts, 0)
			continue
		}
		parts = append(parts, n)
	}
	if len(parts) == 0 {
		return []int{0}
	}
	return parts
}

func preferredUpdateAssetIndex(assets []struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}) int {
	if len(assets) == 0 {
		return -1
	}
	bestIndex := 0
	bestScore := updateAssetPreferenceScore(assets[0].Name)
	for i := 1; i < len(assets); i++ {
		if score := updateAssetPreferenceScore(assets[i].Name); score > bestScore {
			bestIndex = i
			bestScore = score
		}
	}
	if bestScore > 0 {
		return bestIndex
	}

	osKey := runtime.GOOS
	archKey := runtime.GOARCH
	if osKey == "darwin" {
		osKey = "mac"
	}
	if archKey == "amd64" {
		archKey = "x64"
	}
	for i, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, osKey) && (strings.Contains(name, archKey) || strings.Contains(name, runtime.GOARCH)) {
			return i
		}
	}
	for i, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, runtime.GOOS) || strings.Contains(name, osKey) {
			return i
		}
	}
	return 0
}

func updateAssetPreferenceScore(name string) int {
	name = strings.ToLower(name)
	ext := filepath.Ext(name)
	score := 0
	targetName := strings.ToLower(filepath.Base(strings.TrimSpace(currentExecutableName())))
	if strings.Contains(targetName, "desktop") {
		if strings.Contains(name, "desktop") {
			score += 35
		} else {
			score -= 20
		}
	}
	if strings.Contains(targetName, "rust") && strings.Contains(name, "rust") {
		score += 15
	}

	osKey := runtime.GOOS
	if osKey == "darwin" {
		osKey = "mac"
	}
	if strings.Contains(name, runtime.GOOS) || strings.Contains(name, osKey) {
		score += 40
	}

	archKey := runtime.GOARCH
	if archKey == "amd64" {
		archKey = "x64"
	}
	if strings.Contains(name, runtime.GOARCH) || strings.Contains(name, archKey) {
		score += 20
	}

	if runtime.GOOS == "windows" && ext == ".exe" {
		score += 30
	}
	if runtime.GOOS != "windows" && !isUpdateArchiveExt(ext) {
		score += 15
	}
	if isUpdateArchiveExt(ext) {
		score -= 10
	}
	return score
}

func currentExecutableName() string {
	if hint := strings.TrimSpace(os.Getenv("MUSIC_DL_APP_MODE")); hint != "" {
		return hint
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}

func isUpdateArchiveExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".zip", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".7z", ".rar",
		".dmg", ".pkg", ".msi", ".deb", ".rpm", ".apk":
		return true
	default:
		return false
	}
}
