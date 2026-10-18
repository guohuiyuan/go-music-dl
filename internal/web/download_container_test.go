package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
	"gorm.io/gorm"
)

func setupContainerTests(t *testing.T) (*gin.Engine, *core.DownloadQueue) {
	t.Helper()
	core.CloseConfigDB()
	initCollectionDBForTest(t)
	t.Cleanup(func() { core.CloseConfigDB() })
	q, err := core.OpenDownloadQueue()
	if err != nil {
		t.Fatal(err)
	}
	r := newCollectionTestRouter()
	RegisterDownloadQueueRoutes(r.Group(RoutePrefix), q)
	return r, q
}
func containerRequest(r *gin.Engine, path string, body interface{}) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, RoutePrefix+path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func TestDownloadWholePlaylistHasNoPageOr2000SongLimit(t *testing.T) {
	r, q := setupContainerTests(t)
	original := playlistDetailFuncProvider
	t.Cleanup(func() { playlistDetailFuncProvider = original })
	playlistDetailFuncProvider = func(source string) func(string) ([]model.Song, error) {
		return func(id string) ([]model.Song, error) {
			if source != "kuwo" || id != "large" {
				t.Fatalf("unexpected container %s %s", source, id)
			}
			songs := make([]model.Song, 2205)
			for i := range songs {
				songs[i] = model.Song{ID: fmt.Sprint(i), Name: fmt.Sprintf("Song %d", i)}
			}
			return songs, nil
		}
	}
	req := downloadContainerRequest{RequestID: "whole", Kind: "playlist", ID: "large", Source: "kuwo", Embed: true}
	w := containerRequest(r, "/api/downloads/queue/container", req)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct{ Added, Total int }
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.Added != 2205 || result.Total != 2205 {
		t.Fatal(w.Body.String())
	}
	tasks, total, _, err := q.List(45, 50, "")
	if err != nil || total != 2205 || len(tasks) != 5 || tasks[4].Name != "Song 2204" || tasks[4].Source != "kuwo" {
		t.Fatal(total, tasks, err)
	}
	w = containerRequest(r, "/api/downloads/queue/container", req)
	if w.Code != 202 || !bytes.Contains(w.Body.Bytes(), []byte(`"added":0`)) {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestDownloadWholeAlbumAndImportedLocalAlbum(t *testing.T) {
	r, q := setupContainerTests(t)
	original := albumDetailFuncProvider
	t.Cleanup(func() { albumDetailFuncProvider = original })
	albumDetailFuncProvider = func(string) func(string) ([]model.Song, error) {
		return func(id string) ([]model.Song, error) {
			return []model.Song{{ID: id + "-1", Name: "Track 1", Album: "Album", Extra: map[string]string{"album_id": id}}, {ID: id + "-2", Name: "Track 2", Album: "Album"}}, nil
		}
	}
	w := containerRequest(r, "/api/downloads/queue/container", downloadContainerRequest{RequestID: "album", Kind: "album", ID: "album1", Source: "qq"})
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	imported := Collection{Name: "Imported Album", Kind: collectionKindImported, ContentType: collectionContentAlbum, Source: "qq", ExternalID: "album2"}
	if err := db.Create(&imported).Error; err != nil {
		t.Fatal(err)
	}
	w = containerRequest(r, "/api/downloads/queue/container", downloadContainerRequest{RequestID: "imported", Kind: "collection", ID: fmt.Sprint(imported.ID)})
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	_, total, _, err := q.List(1, 50, "")
	if err != nil || total != 4 {
		t.Fatal(total, err)
	}
}
func TestDownloadManualCollectionSkipsLocalAudioAndIncludesAllPages(t *testing.T) {
	r, q := setupContainerTests(t)
	col := Collection{Name: "Manual"}
	if err := db.Create(&col).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 125; i++ {
		if err := db.Create(&SavedSong{CollectionID: col.ID, SongID: fmt.Sprint(i), Source: "kuwo", Name: fmt.Sprint(i)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	db.Create(&SavedSong{CollectionID: col.ID, SongID: "existing-local", Source: "local", Name: "Local"})
	w := containerRequest(r, "/api/downloads/queue/container", downloadContainerRequest{RequestID: "manual", Kind: "collection", ID: fmt.Sprint(col.ID)})
	if w.Code != 202 || !bytes.Contains(w.Body.Bytes(), []byte(`"skipped_local":1`)) {
		t.Fatal(w.Code, w.Body.String())
	}
	_, total, _, err := q.List(1, 50, "")
	if err != nil || total != 125 {
		t.Fatal(total, err)
	}
}
func TestDownloadContainerUpstreamErrorDoesNotEnqueuePartialResults(t *testing.T) {
	originalParser := parsePlaylistFuncProvider
	parsePlaylistFuncProvider = func(string) func(string) (*model.Playlist, []model.Song, error) { return nil }
	t.Cleanup(func() { parsePlaylistFuncProvider = originalParser })

	r, q := setupContainerTests(t)
	original := playlistDetailFuncProvider
	t.Cleanup(func() { playlistDetailFuncProvider = original })
	playlistDetailFuncProvider = func(string) func(string) ([]model.Song, error) {
		return func(string) ([]model.Song, error) {
			return []model.Song{{ID: "partial", Source: "kuwo"}}, errors.New("upstream error")
		}
	}
	w := containerRequest(r, "/api/downloads/queue/container", downloadContainerRequest{RequestID: "failed", Kind: "playlist", ID: "playlist", Source: "kuwo"})
	if w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	_, total, _, err := q.List(1, 50, "")
	if err != nil || total != 0 {
		t.Fatal(total, err)
	}
}
func TestDeleteCollectionRemovesReferencesAndKeepsFilesAndQueue(t *testing.T) {
	r, q := setupContainerTests(t)
	col := Collection{Name: "Disposable"}
	db.Create(&col)
	db.Create(&SavedSong{CollectionID: col.ID, SongID: "song", Source: "kuwo", Name: "Song"})
	q.Enqueue("saved-queue", []core.DownloadTaskInput{{Song: model.Song{ID: "song", Source: "kuwo", Name: "Song"}, Directory: t.TempDir()}})
	audio := filepath.Join(t.TempDir(), "audio.mp3")
	os.WriteFile(audio, []byte("audio"), 0644)
	deleteRequest := func(id, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodDelete, RoutePrefix+"/collections/"+id, nil)
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	if w := deleteRequest(fmt.Sprint(col.ID), "https://other.example"); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := deleteRequest("invalid", ""); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w := deleteRequest(fmt.Sprint(col.ID), ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var count int64
	db.Model(&SavedSong{}).Where("collection_id = ?", col.ID).Count(&count)
	if count != 0 {
		t.Fatal("orphan saved songs", count)
	}
	db.Model(&Collection{}).Where("id = ?", col.ID).Count(&count)
	if count != 0 {
		t.Fatal("collection still present")
	}
	_, total, _, err := q.List(1, 50, "")
	if err != nil || total != 1 {
		t.Fatal("queue modified", total, err)
	}
	if data, err := os.ReadFile(audio); err != nil || string(data) != "audio" {
		t.Fatal("audio changed", err)
	}
	if w := deleteRequest(fmt.Sprint(col.ID), ""); w.Code != 404 {
		t.Fatal(w.Code)
	}
}
func TestDeleteCollectionRollsBackSavedSongsOnFailure(t *testing.T) {
	r, _ := setupContainerTests(t)
	col := Collection{Name: "Rollback"}
	db.Create(&col)
	db.Create(&SavedSong{CollectionID: col.ID, SongID: "keep", Source: "kuwo"})
	name := "test:fail_collection_delete"
	db.Callback().Delete().Before("gorm:delete").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "collections" {
			tx.AddError(errors.New("simulated failure"))
		}
	})
	t.Cleanup(func() { db.Callback().Delete().Remove(name) })
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("%s/collections/%d", RoutePrefix, col.ID), nil)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 500 {
		t.Fatal(w.Code, w.Body.String())
	}
	var count int64
	db.Model(&SavedSong{}).Where("collection_id = ?", col.ID).Count(&count)
	if count != 1 {
		t.Fatal("saved songs not restored", count)
	}
}

func TestWholeDownloadButtonsRenderOnCardsAndDetail(t *testing.T) {
	for _, kind := range []string{"playlist", "album"} {
		t.Run(kind, func(t *testing.T) {
			r, _ := setupContainerTests(t)
			r.SetHTMLTemplate(newTestTemplate(t))
			r.GET("/test-card", func(c *gin.Context) {
				renderIndex(c, nil, []model.Playlist{{ID: "remote1", Source: "kuwo", Name: "Container", TrackCount: 125}}, "", []string{"kuwo"}, "", kind, "", "", "", false, "", nil)
			})
			r.GET("/test-detail", func(c *gin.Context) {
				renderIndex(c, []model.Song{{ID: "s1", Source: "kuwo", Name: "Song"}}, nil, "", []string{"kuwo"}, "", kind, "", "", "", false, "", &importCollectionMeta{Enabled: true, ContentType: kind, ExternalID: "remote1", Source: "kuwo"})
			})
			for _, path := range []string{"/test-card", "/test-detail"} {
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
				if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`data-container-kind="`+kind+`"`)) || !bytes.Contains(w.Body.Bytes(), []byte(`data-container-id="remote1"`)) {
					t.Fatal(path, w.Code, w.Body.String())
				}
			}
		})
	}
}
