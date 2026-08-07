package kugou

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
)

func SearchAlbum(keyword string) ([]model.Playlist, error) {
	return defaultKugou.SearchAlbum(keyword)
}

func GetAlbumSongs(id string) ([]model.Song, error) {
	_, songs, err := defaultKugou.fetchAlbumDetail(id)
	return songs, err
}

func ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	return defaultKugou.ParseAlbum(link)
}

// SearchPlaylist 搜索歌单
// SearchAlbum searches albums.
func (k *Kugou) SearchAlbum(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("keyword", keyword)
	params.Set("format", "json")
	params.Set("page", "1")
	params.Set("pagesize", "10")
	apiURL := "http://mobilecdn.kugou.com/api/v3/search/album?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Status  int    `json:"status"`
		Errcode int    `json:"errcode"`
		Error   string `json:"error"`
		Data    struct {
			Info []struct {
				AlbumID     int    `json:"albumid"`
				AlbumName   string `json:"albumname"`
				SingerName  string `json:"singername"`
				PublishTime string `json:"publishtime"`
				ImgURL      string `json:"imgurl"`
				Intro       string `json:"intro"`
				SongCount   int    `json:"songcount"`
			} `json:"info"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kugou album search json error: %w", err)
	}
	if resp.Errcode != 0 || resp.Status != 1 {
		return nil, fmt.Errorf("kugou album search api error: status=%d errcode=%d error=%s", resp.Status, resp.Errcode, resp.Error)
	}

	albums := make([]model.Playlist, 0, len(resp.Data.Info))
	for _, item := range resp.Data.Info {
		if item.AlbumID == 0 {
			continue
		}

		albums = append(albums, model.Playlist{
			Source:      "kugou",
			ID:          strconv.Itoa(item.AlbumID),
			Name:        item.AlbumName,
			Cover:       strings.Replace(item.ImgURL, "{size}", "240", 1),
			TrackCount:  item.SongCount,
			Creator:     item.SingerName,
			Description: item.Intro,
			Link:        fmt.Sprintf("https://www.kugou.com/album/%d.html", item.AlbumID),
			Extra: map[string]string{
				"type":         "album",
				"album_id":     strconv.Itoa(item.AlbumID),
				"publish_time": item.PublishTime,
			},
		})
	}

	if len(albums) == 0 {
		return nil, errors.New("no albums found")
	}

	return albums, nil
}

// GetPlaylistSongs 获取歌单详情 (仅返回 Songs, 兼容旧接口)
// GetAlbumSongs returns songs in an album.
func (k *Kugou) GetAlbumSongs(id string) ([]model.Song, error) {
	_, songs, err := k.fetchAlbumDetail(id)
	return songs, err
}

// ParseAlbum parses an album link.
func (k *Kugou) ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`album/single/(\d+)\.html`),
		regexp.MustCompile(`yy/album/single/(\d+)\.html`),
		regexp.MustCompile(`album/(\d+)\.html`),
		regexp.MustCompile(`albumid=(\d+)`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(link)
		if len(matches) >= 2 {
			return k.fetchAlbumDetail(matches[1])
		}
	}

	return nil, nil, errors.New("invalid kugou album link")
}
