package web

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
)

var (
	openDownloadDirectory         = launchOpenDownloadDirectory
	openDownloadDirectorySettings = core.GetWebSettings
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
	if err := command.Start(); err != nil {
		return fmt.Errorf("start file manager: %w", err)
	}
	go func() {
		_ = command.Wait()
	}()
	return nil
}

func openDownloadDirectoryCommand(goos, path string) (*exec.Cmd, error) {
	switch goos {
	case "windows":
		return exec.Command("explorer.exe", path), nil
	case "darwin":
		return exec.Command("open", path), nil
	case "linux":
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

func openDownloadDirectoryHandler(c *gin.Context) {
	if c.Request.Method != http.MethodPost {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "open_download_dir requires POST"})
		return
	}
	if !allowSameOriginWrite(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if !isLoopbackOpenDownloadRequest(c.Request) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "opening the download directory is only available on the computer running go-music-dl",
		})
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
