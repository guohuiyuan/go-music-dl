package core

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

// setupWebDAVOnlyTest isolates the config DB and registers a fake WebDAV server
// that records MKCOL/PUT/HEAD requests.
func setupWebDAVOnlyTest(t *testing.T) (WebSettings, *webdavRecorder) {
	t.Helper()
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "data", "settings.db"))
	t.Setenv("MUSIC_DL_COOKIE_FILE", filepath.Join(baseDir, "data", "cookies.json"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)
	if err := SaveWebSettings(WebSettings{
		WebDAVEnabled:         true,
		DownloadToLocal:       false,
		DownloadLyricsToLocal: true,
	}); err != nil {
		t.Fatalf("save base settings: %v", err)
	}
	rec := newWebDAVRecorder(t)
	settings := WebSettings{
		WebDAVEnabled:         true,
		WebDAVURL:             rec.server.URL + "/dav",
		WebDAVUsername:        rec.user,
		WebDAVPassword:        rec.pass,
		WebDAVDir:             "music",
		DownloadToLocal:       false,
		DownloadLyricsToLocal: true,
	}
	if err := SaveWebSettings(settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	return settings, rec
}

type webdavRecorder struct {
	t          *testing.T
	server     *httptest.Server
	user, pass string
	mu         sync.Mutex
	put        map[string]string
	head       []string
	existing   map[string]bool
}

func newWebDAVRecorder(t *testing.T) *webdavRecorder {
	r := &webdavRecorder{
		t:        t,
		user:     "test",
		pass:     "123456",
		put:      map[string]string{},
		existing: map[string]bool{},
	}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		user, pass, ok := req.BasicAuth()
		if !ok || user != r.user || pass != r.pass {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch req.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(req.Body)
			r.mu.Lock()
			r.put[req.URL.Path] = string(body)
			r.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
		case http.MethodHead:
			r.mu.Lock()
			r.head = append(r.head, req.URL.Path)
			_, exists := r.put[req.URL.Path]
			pre := r.existing[req.URL.Path]
			r.mu.Unlock()
			if exists || pre {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case "MKCOL":
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(r.server.Close)
	return r
}

func (r *webdavRecorder) puts() map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]string{}
	for k, v := range r.put {
		out[k] = v
	}
	return out
}

func TestDownloadSongToWebDAVOnlyUploadsAudioAndLyrics(t *testing.T) {
	orig := webDAVOnlyFetcher
	webDAVOnlyFetcher = func(song *model.Song, withCover, withLyrics bool, filenameTemplate string) (*DownloadedSong, error) {
		return &DownloadedSong{Data: []byte("fake-audio"), Ext: "mp3", Filename: "fake.mp3", Lyric: "[00:00.00]hello\n[00:01.00]world"}, nil
	}
	t.Cleanup(func() { webDAVOnlyFetcher = orig })
	_, rec := setupWebDAVOnlyTest(t)
	song := &model.Song{ID: "1", Source: "netease", Name: "Test Song", Artist: "Tester"}

	result, err := DownloadSongToWebDAVOnly(song, false, true, "{name} - {artist}", nil)
	if err != nil {
		t.Fatalf("DownloadSongToWebDAVOnly: %v", err)
	}
	if result.Skipped {
		t.Fatal("first download should not be skipped")
	}
	if result.SavedPath != "" {
		t.Fatalf("webdav-only mode must not report a local path, got %q", result.SavedPath)
	}

	puts := rec.puts()
	var audioUploaded bool
	for p := range puts {
		if strings.HasSuffix(p, ".flac") || strings.HasSuffix(p, ".mp3") || strings.HasSuffix(p, ".m4a") {
			audioUploaded = true
		}
	}
	if !audioUploaded {
		t.Fatalf("expected an audio PUT, got %v", puts)
	}
	var lyricUploaded bool
	for p := range puts {
		if strings.HasSuffix(p, ".lrc") {
			lyricUploaded = true
		}
	}
	if !lyricUploaded {
		t.Fatalf("expected a lyrics PUT, got %v", puts)
	}
}

func TestDownloadSongToWebDAVOnlySkipsWhenCloudFileExists(t *testing.T) {
	_, rec := setupWebDAVOnlyTest(t)
	song := &model.Song{ID: "2", Source: "netease", Name: "Existing Song", Artist: "Tester"}
	base := strings.TrimSpace(BuildDownloadFilename(&model.Song{Name: "Existing Song", Artist: "Tester"}, "", ""))
	if base == "" {
		t.Fatal("probe base name should not be empty")
	}
	// Simulate an existing remote file for every probed extension.
	for _, ext := range []string{".flac", ".mp3", ".m4a", ".wav", ".ogg", ".wma"} {
		rec.mu.Lock()
		rec.existing["/dav/music/"+base+ext] = true
		rec.mu.Unlock()
	}

	result, err := DownloadSongToWebDAVOnly(song, false, false, "", nil)
	if err != nil {
		t.Fatalf("DownloadSongToWebDAVOnly: %v", err)
	}
	if !result.Skipped {
		t.Fatal("expected skip when cloud file exists")
	}
	if !strings.Contains(result.Warning, "WebDAV") {
		t.Fatalf("skip warning should mention WebDAV, got %q", result.Warning)
	}
	if len(rec.puts()) != 0 {
		t.Fatalf("no uploads expected on skip, got %v", rec.puts())
	}
}

func TestDownloadSongToWebDAVOnlyLocalRecordDoesNotBlockCloudUpload(t *testing.T) {
	orig := webDAVOnlyFetcher
	webDAVOnlyFetcher = func(song *model.Song, withCover, withLyrics bool, filenameTemplate string) (*DownloadedSong, error) {
		return &DownloadedSong{Data: []byte("fake-audio"), Ext: "mp3", Filename: "fake.mp3", Lyric: "[00:00.00]hello\n[00:01.00]world"}, nil
	}
	t.Cleanup(func() { webDAVOnlyFetcher = orig })
	_, rec := setupWebDAVOnlyTest(t)
	song := &model.Song{ID: "3", Source: "netease", Name: "Local Only Song", Artist: "Tester"}
	// A success record with empty relPath (like older webdav-only runs) must
	// not cause a skip: the cloud copy decides, not the local history.
	if err := SaveDownloadRecordWithRelPath(song.Name, song.Artist, song.Source, DownloadStatusSuccess, "", ""); err != nil {
		t.Fatalf("seed record: %v", err)
	}

	result, err := DownloadSongToWebDAVOnly(song, false, false, "", nil)
	if err != nil {
		t.Fatalf("DownloadSongToWebDAVOnly: %v", err)
	}
	if result.Skipped {
		t.Fatal("local history must not block a webdav-only upload when cloud file is absent")
	}
	if len(rec.puts()) == 0 {
		t.Fatal("expected uploads for a song that only exists locally")
	}
}

func TestDownloadSongToWebDAVOnlyNormalizesLyrics(t *testing.T) {
	orig := webDAVOnlyFetcher
	webDAVOnlyFetcher = func(song *model.Song, withCover, withLyrics bool, filenameTemplate string) (*DownloadedSong, error) {
		return &DownloadedSong{Data: []byte("fake-audio"), Ext: "mp3", Filename: "fake.mp3", Lyric: "[00:00.00]hello\n[00:01.00]world"}, nil
	}
	t.Cleanup(func() { webDAVOnlyFetcher = orig })
	_, rec := setupWebDAVOnlyTest(t)
	song := &model.Song{ID: "4", Source: "netease", Name: "Lyric Song", Artist: "Tester"}
	result, err := DownloadSongToWebDAVOnly(song, false, true, "", nil)
	if err != nil {
		t.Fatalf("DownloadSongToWebDAVOnly: %v", err)
	}
	_ = result
	for path, body := range rec.puts() {
		if strings.HasSuffix(path, ".lrc") {
			if !strings.Contains(body, "\r\n") {
				t.Fatalf("uploaded lrc should use CRLF endings, got %q", body[:min(80, len(body))])
			}
			return
		}
	}
	t.Fatal("expected an lrc upload")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
