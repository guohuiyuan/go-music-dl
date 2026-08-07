package soda

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

func GetDownloadInfo(s *model.Song) (*DownloadInfo, error) { return defaultSoda.GetDownloadInfo(s) }

func GetDownloadURL(s *model.Song) (string, error) { return defaultSoda.GetDownloadURL(s) }

func Download(s *model.Song, outputPath string) error { return defaultSoda.Download(s, outputPath) }

func (s *Soda) GetDownloadInfo(song *model.Song) (*DownloadInfo, error) {
	if song == nil {
		return nil, errors.New("song is nil")
	}

	if song.Source != "" && song.Source != "soda" {
		return nil, errors.New("source mismatch")
	}

	trackID := sodaSongTrackID(song)
	cachedInfo := sodaCachedDownloadInfo(song)
	if trackID != "" {
		info, err := s.resolveDownloadInfo(trackID, nil)
		if err == nil {
			return info, nil
		}
		if cachedInfo != nil && cachedInfo.URL != "" {
			return cachedInfo, nil
		}
		return nil, err
	}
	if cachedInfo != nil && cachedInfo.URL != "" {
		return cachedInfo, nil
	}
	return nil, errors.New("track id is empty")
}

func sodaCachedDownloadInfo(song *model.Song) *DownloadInfo {
	if song == nil || !strings.Contains(song.URL, "#auth=") {
		return nil
	}
	parts := strings.Split(song.URL, "#auth=")
	if len(parts) != 2 {
		return nil
	}
	auth, _ := url.QueryUnescape(parts[1])
	quality := ""
	if song.Extra != nil {
		quality = strings.TrimSpace(song.Extra["quality"])
	}
	return &DownloadInfo{
		URL:      parts[0],
		PlayAuth: auth,
		Format:   song.Ext,
		Size:     song.Size,
		Duration: float64(song.Duration),
		Bitrate:  song.Bitrate,
		Quality:  quality,
	}
}

func sodaSongTrackID(song *model.Song) string {
	if song == nil {
		return ""
	}
	if song.Extra != nil && strings.TrimSpace(song.Extra["track_id"]) != "" {
		return strings.TrimSpace(song.Extra["track_id"])
	}
	return strings.TrimSpace(song.ID)
}

func (s *Soda) resolveDownloadInfo(trackID string, webResp *sodaTrackV2Response) (*DownloadInfo, error) {
	trackID = strings.TrimSpace(trackID)
	if trackID == "" {
		return nil, errors.New("track id is empty")
	}

	var err error
	if webResp == nil {
		webResp, err = s.fetchWebTrackV2(trackID)
		if err != nil {
			return nil, err
		}
	}

	track := webResp.primaryTrack()
	fullDuration := sodaTrackDurationSeconds(track)
	isVIPTrack := track.LabelInfo.IsVIP()

	var lastErr error
	var webInfo *DownloadInfo
	if videoInfo, ok := sodaBestFromVideoModel(webResp.TrackPlayer.VideoModel); ok {
		webInfo = videoInfo
	}
	if (webInfo == nil || sodaDownloadInfoIsPreview(webInfo, fullDuration)) && webResp.TrackPlayer.URLPlayerInfo != "" {
		webInfo, err = s.fetchPlayerInfo(webResp.TrackPlayer.URLPlayerInfo)
		if err != nil {
			lastErr = err
		}
	}

	webIsPreview := sodaDownloadInfoIsPreview(webInfo, fullDuration)
	if strings.TrimSpace(s.cookie) != "" && (isVIPTrack || webIsPreview || !sodaDownloadInfoIsLossless(webInfo)) {
		if pcResp, pcErr := s.fetchPCTrackV2(trackID); pcErr == nil {
			pcTrack := pcResp.primaryTrack()
			if fullDuration == 0 {
				fullDuration = sodaTrackDurationSeconds(pcTrack)
			}
			if pcTrack.LabelInfo.IsVIP() {
				isVIPTrack = true
			}

			if videoInfo, ok := sodaBestFromVideoModel(pcResp.TrackPlayer.VideoModel); ok {
				if !sodaDownloadInfoIsPreview(videoInfo, fullDuration) {
					if isVIPTrack {
						isVip := true
						s.isVipCache = &isVip
					}
					return videoInfo, nil
				}
				lastErr = errors.New("soda pc track_v2 returned preview stream")
			}

			if pcResp.TrackPlayer.URLPlayerInfo != "" {
				if pcInfo, infoErr := s.fetchPlayerInfo(pcResp.TrackPlayer.URLPlayerInfo); infoErr == nil {
					if !sodaDownloadInfoIsPreview(pcInfo, fullDuration) {
						if isVIPTrack {
							isVip := true
							s.isVipCache = &isVip
						}
						return pcInfo, nil
					}
					lastErr = errors.New("soda pc track_v2 returned preview stream")
				} else {
					lastErr = infoErr
				}
			} else {
				lastErr = errors.New("soda pc track_v2 missing player info url")
			}
		} else {
			lastErr = pcErr
		}
	}

	if webInfo != nil && webInfo.URL != "" {
		if isVIPTrack && webIsPreview {
			if strings.TrimSpace(s.cookie) != "" {
				isVip := false
				s.isVipCache = &isVip
			}
			if lastErr != nil {
				return nil, fmt.Errorf("soda vip full stream unavailable: %w", lastErr)
			}
			if strings.TrimSpace(s.cookie) == "" {
				return nil, errors.New("soda vip download requires cookie")
			}
			return nil, errors.New("soda vip full stream unavailable")
		}
		return webInfo, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("player info url not found")
}

func sodaDownloadInfoIsLossless(info *DownloadInfo) bool {
	if info == nil {
		return false
	}
	return sodaQualityRank(info.Quality, info.Format, info.Bitrate) >= 100
}

// GetDownloadURL 返回下载链接
func (s *Soda) GetDownloadURL(song *model.Song) (string, error) {
	info, err := s.GetDownloadInfo(song)
	if err != nil {
		return "", err
	}
	return sodaDownloadInfoURL(info), nil
}

// Download 下载并解密歌曲
func (s *Soda) Download(song *model.Song, outputPath string) error {
	info, err := s.GetDownloadInfo(song)
	if err != nil {
		return fmt.Errorf("get download info failed: %w", err)
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", info.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download status: %d", resp.StatusCode)
	}

	encryptedData, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	decryptedData, err := DecryptAudio(encryptedData, info.PlayAuth)
	if err != nil {
		return fmt.Errorf("decrypt failed: %w", err)
	}

	err = os.WriteFile(outputPath, decryptedData, 0644)
	if err != nil {
		return err
	}
	return nil
}
