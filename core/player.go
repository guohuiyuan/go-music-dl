package core

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

// ResolveFFplayPath 解析 ffplay 可执行文件路径。
// 顺序：MUSIC_DL_FFPLAY 环境变量 → 与 ffmpeg 同目录下的 ffplay → PATH。
func ResolveFFplayPath() (string, error) {
	if configured := strings.TrimSpace(os.Getenv(ffplayEnvName)); configured != "" {
		return validateConfiguredMediaTool(ffplayEnvName, configured)
	}

	if ffmpegPath, err := ResolveFFmpegPath(); err == nil && ffmpegPath != "" {
		name := "ffplay"
		if ext := filepath.Ext(ffmpegPath); ext != "" {
			name += ext
		}
		candidate := filepath.Join(filepath.Dir(ffmpegPath), name)
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return exec.LookPath("ffplay")
}

// playbackUserAgent 返回与 BuildSourceRequest 一致的 UA。
func playbackUserAgent(source string) string {
	if source == "migu" {
		return UA_Mobile
	}
	return UA_Common
}

// playbackReferer 返回与 BuildSourceRequest 一致的 Referer，无则返回空串。
func playbackReferer(source string) string {
	switch source {
	case "bilibili":
		return Ref_Bilibili
	case "netease":
		return Ref_Netease
	case "migu":
		return Ref_Migu
	case "qq":
		return "http://y.qq.com"
	}
	return ""
}

// PlaybackArgs 构造 ffplay 的命令行参数，注入与下载一致的 UA / Referer / Cookie。
func PlaybackArgs(song *model.Song, urlStr string) []string {
	args := []string{"-nodisp", "-autoexit", "-loglevel", "quiet"}

	source := ""
	if song != nil {
		source = song.Source
	}

	args = append(args, "-user_agent", playbackUserAgent(source))

	var headers strings.Builder
	if referer := playbackReferer(source); referer != "" {
		headers.WriteString("Referer: " + referer + "\r\n")
	}
	if cookie := CM.Get(source); cookie != "" {
		headers.WriteString("Cookie: " + cookie + "\r\n")
	}
	if headers.Len() > 0 {
		args = append(args, "-headers", headers.String())
	}

	args = append(args, urlStr)
	return args
}

// PreparePlaybackSource 为播放准备音频来源。
// 对 soda 等加密源，下载解密为临时文件并返回其路径（tempFile 非空，调用方负责删除）；
// 其余源返回可直接播放的直链 URL（tempFile 为空）。
func PreparePlaybackSource(song *model.Song) (playURL string, tempFile string, err error) {
	if song == nil {
		return "", "", errors.New("song is nil")
	}
	if strings.TrimSpace(song.ID) == "" || strings.TrimSpace(song.Source) == "" {
		return "", "", errors.New("missing song id or source")
	}

	if song.Source == "soda" {
		data, decErr := FetchDecryptedSodaAudio(song)
		if decErr != nil {
			return "", "", decErr
		}
		ext := DetectAudioExt(data)
		f, createErr := os.CreateTemp("", "gomusicdl-play-*."+ext)
		if createErr != nil {
			return "", "", createErr
		}
		path := f.Name()
		if _, writeErr := f.Write(data); writeErr != nil {
			f.Close()
			os.Remove(path)
			return "", "", writeErr
		}
		f.Close()
		return path, path, nil
	}

	dlFunc := GetDownloadFunc(song.Source)
	if dlFunc == nil {
		return "", "", fmt.Errorf("unsupported source: %s", song.Source)
	}
	urlStr, err := dlFunc(song)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(urlStr) == "" {
		return "", "", errors.New("empty download url")
	}
	return urlStr, "", nil
}
