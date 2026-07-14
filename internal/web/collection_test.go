package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func resetCollectionStateForTest() {
	if db != nil {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	db = nil
}

func TestInitDBUsesUnifiedSettingsDatabase(t *testing.T) {
	baseDir := t.TempDir()
	settingsDB := filepath.Join(baseDir, "data", "settings.db")
	legacyDB := filepath.Join(baseDir, "data", "favorites.db")

	t.Setenv("MUSIC_DL_CONFIG_DB", settingsDB)
	t.Setenv("MUSIC_DL_FAVORITES_DB", legacyDB)
	resetCollectionStateForTest()
	t.Cleanup(resetCollectionStateForTest)

	InitDB()

	record := Collection{Name: "Unified DB"}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("create collection: %v", err)
	}

	if _, err := os.Stat(settingsDB); err != nil {
		t.Fatalf("expected unified sqlite db to exist: %v", err)
	}
	if _, err := os.Stat(legacyDB); !os.IsNotExist(err) {
		t.Fatalf("expected legacy favorites db to stay unused, stat err: %v", err)
	}

	sqlDB, err := gorm.Open(sqlite.Open(settingsDB), &gorm.Config{})
	if err != nil {
		t.Fatalf("open unified sqlite db: %v", err)
	}
	sqliteDB, err := sqlDB.DB()
	if err == nil {
		defer sqliteDB.Close()
	}

	var count int64
	if err := sqlDB.Model(&Collection{}).Count(&count).Error; err != nil {
		t.Fatalf("count collections: %v", err)
	}
	if count != 1 {
		t.Fatalf("unexpected collection count in unified db: got %d want 1", count)
	}
}

func TestInitDBMigratesLegacyFavoritesIntoUnifiedDatabase(t *testing.T) {
	baseDir := t.TempDir()
	settingsDB := filepath.Join(baseDir, "data", "settings.db")
	legacyDBPath := filepath.Join(baseDir, "data", "favorites.db")

	t.Setenv("MUSIC_DL_CONFIG_DB", settingsDB)
	t.Setenv("MUSIC_DL_FAVORITES_DB", legacyDBPath)
	resetCollectionStateForTest()
	t.Cleanup(resetCollectionStateForTest)

	if err := os.MkdirAll(filepath.Dir(legacyDBPath), 0755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}

	legacyDB, err := gorm.Open(sqlite.Open(legacyDBPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	legacySQLDB, err := legacyDB.DB()
	if err != nil {
		t.Fatalf("legacy db handle: %v", err)
	}
	if err := legacyDB.AutoMigrate(&Collection{}, &SavedSong{}); err != nil {
		t.Fatalf("migrate legacy db: %v", err)
	}

	collection := Collection{ID: 7, Name: "Migrated Playlist"}
	if err := legacyDB.Create(&collection).Error; err != nil {
		t.Fatalf("insert legacy collection: %v", err)
	}
	savedSong := SavedSong{
		CollectionID: collection.ID,
		SongID:       "song-1",
		Source:       "qq",
		Name:         "Track",
		Artist:       "Artist",
	}
	if err := legacyDB.Create(&savedSong).Error; err != nil {
		t.Fatalf("insert legacy song: %v", err)
	}
	if err := legacySQLDB.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	InitDB()

	var migratedCollection Collection
	if err := db.First(&migratedCollection, collection.ID).Error; err != nil {
		t.Fatalf("query migrated collection: %v", err)
	}
	if migratedCollection.Name != collection.Name {
		t.Fatalf("migrated collection mismatch: got %q want %q", migratedCollection.Name, collection.Name)
	}

	var songs []SavedSong
	if err := db.Where("collection_id = ?", collection.ID).Find(&songs).Error; err != nil {
		t.Fatalf("query migrated songs: %v", err)
	}
	if len(songs) != 1 {
		t.Fatalf("unexpected migrated song count: got %d want 1", len(songs))
	}
	if songs[0].SongID != savedSong.SongID || songs[0].Source != savedSong.Source {
		t.Fatalf("migrated song mismatch: got %#v want song_id=%q source=%q", songs[0], savedSong.SongID, savedSong.Source)
	}
	if _, err := os.Stat(legacyDBPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy favorites db to be removed after migration, stat err: %v", err)
	}
}
