package soda

import (
	"fmt"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

func IsVipAccount() (bool, error) { return defaultSoda.IsVipAccount() }

func (s *Soda) IsVipAccount() (bool, error) {
	if s.isVipCache != nil {
		return *s.isVipCache, nil
	}

	if strings.TrimSpace(s.cookie) == "" {
		isVip := false
		s.isVipCache = &isVip
		return false, nil
	}

	info, err := s.GetDownloadInfo(&model.Song{
		ID:     vipProbeTrackID,
		Source: "soda",
		Link:   vipProbeTrackURL,
		Extra: map[string]string{
			"track_id": vipProbeTrackID,
		},
	})
	if err != nil {
		if strings.Contains(err.Error(), "requires cookie") ||
			strings.Contains(err.Error(), "full stream unavailable") ||
			strings.Contains(err.Error(), "returned preview stream") {
			isVip := false
			s.isVipCache = &isVip
			return false, nil
		}
		return false, fmt.Errorf("failed to probe soda vip account: %w", err)
	}

	isVip := info != nil && info.URL != "" && !sodaDownloadInfoIsPreview(info, 180)
	s.isVipCache = &isVip
	return isVip, nil
}
