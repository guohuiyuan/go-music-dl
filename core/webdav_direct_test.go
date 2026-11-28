package core

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

// setupWebDAVDirectTest isolates the config DB and returns settings pointing at
// a fake WebDAV server. Audio download paths are not exercised here; only the
// cloud probe and skip logic are tested to avoid hitting real music sources.
func setupWebDAVDirectTest(t *testing.T, existing []string) (*webdavProbeServer, *webdavProbeServer) {
	t.Helper()
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "data", "settings.db"))
	t.Setenv("MUSIC_DL_COOKIE_FILE", filepath.Join(baseDir, "data", "cookies.json"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)

	rec := newWebDAVProbeServer(t, existing)
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
	return rec, nil
}

type webdavProbeServer struct {
	t        *testing.T
	server   *httptest.Server
	user     string
	pass     string
	existing map[string]bool
	head     []string
}

func newWebDAVProbeServer(t *testing.T, existing []string) *webdavProbeServer {
	r := &webdavProbeServer{
		t:        t,
		user:     "test",
		pass:     "123456",
		existing: map[string]bool{},
	}
	for _, p := range existing {
		r.existing[p] = true
	}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		user, pass, ok := req.BasicAuth()
		if !ok || user != r.user || pass != r.pass {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if req.Method == http.MethodHead {
			r.head = append(r.head, req.URL.Path)
			if r.existing[req.URL.Path] {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	t.Cleanup(r.server.Close)
	return r
}

func TestWebDAVFileExistsProbesConfiguredPath(t *testing.T) {
	rec, _ := setupWebDAVDirectTest(t, []string{"/dav/music/song.flac"})
	settings := WebSettings{
		WebDAVEnabled:  true,
		WebDAVURL:      rec.server.URL + "/dav",
		WebDAVUsername: rec.user,
		WebDAVPassword: rec.pass,
		WebDAVDir:      "music",
	}

	if !webDAVFileExists(settings, "song.flac") {
		t.Fatal("expected probe to find existing remote file")
	}
	if webDAVFileExists(settings, "missing.flac") {
		t.Fatal("expected probe to miss non-existing remote file")
	}
	if len(rec.head) == 0 {
		t.Fatal("expected HEAD requests against the remote server")
	}
}

func TestDownloadSongToWebDAVOnlySkipsWhenCloudFileExists(t *testing.T) {
	rec, _ := setupWebDAVDirectTest(t, nil)
	song := &model.Song{ID: "2", Source: "netease", Name: "Existing Song", Artist: "Tester"}
	base := strings.TrimSpace(BuildDownloadFilename(&model.Song{Name: "Existing Song", Artist: "Tester"}, "", ""))
	if base == "" {
		t.Fatal("probe base name should not be empty")
	}
	for _, ext := range []string{".flac", ".mp3", ".m4a", ".wav", ".ogg", ".wma"} {
		rec.existing["/dav/music/"+base+ext] = true
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
}
