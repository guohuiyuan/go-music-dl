package web

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"gorm.io/gorm"
)

var (
	openDownloadDirectory         = launchOpenDownloadDirectory
	openDownloadFile              = launchOpenDownloadFile
	openDownloadDirectorySettings = core.GetWebSettings
	errDownloadPathOutsideRoot    = errors.New("download file is outside the download directory")
)

func prepareOpenDownloadDirectory(configured string) (string, error) {
	target := core.DownloadDirOrDefault(configured)
	absolute, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve download directory: %w", err)
	}
	if err := os.MkdirAll(absolute, 0o755); err != nil {
		return "", fmt.Errorf("create download directory: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("inspect download directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("download path is not a directory: %s", absolute)
	}
	return absolute, nil
}

func launchOpenDownloadDirectory(path string) error {
	command, err := openDownloadDirectoryCommand(runtime.GOOS, path)
	if err != nil {
		return err
	}
	return startFileManagerCommand(command)
}

func launchOpenDownloadFile(path string) error {
	command, err := openDownloadFileCommand(runtime.GOOS, path)
	if err != nil {
		return err
	}
	launchErr := startFileManagerCommand(command)
	if launchErr == nil || runtime.GOOS != "linux" {
		return launchErr
	}
	fallback, err := openDownloadDirectoryCommand("linux", filepath.Dir(path))
	if err != nil {
		return launchErr
	}
	if fallbackErr := startFileManagerCommand(fallback); fallbackErr == nil {
		return nil
	}
	return launchErr
}

func startFileManagerCommand(command *exec.Cmd) error {
	if err := command.Start(); err != nil {
		return fmt.Errorf("start file manager: %w", err)
	}
	go func() {
		_ = command.Wait()
	}()
	return nil
}

func openDownloadDirectoryCommand(goos, path string) (*exec.Cmd, error) {
	return openDownloadLocationCommand(goos, path, false)
}

func openDownloadFileCommand(goos, path string) (*exec.Cmd, error) {
	return openDownloadLocationCommand(goos, path, true)
}

func openDownloadLocationCommand(goos, path string, selectFile bool) (*exec.Cmd, error) {
	switch goos {
	case "windows":
		if selectFile {
			return exec.Command("explorer.exe", "/select,"+path), nil
		}
		return exec.Command("explorer.exe", path), nil
	case "darwin":
		if selectFile {
			return exec.Command("open", "-R", path), nil
		}
		return exec.Command("open", path), nil
	case "linux":
		if selectFile {
			fileURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
			return exec.Command(
				"dbus-send",
				"--session",
				"--dest=org.freedesktop.FileManager1",
				"--type=method_call",
				"--print-reply",
				"/org/freedesktop/FileManager1",
				"org.freedesktop.FileManager1.ShowItems",
				"array:string:"+fileURL,
				"string:",
			), nil
		}
		return exec.Command("xdg-open", path), nil
	case "android", "ios":
		return nil, fmt.Errorf("opening the host download directory is not supported on %s", goos)
	default:
		return nil, fmt.Errorf("opening the download directory is not supported on %s", goos)
	}
}

func isLoopbackOpenDownloadRequest(request *http.Request) bool {
	if request == nil {
		return false
	}
	if !isLoopbackHost(request.Host) || !isLoopbackHost(request.RemoteAddr) {
		return false
	}
	// A local reverse proxy can make the socket appear loopback even when the
	// originating browser is remote. The desktop application never sends these
	// headers, so reject any forwarded request instead of trusting proxy config.
	for _, header := range []string{"Forwarded", "X-Forwarded-For", "X-Real-IP"} {
		if strings.TrimSpace(request.Header.Get(header)) != "" {
			return false
		}
	}
	return true
}

func isLoopbackHost(address string) bool {
	address = strings.TrimSpace(address)
	if address == "" {
		return false
	}
	host := address
	if parsedHost, _, err := net.SplitHostPort(address); err == nil {
		host = parsedHost
	}
	host = strings.Trim(strings.TrimSuffix(host, "."), "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func resolveDownloadFile(root, target string) (string, error) {
	root = core.DownloadDirOrDefault(root)
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve download directory: %w", err)
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return "", errors.New("download file path is unavailable")
	}

	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", fmt.Errorf("resolve download directory symlinks: %w", err)
	}

	candidates, err := downloadFileCandidates(rootAbs, target)
	if err != nil {
		return "", err
	}

	var (
		lastMissing error
		sawOutside  bool
	)
	for _, targetAbs := range candidates {
		if !pathWithinRoot(rootAbs, targetAbs) {
			sawOutside = true
			continue
		}
		targetReal, err := filepath.EvalSymlinks(targetAbs)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				lastMissing = err
				continue
			}
			return "", fmt.Errorf("resolve download file symlinks: %w", err)
		}
		if !pathWithinRoot(rootReal, targetReal) {
			sawOutside = true
			continue
		}
		info, err := os.Stat(targetReal)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				lastMissing = err
				continue
			}
			return "", err
		}
		if info.Mode().IsRegular() {
			return targetAbs, nil
		}
	}
	if sawOutside {
		return "", fmt.Errorf("%w: %s", errDownloadPathOutsideRoot, target)
	}
	if lastMissing != nil {
		return "", fmt.Errorf("resolve download file symlinks: %w", lastMissing)
	}
	return "", fmt.Errorf("download path is not a file: %s", target)
}

// downloadFileCandidates supports the two path forms persisted by older and
// current queue rows. A relative SavedPath is resolved from the server working
// directory first (for example data/downloads/Song.flac); a history RelPath is
// resolved from the configured download root (for example Artist/Song.flac).
func downloadFileCandidates(rootAbs, target string) ([]string, error) {
	candidates := make([]string, 0, 2)
	if filepath.IsAbs(target) {
		targetAbs, err := filepath.Abs(target)
		if err != nil {
			return nil, fmt.Errorf("resolve download file: %w", err)
		}
		return []string{targetAbs}, nil
	}

	workingDirTarget, err := filepath.Abs(target)
	if err != nil {
		return nil, fmt.Errorf("resolve download file: %w", err)
	}
	candidates = append(candidates, filepath.Clean(workingDirTarget))

	rootRelativeTarget := filepath.Clean(filepath.Join(rootAbs, filepath.FromSlash(target)))
	if workingDirTarget != rootRelativeTarget {
		candidates = append(candidates, rootRelativeTarget)
	}
	return candidates, nil
}

func pathWithinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(rel) || rel == "." {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func allowLocalDownloadOpen(c *gin.Context) bool {
	if c.Request.Method != http.MethodPost {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "opening a download location requires POST"})
		return false
	}
	if !allowSameOriginWrite(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return false
	}
	if !isLoopbackOpenDownloadRequest(c.Request) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "opening download locations is only available on the computer running go-music-dl",
		})
		return false
	}
	return true
}

func openDownloadDirectoryHandler(c *gin.Context) {
	if !allowLocalDownloadOpen(c) {
		return
	}

	target, err := prepareOpenDownloadDirectory(openDownloadDirectorySettings().DownloadDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := openDownloadDirectory(target); err != nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"path":   target,
	})
}

func revealQueuedDownloadFile(c *gin.Context, queue *core.DownloadQueue, id uint) {
	if !allowLocalDownloadOpen(c) {
		return
	}
	task, err := queue.Get(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "download task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if task.Status != core.DownloadStatusSuccess && task.Status != core.DownloadStatusSkipped {
		c.JSON(http.StatusConflict, gin.H{"error": "download file is not available for this task status"})
		return
	}
	target, err := resolveDownloadFile(task.Directory, task.Path)
	if err != nil {
		writeRevealDownloadFileError(c, err)
		return
	}
	if err := openDownloadFile(target); err != nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "path": target})
}

func revealDownloadRecordFile(c *gin.Context) {
	if !allowLocalDownloadOpen(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid download record id"})
		return
	}
	record, err := core.GetDownloadRecord(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "download record not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if record.Status != core.DownloadStatusSuccess && record.Status != core.DownloadStatusSkipped {
		c.JSON(http.StatusConflict, gin.H{"error": "download file is not available for this record status"})
		return
	}
	relPath, err := core.ResolveDownloadRecordRelPath(record)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if relPath == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "download file path is unavailable for this record"})
		return
	}
	target, err := resolveDownloadFile(openDownloadDirectorySettings().DownloadDir, relPath)
	if err != nil {
		writeRevealDownloadFileError(c, err)
		return
	}
	if err := openDownloadFile(target); err != nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "path": target})
}

func writeRevealDownloadFileError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errDownloadPathOutsideRoot):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, os.ErrNotExist):
		c.JSON(http.StatusNotFound, gin.H{"error": "download file no longer exists"})
	default:
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	}
}
