package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

func withLocalMusicDownloadDir(t *testing.T, dir string) {
	t.Helper()

	original := localMusicDownloadDirProvider
	localMusicDownloadDirProvider = func() string {
		return dir
	}
	t.Cleanup(func() {
		localMusicDownloadDirProvider = original
	})
}

func newLocalMusicTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	group := r.Group(RoutePrefix)
	RegisterMusicRoutes(group, group)
	RegisterCollectionRoutes(group)
	RegisterLocalMusicRoutes(group)
	return r
}

func newAutoCacheHTTPRequest(body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "http://music.test"+RoutePrefix+"/local_music/auto_cache", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", "http://music.test")
	return req
}

func withAutoCacheSettings(t *testing.T, settings core.WebSettings) {
	t.Helper()

	original := autoCacheSettingsProvider
	autoCacheSettingsProvider = func() core.WebSettings { return settings }
	t.Cleanup(func() {
		autoCacheSettingsProvider = original
	})
}

func waitForAutoCacheIdle(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		autoCacheMu.Lock()
		idle := len(autoCacheInFlight) == 0
		autoCacheMu.Unlock()
		if idle {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for auto-cache worker")
		}
		time.Sleep(time.Millisecond)
	}
}

func waitForLocalMusicScanRefresh(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		localMusicScanRefreshMu.Lock()
		refreshing := localMusicScanRefreshInFlight
		localMusicScanRefreshMu.Unlock()
		if !refreshing {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for local music scan refresh")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestSearchBoxTemplateShowsLocalMusicEntryNextToCollections(t *testing.T) {
	content, err := templateFS.ReadFile("templates/partials/search_box.html")
	if err != nil {
		t.Fatalf("ReadFile(search_box.html): %v", err)
	}

	html := string(content)
	if !strings.Contains(html, `onclick="openCollectionManager()"`) {
		t.Fatal("search box missing custom collection entry")
	}
	if !strings.Contains(html, `onclick="openLocalMusicPage()"`) {
		t.Fatal("search box missing local music page entry")
	}
	if !strings.Contains(html, `onclick="goToPlaylistCategories()"`) {
		t.Fatal("search box missing playlist categories entry")
	}
	if strings.Index(html, `onclick="openLocalMusicPage()"`) < strings.Index(html, `onclick="openCollectionManager()"`) {
		t.Fatal("local music entry should be placed to the right of custom collection entry")
	}
	if strings.Index(html, `onclick="goToPlaylistCategories()"`) < strings.Index(html, `onclick="openLocalMusicPage()"`) {
		t.Fatal("playlist categories entry should be placed to the right of local music entry")
	}
	if !strings.Contains(html, "本地音乐") {
		t.Fatal("search box missing local music label")
	}
	if !strings.Contains(html, "歌单分类") {
		t.Fatal("search box missing playlist categories label")
	}

	playlistGrid, err := templateFS.ReadFile("templates/partials/playlist_grid.html")
	if err != nil {
		t.Fatalf("ReadFile(playlist_grid.html): %v", err)
	}
	if strings.Contains(string(playlistGrid), `onclick="openLocalMusicModal()"`) {
		t.Fatal("local music entry should not be inside custom collection page header")
	}
}

func TestLocalMusicListScansDownloadDirWithFallbacks(t *testing.T) {
	initCollectionDBForTest(t)

	downloadDir := t.TempDir()
	withLocalMusicDownloadDir(t, downloadDir)

	audioPath := filepath.Join(downloadDir, "Plain Track.mp3")
	if err := os.WriteFile(audioPath, []byte("not a real mp3, but has a supported extension"), 0644); err != nil {
		t.Fatalf("write local audio: %v", err)
	}

	collection := Collection{
		Name:        "Local",
		Kind:        collectionKindManual,
		ContentType: collectionContentPlaylist,
		Source:      "local",
	}
	if err := db.Create(&collection).Error; err != nil {
		t.Fatalf("create collection: %v", err)
	}

	localID := encodeLocalMusicID("Plain Track.mp3")
	if err := db.Create(&SavedSong{
		CollectionID: collection.ID,
		SongID:       localID,
		Source:       localMusicSource,
		Name:         "Plain Track",
		Artist:       "未知歌手",
	}).Error; err != nil {
		t.Fatalf("create saved local song: %v", err)
	}

	router := newLocalMusicTestRouter()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/local_music?collection_id=%d", RoutePrefix, collection.ID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /local_music status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Exists bool              `json:"exists"`
		Tracks []localMusicTrack `json:"tracks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode local music response: %v", err)
	}
	if !resp.Exists {
		t.Fatal("local music response exists = false, want true")
	}
	if len(resp.Tracks) != 1 {
		t.Fatalf("local music tracks len = %d, want 1", len(resp.Tracks))
	}

	track := resp.Tracks[0]
	if track.ID != localID {
		t.Fatalf("track.ID = %q, want %q", track.ID, localID)
	}
	if track.Name != "Plain Track" {
		t.Fatalf("track.Name = %q, want Plain Track", track.Name)
	}
	if track.Artist != "未知歌手" {
		t.Fatalf("track.Artist = %q, want 未知歌手", track.Artist)
	}
	if !track.AlreadyAdded {
		t.Fatal("track.AlreadyAdded = false, want true")
	}
	if track.Source != localMusicSource {
		t.Fatalf("track.Source = %q, want %q", track.Source, localMusicSource)
	}
}

func TestApplyLocalProbeResultFillsMetadata(t *testing.T) {
	track := &localMusicTrack{
		Name:     "file-name",
		Artist:   "unknown",
		Album:    "",
		Duration: 0,
		Missing:  []string{"title", "artist", "album"},
		Extra:    map[string]string{},
	}

	applyLocalProbeResult(track, &localProbeResult{
		Duration: 186,
		Bitrate:  320,
		Title:    "Probe Title",
		Artist:   "Probe Artist",
		Album:    "Probe Album",
	})

	if track.Duration != 186 {
		t.Fatalf("track.Duration = %d, want 186", track.Duration)
	}
	if track.Name != "Probe Title" {
		t.Fatalf("track.Name = %q, want Probe Title", track.Name)
	}
	if track.Artist != "Probe Artist" {
		t.Fatalf("track.Artist = %q, want Probe Artist", track.Artist)
	}
	if track.Album != "Probe Album" {
		t.Fatalf("track.Album = %q, want Probe Album", track.Album)
	}
	if len(track.Missing) != 0 {
		t.Fatalf("track.Missing = %v, want empty", track.Missing)
	}
	if track.Extra["duration"] != "186" || track.Extra["bitrate"] != "320" {
		t.Fatalf("track.Extra = %v, want duration and bitrate", track.Extra)
	}
}

func TestLocalMusicPageRendersSongListWithoutUnsupportedActions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.SetHTMLTemplate(newTestTemplate(t))
	router.GET(RoutePrefix, func(c *gin.Context) {
		renderIndex(c, []model.Song{
			{
				ID:       encodeLocalMusicID("Local Track.mp3"),
				Source:   localMusicSource,
				Name:     "Local Track",
				Artist:   "Local Artist",
				Album:    "Local Album",
				Duration: 125,
				Cover:    RoutePrefix + "/local_music/cover?id=" + url.QueryEscape(encodeLocalMusicID("Local Track.mp3")),
				Extra:    map[string]string{"lyric": "true", "cover": "true"},
			},
		}, nil, "", nil, "", "local_music", "", "", "", false, "", nil)
	})

	req := httptest.NewRequest(http.MethodGet, RoutePrefix, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	required := []string{
		`id="localMusicPageUploadInput"`,
		`id="localMusicPageList"`,
		`data-local-music-page="true"`,
		`onchange="uploadLocalMusicForPage(this)"`,
		`id="btn-batch-delete-local"`,
		`onclick="batchDeleteLocalMusic()"`,
		`data-source="local"`,
		`class="tag tag-local"`,
		`>本地</span>`,
		`class="btn-circle btn-play"`,
		`class="btn-circle btn-dl btn-lyric"`,
		`class="btn-circle btn-dl btn-cover"`,
		`class="btn-circle btn-delete-local"`,
		`class="btn-circle btn-fav"`,
		`onclick="deleteLocalMusicFromButton(this)"`,
		`onclick="openAddToCollectionModal(this)"`,
		`/local_music/cover?id=`,
	}
	for _, token := range required {
		if !strings.Contains(body, token) {
			t.Fatalf("local music page missing %q in rendered body: %s", token, body)
		}
	}

	forbidden := []string{
		`class="btn-circle btn-switch"`,
		`class="btn-circle btn-dl btn-download"`,
		`id="btn-batch-switch"`,
		`id="btn-batch-dl"`,
		`id="btn-reindex"`,
		`onclick="reindexLocalMusic()"`,
		`selectInvalidSongs()`,
		`removeSongFromCollection`,
	}
	for _, token := range forbidden {
		if strings.Contains(body, token) {
			t.Fatalf("local music page should not render %q: %s", token, body)
		}
	}

	playIndex := strings.Index(body, `class="btn-circle btn-play"`)
	favIndex := strings.Index(body, `onclick="openAddToCollectionModal(this)"`)
	lyricIndex := strings.Index(body, `class="btn-circle btn-dl btn-lyric"`)
	coverIndex := strings.Index(body, `class="btn-circle btn-dl btn-cover"`)
	deleteIndex := strings.Index(body, `onclick="deleteLocalMusicFromButton(this)"`)
	if !(playIndex < favIndex && favIndex < lyricIndex && lyricIndex < coverIndex && coverIndex < deleteIndex) {
		t.Fatalf("local music page action order mismatch: play=%d fav=%d lyric=%d cover=%d delete=%d", playIndex, favIndex, lyricIndex, coverIndex, deleteIndex)
	}
}

func TestManualCollectionLocalSongRendersLocalActionOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)

	localID := encodeLocalMusicID("Local Track.mp3")
	router := gin.New()
	router.SetHTMLTemplate(newTestTemplate(t))
	router.GET(RoutePrefix, func(c *gin.Context) {
		renderIndex(c, []model.Song{
			{
				ID:       localID,
				Source:   localMusicSource,
				Name:     "Local Track",
				Artist:   "Local Artist",
				Album:    "Local Album",
				Duration: 125,
				Cover:    RoutePrefix + "/local_music/cover?id=" + url.QueryEscape(localID),
				Extra:    map[string]string{"lyric": "true", "cover": "true"},
			},
		}, nil, "", nil, "", "song", "", "1", "My Playlist", false, collectionKindManual, nil)
	})

	req := httptest.NewRequest(http.MethodGet, RoutePrefix, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	cardStart := strings.Index(body, `data-id="`+localID+`"`)
	if cardStart < 0 {
		t.Fatalf("rendered body missing local song card: %s", body)
	}
	cardEnd := strings.Index(body[cardStart:], `</li>`)
	if cardEnd < 0 {
		t.Fatalf("rendered body missing local song card end: %s", body)
	}
	card := body[cardStart : cardStart+cardEnd]

	forbidden := []string{
		`class="btn-circle btn-dl btn-download"`,
	}
	for _, token := range forbidden {
		if strings.Contains(card, token) {
			t.Fatalf("manual collection local song should not render %q: %s", token, card)
		}
	}

	required := []string{
		`class="btn-circle btn-play"`,
		`class="btn-circle btn-switch"`,
		`removeSongFromCollection`,
		`class="btn-circle btn-dl btn-lyric"`,
		`class="btn-circle btn-dl btn-cover"`,
		`/local_music/cover?id=`,
	}
	for _, token := range required {
		if !strings.Contains(card, token) {
			t.Fatalf("manual collection local song missing %q: %s", token, card)
		}
	}

	playIndex := strings.Index(card, `class="btn-circle btn-play"`)
	switchIndex := strings.Index(card, `class="btn-circle btn-switch"`)
	removeIndex := strings.Index(card, `removeSongFromCollection`)
	lyricIndex := strings.Index(card, `class="btn-circle btn-dl btn-lyric"`)
	coverIndex := strings.Index(card, `class="btn-circle btn-dl btn-cover"`)
	if !(playIndex < switchIndex && switchIndex < removeIndex && removeIndex < lyricIndex && lyricIndex < coverIndex) {
		t.Fatalf("manual collection local song action order mismatch: play=%d switch=%d remove=%d lyric=%d cover=%d", playIndex, switchIndex, removeIndex, lyricIndex, coverIndex)
	}
}

func TestLocalMusicSidecarCoverAndLyricFallbacks(t *testing.T) {
	initCollectionDBForTest(t)

	downloadDir := t.TempDir()
	withLocalMusicDownloadDir(t, downloadDir)

	audioPath := filepath.Join(downloadDir, "Sidecar Song.mp3")
	coverPath := filepath.Join(downloadDir, "Sidecar Song.png")
	lyricPath := filepath.Join(downloadDir, "Sidecar Song.lrc")
	coverBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	lyricText := "[00:01.00]Sidecar lyric line"
	if err := os.WriteFile(audioPath, []byte("not a real mp3"), 0644); err != nil {
		t.Fatalf("write local audio: %v", err)
	}
	if err := os.WriteFile(coverPath, coverBytes, 0644); err != nil {
		t.Fatalf("write sidecar cover: %v", err)
	}
	if err := os.WriteFile(lyricPath, []byte(lyricText), 0644); err != nil {
		t.Fatalf("write sidecar lyric: %v", err)
	}

	router := newLocalMusicTestRouter()
	req := httptest.NewRequest(http.MethodGet, RoutePrefix+"/local_music", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /local_music status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Tracks []localMusicTrack `json:"tracks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode local music response: %v", err)
	}
	if len(resp.Tracks) != 1 {
		t.Fatalf("local music tracks len = %d, want 1", len(resp.Tracks))
	}

	track := resp.Tracks[0]
	if track.Cover == "" {
		t.Fatal("track.Cover is empty, want local cover URL")
	}
	if track.Extra["cover_source"] != "sidecar" {
		t.Fatalf("cover_source = %q, want sidecar", track.Extra["cover_source"])
	}
	if track.Extra["lyric_source"] != "sidecar" {
		t.Fatalf("lyric_source = %q, want sidecar", track.Extra["lyric_source"])
	}

	req = httptest.NewRequest(http.MethodGet, RoutePrefix+"/local_music/cover?id="+url.QueryEscape(track.ID), nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET local cover status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("local cover content type = %q, want image/png", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), coverBytes) {
		t.Fatalf("local cover body = %v, want %v", rec.Body.Bytes(), coverBytes)
	}

	lyricURL := fmt.Sprintf("%s/download_lrc?id=%s&source=%s&name=Sidecar%%20Song&artist=Unknown", RoutePrefix, url.QueryEscape(track.ID), localMusicSource)
	req = httptest.NewRequest(http.MethodGet, lyricURL, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET local download_lrc status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), lyricText) {
		t.Fatalf("local download_lrc body = %q, want lyric %q", rec.Body.String(), lyricText)
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), ".lrc") {
		t.Fatalf("local download_lrc missing lrc download header: %q", rec.Header().Get("Content-Disposition"))
	}

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/lyric?id=%s&source=%s", RoutePrefix, url.QueryEscape(track.ID), localMusicSource), nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET local lyric status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), lyricText) {
		t.Fatalf("local lyric body = %q, want lyric %q", rec.Body.String(), lyricText)
	}
}

func TestLocalMusicListIncludesEmbeddedCover(t *testing.T) {
	initCollectionDBForTest(t)

	downloadDir := t.TempDir()
	withLocalMusicDownloadDir(t, downloadDir)

	coverBytes := []byte{0xff, 0xd8, 0xff, 0xd9}
	embedded, err := core.EmbedSongMetadata(
		[]byte{0xff, 0xfb, 0x90, 0x64, 0x00, 0x00, 0x00, 0x00},
		&model.Song{Name: "Embedded Cover", Artist: "Local Artist", Album: "Local Album", Ext: "mp3"},
		"",
		coverBytes,
		"image/jpeg",
	)
	if err != nil {
		t.Fatalf("EmbedSongMetadata() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(downloadDir, "Embedded Cover.mp3"), embedded, 0644); err != nil {
		t.Fatalf("write embedded cover audio: %v", err)
	}

	router := newLocalMusicTestRouter()
	req := httptest.NewRequest(http.MethodGet, RoutePrefix+"/local_music", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /local_music status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Tracks []localMusicTrack `json:"tracks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode local music response: %v", err)
	}
	if len(resp.Tracks) != 1 {
		t.Fatalf("local music tracks len = %d, want 1", len(resp.Tracks))
	}

	track := resp.Tracks[0]
	if track.Cover == "" {
		t.Fatal("track.Cover is empty, want embedded cover URL")
	}
	if track.Extra["cover_source"] != "embedded" {
		t.Fatalf("cover_source = %q, want embedded", track.Extra["cover_source"])
	}
	if track.Name != "Embedded Cover" || track.Artist != "Local Artist" || track.Album != "Local Album" {
		t.Fatalf("track metadata = %q/%q/%q, want embedded metadata", track.Name, track.Artist, track.Album)
	}

	req = httptest.NewRequest(http.MethodGet, RoutePrefix+"/local_music/cover?id="+url.QueryEscape(track.ID), nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET local embedded cover status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("local embedded cover content type = %q, want image/jpeg", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), coverBytes) {
		t.Fatalf("local embedded cover body = %v, want %v", rec.Body.Bytes(), coverBytes)
	}
}

func TestUploadLocalMusicAddToCollectionAndDownload(t *testing.T) {
	initCollectionDBForTest(t)

	downloadDir := t.TempDir()
	withLocalMusicDownloadDir(t, downloadDir)

	collection := Collection{
		Name:        "Uploads",
		Kind:        collectionKindManual,
		ContentType: collectionContentPlaylist,
		Source:      "local",
	}
	if err := db.Create(&collection).Error; err != nil {
		t.Fatalf("create collection: %v", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "Uploaded Song.flac")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write([]byte("fLaC uploaded audio bytes")); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	router := newLocalMusicTestRouter()
	req := httptest.NewRequest(http.MethodPost, RoutePrefix+"/local_music/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /local_music/upload status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var uploadResp struct {
		Track localMusicTrack `json:"track"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if uploadResp.Track.ID == "" {
		t.Fatal("uploaded track ID is empty")
	}
	if uploadResp.Track.Name != "Uploaded Song" {
		t.Fatalf("uploaded track name = %q, want Uploaded Song", uploadResp.Track.Name)
	}

	addBody, err := json.Marshal(map[string]string{"id": uploadResp.Track.ID})
	if err != nil {
		t.Fatalf("marshal add body: %v", err)
	}
	addPath := fmt.Sprintf("%s/collections/%d/local_music", RoutePrefix, collection.ID)
	req = httptest.NewRequest(http.MethodPost, addPath, bytes.NewReader(addBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST %s status = %d, want %d, body=%s", addPath, rec.Code, http.StatusOK, rec.Body.String())
	}

	var saved SavedSong
	if err := db.Where("collection_id = ? AND song_id = ? AND source = ?", collection.ID, uploadResp.Track.ID, localMusicSource).First(&saved).Error; err != nil {
		t.Fatalf("query saved local song: %v", err)
	}
	if saved.Name != "Uploaded Song" || saved.Artist != "未知歌手" {
		t.Fatalf("saved local song metadata = %q/%q, want Uploaded Song/未知歌手", saved.Name, saved.Artist)
	}

	downloadURL := fmt.Sprintf("%s/download?id=%s&source=%s", RoutePrefix, uploadResp.Track.ID, localMusicSource)
	req = httptest.NewRequest(http.MethodGet, downloadURL, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET local download status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if rec.Body.String() != "fLaC uploaded audio bytes" {
		t.Fatalf("download body = %q, want uploaded bytes", rec.Body.String())
	}
}

func TestDeleteLocalMusicHardDeletesAndKeepsCollectionEntries(t *testing.T) {
	initCollectionDBForTest(t)

	downloadDir := t.TempDir()
	withLocalMusicDownloadDir(t, downloadDir)

	audioPath := filepath.Join(downloadDir, "Delete Me.mp3")
	if err := os.WriteFile(audioPath, []byte("delete me"), 0644); err != nil {
		t.Fatalf("write local audio: %v", err)
	}
	localID := encodeLocalMusicID("Delete Me.mp3")

	collections := []Collection{
		{Name: "Local One", Kind: collectionKindManual, ContentType: collectionContentPlaylist, Source: "local"},
		{Name: "Local Two", Kind: collectionKindManual, ContentType: collectionContentPlaylist, Source: "local"},
	}
	if err := db.Create(&collections).Error; err != nil {
		t.Fatalf("create collections: %v", err)
	}

	saved := []SavedSong{
		{CollectionID: collections[0].ID, SongID: localID, Source: localMusicSource, Name: "Delete Me"},
		{CollectionID: collections[1].ID, SongID: localID, Source: legacyLocalMusicSource, Name: "Delete Me"},
	}
	if err := db.Create(&saved).Error; err != nil {
		t.Fatalf("create saved local songs: %v", err)
	}

	// Seed an index row so we can confirm it is removed on delete.
	if err := db.Create(&LocalMusicIndex{ID: localID, RelPath: "Delete Me.mp3", Name: "Delete Me"}).Error; err != nil {
		t.Fatalf("seed index row: %v", err)
	}

	router := newLocalMusicTestRouter()
	req := httptest.NewRequest(http.MethodDelete, RoutePrefix+"/local_music?id="+url.QueryEscape(localID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Hard delete succeeds even though the track is referenced by collections.
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE /local_music status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if _, err := os.Stat(audioPath); !os.IsNotExist(err) {
		t.Fatalf("deleted local file stat err = %v, want not exists", err)
	}

	// Collection entries remain (they will render as invalid and can be switched).
	var savedCount int64
	if err := db.Model(&SavedSong{}).
		Where("song_id = ? AND source IN ?", localID, []string{localMusicSource, legacyLocalMusicSource}).
		Count(&savedCount).Error; err != nil {
		t.Fatalf("count saved local songs: %v", err)
	}
	if savedCount != 2 {
		t.Fatalf("saved local songs count = %d, want 2 (collection entries must remain)", savedCount)
	}

	// The index row is gone, so the track no longer appears in search.
	var indexCount int64
	if err := db.Model(&LocalMusicIndex{}).Where("id = ?", localID).Count(&indexCount).Error; err != nil {
		t.Fatalf("count index rows: %v", err)
	}
	if indexCount != 0 {
		t.Fatalf("index row count = %d, want 0 after hard delete", indexCount)
	}
}

func TestAutoCacheEndpointRequiresSameOrigin(t *testing.T) {
	body, err := json.Marshal(map[string]string{"id": "song-1", "source": "qq"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := newAutoCacheHTTPRequest(body)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	newLocalMusicTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST /local_music/auto_cache status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestAutoCacheEndpointRejectsOversizedRequest(t *testing.T) {
	body := []byte(`{"id":"song-1","source":"qq","name":"` + strings.Repeat("x", autoCacheMaxRequestBytes) + `"}`)
	rec := httptest.NewRecorder()
	newLocalMusicTestRouter().ServeHTTP(rec, newAutoCacheHTTPRequest(body))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("POST /local_music/auto_cache status = %d, want %d, body=%s", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
	}
}

func TestAutoCacheEndpointSkipsWhenDisabled(t *testing.T) {
	withAutoCacheSettings(t, core.WebSettings{AutoCacheOnPlay: false})

	originalSave := autoCacheSaveSong
	t.Cleanup(func() {
		waitForAutoCacheIdle(t)
		autoCacheSaveSong = originalSave
	})

	called := make(chan struct{}, 1)
	autoCacheSaveSong = func(_ *model.Song, _ string, _ bool, _ bool, _ string) (*core.DownloadedSong, error) {
		called <- struct{}{}
		return nil, nil
	}

	body, err := json.Marshal(map[string]string{"id": "song-1", "source": "qq"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	rec := httptest.NewRecorder()
	newLocalMusicTestRouter().ServeHTTP(rec, newAutoCacheHTTPRequest(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /local_music/auto_cache status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var response map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode auto-cache response: %v", err)
	}
	if response["status"] != "skipped" || response["reason"] != "auto cache disabled" {
		t.Fatalf("auto-cache response = %#v, want skipped because auto cache is disabled", response)
	}

	select {
	case <-called:
		t.Fatal("auto-cache worker started while playback caching was disabled")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAutoCacheEndpointStartsDownloadAndIndexesSavedSong(t *testing.T) {
	withAutoCacheSettings(t, core.WebSettings{AutoCacheOnPlay: true})

	originalSave := autoCacheSaveSong
	originalIndex := autoCacheIndexSavedSong
	t.Cleanup(func() {
		waitForAutoCacheIdle(t)
		autoCacheSaveSong = originalSave
		autoCacheIndexSavedSong = originalIndex
	})

	saved := make(chan *model.Song, 1)
	indexed := make(chan struct{}, 1)
	autoCacheSaveSong = func(song *model.Song, _ string, _ bool, _ bool, _ string) (*core.DownloadedSong, error) {
		saved <- song
		return &core.DownloadedSong{}, nil
	}
	autoCacheIndexSavedSong = func(_ *core.DownloadedSong, _ string) { indexed <- struct{}{} }

	body, err := json.Marshal(map[string]interface{}{
		"id":     "song-1",
		"source": "qq",
		"name":   "Song One",
		"artist": "Artist One",
		"extra":  map[string]string{"quality": "high"},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	rec := httptest.NewRecorder()
	newLocalMusicTestRouter().ServeHTTP(rec, newAutoCacheHTTPRequest(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /local_music/auto_cache status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"status":"started"`) {
		t.Fatalf("auto-cache response = %s, want started", rec.Body.String())
	}

	select {
	case song := <-saved:
		if song.ID != "song-1" || song.Source != "qq" || song.Extra["quality"] != "high" {
			t.Fatalf("saved song = %+v, want normalized request", song)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for auto-cache download")
	}
	select {
	case <-indexed:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for saved local file indexing")
	}
}

func TestAutoCacheEndpointDeduplicatesInFlightRequests(t *testing.T) {
	withAutoCacheSettings(t, core.WebSettings{AutoCacheOnPlay: true})

	originalSave := autoCacheSaveSong
	originalIndex := autoCacheIndexSavedSong
	t.Cleanup(func() {
		waitForAutoCacheIdle(t)
		autoCacheSaveSong = originalSave
		autoCacheIndexSavedSong = originalIndex
	})

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	indexed := make(chan struct{}, 1)
	autoCacheSaveSong = func(_ *model.Song, _ string, _ bool, _ bool, _ string) (*core.DownloadedSong, error) {
		started <- struct{}{}
		<-release
		return &core.DownloadedSong{}, nil
	}
	autoCacheIndexSavedSong = func(_ *core.DownloadedSong, _ string) { indexed <- struct{}{} }

	body, err := json.Marshal(map[string]string{"id": "song-1", "source": "qq"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	router := newLocalMusicTestRouter()

	first := httptest.NewRecorder()
	router.ServeHTTP(first, newAutoCacheHTTPRequest(body))
	if !strings.Contains(first.Body.String(), `"status":"started"`) {
		t.Fatalf("first auto-cache response = %s, want started", first.Body.String())
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first auto-cache worker")
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, newAutoCacheHTTPRequest(body))
	if !strings.Contains(second.Body.String(), `"status":"in_progress"`) {
		t.Fatalf("second auto-cache response = %s, want in_progress", second.Body.String())
	}

	close(release)
	select {
	case <-indexed:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first auto-cache worker to index its saved file")
	}
}

func TestReserveAutoCacheLimitsConcurrentDownloads(t *testing.T) {
	waitForAutoCacheIdle(t)
	if status := reserveAutoCache("qq:first"); status != "started" {
		t.Fatalf("first reservation = %q, want started", status)
	}
	defer releaseAutoCache("qq:first")
	if status := reserveAutoCache("qq:second"); status != "started" {
		t.Fatalf("second reservation = %q, want started", status)
	}
	defer releaseAutoCache("qq:second")
	if status := reserveAutoCache("qq:third"); status != "busy" {
		t.Fatalf("third reservation = %q, want busy", status)
	}
}

func TestAutoCacheClientWaitsForConfirmedLocalMatch(t *testing.T) {
	content, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	js := string(content)
	for _, want := range []string{
		"autoCacheOnPlay: true",
		"function isAutoCacheOnPlayEnabled()",
		"if (!isAutoCacheOnPlayEnabled()) return;",
		"if (!isAutoCacheOnPlayEnabled() || !key || localMusicMatchCache[audio.custom_id])",
		"'X-Requested-With': 'XMLHttpRequest'",
		"scheduleAutoCacheMatch(audio, key)",
		"custom_id: playbackSong.id",
		"original_id: ds.id",
		"getPlaybackCardID(newAudio)",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js missing %q", want)
		}
	}
	if strings.Contains(js, "localMusicMatchCache[audio.custom_id] =") {
		t.Fatal("auto-cache must not mark an online ID as a local file before matching")
	}
}

func TestIndexAutoCachedLocalMusicUpsertsOnlySavedFile(t *testing.T) {
	initCollectionDBForTest(t)

	downloadDir := t.TempDir()
	withLocalMusicDownloadDir(t, downloadDir)
	savedPath := filepath.Join(downloadDir, "Artist", "Cached Song.mp3")
	if err := os.MkdirAll(filepath.Dir(savedPath), 0755); err != nil {
		t.Fatalf("create download subdirectory: %v", err)
	}
	if err := os.WriteFile(savedPath, []byte("audio"), 0644); err != nil {
		t.Fatalf("write cached audio: %v", err)
	}

	indexAutoCachedLocalMusic(&core.DownloadedSong{SavedPath: savedPath}, "")

	var row LocalMusicIndex
	if err := db.Where("rel_path = ?", "Artist/Cached Song.mp3").First(&row).Error; err != nil {
		t.Fatalf("load indexed cached song: %v", err)
	}
	if row.Name != "Cached Song" {
		t.Fatalf("indexed name = %q, want Cached Song", row.Name)
	}
}

func TestBatchMatchLocalMusicSkipsStaleCandidate(t *testing.T) {
	initCollectionDBForTest(t)

	downloadDir := t.TempDir()
	withLocalMusicDownloadDir(t, downloadDir)
	validPath := filepath.Join(downloadDir, "existing.mp3")
	if err := os.WriteFile(validPath, []byte("audio"), 0644); err != nil {
		t.Fatalf("write valid audio: %v", err)
	}

	now := time.Now()
	stale := LocalMusicIndex{
		ID:      encodeLocalMusicID("missing.mp3"),
		RelPath: "missing.mp3",
		Name:    "Same Song",
		Artist:  "Same Artist",
		ModTime: now.Add(time.Minute),
	}
	valid := LocalMusicIndex{
		ID:      encodeLocalMusicID("existing.mp3"),
		RelPath: "existing.mp3",
		Name:    "Same Song",
		Artist:  "Same Artist",
		ModTime: now,
	}
	if err := db.Create(&stale).Error; err != nil {
		t.Fatalf("create stale index row: %v", err)
	}
	if err := db.Create(&valid).Error; err != nil {
		t.Fatalf("create valid index row: %v", err)
	}

	body, err := json.Marshal([]map[string]string{{"name": "Same Song", "artist": "Same Artist"}})
	if err != nil {
		t.Fatalf("marshal batch match request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, RoutePrefix+"/local_music/batch_match", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newLocalMusicTestRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /local_music/batch_match status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Matches []struct {
			ID string `json:"id"`
		} `json:"matches"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode batch match response: %v", err)
	}
	if len(response.Matches) != 1 || response.Matches[0].ID != valid.ID {
		t.Fatalf("matches = %+v, want valid local file %q", response.Matches, valid.ID)
	}

	var staleCount int64
	if err := db.Model(&LocalMusicIndex{}).Where("id = ?", stale.ID).Count(&staleCount).Error; err != nil {
		t.Fatalf("count stale index row: %v", err)
	}
	if staleCount != 0 {
		t.Fatal("batch match should remove stale candidate rows")
	}
}

func TestBatchMatchLocalMusicRequiresMatchingArtistWhenProvided(t *testing.T) {
	initCollectionDBForTest(t)

	downloadDir := t.TempDir()
	withLocalMusicDownloadDir(t, downloadDir)
	filename := "Artist One - Same Title.mp3"
	if err := os.WriteFile(filepath.Join(downloadDir, filename), []byte("audio"), 0644); err != nil {
		t.Fatalf("write local audio: %v", err)
	}

	local := LocalMusicIndex{
		ID:      encodeLocalMusicID(filename),
		RelPath: filename,
		Name:    "Same Title",
		Artist:  "Artist One",
		ModTime: time.Now(),
	}
	if err := db.Create(&local).Error; err != nil {
		t.Fatalf("create local index row: %v", err)
	}

	body, err := json.Marshal([]map[string]string{
		{"name": "Same Title", "artist": "Artist Two"},
		{"name": "Same Title", "artist": "Artist One"},
	})
	if err != nil {
		t.Fatalf("marshal batch match request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, RoutePrefix+"/local_music/batch_match", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newLocalMusicTestRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /local_music/batch_match status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Matches []struct {
			QueryIndex int    `json:"qi"`
			ID         string `json:"id"`
		} `json:"matches"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode batch match response: %v", err)
	}
	if len(response.Matches) != 1 || response.Matches[0].QueryIndex != 1 || response.Matches[0].ID != local.ID {
		t.Fatalf("matches = %+v, want only the same-artist query to match %q", response.Matches, local.ID)
	}
}

func TestRefreshLocalMusicScanSkipsUnchangedIndexWrite(t *testing.T) {
	initCollectionDBForTest(t)

	downloadDir := t.TempDir()
	withLocalMusicDownloadDir(t, downloadDir)
	if err := os.WriteFile(filepath.Join(downloadDir, "Stable.mp3"), []byte("audio"), 0644); err != nil {
		t.Fatalf("write local audio: %v", err)
	}
	tracks, dir, exists, err := scanLocalMusicTracks()
	if err != nil {
		t.Fatalf("scan local music: %v", err)
	}
	if err := syncTracksToIndex(tracks); err != nil {
		t.Fatalf("seed local index: %v", err)
	}
	storeLocalMusicScanSnapshot(localMusicScanSnapshot{
		Dir:       dir,
		Tracks:    cloneLocalMusicTrackSlice(tracks),
		Exists:    exists,
		ScannedAt: time.Now(),
	})

	var before LocalMusicIndex
	if err := db.Where("rel_path = ?", "Stable.mp3").First(&before).Error; err != nil {
		t.Fatalf("load seeded index row: %v", err)
	}

	refreshLocalMusicScanAsync(downloadDir)
	waitForLocalMusicScanRefresh(t)

	var after LocalMusicIndex
	if err := db.Where("rel_path = ?", "Stable.mp3").First(&after).Error; err != nil {
		t.Fatalf("load refreshed index row: %v", err)
	}
	if !after.ScannedAt.Equal(before.ScannedAt) {
		t.Fatalf("unchanged scan rewrote index: scanned_at before=%s after=%s", before.ScannedAt, after.ScannedAt)
	}
}

func TestLocalMusicSlowPathClearsIndexAfterDirectoryBecomesEmpty(t *testing.T) {
	initCollectionDBForTest(t)

	downloadDir := t.TempDir()
	withLocalMusicDownloadDir(t, downloadDir)
	if err := db.Create(&LocalMusicIndex{
		ID:      encodeLocalMusicID("removed.mp3"),
		RelPath: "removed.mp3",
		Name:    "Removed",
	}).Error; err != nil {
		t.Fatalf("seed stale index row: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, RoutePrefix+"/local_music?force=1", nil)
	rec := httptest.NewRecorder()
	newLocalMusicTestRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /local_music?force=1 status = %d, body=%s", rec.Code, rec.Body.String())
	}

	deadline := time.Now().Add(time.Second)
	for {
		var count int64
		if err := db.Model(&LocalMusicIndex{}).Count(&count).Error; err != nil {
			t.Fatalf("count index rows: %v", err)
		}
		if count == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("empty scan did not clear stale index row, count=%d", count)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestAppJSInitializesCurrentPlayingIDBeforeBootstrap(t *testing.T) {
	content, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	js := string(content)
	stateIndex := strings.Index(js, "let currentPlayingId = null;")
	bootstrapIndex := strings.Index(js, "document.addEventListener('DOMContentLoaded'")
	if stateIndex < 0 || bootstrapIndex < 0 {
		t.Fatalf("app.js missing playback state or DOM bootstrap")
	}
	if stateIndex > bootstrapIndex {
		t.Fatal("currentPlayingId must initialize before the page bootstrap")
	}
}

func TestLocalMusicEndpointPaginatesIndexedTracks(t *testing.T) {
	initCollectionDBForTest(t)

	downloadDir := t.TempDir()
	withLocalMusicDownloadDir(t, downloadDir)
	for _, name := range []string{"Page One.mp3", "Page Two.mp3", "Page Three.mp3", "Page Four.mp3", "Page Five.mp3"} {
		if err := os.WriteFile(filepath.Join(downloadDir, name), []byte("audio"), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := syncLocalMusicIndex(); err != nil {
		t.Fatalf("sync local music index: %v", err)
	}
	defer waitForLocalMusicScanRefresh(t)

	router := newLocalMusicTestRouter()
	assertPage := func(offset, limit, wantTracks int, wantMore bool) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/local_music?offset=%d&limit=%d", RoutePrefix, offset, limit), nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /local_music offset=%d limit=%d status=%d body=%s", offset, limit, rec.Code, rec.Body.String())
		}

		var response struct {
			Tracks  []localMusicTrack `json:"tracks"`
			Total   int               `json:"total"`
			HasMore bool              `json:"has_more"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode page response: %v", err)
		}
		if response.Total != 5 || len(response.Tracks) != wantTracks || response.HasMore != wantMore {
			t.Fatalf("page offset=%d limit=%d got total=%d tracks=%d has_more=%t, want 5/%d/%t", offset, limit, response.Total, len(response.Tracks), response.HasMore, wantTracks, wantMore)
		}
	}

	assertPage(0, 2, 2, true)
	assertPage(2, 2, 2, true)
	assertPage(4, 2, 1, false)
}

func TestLocalMusicClientQueuesPageChangesAndRefreshesAfterDuplicateDeletion(t *testing.T) {
	content, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	js := string(content)
	for _, want := range []string{
		"let queuedLocalMusicPageLoad = null;",
		"async function fetchLocalMusicPagePayload(params)",
		"cache: 'no-store'",
		"persistWebSettingsCache();",
		"await refreshLocalMusicPageAfterMutation();",
		"await checkDuplicateSongs();",
		"const btn = document.createElement('button');",
		"btn.className = 'playmode-toggle-btn';",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js missing %q", want)
		}
	}
	if strings.Contains(js, "async function reindexLocalMusic()") {
		t.Fatal("manual reindex control should not remain in the client")
	}
}

func TestPlayModeButtonSitsAboveFixedPlayerCover(t *testing.T) {
	content, err := templateFS.ReadFile("templates/static/css/style.css")
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	css := string(content)
	for _, want := range []string{
		".aplayer.aplayer-fixed .playmode-toggle-btn",
		"top: -34px;",
		"left: 17px;",
		"width: 32px;",
		"height: 28px;",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("style.css missing %q", want)
		}
	}
}
