package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func resetConfigStateForTest() {
	if configDB != nil {
		if sqlDB, err := configDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	configDB = nil
	configInitErr = nil
	configInit = sync.Once{}

	CM.mu.Lock()
	CM.cookies = make(map[string]string)
	CM.mu.Unlock()
}

func TestCookieManagerMigratesLegacyJSONAndPersistsToSQLite(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "data", "settings.db"))
	t.Setenv("MUSIC_DL_COOKIE_FILE", filepath.Join(baseDir, "data", "cookies.json"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)

	if err := os.MkdirAll(filepath.Join(baseDir, "data"), 0755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}

	legacy := map[string]string{
		"netease": "foo=bar",
		"qq":      "uin=123",
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy cookies: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "data", "cookies.json"), raw, 0644); err != nil {
		t.Fatalf("write legacy cookies: %v", err)
	}

	CM.Load()
	if got := CM.GetAll(); !reflect.DeepEqual(got, legacy) {
		t.Fatalf("loaded cookies mismatch\ngot:  %#v\nwant: %#v", got, legacy)
	}

	CM.SetAll(map[string]string{
		"netease": "foo=updated",
		"qq":      "",
		"kugou":   "token=456",
	})
	CM.Save()

	resetConfigStateForTest()
	CM.Load()

	want := map[string]string{
		"netease": "foo=updated",
		"kugou":   "token=456",
	}
	if got := CM.GetAll(); !reflect.DeepEqual(got, want) {
		t.Fatalf("reloaded cookies mismatch\ngot:  %#v\nwant: %#v", got, want)
	}

	if _, err := os.Stat(filepath.Join(baseDir, "data", "settings.db")); err != nil {
		t.Fatalf("expected sqlite db to exist: %v", err)
	}
}

func TestWebSettingsDefaultAndPersist(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "data", "settings.db"))
	t.Setenv("MUSIC_DL_COOKIE_FILE", filepath.Join(baseDir, "data", "cookies.json"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)

	defaults := GetWebSettings()
	if !defaults.EmbedDownload {
		t.Fatalf("default EmbedDownload should be true")
	}
	if defaults.DownloadToLocal {
		t.Fatalf("default DownloadToLocal should be false")
	}
	if defaults.DownloadDir != normalizeWebDownloadDir(DefaultWebDownloadDir) {
		t.Fatalf("default DownloadDir mismatch: got %q want %q", defaults.DownloadDir, normalizeWebDownloadDir(DefaultWebDownloadDir))
	}
	if defaults.DownloadFilenameTemplate != DefaultDownloadFilenameTemplate {
		t.Fatalf("default DownloadFilenameTemplate mismatch: got %q want %q", defaults.DownloadFilenameTemplate, DefaultDownloadFilenameTemplate)
	}
	if defaults.DisableFloatingLyrics {
		t.Fatalf("default DisableFloatingLyrics should be false")
	}
	if defaults.WebPageSize != DefaultWebPageSize {
		t.Fatalf("default WebPageSize mismatch: got %d want %d", defaults.WebPageSize, DefaultWebPageSize)
	}
	if defaults.WebPageSize != 30 {
		t.Fatalf("default WebPageSize should be 30: got %d", defaults.WebPageSize)
	}
	if defaults.CliPageSize != DefaultCLIPageSize {
		t.Fatalf("default CliPageSize mismatch: got %d want %d", defaults.CliPageSize, DefaultCLIPageSize)
	}
	if defaults.CliPageSize != 20 {
		t.Fatalf("default CliPageSize should be 20: got %d", defaults.CliPageSize)
	}
	if defaults.DownloadConcurrency != DefaultWebConcurrency {
		t.Fatalf("default DownloadConcurrency mismatch: got %d want %d", defaults.DownloadConcurrency, DefaultWebConcurrency)
	}
	if !defaults.AutoCheckUpdate {
		t.Fatalf("default AutoCheckUpdate should be true")
	}
	if !defaults.AutoSwitchInvalidSources {
		t.Fatalf("default AutoSwitchInvalidSources should be true")
	}
	if defaults.UpdateRepoURL != DefaultUpdateRepoURL {
		t.Fatalf("default UpdateRepoURL mismatch: got %q want %q", defaults.UpdateRepoURL, DefaultUpdateRepoURL)
	}
	if defaults.GithubProxyEnabled {
		t.Fatalf("default GithubProxyEnabled should be false")
	}
	if defaults.GithubProxyURL != DefaultGithubProxyURL {
		t.Fatalf("default GithubProxyURL mismatch: got %q want %q", defaults.GithubProxyURL, DefaultGithubProxyURL)
	}
	if defaults.VgChangeCover {
		t.Fatalf("default VgChangeCover should be false")
	}
	if defaults.VgChangeAudio {
		t.Fatalf("default VgChangeAudio should be false")
	}
	if defaults.VgChangeLyric {
		t.Fatalf("default VgChangeLyric should be false")
	}
	if defaults.VgExportVideo {
		t.Fatalf("default VgExportVideo should be false")
	}

	if err := SaveWebSettings(WebSettings{
		EmbedDownload:            true,
		DownloadToLocal:          true,
		DownloadDir:              "",
		DownloadFilenameTemplate: "{artist} - {name}.{ext}",
		DisableFloatingLyrics:    true,
		WebPageSize:              100,
		CliPageSize:              120,
		DownloadConcurrency:      5,
		AutoCheckUpdate:          false,
		AutoSwitchInvalidSources: false,
		UpdateRepoURL:            "https://github.com/example/fork",
		GithubProxyEnabled:       true,
		GithubProxyURL:           "https://gh-proxy.com/",
		VgChangeCover:            true,
		VgChangeAudio:            true,
		VgChangeLyric:            true,
		VgExportVideo:            true,
	}); err != nil {
		t.Fatalf("save web settings: %v", err)
	}

	got := GetWebSettings()
	want := WebSettings{
		EmbedDownload:            true,
		DownloadToLocal:          true,
		DownloadDir:              normalizeWebDownloadDir(DefaultWebDownloadDir),
		DownloadFilenameTemplate: "{artist} - {name}.{ext}",
		DisableFloatingLyrics:    true,
		WebPageSize:              100,
		CliPageSize:              120,
		DownloadConcurrency:      5,
		AutoCheckUpdate:          false,
		AutoSwitchInvalidSources: false,
		UpdateRepoURL:            "https://github.com/example/fork",
		GithubProxyEnabled:       true,
		GithubProxyURL:           "https://gh-proxy.com/",
		VgChangeCover:            true,
		VgChangeAudio:            true,
		VgChangeLyric:            true,
		VgExportVideo:            true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("saved settings mismatch\ngot:  %#v\nwant: %#v", got, want)
	}

	customDir := filepath.Join("downloads", "custom")
	if err := SaveWebSettings(WebSettings{
		DownloadDir: customDir,
	}); err != nil {
		t.Fatalf("save custom download dir: %v", err)
	}

	got = GetWebSettings()
	if got.DownloadDir != normalizeWebDownloadDir(customDir) {
		t.Fatalf("custom download dir mismatch: got %q want %q", got.DownloadDir, normalizeWebDownloadDir(customDir))
	}
	if got.DownloadFilenameTemplate != DefaultDownloadFilenameTemplate {
		t.Fatalf("custom save should fallback DownloadFilenameTemplate to default: got %q want %q", got.DownloadFilenameTemplate, DefaultDownloadFilenameTemplate)
	}
	if got.DisableFloatingLyrics {
		t.Fatalf("custom save should fallback DisableFloatingLyrics to default false: %#v", got)
	}
	if got.WebPageSize != DefaultWebPageSize {
		t.Fatalf("custom save should fallback WebPageSize to default: got %d want %d", got.WebPageSize, DefaultWebPageSize)
	}
	if got.CliPageSize != DefaultCLIPageSize {
		t.Fatalf("custom save should fallback CliPageSize to default: got %d want %d", got.CliPageSize, DefaultCLIPageSize)
	}
	if got.DownloadConcurrency != DefaultWebConcurrency {
		t.Fatalf("custom save should fallback DownloadConcurrency to default: got %d want %d", got.DownloadConcurrency, DefaultWebConcurrency)
	}
	if got.AutoCheckUpdate {
		t.Fatalf("custom save should keep AutoCheckUpdate false when omitted: %#v", got)
	}
	if got.AutoSwitchInvalidSources {
		t.Fatalf("custom save should keep AutoSwitchInvalidSources false when omitted: %#v", got)
	}
	if got.UpdateRepoURL != DefaultUpdateRepoURL {
		t.Fatalf("custom save should fallback UpdateRepoURL to default: got %q want %q", got.UpdateRepoURL, DefaultUpdateRepoURL)
	}
	if got.GithubProxyEnabled {
		t.Fatalf("custom save should fallback GithubProxyEnabled to default false: %#v", got)
	}
	if got.GithubProxyURL != DefaultGithubProxyURL {
		t.Fatalf("custom save should fallback GithubProxyURL to default: got %q want %q", got.GithubProxyURL, DefaultGithubProxyURL)
	}
	if got.VgChangeCover || got.VgChangeAudio || got.VgChangeLyric || got.VgExportVideo {
		t.Fatalf("custom save should fallback video generator settings to default false: %#v", got)
	}

	absoluteDir := filepath.Join(baseDir, "downloads", "absolute")
	if err := SaveWebSettings(WebSettings{
		DownloadDir: absoluteDir,
	}); err != nil {
		t.Fatalf("save absolute download dir: %v", err)
	}

	got = GetWebSettings()
	if got.DownloadDir != filepath.Clean(absoluteDir) {
		t.Fatalf("absolute download dir mismatch: got %q want %q", got.DownloadDir, filepath.Clean(absoluteDir))
	}
}

func TestWebSettingsLegacyPayloadDefaultsAutoSwitchInvalidSources(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "data", "settings.db"))
	t.Setenv("MUSIC_DL_COOKIE_FILE", filepath.Join(baseDir, "data", "cookies.json"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)

	legacySettings := map[string]any{
		"downloadDir":     DefaultWebDownloadDir,
		"autoCheckUpdate": true,
	}
	data, err := json.Marshal(legacySettings)
	if err != nil {
		t.Fatalf("marshal legacy settings: %v", err)
	}
	if err := ensureConfigDB(); err != nil {
		t.Fatalf("ensure config db: %v", err)
	}
	if err := configDB.Save(&configKV{Key: webSettingsKey, Value: string(data)}).Error; err != nil {
		t.Fatalf("save legacy settings: %v", err)
	}

	got := GetWebSettings()
	if !got.AutoSwitchInvalidSources {
		t.Fatalf("legacy settings should default AutoSwitchInvalidSources to true: %#v", got)
	}
}

func TestWebAuthSettingsDefaultAndPersist(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "data", "settings.db"))
	t.Setenv("MUSIC_DL_COOKIE_FILE", filepath.Join(baseDir, "data", "cookies.json"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)

	defaults, err := GetWebAuthSettings()
	if err != nil {
		t.Fatalf("get default auth settings: %v", err)
	}
	if defaults.Username != DefaultWebAuthUsername {
		t.Fatalf("default Username = %q, want %q", defaults.Username, DefaultWebAuthUsername)
	}
	if defaults.PasswordHash != "" {
		t.Fatalf("default PasswordHash should be empty")
	}
	if defaults.SessionSecret != "" {
		t.Fatalf("default SessionSecret should be empty")
	}

	want := WebAuthSettings{
		Username:      "owner",
		PasswordHash:  "bcrypt-hash",
		SessionSecret: "session-secret",
	}
	if err := SaveWebAuthSettings(want); err != nil {
		t.Fatalf("save auth settings: %v", err)
	}

	got, err := GetWebAuthSettings()
	if err != nil {
		t.Fatalf("get saved auth settings: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("saved auth settings mismatch\ngot:  %#v\nwant: %#v", got, want)
	}

	if err := SaveWebAuthSettings(WebAuthSettings{}); err != nil {
		t.Fatalf("save empty auth settings: %v", err)
	}
	got, err = GetWebAuthSettings()
	if err != nil {
		t.Fatalf("get normalized auth settings: %v", err)
	}
	if got.Username != DefaultWebAuthUsername {
		t.Fatalf("normalized Username = %q, want %q", got.Username, DefaultWebAuthUsername)
	}
}
