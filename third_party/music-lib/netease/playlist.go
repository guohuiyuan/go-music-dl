package netease

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultNetease.SearchPlaylist(keyword)
}

func GetPlaylistSongs(playlistID string) ([]model.Song, error) {
	return defaultNetease.GetPlaylistSongs(playlistID)
}

func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultNetease.ParsePlaylist(link)
}

// GetRecommendedPlaylists returns recommended playlists without login.
func GetRecommendedPlaylists() ([]model.Playlist, error) {
	return defaultNetease.GetRecommendedPlaylists()
}

func GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return defaultNetease.GetPlaylistCategories()
}

func GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return defaultNetease.GetCategoryPlaylists(categoryID, page, limit)
}

func GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	return defaultNetease.GetUserPlaylists(page, limit)
}

// SearchPlaylist searches playlists.
func (n *Netease) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	body, err := n.cloudSearch(keyword, 1000, 10)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			Playlists []struct {
				ID          int    `json:"id"`
				Name        string `json:"name"`
				CoverImgURL string `json:"coverImgUrl"`
				Creator     struct {
					Nickname string `json:"nickname"`
				} `json:"creator"`
				TrackCount  int    `json:"trackCount"`
				PlayCount   int    `json:"playCount"`
				Description string `json:"description"`
			} `json:"playlists"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease playlist json parse error: %w", err)
	}

	var playlists []model.Playlist
	for _, item := range resp.Result.Playlists {
		playlists = append(playlists, model.Playlist{
			Source:      "netease",
			ID:          strconv.Itoa(item.ID),
			Name:        item.Name,
			Cover:       item.CoverImgURL,
			TrackCount:  item.TrackCount,
			PlayCount:   item.PlayCount,
			Creator:     item.Creator.Nickname,
			Description: item.Description,
			Link:        fmt.Sprintf("https://music.163.com/#/playlist?id=%d", item.ID),
		})
	}
	return playlists, nil
}

// GetPlaylistSongs returns songs in a playlist.
func (n *Netease) GetPlaylistSongs(playlistID string) ([]model.Song, error) {
	_, songs, err := n.fetchPlaylistDetail(playlistID)
	return songs, err
}

// ParsePlaylist parses a playlist link.
func (n *Netease) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	kind, playlistID, err := parseNeteaseLink(link)
	if err != nil || kind != neteaseLinkPlaylist {
		return nil, nil, errNeteaseInvalidListLink
	}
	return n.fetchPlaylistDetail(playlistID)
}

// GetRecommendedPlaylists returns homepage recommended playlists.
func (n *Netease) GetRecommendedPlaylists() ([]model.Playlist, error) {
	reqData := map[string]interface{}{
		"limit": 30,
		"total": true,
		"n":     1000,
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post(RecommendedPlaylistAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code   int `json:"code"`
		Result []struct {
			ID         int     `json:"id"`
			Name       string  `json:"name"`
			PicURL     string  `json:"picUrl"`
			PlayCount  float64 `json:"playCount"`
			TrackCount int     `json:"trackCount"`
			Copywriter string  `json:"copywriter"`
			Alg        string  `json:"alg"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease recommended playlist json parse error: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("netease api error code: %d", resp.Code)
	}

	var playlists []model.Playlist
	for _, item := range resp.Result {
		creatorDisplay := "网易云推荐"
		if item.Copywriter != "" {
			creatorDisplay = item.Copywriter
		}

		pl := model.Playlist{
			Source:      "netease",
			ID:          strconv.Itoa(item.ID),
			Name:        item.Name,
			Cover:       item.PicURL,
			PlayCount:   int(item.PlayCount),
			TrackCount:  item.TrackCount,
			Description: item.Copywriter,
			Creator:     creatorDisplay,
			Link:        fmt.Sprintf("https://music.163.com/#/playlist?id=%d", item.ID),
			Extra:       map[string]string{},
		}

		if item.Alg != "" {
			pl.Extra["alg"] = item.Alg
		}

		playlists = append(playlists, pl)
	}

	return playlists, nil
}

func (n *Netease) GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	reqData := map[string]interface{}{
		"csrf_token": "",
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	body, err := utils.Post(PlaylistCategoryAPI, strings.NewReader(form.Encode()),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code       int               `json:"code"`
		Categories map[string]string `json:"categories"`
		All        struct {
			Name          string `json:"name"`
			Hot           bool   `json:"hot"`
			ResourceCount int    `json:"resourceCount"`
		} `json:"all"`
		Sub []struct {
			Name          string `json:"name"`
			Category      int    `json:"category"`
			Hot           bool   `json:"hot"`
			ResourceCount int    `json:"resourceCount"`
		} `json:"sub"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease playlist category json parse error: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("netease api error code: %d", resp.Code)
	}

	categories := make([]model.PlaylistCategory, 0, len(resp.Sub)+1)
	categories = append(categories, model.PlaylistCategory{
		Source: "netease",
		ID:     "",
		Name:   "全部",
		Group:  "全部",
		Count:  resp.All.ResourceCount,
		Hot:    resp.All.Hot,
	})
	for _, item := range resp.Sub {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		group := resp.Categories[strconv.Itoa(item.Category)]
		categories = append(categories, model.PlaylistCategory{
			Source: "netease",
			ID:     name,
			Name:   name,
			Group:  group,
			Count:  item.ResourceCount,
			Hot:    item.Hot,
			Extra: map[string]string{
				"category": strconv.Itoa(item.Category),
			},
		})
	}
	return categories, nil
}

func (n *Netease) GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	categoryID = strings.TrimSpace(categoryID)
	if categoryID == "" {
		categoryID = "全部"
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	reqData := map[string]interface{}{
		"cat":        categoryID,
		"order":      "hot",
		"limit":      limit,
		"offset":     (page - 1) * limit,
		"total":      page == 1,
		"csrf_token": "",
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	body, err := utils.Post(CategoryPlaylistAPI, strings.NewReader(form.Encode()),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code      int `json:"code"`
		Playlists []struct {
			ID          int64   `json:"id"`
			Name        string  `json:"name"`
			CoverImgURL string  `json:"coverImgUrl"`
			TrackCount  int     `json:"trackCount"`
			PlayCount   float64 `json:"playCount"`
			Description string  `json:"description"`
			Creator     struct {
				Nickname string `json:"nickname"`
			} `json:"creator"`
		} `json:"playlists"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease category playlist json parse error: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("netease api error code: %d", resp.Code)
	}

	playlists := make([]model.Playlist, 0, len(resp.Playlists))
	for _, item := range resp.Playlists {
		playlistID := strconv.FormatInt(item.ID, 10)
		playlists = append(playlists, model.Playlist{
			Source:      "netease",
			ID:          playlistID,
			Name:        item.Name,
			Cover:       item.CoverImgURL,
			TrackCount:  item.TrackCount,
			PlayCount:   int(item.PlayCount),
			Creator:     item.Creator.Nickname,
			Description: item.Description,
			Link:        fmt.Sprintf("https://music.163.com/#/playlist?id=%s", playlistID),
			Extra: map[string]string{
				"category_id": categoryID,
			},
		})
	}
	return playlists, nil
}
