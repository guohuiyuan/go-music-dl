package qq

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"regexp"
	"strconv"
)

func SearchAlbum(keyword string) ([]model.Playlist, error) {
	return defaultQQ.SearchAlbum(keyword)
}

func GetAlbumSongs(id string) ([]model.Song, error) {
	_, songs, err := defaultQQ.fetchAlbumDetail(id)
	return songs, err
}

func ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	return defaultQQ.ParseAlbum(link)
}

// SearchAlbum searches albums.
func (q *QQ) SearchAlbum(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("format", "json")
	params.Set("p", "1")
	params.Set("n", "10")
	params.Set("w", keyword)
	params.Set("t", "8")
	apiURL := "http://c.y.qq.com/soso/fcgi-bin/search_for_qq_cp?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
		utils.WithHeader("Referer", "https://y.qq.com/portal/search.html"),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Album struct {
				List []struct {
					AlbumID    int64  `json:"albumID"`
					AlbumMID   string `json:"albumMID"`
					AlbumName  string `json:"albumName"`
					PublicTime string `json:"publicTime"`
					SingerName string `json:"singerName"`
				} `json:"list"`
			} `json:"album"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq album json parse error: %w", err)
	}

	var albums []model.Playlist
	for _, item := range resp.Data.Album.List {
		if item.AlbumMID == "" {
			continue
		}

		albums = append(albums, model.Playlist{
			Source:      "qq",
			ID:          item.AlbumMID,
			Name:        item.AlbumName,
			Cover:       fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", item.AlbumMID),
			Creator:     item.SingerName,
			Description: "",
			Link:        fmt.Sprintf("https://y.qq.com/n/ryqq/albumDetail/%s", item.AlbumMID),
			Extra: map[string]string{
				"type":         "album",
				"album_id":     strconv.FormatInt(item.AlbumID, 10),
				"album_mid":    item.AlbumMID,
				"publish_time": item.PublicTime,
			},
		})
	}

	if len(albums) == 0 {
		return nil, errors.New("no albums found")
	}

	return albums, nil
}

func (q *QQ) GetAlbumSongs(id string) ([]model.Song, error) {
	_, songs, err := q.fetchAlbumDetail(id)
	return songs, err
}

func (q *QQ) ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`albumDetail/([A-Za-z0-9]+)`),
		regexp.MustCompile(`album/([A-Za-z0-9]+)`),
		regexp.MustCompile(`albummid=([A-Za-z0-9]+)`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(link)
		if len(matches) >= 2 {
			return q.fetchAlbumDetail(matches[1])
		}
	}

	return nil, nil, errors.New("invalid qq album link")
}
