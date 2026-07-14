package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guohuiyuan/go-music-dl/core"
)

func TestLocalMusicIndexSyncUpsertsAndSweeps(t *testing.T) {
	initCollectionDBForTest(t)

	downloadDir := t.TempDir()
	withLocalMusicDownloadDir(t, downloadDir)

	keepPath := filepath.Join(downloadDir, "Keep Me.mp3")
	dropPath := filepath.Join(downloadDir, "Drop Me.mp3")
	if err := os.WriteFile(keepPath, []byte("keep"), 0644); err != nil {
		t.Fatalf("write keep: %v", err)
	}
	if err := os.WriteFile(dropPath, []byte("drop"), 0644); err != nil {
		t.Fatalf("write drop: %v", err)
	}

	if err := syncLocalMusicIndex(); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	var count int64
	db.Model(&LocalMusicIndex{}).Count(&count)
	if count != 2 {
		t.Fatalf("index count after first sync = %d, want 2", count)
	}

	// Remove one file; next sync should sweep its row.
	if err := os.Remove(dropPath); err != nil {
		t.Fatalf("remove drop: %v", err)
	}
	if err := syncLocalMusicIndex(); err != nil {
		t.Fatalf("second sync: %v", err)
	}

	db.Model(&LocalMusicIndex{}).Count(&count)
	if count != 1 {
		t.Fatalf("index count after sweep = %d, want 1", count)
	}

	dropID := encodeLocalMusicID("Drop Me.mp3")
	var dropCount int64
	db.Model(&LocalMusicIndex{}).Where("id = ?", dropID).Count(&dropCount)
	if dropCount != 0 {
		t.Fatalf("swept row still present for %s", dropID)
	}
}

func TestLocalMusicSearchSongsMatchesAndExcludesDeleted(t *testing.T) {
	initCollectionDBForTest(t)

	downloadDir := t.TempDir()
	withLocalMusicDownloadDir(t, downloadDir)

	songPath := filepath.Join(downloadDir, "Hello World.mp3")
	if err := os.WriteFile(songPath, []byte("hello"), 0644); err != nil {
		t.Fatalf("write song: %v", err)
	}
	if err := syncLocalMusicIndex(); err != nil {
		t.Fatalf("sync: %v", err)
	}

	// Case-insensitive match on name.
	got := localMusicSearchSongs("hello", 50)
	if len(got) != 1 || got[0].Source != localMusicSource {
		t.Fatalf("search = %+v, want 1 local song", got)
	}

	// No keyword match -> empty.
	if res := localMusicSearchSongs("nonexistent-keyword", 50); len(res) != 0 {
		t.Fatalf("search for missing keyword = %+v, want empty", res)
	}

	// Delete the file on disk but leave the row; search must os.Stat-guard it out.
	if err := os.Remove(songPath); err != nil {
		t.Fatalf("remove song: %v", err)
	}
	if res := localMusicSearchSongs("hello", 50); len(res) != 0 {
		t.Fatalf("search after file delete = %+v, want empty (os.Stat guard)", res)
	}
	// And the guard should have removed the stale row.
	var count int64
	db.Model(&LocalMusicIndex{}).Count(&count)
	if count != 0 {
		t.Fatalf("stale row not removed by search guard, count = %d", count)
	}
}

func TestSearchRouteIncludesLocalSourceForSongType(t *testing.T) {
	initCollectionDBForTest(t)

	downloadDir := t.TempDir()
	withLocalMusicDownloadDir(t, downloadDir)
	if err := os.WriteFile(filepath.Join(downloadDir, "Local Hit.mp3"), []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := syncLocalMusicIndex(); err != nil {
		t.Fatalf("sync: %v", err)
	}

	songs := localMusicSearchSongs("Local Hit", 50)
	if len(songs) != 1 {
		t.Fatalf("index search = %d, want 1", len(songs))
	}

	if !containsLocalSource([]string{"netease", "local"}) {
		t.Fatal("containsLocalSource should detect local")
	}
	if containsLocalSource([]string{"netease", "qq"}) {
		t.Fatal("containsLocalSource should be false without local")
	}
}

func TestLocalCollectionSearchPlaylists(t *testing.T) {
	initCollectionDBForTest(t)

	cols := []Collection{
		{Name: "我的摇滚", Kind: collectionKindManual, ContentType: collectionContentPlaylist, Source: "local"},
		{Name: "爵士精选", Kind: collectionKindManual, ContentType: collectionContentPlaylist, Source: "local"},
	}
	if err := db.Create(&cols).Error; err != nil {
		t.Fatalf("create collections: %v", err)
	}

	got := localCollectionSearchPlaylists("摇滚")
	if len(got) != 1 || got[0].Name != "我的摇滚" || got[0].Source != "local" {
		t.Fatalf("search = %+v, want single 我的摇滚 local playlist", got)
	}

	if res := localCollectionSearchPlaylists("不存在"); len(res) != 0 {
		t.Fatalf("search miss = %+v, want empty", res)
	}
}

func TestLocalPlaylistSupportedInRenderIndex(t *testing.T) {
	if !containsStringValue(core.GetAllSourceNames(), "local") {
		t.Fatal("local missing from all sources")
	}
	// 歌单模式下 local 复选框应可用：renderIndex 显式开启 PlaylistSupported[local]。
	content, err := templateFS.ReadFile("templates/partials/search_box.html")
	if err != nil {
		t.Fatalf("read search_box: %v", err)
	}
	if !strings.Contains(string(content), "data-playlist-supported") {
		t.Fatal("search box missing data-playlist-supported wiring")
	}
}

func TestBatchAddSongsEndpoint(t *testing.T) {
	initCollectionDBForTest(t)

	col := Collection{Name: "Mix", Kind: collectionKindManual, ContentType: collectionContentPlaylist, Source: "local"}
	if err := db.Create(&col).Error; err != nil {
		t.Fatalf("create collection: %v", err)
	}

	router := newLocalMusicTestRouter()

	payload := map[string]any{
		"songs": []map[string]any{
			{"id": "111", "source": "netease", "name": "A"},
			{"id": "222", "source": "qq", "name": "B"},
			{"id": "", "source": "qq", "name": "bad"}, // failed: missing id
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, RoutePrefix+"/collections/"+collectionIDString(col.ID)+"/songs/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("batch add status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Added     int `json:"added"`
		Duplicate int `json:"duplicate"`
		Failed    int `json:"failed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Added != 2 || resp.Failed != 1 {
		t.Fatalf("resp = %+v, want added=2 failed=1", resp)
	}

	// Re-adding the two valid songs -> duplicates.
	payload2 := map[string]any{"songs": []map[string]any{
		{"id": "111", "source": "netease", "name": "A"},
		{"id": "222", "source": "qq", "name": "B"},
	}}
	body2, _ := json.Marshal(payload2)
	req = httptest.NewRequest(http.MethodPost, RoutePrefix+"/collections/"+collectionIDString(col.ID)+"/songs/batch", bytes.NewReader(body2))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode 2: %v", err)
	}
	if resp.Added != 0 || resp.Duplicate != 2 {
		t.Fatalf("resp 2 = %+v, want added=0 duplicate=2", resp)
	}
}

func TestBatchAddLocalMusicEndpoint(t *testing.T) {
	initCollectionDBForTest(t)

	downloadDir := t.TempDir()
	withLocalMusicDownloadDir(t, downloadDir)

	idA := writeLocalTrackForTest(t, downloadDir, "A.mp3")
	idB := writeLocalTrackForTest(t, downloadDir, "B.mp3")

	col := Collection{Name: "Fav", Kind: collectionKindManual, ContentType: collectionContentPlaylist, Source: "local"}
	if err := db.Create(&col).Error; err != nil {
		t.Fatalf("create collection: %v", err)
	}

	router := newLocalMusicTestRouter()

	body, _ := json.Marshal(map[string][]string{"ids": {idA, idB}})
	req := httptest.NewRequest(http.MethodPost, RoutePrefix+"/collections/"+collectionIDString(col.ID)+"/local_music/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("batch add status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Added     int `json:"added"`
		Duplicate int `json:"duplicate"`
		Failed    int `json:"failed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode resp: %v", err)
	}
	if resp.Added != 2 || resp.Failed != 0 {
		t.Fatalf("first batch resp = %+v, want added=2 failed=0", resp)
	}

	// Re-adding the same ids should be all duplicates.
	req = httptest.NewRequest(http.MethodPost, RoutePrefix+"/collections/"+collectionIDString(col.ID)+"/local_music/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode resp 2: %v", err)
	}
	if resp.Added != 0 || resp.Duplicate != 2 {
		t.Fatalf("second batch resp = %+v, want added=0 duplicate=2", resp)
	}
}

func TestLocalRegisteredAsSearchSource(t *testing.T) {
	all := core.GetAllSourceNames()
	if !containsStringValue(all, "local") {
		t.Fatalf("GetAllSourceNames missing local: %v", all)
	}
	if core.GetSourceDescription("local") != "本地音乐" {
		t.Fatalf("GetSourceDescription(local) = %q", core.GetSourceDescription("local"))
	}
	if containsStringValue(core.GetDefaultSourceNames(), "local") {
		t.Fatal("local should be OFF by default")
	}
	if containsStringValue(core.GetPlaylistSourceNames(), "local") {
		t.Fatal("local should not be a playlist source")
	}
	if isSwitchSourceAllowed("local", "netease") {
		t.Fatal("switch-source must never target local")
	}
}

func writeLocalTrackForTest(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("audio"), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return encodeLocalMusicID(name)
}

func collectionIDString(id uint) string {
	return url.PathEscape(uintToString(id))
}

func uintToString(id uint) string {
	if id == 0 {
		return "0"
	}
	digits := []byte{}
	for id > 0 {
		digits = append([]byte{byte('0' + id%10)}, digits...)
		id /= 10
	}
	return string(digits)
}

func containsStringValue(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
