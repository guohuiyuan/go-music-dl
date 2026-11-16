package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

func withOpenDownloadFileLauncher(t *testing.T, launcher func(string) error) {
	t.Helper()

	original := openDownloadFile
	openDownloadFile = launcher
	t.Cleanup(func() {
		openDownloadFile = original
	})
}

func newRevealDownloadRequest(target string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader("{}"))
	request.RemoteAddr = "127.0.0.1:53142"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	request.Header.Set("Origin", "http://127.0.0.1:8083")
	return request
}

func TestOpenDownloadFileCommandSelectsTheFile(t *testing.T) {
	const path = "/music/downloads/Artist/Song.flac"
	tests := []struct {
		goos     string
		command  string
		wantArgs []string
	}{
		{goos: "windows", command: "explorer.exe", wantArgs: []string{"/select," + path}},
		{goos: "darwin", command: "open", wantArgs: []string{"-R", path}},
		{
			goos:    "linux",
			command: "dbus-send",
			wantArgs: []string{
				"--session",
				"--dest=org.freedesktop.FileManager1",
				"--type=method_call",
				"--print-reply",
				"/org/freedesktop/FileManager1",
				"org.freedesktop.FileManager1.ShowItems",
				"array:string:file:///music/downloads/Artist/Song.flac",
				"string:",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.goos, func(t *testing.T) {
			command, err := openDownloadFileCommand(test.goos, path)
			if err != nil {
				t.Fatalf("openDownloadFileCommand(%q): %v", test.goos, err)
			}
			if filepath.Base(command.Path) != test.command {
				t.Fatalf("command = %q, want %q", command.Path, test.command)
			}
			if len(command.Args) != len(test.wantArgs)+1 {
				t.Fatalf("command args = %#v, want %#v", command.Args, test.wantArgs)
			}
			for i, want := range test.wantArgs {
				if command.Args[i+1] != want {
					t.Fatalf("command args = %#v, want %#v", command.Args, test.wantArgs)
				}
			}
		})
	}

	for _, goos := range []string{"android", "ios", "plan9"} {
		if _, err := openDownloadFileCommand(goos, path); err == nil {
			t.Fatalf("openDownloadFileCommand(%q) unexpectedly succeeded", goos)
		}
	}
}

func TestResolveDownloadFileRejectsEscapesAndMissingFiles(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "Artist", "Song.flac")
	if err := os.MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inside, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveDownloadFile(root, "Artist/Song.flac")
	if err != nil {
		t.Fatalf("resolveDownloadFile: %v", err)
	}
	resolvedInfo, err := os.Stat(resolved)
	if err != nil || !resolvedInfo.Mode().IsRegular() {
		t.Fatalf("resolved path = %q, stat=%v err=%v", resolved, resolvedInfo, err)
	}

	outside := filepath.Join(t.TempDir(), "Outside.flac")
	if err := os.WriteFile(outside, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveDownloadFile(root, outside); !errors.Is(err, errDownloadPathOutsideRoot) {
		t.Fatalf("outside path error = %v, want path escape rejection", err)
	}
	if _, err := resolveDownloadFile(root, "../Outside.flac"); !errors.Is(err, errDownloadPathOutsideRoot) {
		t.Fatalf("traversal path error = %v, want path escape rejection", err)
	}

	missing := filepath.Join(root, "missing.flac")
	if _, err := resolveDownloadFile(root, missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file error = %v, want os.ErrNotExist", err)
	}
}

func TestResolveDownloadFileAcceptsSavedPathRelativeToWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	savedPath := filepath.Join(root, "Artist", "Song.flac")
	if err := os.MkdirAll(filepath.Dir(savedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(savedPath, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rootRelative, err := filepath.Rel(cwd, root)
	if err != nil {
		t.Fatal(err)
	}
	savedPathRelative, err := filepath.Rel(cwd, savedPath)
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := resolveDownloadFile(rootRelative, savedPathRelative)
	if err != nil {
		t.Fatalf("resolve relative saved path: %v", err)
	}
	resolvedAbs, err := filepath.Abs(resolved)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(resolvedAbs, savedPath) {
		t.Fatalf("resolved path = %q, want %q", resolvedAbs, savedPath)
	}
}

func TestResolveDownloadFileRejectsSymlinkEscapes(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "Outside.flac")
	if err := os.WriteFile(outside, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked.flac")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	if _, err := resolveDownloadFile(root, link); !errors.Is(err, errDownloadPathOutsideRoot) {
		t.Fatalf("symlink escape error = %v, want path escape rejection", err)
	}
}

func TestRevealQueuedDownloadFileSelectsSavedFile(t *testing.T) {
	if err := core.CloseConfigDB(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(t.TempDir(), "settings.db"))
	t.Cleanup(func() {
		_ = core.CloseConfigDB()
	})

	downloadDir := t.TempDir()
	savedPath := filepath.Join(downloadDir, "Artist", "Song.flac")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	downloadDirRelative, err := filepath.Rel(cwd, downloadDir)
	if err != nil {
		t.Fatal(err)
	}
	savedPathRelative, err := filepath.Rel(cwd, savedPath)
	if err != nil {
		t.Fatal(err)
	}
	queue, err := core.OpenDownloadQueue()
	if err != nil {
		t.Fatal(err)
	}
	added, err := queue.Enqueue("reveal-queue", []core.DownloadTaskInput{{
		Song:      model.Song{ID: "song-1", Source: "qq", Name: "Song", Artist: "Artist"},
		Directory: downloadDirRelative,
		Embed:     false,
	}})
	if err != nil || added != 1 {
		t.Fatalf("Enqueue: added=%d err=%v", added, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := queue.Start(ctx, func(core.DownloadTaskInput) (core.DownloadTaskResult, error) {
		if err := os.MkdirAll(filepath.Dir(savedPath), 0o755); err != nil {
			return core.DownloadTaskResult{}, err
		}
		if err := os.WriteFile(savedPath, []byte("audio"), 0o644); err != nil {
			return core.DownloadTaskResult{}, err
		}
		return core.DownloadTaskResult{Status: core.DownloadStatusSuccess, Path: savedPathRelative}, nil
	}); err != nil {
		t.Fatal(err)
	}

	var task core.DownloadTask
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		tasks, _, _, err := queue.List(1, 50, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) == 1 && tasks[0].Status == core.DownloadStatusSuccess {
			task = tasks[0]
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if task.ID == 0 {
		t.Fatal("queued task did not finish")
	}

	var openedPath string
	withOpenDownloadFileLauncher(t, func(path string) error {
		openedPath = path
		return nil
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterDownloadQueueRoutes(router.Group("/music"), queue)
	target := "http://127.0.0.1:8083/music/api/downloads/queue/" + jsonNumber(task.ID) + "/reveal"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, newRevealDownloadRequest(target))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	if openedPath != savedPath {
		t.Fatalf("opened path = %q, want %q", openedPath, savedPath)
	}
}

func TestRevealDownloadRecordFileSelectsSavedFile(t *testing.T) {
	if err := core.CloseConfigDB(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(t.TempDir(), "settings.db"))
	t.Cleanup(func() {
		_ = core.CloseConfigDB()
	})

	downloadDir := t.TempDir()
	relPath := "Artist/Song.flac"
	savedPath := filepath.Join(downloadDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(savedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(savedPath, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.SaveDownloadRecordWithRelPath("Song", "Artist", "qq", core.DownloadStatusSuccess, "", relPath); err != nil {
		t.Fatal(err)
	}
	records, err := core.GetDownloadRecords()
	if err != nil || len(records) != 1 {
		t.Fatalf("GetDownloadRecords: records=%#v err=%v", records, err)
	}

	withOpenDownloadDirectorySettings(t, downloadDir)
	var openedPath string
	withOpenDownloadFileLauncher(t, func(path string) error {
		openedPath = path
		return nil
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST(RoutePrefix+"/api/downloads/records/:id/reveal", revealDownloadRecordFile)
	target := "http://127.0.0.1:8083" + RoutePrefix + "/api/downloads/records/" + jsonNumber(records[0].ID) + "/reveal"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, newRevealDownloadRequest(target))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	if openedPath != savedPath {
		t.Fatalf("opened path = %q, want %q", openedPath, savedPath)
	}
}

func TestDownloadManagerUsesDownloadIconAndPerItemRevealActions(t *testing.T) {
	indexContent, err := templateFS.ReadFile("templates/pages/index.html")
	if err != nil {
		t.Fatal(err)
	}
	modalContent, err := templateFS.ReadFile("templates/partials/modals.html")
	if err != nil {
		t.Fatal(err)
	}
	appContent, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatal(err)
	}

	indexHTML := string(indexContent)
	modalHTML := string(modalContent)
	appJS := string(appContent)
	if strings.Contains(indexHTML, `onclick="openDownloadDirectory()"`) {
		t.Fatal("download management should not keep the standalone toolbar directory button")
	}
	if !strings.Contains(indexHTML, `<i class="fa-solid fa-download"></i>`) {
		t.Fatal("download management entry should use the download icon")
	}
	if strings.Contains(indexHTML, `fa-clock-rotate-left`) {
		t.Fatal("download management entry should not use the history icon")
	}
	if strings.Contains(modalHTML, `onclick="openDownloadDirectory()"`) &&
		strings.Contains(modalHTML, `<i class="fa-solid fa-folder-open"></i> 打开目录`) {
		t.Fatal("download history toolbar should not keep a separate open-directory action")
	}
	for _, want := range []string{
		"data-reveal-download-queue=",
		"data-reveal-download-record=",
		"`${API_ROOT}/api/downloads/queue/${numericID}/reveal`",
		"`${API_ROOT}/api/downloads/records/${numericID}/reveal`",
		"打开文件目录",
	} {
		if !strings.Contains(appJS, want) {
			t.Fatalf("download manager frontend missing %q", want)
		}
	}
}

func jsonNumber(value uint) string {
	data, _ := json.Marshal(value)
	return string(data)
}
