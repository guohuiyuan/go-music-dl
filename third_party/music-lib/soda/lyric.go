package soda

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
)

func GetLyrics(s *model.Song) (string, error) { return defaultSoda.GetLyrics(s) }

// GetLyrics 获取歌词
func (s *Soda) GetLyrics(song *model.Song) (string, error) {
	if song.Source != "soda" {
		return "", errors.New("source mismatch")
	}

	trackID := song.ID
	if song.Extra != nil && song.Extra["track_id"] != "" {
		trackID = song.Extra["track_id"]
	}

	params := url.Values{}
	params.Set("track_id", trackID)
	params.Set("media_type", "track")
	params.Set("aid", "386088")
	params.Set("device_platform", "web")
	params.Set("channel", "pc_web")

	v2URL := "https://api.qishui.com/luna/pc/track_v2?" + params.Encode()
	body, err := utils.Get(v2URL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil {
		return "", fmt.Errorf("failed to fetch lyric API: %w", err)
	}

	var resp struct {
		Lyric struct {
			Content string `json:"content"`
		} `json:"lyric"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse lyric JSON: %w", err)
	}
	if resp.Lyric.Content == "" {
		return "", nil
	}

	return parseSodaLyric(resp.Lyric.Content), nil
}
