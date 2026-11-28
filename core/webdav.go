package core

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
)

const (
	DefaultWebDAVDir     = "music-dl"
	webDAVUploadTimeout  = 5 * time.Minute
	webDAVErrorBodyLimit = 4096
)

var webDAVHTTPClient = &http.Client{Timeout: webDAVUploadTimeout}

// WebDAVConfigured reports whether WebDAV should be used for downloads.
func WebDAVConfigured(settings WebSettings) bool {
	return settings.WebDAVEnabled && strings.TrimSpace(settings.WebDAVURL) != ""
}

// UploadSongToWebDAV uploads a downloaded song using the configured WebDAV
// server. The remote file keeps the same relative layout as the local download.
func UploadSongToWebDAV(settings WebSettings, filename string, data []byte) error {
	if len(data) == 0 {
		return errors.New("empty webdav upload data")
	}
	if strings.TrimSpace(filename) == "" {
		return errors.New("empty webdav upload filename")
	}
	return uploadBytesToWebDAV(settings, filename, data)
}

func uploadBytesToWebDAV(settings WebSettings, filename string, data []byte) error {
	if !WebDAVConfigured(settings) {
		return nil
	}

	base, err := parseWebDAVBaseURL(settings.WebDAVURL)
	if err != nil {
		return err
	}

	remoteRel, err := webDAVRemoteRelativePath(settings, filename)
	if err != nil {
		return err
	}

	if err := ensureWebDAVDirectories(webDAVHTTPClient, base, settings, remoteRel); err != nil {
		return err
	}

	target := *base
	target.Path = strings.TrimRight(base.Path, "/") + "/" + remoteRel
	target.RawQuery = ""
	target.Fragment = ""

	req, err := http.NewRequest(http.MethodPut, target.String(), bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build webdav request: %w", err)
	}
	req.SetBasicAuth(settings.WebDAVUsername, settings.WebDAVPassword)
	if ext := strings.TrimPrefix(path.Ext(filename), "."); ext != "" {
		if mime := AudioMimeByExt(ext); mime != "" {
			req.Header.Set("Content-Type", mime)
		}
	}

	resp, err := webDAVHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload to webdav: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, webDAVErrorBodyLimit))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return fmt.Errorf("upload to webdav failed: %s", message)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// webDAVOnlyFetcher allows tests to stub the audio download step so the
// WebDAV-only path can be exercised without hitting real music sources.
var webDAVOnlyFetcher = DownloadSongDataWithTemplate

func DownloadSongToWebDAVOnly(song *model.Song, withCover, withLyrics bool, filenameTemplate string, dedupSet DownloadDedupIndex) (*DownloadedSong, error) {
	settings := GetWebSettings()
	if !WebDAVConfigured(settings) {
		return nil, errors.New("webdav not configured")
	}
	key := SongKey(song)
	// 先查 WebDAV 云端是否已存在该文件（而不是沿用本地下载记录）：
	// 本地已有不代表云端已有，反之亦然。下载前不知道最终扩展名（由音质决定），
	// 因此按常见音频扩展名逐个探测。
	probeBase := BuildDownloadFilename(song, "", filenameTemplate)
	probeName := ""
	if strings.TrimSpace(probeBase) != "" {
		for _, ext := range []string{".flac", ".mp3", ".m4a", ".wav", ".ogg", ".wma"} {
			if webDAVFileExists(settings, probeBase+ext) {
				probeName = probeBase + ext
				break
			}
		}
	}
	if probeName != "" {
		_ = SaveDownloadRecordWithRelPath(song.Name, song.Artist, song.Source, DownloadStatusSkipped, "", "")
		if dedupSet != nil {
			dedupSet[key] = ""
		}
		return &DownloadedSong{Skipped: true, Filename: probeName, Warning: "WebDAV 云端已存在 " + probeName + "，跳过；如需重传请先删除云端同名文件"}, nil
	}
	result, err := webDAVOnlyFetcher(song, withCover, withLyrics, filenameTemplate)
	if err != nil {
		_ = SaveDownloadRecord(song.Name, song.Artist, song.Source, DownloadStatusFailed, err.Error())
		return nil, err
	}
	if err := UploadSongToWebDAV(settings, result.Filename, result.Data); err != nil {
		_ = SaveDownloadRecord(song.Name, song.Artist, song.Source, DownloadStatusFailed, "webdav: "+err.Error())
		return result, err
	}
	if strings.TrimSpace(result.Lyric) != "" && settings.DownloadLyricsToLocal {
		lrcName := strings.TrimSuffix(result.Filename, path.Ext(result.Filename)) + ".lrc"
		lrcContent := StandardLRCForExport(result.Lyric)
		if lrcContent == "" {
			lrcContent = result.Lyric
		}
		if lrcErr := uploadBytesToWebDAV(settings, lrcName, []byte(lrcContent)); lrcErr != nil {
			result.Warning = strings.TrimSpace(result.Warning + " WebDAV lyric: " + lrcErr.Error())
		}
	}
	_ = SaveDownloadRecordWithRelPath(song.Name, song.Artist, song.Source, DownloadStatusSuccess, "", "")
	if dedupSet != nil {
		dedupSet[key] = ""
	}
	result.SavedPath = ""
	return result, nil
}

// webDAVFileExists probes the remote WebDAV server for an existing file.
// The probe uses the configured audio-quality default when the filename has no
// extension yet, trying the common quality variants used by downloads.
func webDAVFileExists(settings WebSettings, filename string) bool {
	base, err := parseWebDAVBaseURL(settings.WebDAVURL)
	if err != nil {
		return false
	}
	rel, err := webDAVRemoteRelativePath(settings, filename)
	if err != nil {
		return false
	}
	target := *base
	target.Path = strings.TrimRight(base.Path, "/") + "/" + rel
	target.RawQuery = ""
	target.Fragment = ""

	req, err := http.NewRequest(http.MethodHead, target.String(), nil)
	if err != nil {
		return false
	}
	req.SetBasicAuth(settings.WebDAVUsername, settings.WebDAVPassword)
	resp, err := webDAVHTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
}

func parseWebDAVBaseURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	base, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid webdav url: %w", err)
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return nil, errors.New("webdav url must start with http:// or https://")
	}
	if base.Host == "" {
		return nil, errors.New("webdav url is missing host")
	}
	base.Path = strings.TrimRight(base.Path, "/")
	return base, nil
}

func webDAVRemoteRelativePath(settings WebSettings, filename string) (string, error) {
	rel := filepath.ToSlash(sanitizeDownloadRelativePath(filename))
	rel = strings.Trim(rel, "/")
	if rel == "" {
		return "", errors.New("webdav upload path is empty")
	}

	dir := strings.Trim(strings.TrimSpace(settings.WebDAVDir), "/")
	if dir != "" {
		rel = path.Join(dir, rel)
	}
	if strings.Trim(rel, "/") == "" {
		return "", errors.New("webdav upload path is empty")
	}
	return rel, nil
}

func ensureWebDAVDirectories(client *http.Client, base *url.URL, settings WebSettings, remoteRel string) error {
	parts := strings.Split(remoteRel, "/")
	if len(parts) <= 1 {
		return nil
	}

	current := base.Path
	for _, part := range parts[:len(parts)-1] {
		if strings.TrimSpace(part) == "" {
			continue
		}
		current = strings.TrimRight(current, "/") + "/" + part
		target := *base
		target.Path = current + "/"
		target.RawQuery = ""
		target.Fragment = ""

		req, err := http.NewRequest("MKCOL", target.String(), nil)
		if err != nil {
			return fmt.Errorf("build webdav directory request: %w", err)
		}
		req.SetBasicAuth(settings.WebDAVUsername, settings.WebDAVPassword)

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("create webdav directory %s: %w", part, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK, http.StatusCreated, http.StatusNoContent, http.StatusMethodNotAllowed:
			continue
		default:
			return fmt.Errorf("create webdav directory %s: status %s", part, resp.Status)
		}
	}
	return nil
}
