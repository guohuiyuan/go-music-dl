package migu

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"regexp"
	"strings"
)

func SearchAlbum(keyword string) ([]model.Playlist, error) {
	return defaultMigu.SearchAlbum(keyword)
}

func GetAlbumSongs(id string) ([]model.Song, error) { return defaultMigu.GetAlbumSongs(id) }

func ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	return defaultMigu.ParseAlbum(link)
}

// SearchPlaylist 搜索歌单
// SearchAlbum 鎼滅储涓撹緫
func (m *Migu) SearchAlbum(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("ua", "Android_migu")
	params.Set("version", "5.0.1")
	params.Set("text", keyword)
	params.Set("pageNo", "1")
	params.Set("pageSize", "10")
	params.Set("searchSwitch", `{"song":0,"album":1,"singer":0,"tagSong":0,"mvSong":0,"songlist":0,"bestShow":1}`)

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
		AlbumResultData struct {
			Result []struct {
				ID           string          `json:"id"`
				ResourceType string          `json:"resourceType"`
				Name         string          `json:"name"`
				Singer       string          `json:"singer"`
				PublishDate  string          `json:"publishDate"`
				Desc         string          `json:"desc"`
				ImgItems     []miguImageItem `json:"imgItems"`
			} `json:"result"`
		} `json:"albumResultData"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("migu album json parse error: %w", err)
	}

	albums := make([]model.Playlist, 0, len(resp.AlbumResultData.Result))
	for _, item := range resp.AlbumResultData.Result {
		albumID := strings.TrimSpace(item.ID)
		if albumID == "" {
			continue
		}

		description := strings.TrimSpace(item.Desc)
		if description == "" {
			description = strings.TrimSpace(item.PublishDate)
		}

		albums = append(albums, model.Playlist{
			Source:      "migu",
			ID:          albumID,
			Name:        strings.TrimSpace(item.Name),
			Cover:       pickMiguImage(item.ImgItems),
			Creator:     strings.TrimSpace(item.Singer),
			Description: description,
			Link:        miguAlbumLink(albumID),
			Extra: map[string]string{
				"type":          "album",
				"album_id":      albumID,
				"resource_type": firstNonEmpty(strings.TrimSpace(item.ResourceType), "2003"),
				"publish_date":  strings.TrimSpace(item.PublishDate),
			},
		})
	}

	if len(albums) == 0 {
		return nil, errors.New("no albums found")
	}

	return albums, nil
}

// GetPlaylistSongs 获取歌单详情（解析歌曲列表）
func (m *Migu) GetAlbumSongs(id string) ([]model.Song, error) {
	songs, _, err := m.fetchAlbumSongs(id)
	return songs, err
}

func (m *Migu) ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`music\.migu\.cn/(?:v3|v5)/music/album/(\d+)`),
		regexp.MustCompile(`albumId=(\d+)`),
		regexp.MustCompile(`resourceId=(\d+)`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(link)
		if len(matches) >= 2 {
			return m.fetchAlbumDetail(matches[1])
		}
	}

	return nil, nil, errors.New("invalid migu album link")
}
