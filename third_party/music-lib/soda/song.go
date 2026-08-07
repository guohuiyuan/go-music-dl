package soda

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
)

func Search(keyword string) ([]model.Song, error) { return defaultSoda.Search(keyword) }

func Parse(link string) (*model.Song, error) { return defaultSoda.Parse(link) }

// Search 搜索歌曲 (PC API)
func (s *Soda) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("q", keyword)
	params.Set("cursor", "0")
	params.Set("search_method", "input")
	params.Set("aid", "386088")
	params.Set("device_platform", "web")
	params.Set("channel", "pc_web")

	apiURL := "https://api.qishui.com/luna/pc/search/track?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		ResultGroups []struct {
			Data []struct {
				Entity struct {
					Track sodaTrack `json:"track"`
				} `json:"entity"`
			} `json:"data"`
		} `json:"result_groups"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda search json parse error: %w", err)
	}
	if len(resp.ResultGroups) == 0 {
		return nil, nil
	}

	var songs []model.Song
	for _, item := range resp.ResultGroups[0].Data {
		track := item.Entity.Track
		if track.ID == "" {
			continue
		}
		songs = append(songs, sodaBuildSongFromTrack(track))
	}
	return songs, nil
}

// Parse 解析链接并获取完整信息
func (s *Soda) Parse(link string) (*model.Song, error) {
	trackID, err := s.extractTrackID(link)
	if err != nil || trackID == "" {
		return nil, errors.New("invalid soda link")
	}
	return s.fetchSongDetail(trackID)
}
