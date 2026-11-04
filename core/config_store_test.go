package core

import (
	"encoding/json"
	"errors"
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
	if defaults.DownloadLyricsToLocal {
		t.Fatalf("default DownloadLyricsToLocal should be false")
	}
	if defaults.DownloadDir != normalizeWebDownloadDir(DefaultWebDownloadDir) {
		t.Fatalf("default DownloadDir mismatch: got %q want %q", defaults.DownloadDir, normalizeWebDownloadDir(DefaultWebDownloadDir))
	}
	if defaults.DownloadFilenameTemplate != DefaultDownloadFilenameTemplate {
		t.Fatalf("default DownloadFilenameTemplate mismatch: got %q want %q", defaults.DownloadFilenameTemplate, DefaultDownloadFilenameTemplate)
	}
	if defaults.WebDAVEnabled {
		t.Fatalf("default WebDAVEnabled should be false")
	}
	if defaults.WebDAVURL != "" {
		t.Fatalf("default WebDAVURL should be empty")
	}
	if defaults.WebDAVDir != DefaultWebDAVDir {
		t.Fatalf("default WebDAVDir mismatch: got %q want %q", defaults.WebDAVDir, DefaultWebDAVDir)
	}
	if defaults.DisableFloatingLyrics {
		t.Fatalf("default DisableFloatingLyrics should be false")
	}
	if defaults.WebPageSize != DefaultWebPageSize {
		t.Fatalf("default WebPageSize mismatch: got %d want %d", defaults.WebPageSize, DefaultWebPageSize)
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
	if defaults.DownloadTipDuration != DefaultDownloadTipDuration {
		t.Fatalf("default DownloadTipDuration mismatch: got %d want %d", defaults.DownloadTipDuration, DefaultDownloadTipDuration)
	}
	if !defaults.AutoCheckUpdate {
		t.Fatalf("default AutoCheckUpdate should be true")
	}
	if !defaults.AutoSwitchInvalidSources {
		t.Fatalf("default AutoSwitchInvalidSources should be true")
	}
	if defaults.AutoCacheOnPlay {
		t.Fatalf("default AutoCacheOnPlay should be false")
	}
	if defaults.FilterVersionKeywords || defaults.FilterShortTracks {
		t.Fatalf("default search filters should be disabled: %#v", defaults)
	}
	if defaults.VersionFilterKeywords != "Live, Remix, Demo" {
		t.Fatalf("default VersionFilterKeywords mismatch: %q", defaults.VersionFilterKeywords)
	}
	if !defaults.NormalizeChineseVariants {
		t.Fatal("default NormalizeChineseVariants should be true")
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
	if defaults.SodaSignerURL != "" || defaults.SodaSignerToken != "" {
		t.Fatalf("default soda signer should be empty (fallback to local signer): got %q / %q", defaults.SodaSignerURL, defaults.SodaSignerToken)
	}
	if defaults.ShortcutPlayPause != DefaultShortcutPlayPause {
		t.Fatalf("default ShortcutPlayPause mismatch: got %q want %q", defaults.ShortcutPlayPause, DefaultShortcutPlayPause)
	}
	if defaults.ShortcutPrevious != "" || defaults.ShortcutNext != "" || defaults.ShortcutSeekBack != "" || defaults.ShortcutSeekForward != "" {
		t.Fatalf("default optional playback shortcuts should be empty: %#v", defaults)
	}
	if defaults.LyricFontFamily != DefaultLyricFontFamily {
		t.Fatalf("default LyricFontFamily mismatch: got %q want %q", defaults.LyricFontFamily, DefaultLyricFontFamily)
	}
	if defaults.LyricFontSize != DefaultLyricFontSize {
		t.Fatalf("default LyricFontSize mismatch: got %d want %d", defaults.LyricFontSize, DefaultLyricFontSize)
	}

	if err := SaveWebSettings(WebSettings{
		EmbedDownload:            true,
		DownloadToLocal:          true,
		DownloadLyricsToLocal:    true,
		DownloadDir:              "",
		DownloadFilenameTemplate: "{artist} - {name}.{ext}",
		WebDAVEnabled:            true,
		WebDAVURL:                "http://127.0.0.1:18080/dav",
		WebDAVUsername:           "test",
		WebDAVPassword:           "123456",
		WebDAVDir:                "music",
		DisableFloatingLyrics:    true,
		WebPageSize:              100,
		CliPageSize:              120,
		DownloadConcurrency:      5,
		DownloadTipDuration:      15,
		AutoCheckUpdate:          false,
		AutoSwitchInvalidSources: false,
		AutoCacheOnPlay:          true,
		FilterVersionKeywords:    true,
		VersionFilterKeywords:    "Live; Demo",
		FilterShortTracks:        true,
		NormalizeChineseVariants: true,
		UpdateRepoURL:            "https://github.com/example/fork",
		GithubProxyEnabled:       true,
		GithubProxyURL:           "https://gh-proxy.com/",
		VgChangeCover:            true,
		VgChangeAudio:            true,
		VgChangeLyric:            true,
		VgExportVideo:            true,
		ShortcutPlayPause:        DefaultShortcutPlayPause,
		ShortcutPrevious:         "Ctrl+ArrowLeft",
		ShortcutNext:             "Ctrl+ArrowRight",
		ShortcutSeekBack:         "Ctrl+Shift+ArrowLeft",
		ShortcutSeekForward:      "Ctrl+Shift+ArrowRight",
		LyricFontFamily:          "pingfang",
		LyricFontSize:            28,
	}); err != nil {
		t.Fatalf("save web settings: %v", err)
	}

	got := GetWebSettings()
	want := WebSettings{
		EmbedDownload:            true,
		DownloadToLocal:          true,
		DownloadLyricsToLocal:    true,
		DownloadDir:              normalizeWebDownloadDir(DefaultWebDownloadDir),
		DownloadFilenameTemplate: "{artist} - {name}.{ext}",
		WebDAVEnabled:            true,
		WebDAVURL:                "http://127.0.0.1:18080/dav",
		WebDAVUsername:           "test",
		WebDAVPassword:           "123456",
		WebDAVDir:                "music",
		DisableFloatingLyrics:    true,
		WebPageSize:              100,
		CliPageSize:              120,
		DownloadConcurrency:      5,
		DownloadTipDuration:      15,
		AutoCheckUpdate:          false,
		AutoSwitchInvalidSources: false,
		AutoCacheOnPlay:          true,
		FilterVersionKeywords:    true,
		VersionFilterKeywords:    "Live; Demo",
		FilterShortTracks:        true,
		NormalizeChineseVariants: true,
		UpdateRepoURL:            "https://github.com/example/fork",
		GithubProxyEnabled:       true,
		GithubProxyURL:           "https://gh-proxy.com/",
		VgChangeCover:            true,
		VgChangeAudio:            true,
		VgChangeLyric:            true,
		VgExportVideo:            true,
		ShortcutPlayPause:        DefaultShortcutPlayPause,
		ShortcutPrevious:         "Ctrl+ArrowLeft",
		ShortcutNext:             "Ctrl+ArrowRight",
		ShortcutSeekBack:         "Ctrl+Shift+ArrowLeft",
		ShortcutSeekForward:      "Ctrl+Shift+ArrowRight",
		LyricFontFamily:          "pingfang",
		LyricFontSize:            28,
		// 未配置远程签名器时保持为空 = 使用本机签名器；空 Token 保存时保留已保存的值。
		SodaSignerToken: "",
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
	if got.DownloadLyricsToLocal {
		t.Fatalf("custom save should fallback DownloadLyricsToLocal to default false: %#v", got)
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
	if got.DownloadTipDuration != DefaultDownloadTipDuration {
		t.Fatalf("custom save should fallback DownloadTipDuration to default: got %d want %d", got.DownloadTipDuration, DefaultDownloadTipDuration)
	}
	if got.AutoCheckUpdate {
		t.Fatalf("custom save should keep AutoCheckUpdate false when omitted: %#v", got)
	}
	if got.AutoSwitchInvalidSources {
		t.Fatalf("custom save should keep AutoSwitchInvalidSources false when omitted: %#v", got)
	}
	if got.AutoCacheOnPlay {
		t.Fatalf("custom save should keep AutoCacheOnPlay false when omitted: %#v", got)
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
	if got.SodaSignerURL != "" {
		t.Fatalf("custom save should keep SodaSignerURL empty when omitted: got %q", got.SodaSignerURL)
	}
	if got.SodaSignerToken != "" {
		t.Fatalf("custom save should keep SodaSignerToken empty when omitted: got %q", got.SodaSignerToken)
	}
	if got.ShortcutPlayPause != "" || got.ShortcutPrevious != "" || got.ShortcutNext != "" || got.ShortcutSeekBack != "" || got.ShortcutSeekForward != "" {
		t.Fatalf("custom save should clear omitted shortcuts: %#v", got)
	}
	if got.LyricFontFamily != DefaultLyricFontFamily || got.LyricFontSize != DefaultLyricFontSize {
		t.Fatalf("custom save should restore default lyric appearance: %#v", got)
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

func TestWebSettingsLegacyPayloadUsesCurrentSwitchDefaults(t *testing.T) {
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
	if got.AutoCacheOnPlay {
		t.Fatalf("legacy settings should default AutoCacheOnPlay to false: %#v", got)
	}
	if got.DownloadLyricsToLocal {
		t.Fatalf("legacy settings should default DownloadLyricsToLocal to false: %#v", got)
	}
	if got.ShortcutPlayPause != DefaultShortcutPlayPause || got.LyricFontFamily != DefaultLyricFontFamily || got.LyricFontSize != DefaultLyricFontSize {
		t.Fatalf("legacy settings should receive playback setting defaults: %#v", got)
	}
}

func TestWebSettingsPlaybackShortcutsAndLyricAppearance(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "data", "settings.db"))
	t.Setenv("MUSIC_DL_COOKIE_FILE", filepath.Join(baseDir, "data", "cookies.json"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)

	settings := WebSettings{
		ShortcutPlayPause:   " control + spacebar ",
		ShortcutPrevious:    "command+KeyK",
		ShortcutNext:        "pgdn",
		ShortcutSeekBack:    "Shift+ArrowLeft",
		ShortcutSeekForward: "ArrowRight",
		LyricFontFamily:     "SOURCE-HAN",
		LyricFontSize:       100,
	}
	normalized := normalizeWebSettings(settings)
	if normalized.ShortcutPlayPause != "Ctrl+Space" {
		t.Fatalf("normalized play/pause shortcut = %q, want Ctrl+Space", normalized.ShortcutPlayPause)
	}
	if normalized.ShortcutPrevious != "Meta+K" {
		t.Fatalf("normalized previous shortcut = %q, want Meta+K", normalized.ShortcutPrevious)
	}
	if normalized.ShortcutNext != "PageDown" {
		t.Fatalf("normalized next shortcut = %q, want PageDown", normalized.ShortcutNext)
	}
	if normalized.LyricFontFamily != "source-han" || normalized.LyricFontSize != 48 {
		t.Fatalf("normalized lyric appearance = %q/%d, want source-han/48", normalized.LyricFontFamily, normalized.LyricFontSize)
	}

	if err := SaveWebSettings(settings); !errors.Is(err, ErrInvalidWebSettings) {
		t.Fatalf("reserved page shortcut should be rejected, got %v", err)
	}

	settings.ShortcutNext = "Ctrl+Space"
	if err := SaveWebSettings(settings); !errors.Is(err, ErrInvalidWebSettings) {
		t.Fatalf("duplicate shortcut should be rejected, got %v", err)
	}

	settings.ShortcutNext = "F9"
	settings.LyricFontFamily = "unknown-font"
	settings.LyricFontSize = 1
	if err := SaveWebSettings(settings); err != nil {
		t.Fatalf("save valid playback settings: %v", err)
	}
	got := GetWebSettings()
	if got.ShortcutPlayPause != "Ctrl+Space" || got.ShortcutPrevious != "Meta+K" || got.ShortcutNext != "F9" {
		t.Fatalf("saved shortcuts mismatch: %#v", got)
	}
	if got.LyricFontFamily != DefaultLyricFontFamily || got.LyricFontSize != 14 {
		t.Fatalf("saved lyric appearance mismatch: got %q/%d", got.LyricFontFamily, got.LyricFontSize)
	}
}

func TestWebSettingsPreservesWebDAVPasswordWhenOmitted(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "data", "settings.db"))
	t.Setenv("MUSIC_DL_COOKIE_FILE", filepath.Join(baseDir, "data", "cookies.json"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)

	if err := SaveWebSettings(WebSettings{
		WebDAVEnabled:  true,
		WebDAVURL:      "http://127.0.0.1:18080/dav",
		WebDAVUsername: "test",
		WebDAVPassword: "secret",
		WebDAVDir:      "music",
	}); err != nil {
		t.Fatalf("save initial webdav settings: %v", err)
	}

	if err := SaveWebSettings(WebSettings{
		WebDAVEnabled: true,
		WebDAVURL:     "http://127.0.0.1:18080/dav",
	}); err != nil {
		t.Fatalf("save webdav settings without password: %v", err)
	}

	got := GetWebSettings()
	if got.WebDAVPassword != "secret" {
		t.Fatalf("WebDAVPassword should be preserved when omitted: got %q", got.WebDAVPassword)
	}
}

func TestWebSettingsSodaSignerDefaultsAndPreserve(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "data", "settings.db"))
	t.Setenv("MUSIC_DL_COOKIE_FILE", filepath.Join(baseDir, "data", "cookies.json"))
	t.Setenv("QISHUI_SIGNER_URL", "")
	t.Setenv("QISHUI_SIGNER_TOKEN", "")
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)

	defaults := GetWebSettings()
	if defaults.SodaSignerURL != "" || defaults.SodaSignerToken != "" {
		t.Fatalf("default soda signer should be empty: got %q / %q", defaults.SodaSignerURL, defaults.SodaSignerToken)
	}
	url, token := SodaSignerConfig()
	if url != "" || token != "" {
		t.Fatalf("SodaSignerConfig default should be empty: got %q / %q", url, token)
	}

	if err := SaveWebSettings(WebSettings{
		SodaSignerURL:   "http://127.0.0.1:8899/sign",
		SodaSignerToken: "self-hosted-token",
	}); err != nil {
		t.Fatalf("save custom soda signer: %v", err)
	}
	got := GetWebSettings()
	if got.SodaSignerURL != "http://127.0.0.1:8899/sign" || got.SodaSignerToken != "self-hosted-token" {
		t.Fatalf("custom soda signer mismatch: got %q / %q", got.SodaSignerURL, got.SodaSignerToken)
	}

	// 只改地址、不带 Token（界面不回显 Token）时应保留已保存的 Token。
	if err := SaveWebSettings(WebSettings{
		SodaSignerURL: "http://127.0.0.1:9999/sign",
	}); err != nil {
		t.Fatalf("save soda signer without token: %v", err)
	}
	got = GetWebSettings()
	if got.SodaSignerURL != "http://127.0.0.1:9999/sign" {
		t.Fatalf("soda signer url should update: got %q", got.SodaSignerURL)
	}
	if got.SodaSignerToken != "self-hosted-token" {
		t.Fatalf("SodaSignerToken should be preserved when omitted: got %q", got.SodaSignerToken)
	}

	// 清空地址表示回到本机签名器（没有本机客户端时才会退化为试听）。
	if err := SaveWebSettings(WebSettings{
		SodaSignerURL: "",
	}); err != nil {
		t.Fatalf("save empty soda signer url: %v", err)
	}
	got = GetWebSettings()
	if got.SodaSignerURL != "" {
		t.Fatalf("empty SodaSignerURL should fallback to local signer: got %q", got.SodaSignerURL)
	}
	if got.SodaSignerToken != "self-hosted-token" {
		t.Fatalf("SodaSignerToken should survive empty url save: got %q", got.SodaSignerToken)
	}

	// 环境变量优先于界面设置。
	t.Setenv("QISHUI_SIGNER_URL", "http://127.0.0.1:7777/sign")
	t.Setenv("QISHUI_SIGNER_TOKEN", "env-token")
	url, token = SodaSignerConfig()
	if url != "http://127.0.0.1:7777/sign" || token != "env-token" {
		t.Fatalf("env soda signer should win: got %q / %q", url, token)
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
