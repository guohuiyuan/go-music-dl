package joox

import (
	"errors"
	"github.com/guohuiyuan/music-lib/model"
)

func GetDownloadURL(s *model.Song) (string, error) { return defaultJoox.GetDownloadURL(s) }

// GetDownloadURL 获取下载链接
func (j *Joox) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "joox" {
		return "", errors.New("source mismatch")
	}
	if s.URL != "" {
		return s.URL, nil
	}

	songID := s.ID
	if s.Extra != nil && s.Extra["songid"] != "" {
		songID = s.Extra["songid"]
	}

	// 复用核心逻辑
	info, err := j.fetchSongInfo(songID)
	if err != nil {
		return "", err
	}
	return info.URL, nil
}
