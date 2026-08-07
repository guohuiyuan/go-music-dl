package qianqian

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultQianqian.SearchPlaylist(keyword)
}

func GetPlaylistSongs(id string) ([]model.Song, error) { return defaultQianqian.GetPlaylistSongs(id) }

func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultQianqian.ParsePlaylist(link)
}

func GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return defaultQianqian.GetPlaylistCategories()
}

func GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return defaultQianqian.GetCategoryPlaylists(categoryID, page, limit)
}

func (q *Qianqian) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	// [参数修正] timestamp 是必须的，type=6 代表歌单 (之前可能用了 10000 导致报错)
	params := url.Values{}
	params.Set("word", keyword)
	params.Set("type", "6") // 6 = 歌单
	params.Set("pageNo", "1")
	params.Set("pageSize", "10")
	params.Set("appid", AppID)
	params.Set("timestamp", strconv.FormatInt(time.Now().Unix(), 10))

	// 签名参数 (Search 接口通常也需要签名)
	signParams(params)

	apiURL := "https://music.91q.com/v1/search?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", q.cookie),
	)
	if err != nil {
		return nil, err
	}

	// [结构修正] 兼容处理 Data 字段
	// 成功时 Data 是对象，失败或空时 Data 可能是空数组 []
	// 我们先定义一个外层结构检查 State
	var rawResp struct {
		State bool            `json:"state"`
		Errno int             `json:"errno"`
		Msg   string          `json:"errmsg"`
		Data  json.RawMessage `json:"data"` // 延迟解析
	}

	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, fmt.Errorf("qianqian playlist json parse error: %w", err)
	}

	if !rawResp.State {
		// 如果 API 返回失败，通常 Data 是 []，直接返回空或错误
		// 忽略 "没有结果" 的错误，返回空列表
		return nil, nil // 或者 fmt.Errorf("api error: %s", rawResp.Msg)
	}

	// 解析 Data 部分
	var dataObj struct {
		TypeSonglist []struct {
			ID         interface{} `json:"id"` // 有时是 int 有时是 string，兼容一下
			Title      string      `json:"title"`
			Pic        string      `json:"pic"`
			TrackCount int         `json:"trackCount"`
			Tag        string      `json:"tag"`
		} `json:"typeSonglist"`
	}

	// 尝试将 RawMessage 解析为对象
	if err := json.Unmarshal(rawResp.Data, &dataObj); err != nil {
		// 如果解析失败，可能是因为 Data 是 [] (空结果)
		return nil, nil
	}

	var playlists []model.Playlist
	for _, item := range dataObj.TypeSonglist {
		// ID 转换
		var id string
		switch v := item.ID.(type) {
		case float64:
			id = strconv.FormatInt(int64(v), 10)
		case string:
			id = v
		default:
			continue
		}

		playlists = append(playlists, model.Playlist{
			Source:      "qianqian",
			ID:          id,
			Name:        item.Title,
			Cover:       item.Pic,
			TrackCount:  item.TrackCount,
			Description: item.Tag,
			Link:        qianqianPlaylistLink(id),
			Extra: map[string]string{
				"type":        "playlist",
				"playlist_id": id,
			},
			// 千千搜索结果不返回 Creator，留空
		})
	}

	return playlists, nil
}

func (q *Qianqian) GetPlaylistSongs(id string) ([]model.Song, error) {
	_, songs, err := q.fetchPlaylistDetail(id)
	return songs, err
}

func (q *Qianqian) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`music\.91q\.com/(?:songlist|tracklist|playlist)/([A-Za-z0-9]+)`),
		regexp.MustCompile(`(?:songlistid|tracklistid|playlistid|id)=([A-Za-z0-9]+)`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(link)
		if len(matches) >= 2 {
			return q.fetchPlaylistDetail(matches[1])
		}
	}

	if len(link) > 0 && !strings.Contains(link, "/") {
		return q.fetchPlaylistDetail(link)
	}

	return nil, nil, errors.New("invalid qianqian playlist link")
}

func (q *Qianqian) GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	params := url.Values{}
	params.Set("appid", AppID)
	signParams(params)

	apiURL := "https://music.91q.com/v1/tracklist/category?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", q.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		State  bool   `json:"state"`
		Errno  int    `json:"errno"`
		ErrMsg string `json:"errmsg"`
		Data   []struct {
			CategoryName string `json:"categoryName"`
			SubCate      []struct {
				ID           string `json:"id"`
				CategoryName string `json:"categoryName"`
				Count        int    `json:"count"`
			} `json:"subCate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qianqian playlist category json parse error: %w", err)
	}
	if !resp.State && resp.Errno != 22000 {
		return nil, fmt.Errorf("api error: %s (code %d)", resp.ErrMsg, resp.Errno)
	}

	categories := []model.PlaylistCategory{{
		Source: "qianqian",
		ID:     "",
		Name:   "全部",
		Group:  "全部",
	}}
	for _, group := range resp.Data {
		groupName := strings.TrimSpace(group.CategoryName)
		for _, item := range group.SubCate {
			id := strings.TrimSpace(item.ID)
			name := strings.TrimSpace(item.CategoryName)
			if id == "" || name == "" {
				continue
			}
			categories = append(categories, model.PlaylistCategory{
				Source: "qianqian",
				ID:     id,
				Name:   name,
				Group:  groupName,
				Count:  item.Count,
			})
		}
	}
	return categories, nil
}

func (q *Qianqian) GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	categoryID = strings.TrimSpace(categoryID)
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	params := url.Values{}
	params.Set("appid", AppID)
	params.Set("pageNo", strconv.Itoa(page))
	params.Set("pageSize", strconv.Itoa(limit))
	if categoryID != "" {
		params.Set("subCateId", categoryID)
	}
	signParams(params)

	apiURL := "https://music.91q.com/v1/tracklist/list?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", q.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		State  bool   `json:"state"`
		Errno  int    `json:"errno"`
		ErrMsg string `json:"errmsg"`
		Data   struct {
			Result []struct {
				ID         interface{} `json:"id"`
				Title      string      `json:"title"`
				Pic        string      `json:"pic"`
				TrackCount int         `json:"trackCount"`
				Desc       string      `json:"desc"`
				TagList    []string    `json:"tagList"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qianqian category playlist json parse error: %w", err)
	}
	if !resp.State && resp.Errno != 22000 {
		return nil, fmt.Errorf("api error: %s (code %d)", resp.ErrMsg, resp.Errno)
	}

	playlists := make([]model.Playlist, 0, len(resp.Data.Result))
	for _, item := range resp.Data.Result {
		id := qianqianValueString(item.ID)
		if id == "" {
			continue
		}
		description := strings.TrimSpace(item.Desc)
		if description == "" && len(item.TagList) > 0 {
			description = strings.Join(item.TagList, "、")
		}
		playlists = append(playlists, model.Playlist{
			Source:      "qianqian",
			ID:          id,
			Name:        item.Title,
			Cover:       item.Pic,
			TrackCount:  item.TrackCount,
			Description: description,
			Link:        qianqianPlaylistLink(id),
			Extra: map[string]string{
				"type":        "playlist",
				"playlist_id": id,
				"category_id": categoryID,
			},
		})
	}
	return playlists, nil
}
