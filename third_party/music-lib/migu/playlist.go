package migu

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultMigu.SearchPlaylist(keyword)
}

func GetPlaylistSongs(id string) ([]model.Song, error) { return defaultMigu.GetPlaylistSongs(id) }

func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultMigu.ParsePlaylist(link)
}

func GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return defaultMigu.GetPlaylistCategories()
}

func GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return defaultMigu.GetCategoryPlaylists(categoryID, page, limit)
}

func (m *Migu) GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	categories := []model.PlaylistCategory{{
		Source: "migu",
		ID:     "",
		Name:   "全部",
		Group:  "全部",
	}}
	for _, item := range miguPlaylistCategoryKeywords {
		categories = append(categories, model.PlaylistCategory{
			Source: "migu",
			ID:     item.Name,
			Name:   item.Name,
			Group:  item.Group,
			Hot:    item.Hot,
		})
	}
	return categories, nil
}

func (m *Migu) GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	categoryID = strings.TrimSpace(categoryID)
	if categoryID == "" {
		categoryID = "华语"
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	playlists, err := m.searchPlaylists(categoryID, page, limit)
	if err != nil {
		return nil, err
	}
	for i := range playlists {
		if playlists[i].Extra == nil {
			playlists[i].Extra = map[string]string{}
		}
		playlists[i].Extra["category_id"] = categoryID
	}
	if len(playlists) == 0 {
		return nil, errors.New("no category playlists found")
	}
	return playlists, nil
}

var miguPlaylistCategoryKeywords = []struct {
	Name  string
	Group string
	Hot   bool
}{
	{Name: "华语", Group: "语种", Hot: true},
	{Name: "欧美", Group: "语种", Hot: true},
	{Name: "日语", Group: "语种"},
	{Name: "韩语", Group: "语种"},
	{Name: "粤语", Group: "语种"},
	{Name: "流行", Group: "风格", Hot: true},
	{Name: "摇滚", Group: "风格", Hot: true},
	{Name: "民谣", Group: "风格", Hot: true},
	{Name: "电子", Group: "风格"},
	{Name: "说唱", Group: "风格"},
	{Name: "古风", Group: "风格"},
	{Name: "轻音乐", Group: "风格"},
	{Name: "影视", Group: "场景", Hot: true},
	{Name: "ACG", Group: "场景"},
	{Name: "治愈", Group: "场景"},
	{Name: "运动", Group: "场景"},
	{Name: "学习", Group: "场景"},
	{Name: "睡前", Group: "场景"},
}

func (m *Migu) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return m.searchPlaylists(keyword, 1, 10)
}

func (m *Migu) searchPlaylists(keyword string, page, limit int) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("ua", "Android_migu")
	params.Set("version", "5.0.1")
	params.Set("text", keyword)
	params.Set("pageNo", strconv.Itoa(page))
	params.Set("pageSize", strconv.Itoa(limit))
	// 切换开关：songlist:1
	params.Set("searchSwitch", `{"song":0,"album":0,"singer":0,"tagSong":0,"mvSong":0,"songlist":1,"bestShow":1}`)

	apiURL := "http://pd.musicapp.migu.cn/MIGUM2.0/v1.0/content/search_all.do?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", m.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		SongListResultData struct {
			Result []struct {
				ID              string          `json:"id"`
				Name            string          `json:"name"`
				MusicNum        string          `json:"musicNum"`
				UserName        string          `json:"userName"`
				OwnerName       string          `json:"ownerName"`
				MusicListPicURL string          `json:"musicListPicUrl"`
				PlayNum         string          `json:"playNum"`
				ResourceType    string          `json:"resourceType"`
				ImgItems        []miguImageItem `json:"imgItems"`
			} `json:"result"`
		} `json:"songListResultData"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("migu playlist json parse error: %w", err)
	}

	var playlists []model.Playlist
	for _, item := range resp.SongListResultData.Result {
		playlistID := strings.TrimSpace(item.ID)
		name := strings.TrimSpace(item.Name)
		if playlistID == "" || name == "" {
			continue
		}
		trackCount, _ := strconv.Atoi(item.MusicNum)
		playCount, _ := strconv.Atoi(item.PlayNum)
		cover := firstNonEmpty(item.MusicListPicURL, pickMiguImage(item.ImgItems))

		playlists = append(playlists, model.Playlist{
			Source:     "migu",
			ID:         playlistID,
			Name:       name,
			Cover:      cover,
			TrackCount: trackCount,
			PlayCount:  playCount,
			Creator:    firstNonEmpty(item.UserName, item.OwnerName),
			Link:       miguPlaylistLink(playlistID),
			Extra: map[string]string{
				"type":          "playlist",
				"playlist_id":   playlistID,
				"resource_type": firstNonEmpty(item.ResourceType, "2021"),
			},
		})
	}
	return playlists, nil
}

func (m *Migu) GetPlaylistSongs(id string) ([]model.Song, error) {
	playlistID := strings.TrimSpace(id)
	if playlistID == "" {
		return nil, errors.New("playlist id is empty")
	}

	const pageSize = 50
	seen := make(map[string]struct{})
	songs := make([]model.Song, 0, pageSize)
	totalCount := 0

	for pageNo := 1; ; pageNo++ {
		params := url.Values{}
		params.Set("pageNo", strconv.Itoa(pageNo))
		params.Set("pageSize", strconv.Itoa(pageSize))
		params.Set("playlistId", playlistID)

		apiURL := "https://app.c.nf.migu.cn/MIGUM3.0/resource/playlist/song/v2.0?" + params.Encode()
		body, err := utils.Get(apiURL,
			utils.WithHeader("User-Agent", UserAgent),
			utils.WithHeader("Referer", Referer),
			utils.WithHeader("Cookie", m.cookie),
		)
		if err != nil {
			return nil, err
		}

		var resp struct {
			Code string `json:"code"`
			Info string `json:"info"`
			Data struct {
				SongList   []MiguSongItem `json:"songList"`
				TotalCount int            `json:"totalCount"`
			} `json:"data"`
		}

		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("migu playlist json parse error: %w", err)
		}
		if resp.Code != "" && resp.Code != "000000" {
			return nil, fmt.Errorf("migu api error: %s (code %s)", resp.Info, resp.Code)
		}
		if totalCount == 0 {
			totalCount = resp.Data.TotalCount
		}
		if len(resp.Data.SongList) == 0 {
			break
		}

		before := len(songs)
		for _, item := range resp.Data.SongList {
			song := m.convertItemToSongAllowPaid(item)
			if song == nil {
				continue
			}

			key := firstNonEmpty(song.Extra["content_id"], item.ContentID, song.ID, item.CopyrightID)
			if key == "" {
				key = song.Name + "|" + song.Artist
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			songs = append(songs, *song)
		}

		if len(resp.Data.SongList) < pageSize {
			break
		}
		if totalCount > 0 && len(songs) >= totalCount {
			break
		}
		if len(songs) == before {
			break
		}
	}

	if len(songs) == 0 {
		return nil, errors.New("playlist has no playable songs")
	}

	return songs, nil
}

func (m *Migu) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`playlistId=(\d+)`),
		regexp.MustCompile(`musicListId=(\d+)`),
		regexp.MustCompile(`(?:playlist|songlist)/(\d+)`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(link)
		if len(matches) >= 2 {
			return m.fetchPlaylistDetail(matches[1])
		}
	}

	if len(link) > 0 && !strings.Contains(link, "/") {
		return m.fetchPlaylistDetail(link)
	}

	return nil, nil, errors.New("invalid migu playlist link")
}
