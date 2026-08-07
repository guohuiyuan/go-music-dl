package soda

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"strings"
)

func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultSoda.SearchPlaylist(keyword)
}

func GetPlaylistSongs(id string) ([]model.Song, error) {
	// 复用 fetchPlaylistDetail，只返回歌曲列表
	_, songs, err := defaultSoda.fetchPlaylistDetail(id)
	return songs, err
}

func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultSoda.ParsePlaylist(link)
}

// GetRecommendedPlaylists [新增] 获取推荐歌单 (空实现)
func GetRecommendedPlaylists() ([]model.Playlist, error) {
	return defaultSoda.GetRecommendedPlaylists()
}

func GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return defaultSoda.GetPlaylistCategories()
}

func GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return defaultSoda.GetCategoryPlaylists(categoryID, page, limit)
}

func (s *Soda) GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return nil, model.ErrPlaylistCategoriesUnsupported
}

func (s *Soda) GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return nil, model.ErrPlaylistCategoriesUnsupported
}

func (s *Soda) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("q", keyword)
	params.Set("cursor", "0")
	params.Set("search_method", "input")
	params.Set("aid", "386088")
	params.Set("device_platform", "web")
	params.Set("channel", "pc_web")

	apiURL := "https://api.qishui.com/luna/pc/search/playlist?" + params.Encode()

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
					Playlist struct {
						ID    string `json:"id"`
						Title string `json:"title"`
						Desc  string `json:"desc"`
						Owner struct {
							Nickname   string `json:"nickname"`
							PublicName string `json:"public_name"`
						} `json:"owner"`
						CountTracks int `json:"count_tracks"`
						UrlCover    struct {
							Urls []string `json:"urls"`
							Uri  string   `json:"uri"`
						} `json:"url_cover"`
					} `json:"playlist"`
				} `json:"entity"`
			} `json:"data"`
		} `json:"result_groups"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda playlist json parse error: %w", err)
	}

	var playlists []model.Playlist
	if len(resp.ResultGroups) == 0 || len(resp.ResultGroups[0].Data) == 0 {
		return nil, nil
	}

	for _, item := range resp.ResultGroups[0].Data {
		pl := item.Entity.Playlist
		if pl.ID == "" {
			continue
		}

		cover := ""
		if len(pl.UrlCover.Urls) > 0 {
			domain := pl.UrlCover.Urls[0]
			if pl.UrlCover.Uri != "" && !strings.Contains(domain, pl.UrlCover.Uri) {
				cover = domain + pl.UrlCover.Uri
			} else {
				cover = domain
			}
			if cover != "" && !strings.Contains(cover, "~") {
				cover += "~c5_300x300.jpg"
			}
		}

		creator := pl.Owner.PublicName
		if creator == "" {
			creator = pl.Owner.Nickname
		}

		playlists = append(playlists, model.Playlist{
			Source:      "soda",
			ID:          pl.ID,
			Name:        pl.Title,
			Cover:       cover,
			TrackCount:  pl.CountTracks,
			Creator:     creator,
			Description: pl.Desc,
			Link:        fmt.Sprintf("https://www.qishui.com/playlist/%s", pl.ID),
		})
	}
	return playlists, nil
}

func (s *Soda) GetPlaylistSongs(id string) ([]model.Song, error) {
	_, songs, err := s.fetchPlaylistDetail(id)
	return songs, err
}

func (s *Soda) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	playlistID, err := s.extractPlaylistID(link)
	if err != nil || playlistID == "" {
		return nil, nil, errors.New("invalid soda playlist link")
	}
	return s.fetchPlaylistDetail(playlistID)
}

// GetRecommendedPlaylists [新增] 获取推荐歌单 (空实现)
func (s *Soda) GetRecommendedPlaylists() ([]model.Playlist, error) {
	// 汽水音乐目前没有公开的每日推荐歌单 PC 接口
	return nil, errors.New("soda daily recommendation not supported")
}
