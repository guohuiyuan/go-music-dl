package core

import (
	"encoding/json"
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
	DefaultDownloadFilenameTemplate = "{name} - {artist}"
	DefaultWebPageSize              = 50
	DefaultCLIPageSize              = 20
	DefaultWebConcurrency           = 3
	webSettingsKey                  = "web_settings"
)

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
	DownloadDir              string `json:"downloadDir"`
	DownloadFilenameTemplate string `json:"downloadFilenameTemplate"`
	DisableFloatingLyrics    bool   `json:"disableFloatingLyrics"`
	WebPageSize              int    `json:"webPageSize"`
	CliPageSize              int    `json:"cliPageSize"`
	DownloadConcurrency      int    `json:"downloadConcurrency"`
	VgChangeCover            bool   `json:"vgChangeCover"`
	VgChangeAudio            bool   `json:"vgChangeAudio"`
	VgChangeLyric            bool   `json:"vgChangeLyric"`
	VgExportVideo            bool   `json:"vgExportVideo"`
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

		if err := db.AutoMigrate(&configKV{}, &cookieEntry{}); err != nil {
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
		EmbedDownload:            false,
		DownloadToLocal:          false,
		DownloadDir:              DefaultWebDownloadDir,
		DownloadFilenameTemplate: DefaultDownloadFilenameTemplate,
		DisableFloatingLyrics:    false,
		WebPageSize:              DefaultWebPageSize,
		CliPageSize:              DefaultCLIPageSize,
		DownloadConcurrency:      DefaultWebConcurrency,
	})
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
	settings.DownloadDir = normalizeWebDownloadDir(settings.DownloadDir)
	return settings
}

func normalizeWebDownloadDir(dir string) string {
	cleaned := filepath.Clean(dir)
	if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, `\\`) {
		return cleaned
	}
	return filepath.ToSlash(cleaned)
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

	settings = normalizeWebSettings(settings)
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
