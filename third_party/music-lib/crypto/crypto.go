package crypto

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qq"
)

// DecryptByFilename 根据文件扩展名解密已购加密音频。
// 返回值：解密后的音频数据、推荐输出扩展名、来源平台。
func DecryptByFilename(filename string, encrypted []byte) ([]byte, string, string, error) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))

	switch ext {
	case "ncm":
		plain, outExt, err := netease.DecryptNCM(encrypted)
		if err != nil {
			return nil, "", "", err
		}
		if outExt == "" {
			outExt = "mp3"
		}
		return plain, outExt, "netease", nil

	case "qmc0", "qmc3", "qmcflac", "qmcogg", "bkcmp3", "bkcflac", "tkm", "mflac", "mgg":
		plain, outExt, err := qq.DecryptQQ(encrypted, ext)
		if err != nil {
			return nil, "", "", err
		}
		return plain, outExt, "qq", nil
	}

	if len(encrypted) >= 8 && string(encrypted[:8]) == "CTENFDAM" {
		plain, outExt, err := netease.DecryptNCM(encrypted)
		if err != nil {
			return nil, "", "", err
		}
		if outExt == "" {
			outExt = "mp3"
		}
		return plain, outExt, "netease", nil
	}

	return nil, "", "", fmt.Errorf("unsupported encrypted format: %s", ext)
}

func MimeByExt(ext string) string {
	switch strings.ToLower(ext) {
	case "flac":
		return "audio/flac"
	case "ogg":
		return "audio/ogg"
	case "m4a":
		return "audio/mp4"
	default:
		return "audio/mpeg"
	}
}

func DetectAudioExt(data []byte) string {
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{'f', 'L', 'a', 'C'}) {
		return "flac"
	}
	if len(data) >= 3 && bytes.Equal(data[:3], []byte{'I', 'D', '3'}) {
		return "mp3"
	}
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{'O', 'g', 'g', 'S'}) {
		return "ogg"
	}
	if len(data) >= 8 && bytes.Equal(data[4:8], []byte{'f', 't', 'y', 'p'}) {
		return "m4a"
	}
	return "mp3"
}