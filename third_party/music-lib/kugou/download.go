package kugou

import (
	"errors"
	"github.com/guohuiyuan/music-lib/model"
	"strings"
)

func GetDownloadURL(s *model.Song) (string, error) { return defaultKugou.GetDownloadURL(s) }

func GetDownloadURLBySonginfo(s *model.Song) (string, error) {
	return defaultKugou.GetDownloadURLBySonginfo(s)
}

// GetDownloadURL 获取下载链接
func (k *Kugou) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "kugou" {
		return "", errors.New("source mismatch")
	}
	if s.URL != "" {
		return s.URL, nil
	}

	hash := s.ID
	if s.Extra != nil && s.Extra["hash"] != "" {
		hash = s.Extra["hash"]
	}

	privilege := getKugouPrivilege(s)

	if shouldTryKugouHighQualityDownload(k.cookie, privilege) {
		if info, err := k.fetchVIPSongInfo(s); err == nil && info != nil && info.URL != "" {
			return info.URL, nil
		}
	}

	isVip, vipErr := k.IsVipAccount()
	if vipErr == nil && isVip {
		if info, err := k.fetchTrackerSongInfo(hash); err == nil && info != nil && info.URL != "" {
			return info.URL, nil
		}
	}

	info, err := k.fetchSongInfo(hash)
	if err != nil {
		return "", err
	}
	return info.URL, nil
}

func (k *Kugou) GetDownloadURLBySonginfo(s *model.Song) (string, error) {
	var lastErr error
	for _, hash := range collectCandidateHashes(s) {
		info, err := k.fetchSonginfoV2(hash)
		if err != nil {
			lastErr = err
			continue
		}
		if info != nil && strings.TrimSpace(info.URL) != "" {
			return info.URL, nil
		}
		lastErr = errors.New("kugou songinfo v2 download url not found")
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", errors.New("kugou songinfo v2 download url not found")
}

func shouldTryKugouHighQualityDownload(cookie string, privilege int) bool {
	if privilege == 10 || privilege == 8 {
		return true
	}
	return kugouHasAppCookie(parseKugouCookie(cookie))
}
