package bilibili

import (
	"errors"
	"github.com/guohuiyuan/music-lib/model"
	"strings"
)

func GetDownloadURL(s *model.Song) (string, error) { return defaultBilibili.GetDownloadURL(s) }

// GetDownloadURL 获取下载链接
func (b *Bilibili) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "bilibili" {
		return "", errors.New("source mismatch")
	}

	if s.URL != "" {
		return s.URL, nil
	}

	var bvid, cid string
	if s.Extra != nil {
		bvid = s.Extra["bvid"]
		cid = s.Extra["cid"]
	}

	if bvid == "" || cid == "" {
		parts := strings.Split(s.ID, "|")
		if len(parts) == 2 {
			bvid = parts[0]
			cid = parts[1]
		} else {
			return "", errors.New("invalid id structure")
		}
	}

	isVip, _ := b.IsVipAccount()
	return b.fetchAudioURL(bvid, cid, isVip)
}
