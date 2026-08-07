package kuwo

import (
	"errors"
	"github.com/guohuiyuan/music-lib/model"
)

func GetDownloadURL(s *model.Song) (string, error) { return defaultKuwo.GetDownloadURL(s) }

// GetDownloadURL 获取下载链接
func (k *Kuwo) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "kuwo" {
		return "", errors.New("source mismatch")
	}
	if s.URL != "" {
		return s.URL, nil
	}

	rid := s.ID
	if s.Extra != nil && s.Extra["rid"] != "" {
		rid = s.Extra["rid"]
	}

	return k.fetchAudioURL(rid)
}
