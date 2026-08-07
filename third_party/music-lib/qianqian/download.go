package qianqian

import (
	"errors"
	"github.com/guohuiyuan/music-lib/model"
)

func GetDownloadURL(s *model.Song) (string, error) { return defaultQianqian.GetDownloadURL(s) }

// GetDownloadURL 获取下载链接
func (q *Qianqian) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "qianqian" {
		return "", errors.New("source mismatch")
	}
	if s.URL != "" {
		return s.URL, nil
	}

	tsid := s.ID
	if s.Extra != nil && s.Extra["tsid"] != "" {
		tsid = s.Extra["tsid"]
	}

	return q.fetchDownloadURL(tsid)
}
