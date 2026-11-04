package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ConfigDBFile                    = "data/settings.db"
	DefaultWebDownloadDir           = "data/downloads"
	DefaultDownloadFilenameTemplate = "{artist} - {name}"
	DefaultWebAuthUsername          = "admin"
	DefaultWebPageSize              = 50
	DefaultCLIPageSize              = 20
	DefaultWebConcurrency           = 3
	DefaultDownloadTipDuration      = 8
	DefaultShortcutPlayPause        = "Space"
	DefaultLyricFontFamily          = "system"
	DefaultLyricFontSize            = 24
	DefaultUpdateRepoURL            = "https://github.com/guohuiyuan/go-music-dl"
	DefaultGithubProxyURL           = "https://edgeone.gh-proxy.com"
	webSettingsKey                  = "web_settings"
	webAuthSettingsKey              = "web_auth_settings"
)

var ErrInvalidWebSettings = errors.New("invalid web settings")

type configKV struct {
	Key       string    `gorm:"primaryKey;size:128"`
	Value     string    `gorm:"type:text;not null"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

type cookieEntry struct {
	Source    string    `gorm:"primaryKey;size:64"`
	Value     string    `gorm:"type:text;not null"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

type WebSettings struct {
	EmbedDownload            bool   `json:"embedDownload"`
	DownloadToLocal          bool   `json:"downloadToLocal"`
	DownloadLyricsToLocal    bool   `json:"downloadLyricsToLocal"`
	DownloadDir              string `json:"downloadDir"`
	DownloadFilenameTemplate string `json:"downloadFilenameTemplate"`
	DownloadTipDuration      int    `json:"downloadTipDuration"`
	WebDAVEnabled            bool   `json:"webdavEnabled"`
	WebDAVURL                string `json:"webdavUrl"`
	WebDAVUsername           string `json:"webdavUsername"`
	WebDAVPassword           string `json:"webdavPassword"`
	WebDAVDir                string `json:"webdavDir"`
	DisableFloatingLyrics    bool   `json:"disableFloatingLyrics"`
	WebPageSize              int    `json:"webPageSize"`
	CliPageSize              int    `json:"cliPageSize"`
	DownloadConcurrency      int    `json:"downloadConcurrency"`
	AutoCheckUpdate          bool   `json:"autoCheckUpdate"`
	AutoSwitchInvalidSources bool   `json:"autoSwitchInvalidSources"`
	AutoCacheOnPlay          bool   `json:"autoCacheOnPlay"`
	FilterVersionKeywords    bool   `json:"filterVersionKeywords"`
	VersionFilterKeywords    string `json:"versionFilterKeywords"`
	FilterShortTracks        bool   `json:"filterShortTracks"`
	NormalizeChineseVariants bool   `json:"normalizeChineseVariants"`
	UpdateRepoURL            string `json:"updateRepoUrl"`
	GithubProxyEnabled       bool   `json:"githubProxyEnabled"`
	GithubProxyURL           string `json:"githubProxyUrl"`
	SodaSignerURL            string `json:"sodaSignerUrl"`
	SodaSignerToken          string `json:"sodaSignerToken"`
	SodaClientDir            string `json:"sodaClientDir"`
	VgChangeCover            bool   `json:"vgChangeCover"`
	VgChangeAudio            bool   `json:"vgChangeAudio"`
	VgChangeLyric            bool   `json:"vgChangeLyric"`
	VgExportVideo            bool   `json:"vgExportVideo"`
	ShortcutPlayPause        string `json:"shortcutPlayPause"`
	ShortcutPrevious         string `json:"shortcutPrevious"`
	ShortcutNext             string `json:"shortcutNext"`
	ShortcutSeekBack         string `json:"shortcutSeekBack"`
	ShortcutSeekForward      string `json:"shortcutSeekForward"`
	LyricFontFamily          string `json:"lyricFontFamily"`
	LyricFontSize            int    `json:"lyricFontSize"`
}

type WebAuthSettings struct {
	Username      string `json:"username"`
	PasswordHash  string `json:"passwordHash"`
	SessionSecret string `json:"sessionSecret"`
}

var (
	configDB      *gorm.DB
	configInit    sync.Once
	configInitErr error
)

func configDBPath() string {
	if path := strings.TrimSpace(os.Getenv("MUSIC_DL_CONFIG_DB")); path != "" {
		return path
	}
	return ConfigDBFile
}

// ConfigDBPath returns the canonical SQLite file used by the app.
func ConfigDBPath() string {
	return configDBPath()
}

// CloseConfigDB closes the shared settings database connection. Tests and
// graceful shutdown use this to release the SQLite file handle.
func CloseConfigDB() error {
	if configDB == nil {
		return nil
	}
	sqlDB, err := configDB.DB()
	if err != nil {
		return err
	}
	closeErr := sqlDB.Close()
	configDB = nil
	configInitErr = nil
	configInit = sync.Once{}
	return closeErr
}

func legacyCookieFilePath() string {
	if path := strings.TrimSpace(os.Getenv("MUSIC_DL_COOKIE_FILE")); path != "" {
		return path
	}
	return CookieFile
}

func ensureConfigDB() error {
	configInit.Do(func() {
		dbPath := filepath.Clean(ConfigDBPath())
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			configInitErr = err
			return
		}

		db, err := gorm.Open(sqlite.Open(dbPath+"?_pragma=busy_timeout(5000)"), &gorm.Config{})
		if err != nil {
			configInitErr = err
			return
		}

		if err := db.AutoMigrate(&configKV{}, &cookieEntry{}, &DownloadRecord{}); err != nil {
			configInitErr = err
			return
		}

		configDB = db
		configInitErr = migrateLegacyCookies()
	})

	return configInitErr
}

func migrateLegacyCookies() error {
	legacyPath := filepath.Clean(legacyCookieFilePath())
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var legacy map[string]string
	if err := json.Unmarshal(data, &legacy); err != nil || len(legacy) == 0 {
		return nil
	}

	var count int64
	if err := configDB.Model(&cookieEntry{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	entries := make([]cookieEntry, 0, len(legacy))
	for source, value := range legacy {
		source = strings.TrimSpace(source)
		value = strings.TrimSpace(value)
		if source == "" || value == "" {
			continue
		}
		entries = append(entries, cookieEntry{Source: source, Value: value})
	}
	if len(entries) == 0 {
		return nil
	}

	return configDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "source"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&entries).Error
}

func defaultWebSettings() WebSettings {
	return normalizeWebSettings(WebSettings{
		EmbedDownload:            true,
		DownloadToLocal:          false,
		DownloadLyricsToLocal:    false,
		DownloadDir:              DefaultWebDownloadDir,
		DownloadFilenameTemplate: DefaultDownloadFilenameTemplate,
		DisableFloatingLyrics:    false,
		WebPageSize:              DefaultWebPageSize,
		CliPageSize:              DefaultCLIPageSize,
		DownloadConcurrency:      DefaultWebConcurrency,
		DownloadTipDuration:      DefaultDownloadTipDuration,
		AutoCheckUpdate:          true,
		AutoSwitchInvalidSources: true,
		AutoCacheOnPlay:          false,
		FilterVersionKeywords:    false,
		VersionFilterKeywords:    "Live, Remix, Demo",
		FilterShortTracks:        false,
		NormalizeChineseVariants: true,
		UpdateRepoURL:            DefaultUpdateRepoURL,
		GithubProxyEnabled:       false,
		GithubProxyURL:           DefaultGithubProxyURL,
		ShortcutPlayPause:        DefaultShortcutPlayPause,
		LyricFontFamily:          DefaultLyricFontFamily,
		LyricFontSize:            DefaultLyricFontSize,
		// 默认不配远程签名服务：soda 包会优先用本机官方客户端的 bdms.node 签名。
		SodaSignerURL:   "",
		SodaSignerToken: "",
	})
}

func defaultWebAuthSettings() WebAuthSettings {
	return WebAuthSettings{
		Username: DefaultWebAuthUsername,
	}
}

func normalizeWebSettings(settings WebSettings) WebSettings {
	settings.DownloadDir = strings.TrimSpace(settings.DownloadDir)
	if settings.DownloadDir == "" {
		settings.DownloadDir = DefaultWebDownloadDir
	}
	settings.DownloadFilenameTemplate = strings.TrimSpace(settings.DownloadFilenameTemplate)
	if settings.DownloadFilenameTemplate == "" {
		settings.DownloadFilenameTemplate = DefaultDownloadFilenameTemplate
	}
	settings.WebDAVURL = strings.TrimSpace(settings.WebDAVURL)
	settings.WebDAVUsername = strings.TrimSpace(settings.WebDAVUsername)
	settings.WebDAVDir = strings.Trim(strings.TrimSpace(settings.WebDAVDir), "/")
	if settings.WebDAVDir == "" {
		settings.WebDAVDir = DefaultWebDAVDir
	}
	if settings.WebPageSize <= 0 {
		settings.WebPageSize = DefaultWebPageSize
	}
	if settings.CliPageSize <= 0 {
		settings.CliPageSize = DefaultCLIPageSize
	}
	if settings.DownloadConcurrency <= 0 {
		settings.DownloadConcurrency = DefaultWebConcurrency
	}
	if settings.DownloadConcurrency > 5 {
		settings.DownloadConcurrency = 5
	}
	if settings.DownloadConcurrency < 1 {
		settings.DownloadConcurrency = 1
	}
	if settings.DownloadTipDuration <= 0 {
		settings.DownloadTipDuration = DefaultDownloadTipDuration
	}
	if settings.DownloadTipDuration > 60 {
		settings.DownloadTipDuration = 60
	}
	settings.VersionFilterKeywords = strings.TrimSpace(settings.VersionFilterKeywords)
	if settings.VersionFilterKeywords == "" {
		settings.VersionFilterKeywords = "Live, Remix, Demo"
	}
	if len([]rune(settings.VersionFilterKeywords)) > 240 {
		settings.VersionFilterKeywords = string([]rune(settings.VersionFilterKeywords)[:240])
	}
	settings.UpdateRepoURL = strings.TrimSpace(settings.UpdateRepoURL)
	if settings.UpdateRepoURL == "" {
		settings.UpdateRepoURL = DefaultUpdateRepoURL
	}
	settings.GithubProxyURL = strings.TrimSpace(settings.GithubProxyURL)
	if settings.GithubProxyURL == "" {
		settings.GithubProxyURL = DefaultGithubProxyURL
	}
	settings.SodaSignerURL = strings.TrimSpace(settings.SodaSignerURL)
	settings.SodaSignerToken = strings.TrimSpace(settings.SodaSignerToken)
	settings.SodaClientDir = normalizeSodaClientDir(settings.SodaClientDir)
	settings.ShortcutPlayPause = normalizeShortcutBinding(settings.ShortcutPlayPause)
	settings.ShortcutPrevious = normalizeShortcutBinding(settings.ShortcutPrevious)
	settings.ShortcutNext = normalizeShortcutBinding(settings.ShortcutNext)
	settings.ShortcutSeekBack = normalizeShortcutBinding(settings.ShortcutSeekBack)
	settings.ShortcutSeekForward = normalizeShortcutBinding(settings.ShortcutSeekForward)
	settings.LyricFontFamily = normalizeLyricFontFamily(settings.LyricFontFamily)
	if settings.LyricFontSize <= 0 {
		settings.LyricFontSize = DefaultLyricFontSize
	}
	if settings.LyricFontSize < 14 {
		settings.LyricFontSize = 14
	}
	if settings.LyricFontSize > 48 {
		settings.LyricFontSize = 48
	}
	settings.DownloadDir = normalizeWebDownloadDir(settings.DownloadDir)
	return settings
}

func normalizeLyricFontFamily(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "sans", "serif", "yahei", "pingfang", "source-han", "lxgw":
		return value
	default:
		return DefaultLyricFontFamily
	}
}

func normalizeShortcutBinding(binding string) string {
	binding = strings.TrimSpace(binding)
	if binding == "" {
		return ""
	}

	modifiers := map[string]bool{}
	key := ""
	for _, part := range strings.Split(binding, "+") {
		part = strings.TrimSpace(part)
		if part == "" {
			return ""
		}

		switch strings.ToLower(part) {
		case "ctrl", "control":
			modifiers["Ctrl"] = true
		case "alt", "option":
			modifiers["Alt"] = true
		case "shift":
			modifiers["Shift"] = true
		case "meta", "cmd", "command", "win", "super":
			modifiers["Meta"] = true
		default:
			if key != "" {
				return ""
			}
			key = normalizeShortcutKey(part)
			if key == "" {
				return ""
			}
		}
	}
	if key == "" {
		return ""
	}

	parts := make([]string, 0, 5)
	for _, modifier := range []string{"Ctrl", "Alt", "Shift", "Meta"} {
		if modifiers[modifier] {
			parts = append(parts, modifier)
		}
	}
	parts = append(parts, key)
	return strings.Join(parts, "+")
}

func normalizeShortcutKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}

	switch strings.ToLower(key) {
	case "space", "spacebar":
		return "Space"
	case "esc", "escape":
		return "Escape"
	case "enter", "return":
		return "Enter"
	case "tab":
		return "Tab"
	case "backspace":
		return "Backspace"
	case "delete", "del":
		return "Delete"
	case "insert", "ins":
		return "Insert"
	case "home":
		return "Home"
	case "end":
		return "End"
	case "pageup", "pgup":
		return "PageUp"
	case "pagedown", "pgdn":
		return "PageDown"
	case "arrowleft", "left":
		return "ArrowLeft"
	case "arrowright", "right":
		return "ArrowRight"
	case "arrowup", "up":
		return "ArrowUp"
	case "arrowdown", "down":
		return "ArrowDown"
	case "mediaplaypause":
		return "MediaPlayPause"
	case "mediatracknext":
		return "MediaTrackNext"
	case "mediatrackprevious":
		return "MediaTrackPrevious"
	case "mediastop":
		return "MediaStop"
	}

	if len(key) == 4 && strings.EqualFold(key[:3], "key") {
		r := key[3]
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return strings.ToUpper(key[3:])
		}
	}
	if len(key) == 6 && strings.EqualFold(key[:5], "digit") {
		r := key[5]
		if r >= '0' && r <= '9' {
			return key[5:]
		}
	}
	if len(key) == 1 {
		return strings.ToUpper(key)
	}
	if len(key) <= 32 {
		return key
	}
	return ""
}

func validateWebSettings(settings WebSettings) error {
	shortcuts := []struct {
		name  string
		value string
	}{
		{"play/pause", settings.ShortcutPlayPause},
		{"previous track", settings.ShortcutPrevious},
		{"next track", settings.ShortcutNext},
		{"seek back", settings.ShortcutSeekBack},
		{"seek forward", settings.ShortcutSeekForward},
	}

	seen := map[string]string{}
	for _, shortcut := range shortcuts {
		if shortcut.value == "" {
			continue
		}
		parts := strings.Split(shortcut.value, "+")
		key := parts[len(parts)-1]
		if key == "PageUp" || key == "PageDown" {
			return fmt.Errorf("%w: %s is reserved for list pagination", ErrInvalidWebSettings, key)
		}
		if previous, exists := seen[shortcut.value]; exists {
			return fmt.Errorf("%w: shortcut %q is used by both %s and %s", ErrInvalidWebSettings, shortcut.value, previous, shortcut.name)
		}
		seen[shortcut.value] = shortcut.name
	}
	return nil
}

func normalizeWebAuthSettings(settings WebAuthSettings) WebAuthSettings {
	settings.Username = strings.TrimSpace(settings.Username)
	if settings.Username == "" {
		settings.Username = DefaultWebAuthUsername
	}
	settings.PasswordHash = strings.TrimSpace(settings.PasswordHash)
	settings.SessionSecret = strings.TrimSpace(settings.SessionSecret)
	return settings
}

func normalizeWebDownloadDir(dir string) string {
	cleaned := filepath.Clean(dir)
	if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, `\\`) {
		return cleaned
	}
	return filepath.ToSlash(cleaned)
}

func normalizeSodaClientDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if len(dir) >= 2 {
		first, last := dir[0], dir[len(dir)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			dir = strings.TrimSpace(dir[1 : len(dir)-1])
		}
	}
	if dir == "" {
		return ""
	}
	return filepath.Clean(dir)
}

func GetWebSettings() WebSettings {
	settings := defaultWebSettings()
	if err := ensureConfigDB(); err != nil {
		return settings
	}

	var row configKV
	if err := configDB.Where("key = ?", webSettingsKey).Limit(1).Find(&row).Error; err != nil {
		return settings
	}
	if row.Key == "" {
		return settings
	}

	if err := json.Unmarshal([]byte(row.Value), &settings); err != nil {
		return defaultWebSettings()
	}
	return normalizeWebSettings(settings)
}

func SaveWebSettings(settings WebSettings) error {
	if err := ensureConfigDB(); err != nil {
		return err
	}

	if strings.TrimSpace(settings.WebDAVPassword) == "" {
		settings.WebDAVPassword = GetWebSettings().WebDAVPassword
	}
	if strings.TrimSpace(settings.SodaSignerToken) == "" {
		settings.SodaSignerToken = GetWebSettings().SodaSignerToken
	}
	settings = normalizeWebSettings(settings)
	if err := validateWebSettings(settings); err != nil {
		return err
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	return configDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&configKV{
		Key:   webSettingsKey,
		Value: string(data),
	}).Error
}

func GetWebAuthSettings() (WebAuthSettings, error) {
	settings := defaultWebAuthSettings()
	if err := ensureConfigDB(); err != nil {
		return settings, err
	}

	var row configKV
	if err := configDB.Where("key = ?", webAuthSettingsKey).Limit(1).Find(&row).Error; err != nil {
		return settings, err
	}
	if row.Key == "" {
		return settings, nil
	}

	if err := json.Unmarshal([]byte(row.Value), &settings); err != nil {
		return defaultWebAuthSettings(), err
	}
	return normalizeWebAuthSettings(settings), nil
}

func SaveWebAuthSettings(settings WebAuthSettings) error {
	if err := ensureConfigDB(); err != nil {
		return err
	}

	settings = normalizeWebAuthSettings(settings)
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	return configDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&configKV{
		Key:   webAuthSettingsKey,
		Value: string(data),
	}).Error
}
