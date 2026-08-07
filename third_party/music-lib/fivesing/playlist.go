package fivesing

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"html"
	"net/url"
	"regexp"
	"sync"
)

func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultFivesing.SearchPlaylist(keyword)
}

func GetPlaylistSongs(id string) ([]model.Song, error) {
	return defaultFivesing.GetPlaylistSongs(id)
}

func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultFivesing.ParsePlaylist(link)
}

func GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return defaultFivesing.GetPlaylistCategories()
}

func GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return defaultFivesing.GetCategoryPlaylists(categoryID, page, limit)
}

func (f *Fivesing) GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return nil, model.ErrPlaylistCategoriesUnsupported
}

func (f *Fivesing) GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return nil, model.ErrPlaylistCategoriesUnsupported
}

// SearchPlaylist 搜索歌单
func (f *Fivesing) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("keyword", keyword)
	params.Set("sort", "1")
	params.Set("page", "1")
	params.Set("filter", "0")
	params.Set("type", "1")

	apiURL := "http://search.5sing.kugou.com/home/json?" + params.Encode()
	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Cookie", f.cookie))
	if err != nil {
		return nil, err
	}

	var resp struct {
		List []struct {
			SongListId string `json:"songListId"`
			Title      string `json:"title"`
			Picture    string `json:"pictureUrl"`
			PlayCount  int    `json:"playCount"`
			UserName   string `json:"userName"`
			SongCnt    int    `json:"songCnt"`
			Content    string `json:"content"`
			UserId     string `json:"userId"`
		} `json:"list"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("fivesing playlist json parse error: %w", err)
	}

	playlists := make([]model.Playlist, len(resp.List))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)

	for i, item := range resp.List {
		title := removeEmTags(html.UnescapeString(item.Title))
		desc := removeEmTags(html.UnescapeString(item.Content))
		if desc == "0" {
			desc = ""
		}

		link := fmt.Sprintf("http://5sing.kugou.com/%s/dj/%s.html", item.UserId, item.SongListId)
		creator := item.UserName
		if creator == "" {
			creator = "ID: " + item.UserId
		}

		playlists[i] = model.Playlist{
			Source:      "fivesing",
			ID:          item.SongListId,
			Name:        title,
			Cover:       item.Picture,
			TrackCount:  item.SongCnt,
			PlayCount:   item.PlayCount,
			Creator:     creator,
			Description: desc,
			Link:        link,
			Extra: map[string]string{
				"user_id": item.UserId,
			},
		}

		// 并行获取缺失的创建者名称
		if item.UserName == "" && item.SongListId != "" {
			wg.Add(1)
			go func(idx int, plID string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				if name, err := f.fetchCreatorName(plID); err == nil && name != "" {
					playlists[idx].Creator = name
				}
			}(i, item.SongListId)
		}
	}
	wg.Wait()

	return playlists, nil
}

// GetPlaylistSongs 获取歌单详情 (简化版：直接复用 fetchPlaylistDetail)
func (f *Fivesing) GetPlaylistSongs(id string) ([]model.Song, error) {
	// 复用核心逻辑，只返回歌曲切片
	_, songs, err := f.fetchPlaylistDetail(id)
	return songs, err
}

// ParsePlaylist 解析歌单链接并返回详情
func (f *Fivesing) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	re := regexp.MustCompile(`5sing\.kugou\.com/(?:(\d+)/)?dj/([a-zA-Z0-9]+)\.html`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 3 {
		return nil, nil, errors.New("invalid 5sing playlist link")
	}
	playlistId := matches[2]
	// userId (matches[1]) 可选，因为 API 验证更准确，这里只用 ID
	return f.fetchPlaylistDetail(playlistId)
}
