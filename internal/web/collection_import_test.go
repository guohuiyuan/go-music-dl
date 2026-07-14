package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/music-lib/model"
)

func initCollectionDBForTest(t *testing.T) {
	t.Helper()

	baseDir := t.TempDir()
	settingsDB := filepath.Join(baseDir, "data", "settings.db")
	legacyDB := filepath.Join(baseDir, "data", "favorites.db")

	t.Setenv("MUSIC_DL_CONFIG_DB", settingsDB)
	t.Setenv("MUSIC_DL_FAVORITES_DB", legacyDB)
	resetCollectionStateForTest()
	t.Cleanup(resetCollectionStateForTest)

	InitDB()
}

func newCollectionTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterCollectionRoutes(r.Group(RoutePrefix))
	return r
}

func TestCollectionsEndpointDefaultsToManualCollections(t *testing.T) {
	initCollectionDBForTest(t)

	manual := Collection{Name: "Manual", Kind: collectionKindManual, ContentType: collectionContentPlaylist, Source: "local"}
	imported := Collection{Name: "Imported", Kind: collectionKindImported, ContentType: collectionContentAlbum, Source: "qq", ExternalID: "album-1"}
	if err := db.Create(&manual).Error; err != nil {
		t.Fatalf("create manual collection: %v", err)
	}
	if err := db.Create(&imported).Error; err != nil {
		t.Fatalf("create imported collection: %v", err)
	}

	router := newCollectionTestRouter()

	req := httptest.NewRequest(http.MethodGet, RoutePrefix+"/collections", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /collections status = %d, want %d", rec.Code, http.StatusOK)
	}

	var collections []Collection
	if err := json.Unmarshal(rec.Body.Bytes(), &collections); err != nil {
		t.Fatalf("decode manual collections: %v", err)
	}
	if len(collections) != 1 {
		t.Fatalf("manual collections len = %d, want 1", len(collections))
	}
	if collections[0].Name != manual.Name {
		t.Fatalf("manual collections[0].Name = %q, want %q", collections[0].Name, manual.Name)
	}

	req = httptest.NewRequest(http.MethodGet, RoutePrefix+"/collections?include_imported=1", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /collections?include_imported=1 status = %d, want %d", rec.Code, http.StatusOK)
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &collections); err != nil {
		t.Fatalf("decode all collections: %v", err)
	}
	if len(collections) != 2 {
		t.Fatalf("all collections len = %d, want 2", len(collections))
	}
}

func TestImportCollectionEndpointCreatesImportedRecord(t *testing.T) {
	initCollectionDBForTest(t)
	router := newCollectionTestRouter()

	body, err := json.Marshal(importCollectionRequest{
		Name:        "QQ 精选",
		Description: "收藏的外部歌单",
		Cover:       "https://example.com/cover.jpg",
		Creator:     "QQ 音乐",
		TrackCount:  18,
		Source:      "qq",
		ExternalID:  "playlist-123",
		ContentType: collectionContentPlaylist,
	})
	if err != nil {
		t.Fatalf("marshal import request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, RoutePrefix+"/collections/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /collections/import status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode import response: %v", err)
	}

	var collection Collection
	if err := db.First(&collection, resp.ID).Error; err != nil {
		t.Fatalf("query imported collection: %v", err)
	}

	if collection.Kind != collectionKindImported {
		t.Fatalf("collection.Kind = %q, want %q", collection.Kind, collectionKindImported)
	}
	if collection.Source != "qq" {
		t.Fatalf("collection.Source = %q, want qq", collection.Source)
	}
	if collection.ContentType != collectionContentPlaylist {
		t.Fatalf("collection.ContentType = %q, want %q", collection.ContentType, collectionContentPlaylist)
	}
	if collection.ExternalID != "playlist-123" {
		t.Fatalf("collection.ExternalID = %q, want playlist-123", collection.ExternalID)
	}
}

func TestImportedCollectionSongsEndpointUsesLiveFetchAndBlocksMutations(t *testing.T) {
	initCollectionDBForTest(t)

	collection := Collection{
		Name:        "Imported Playlist",
		Kind:        collectionKindImported,
		ContentType: collectionContentPlaylist,
		Source:      "qq",
		ExternalID:  "playlist-1",
		TrackCount:  2,
	}
	if err := db.Create(&collection).Error; err != nil {
		t.Fatalf("create imported collection: %v", err)
	}

	origPlaylistDetail := playlistDetailFuncProvider
	playlistDetailFuncProvider = func(source string) func(string) ([]model.Song, error) {
		if source != "qq" {
			t.Fatalf("playlist detail source = %q, want qq", source)
		}
		return func(id string) ([]model.Song, error) {
			if id != "playlist-1" {
				t.Fatalf("playlist detail id = %q, want playlist-1", id)
			}
			return []model.Song{
				{ID: "song-1", Source: "qq", Name: "Song One", Artist: "Artist A"},
				{ID: "song-2", Source: "qq", Name: "Song Two", Artist: "Artist B"},
			}, nil
		}
	}
	t.Cleanup(func() {
		playlistDetailFuncProvider = origPlaylistDetail
	})

	router := newCollectionTestRouter()

	songsPath := fmt.Sprintf("%s/collections/%d/songs", RoutePrefix, collection.ID)

	req := httptest.NewRequest(http.MethodGet, songsPath, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want %d, body=%s", songsPath, rec.Code, http.StatusOK, rec.Body.String())
	}

	var songs []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &songs); err != nil {
		t.Fatalf("decode live songs: %v", err)
	}
	if len(songs) != 2 {
		t.Fatalf("live songs len = %d, want 2", len(songs))
	}
	if songs[0]["id"] != "song-1" {
		t.Fatalf("first live song id = %#v, want song-1", songs[0]["id"])
	}

	addBody := bytes.NewBufferString(`{"id":"song-1","source":"qq","name":"Song One"}`)
	req = httptest.NewRequest(http.MethodPost, songsPath, addBody)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST %s status = %d, want %d", songsPath, rec.Code, http.StatusBadRequest)
	}

	req = httptest.NewRequest(http.MethodDelete, songsPath+"?id=song-1&source=qq", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("DELETE %s status = %d, want %d", songsPath, rec.Code, http.StatusBadRequest)
	}
}

func TestManualCollectionSongsEndpointSupportsBatchDelete(t *testing.T) {
	initCollectionDBForTest(t)

	collection := Collection{
		Name:        "Manual Playlist",
		Kind:        collectionKindManual,
		ContentType: collectionContentPlaylist,
		Source:      "local",
	}
	if err := db.Create(&collection).Error; err != nil {
		t.Fatalf("create manual collection: %v", err)
	}

	saved := []SavedSong{
		{CollectionID: collection.ID, SongID: "song-1", Source: "qq", Name: "Song One"},
		{CollectionID: collection.ID, SongID: "song-2", Source: localMusicSource, Name: "Song Two"},
		{CollectionID: collection.ID, SongID: "song-3", Source: "netease", Name: "Song Three"},
	}
	if err := db.Create(&saved).Error; err != nil {
		t.Fatalf("create saved songs: %v", err)
	}

	router := newCollectionTestRouter()
	body := bytes.NewBufferString(`{"songs":[{"id":"song-1","source":"qq"},{"id":"song-2","source":"local"}]}`)
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("%s/collections/%d/songs", RoutePrefix, collection.ID), body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE batch songs status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var remaining []SavedSong
	if err := db.Where("collection_id = ?", collection.ID).Order("song_id ASC").Find(&remaining).Error; err != nil {
		t.Fatalf("query remaining songs: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("remaining songs len = %d, want 1", len(remaining))
	}
	if remaining[0].SongID != "song-3" {
		t.Fatalf("remaining song = %q, want song-3", remaining[0].SongID)
	}
}

func TestLoadImportedCollectionSongsFallsBackToParse(t *testing.T) {
	origPlaylistDetail := playlistDetailFuncProvider
	origParsePlaylist := parsePlaylistFuncProvider
	playlistDetailFuncProvider = func(string) func(string) ([]model.Song, error) {
		return nil
	}
	parsePlaylistFuncProvider = func(source string) func(string) (*model.Playlist, []model.Song, error) {
		if source != "qq" {
			t.Fatalf("parse playlist source = %q, want qq", source)
		}
		return func(link string) (*model.Playlist, []model.Song, error) {
			if link != "https://example.com/playlist/1" {
				t.Fatalf("parse playlist link = %q, want https://example.com/playlist/1", link)
			}
			return &model.Playlist{ID: "playlist-1"}, []model.Song{
				{ID: "song-parse", Name: "Parsed Song", Artist: "Parser"},
			}, nil
		}
	}
	t.Cleanup(func() {
		playlistDetailFuncProvider = origPlaylistDetail
		parsePlaylistFuncProvider = origParsePlaylist
	})

	songs, err := loadImportedCollectionSongs(&Collection{
		Kind:        collectionKindImported,
		ContentType: collectionContentPlaylist,
		Source:      "qq",
		ExternalID:  "playlist-1",
		Link:        "https://example.com/playlist/1",
	})
	if err != nil {
		t.Fatalf("loadImportedCollectionSongs() error = %v", err)
	}
	if len(songs) != 1 {
		t.Fatalf("parsed songs len = %d, want 1", len(songs))
	}
	if songs[0].ID != "song-parse" {
		t.Fatalf("parsed song id = %q, want song-parse", songs[0].ID)
	}
	if songs[0].Source != "qq" {
		t.Fatalf("parsed song source = %q, want qq", songs[0].Source)
	}
}
