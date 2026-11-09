package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
)

func withOpenDownloadDirectorySettings(t *testing.T, downloadDir string) {
	t.Helper()

	original := openDownloadDirectorySettings
	openDownloadDirectorySettings = func() core.WebSettings {
		return core.WebSettings{DownloadDir: downloadDir}
	}
	t.Cleanup(func() {
		openDownloadDirectorySettings = original
	})
}

func withOpenDownloadDirectoryLauncher(t *testing.T, launcher func(string) error) {
	t.Helper()

	original := openDownloadDirectory
	openDownloadDirectory = launcher
	t.Cleanup(func() {
		openDownloadDirectory = original
	})
}

func newOpenDownloadDirectoryRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST(RoutePrefix+"/open_download_dir", openDownloadDirectoryHandler)
	return router
}

func newOpenDownloadDirectoryRequest(method, target, remoteAddr string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader("{}"))
	request.RemoteAddr = remoteAddr
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	request.Header.Set("Origin", "http://127.0.0.1:8083")
	return request
}

func TestOpenDownloadDirectoryCreatesConfiguredDirectory(t *testing.T) {
	root := t.TempDir()
	downloadDir := filepath.Join(root, "not-created-yet", "Music")
	withOpenDownloadDirectorySettings(t, downloadDir)

	var openedPath string
	withOpenDownloadDirectoryLauncher(t, func(path string) error {
		openedPath = path
		return nil
	})

	request := newOpenDownloadDirectoryRequest(
		http.MethodPost,
		"http://127.0.0.1:8083"+RoutePrefix+"/open_download_dir",
		"127.0.0.1:53142",
	)
	response := httptest.NewRecorder()
	newOpenDownloadDirectoryRouter().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	info, err := os.Stat(downloadDir)
	if err != nil {
		t.Fatalf("configured download directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("configured download path is not a directory: %s", downloadDir)
	}
	if openedPath != downloadDir {
		t.Fatalf("opener path = %q, want %q", openedPath, downloadDir)
	}

	var payload struct {
		Status string `json:"status"`
		Path   string `json:"path"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Status != "ok" || payload.Path != downloadDir {
		t.Fatalf("unexpected response: %+v", payload)
	}
}

func TestOpenDownloadDirectoryRejectsUnsafeRequests(t *testing.T) {
	downloadDir := filepath.Join(t.TempDir(), "downloads")
	withOpenDownloadDirectorySettings(t, downloadDir)

	launchCount := 0
	withOpenDownloadDirectoryLauncher(t, func(string) error {
		launchCount++
		return nil
	})

	tests := []struct {
		name       string
		method     string
		target     string
		remoteAddr string
		mutate     func(*http.Request)
		wantStatus int
	}{
		{
			name:       "missing ajax marker",
			method:     http.MethodPost,
			target:     "http://127.0.0.1:8083" + RoutePrefix + "/open_download_dir",
			remoteAddr: "127.0.0.1:53142",
			mutate: func(request *http.Request) {
				request.Header.Del("X-Requested-With")
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "cross origin",
			method:     http.MethodPost,
			target:     "http://127.0.0.1:8083" + RoutePrefix + "/open_download_dir",
			remoteAddr: "127.0.0.1:53142",
			mutate: func(request *http.Request) {
				request.Header.Set("Origin", "https://example.com")
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "remote host",
			method:     http.MethodPost,
			target:     "http://music.example.com" + RoutePrefix + "/open_download_dir",
			remoteAddr: "127.0.0.1:53142",
			mutate: func(request *http.Request) {
				request.Header.Set("Origin", "http://music.example.com")
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "remote address",
			method:     http.MethodPost,
			target:     "http://127.0.0.1:8083" + RoutePrefix + "/open_download_dir",
			remoteAddr: "192.0.2.20:53142",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "forwarded request",
			method:     http.MethodPost,
			target:     "http://127.0.0.1:8083" + RoutePrefix + "/open_download_dir",
			remoteAddr: "127.0.0.1:53142",
			mutate: func(request *http.Request) {
				request.Header.Set("X-Forwarded-For", "198.51.100.12")
			},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := newOpenDownloadDirectoryRequest(test.method, test.target, test.remoteAddr)
			if test.mutate != nil {
				test.mutate(request)
			}
			response := httptest.NewRecorder()
			newOpenDownloadDirectoryRouter().ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}

	if launchCount != 0 {
		t.Fatalf("unsafe requests launched the file manager %d times", launchCount)
	}
	if _, err := os.Stat(downloadDir); !os.IsNotExist(err) {
		t.Fatalf("unsafe requests created the download directory, stat error=%v", err)
	}
}

func TestOpenDownloadDirectoryReportsLauncherFailure(t *testing.T) {
	withOpenDownloadDirectorySettings(t, t.TempDir())
	withOpenDownloadDirectoryLauncher(t, func(string) error {
		return errors.New("file manager unavailable")
	})

	request := newOpenDownloadDirectoryRequest(
		http.MethodPost,
		"http://127.0.0.1:8083"+RoutePrefix+"/open_download_dir",
		"[::1]:53142",
	)
	response := httptest.NewRecorder()
	newOpenDownloadDirectoryRouter().ServeHTTP(response, request)

	if response.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d, body=%s", response.Code, http.StatusNotImplemented, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "file manager unavailable") {
		t.Fatalf("response does not explain launcher failure: %s", response.Body.String())
	}
}

func TestOpenDownloadDirectoryCommand(t *testing.T) {
	tests := []struct {
		goos    string
		command string
	}{
		{goos: "windows", command: "explorer.exe"},
		{goos: "darwin", command: "open"},
		{goos: "linux", command: "xdg-open"},
	}
	for _, test := range tests {
		t.Run(test.goos, func(t *testing.T) {
			command, err := openDownloadDirectoryCommand(test.goos, "/music/downloads")
			if err != nil {
				t.Fatalf("openDownloadDirectoryCommand(%q): %v", test.goos, err)
			}
			if filepath.Base(command.Path) != test.command {
				t.Fatalf("command = %q, want %q", command.Path, test.command)
			}
			if len(command.Args) != 2 || command.Args[1] != "/music/downloads" {
				t.Fatalf("command args = %#v, want path argument", command.Args)
			}
		})
	}

	for _, goos := range []string{"android", "ios", "plan9"} {
		if _, err := openDownloadDirectoryCommand(goos, "/music/downloads"); err == nil {
			t.Fatalf("openDownloadDirectoryCommand(%q) unexpectedly succeeded", goos)
		}
	}
}

func TestOpenDownloadDirectoryUIEntries(t *testing.T) {
	indexContent, err := templateFS.ReadFile("templates/pages/index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	modalsContent, err := templateFS.ReadFile("templates/partials/modals.html")
	if err != nil {
		t.Fatalf("read modals.html: %v", err)
	}
	appContent, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}

	if strings.Count(string(indexContent), `onclick="openDownloadDirectory()"`) != 1 {
		t.Fatal("index.html should expose exactly one open-download-directory toolbar entry")
	}
	if strings.Count(string(modalsContent), `onclick="openDownloadDirectory()"`) != 2 {
		t.Fatal("settings and download history should each expose an open-download-directory entry")
	}
	appJS := string(appContent)
	for _, want := range []string{
		"async function openDownloadDirectory()",
		"let openDownloadDirectoryInFlight = false",
		"`${API_ROOT}/open_download_dir`",
		`method: "POST"`,
		`"X-Requested-With": "XMLHttpRequest"`,
		`"正在打开下载目录"`,
		`button.setAttribute("aria-busy", "true")`,
		`"已打开下载目录"`,
	} {
		if !strings.Contains(appJS, want) {
			t.Fatalf("open-download-directory frontend missing %q", want)
		}
	}
}
