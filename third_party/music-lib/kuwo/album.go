package kuwo

import (
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"regexp"
	"strings"
)

func SearchAlbum(keyword string) ([]model.Playlist, error) {
	return defaultKuwo.SearchAlbum(keyword)
}

func GetAlbumSongs(id string) ([]model.Song, error) {
	_, songs, err := defaultKuwo.fetchAlbumDetail(id)
	return songs, err
}

func ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	return defaultKuwo.ParseAlbum(link)
}

// SearchAlbum 搜索专辑
func (k *Kuwo) SearchAlbum(keyword string) ([]model.Playlist, error) {
	var resp struct {
		AlbumList []struct {
			AlbumID  string `json:"albumid"`
			ID       string `json:"id"`
			Name     string `json:"name"`
			Artist   string `json:"artist"`
			AArtist  string `json:"aartist"`
			HtsImg   string `json:"hts_img"`
			Img      string `json:"img"`
			MusicCnt string `json:"musiccnt"`
			Info     string `json:"info"`
			Company  string `json:"company"`
			Pub      string `json:"pub"`
			PlayCnt  string `json:"PLAYCNT"`
		} `json:"albumlist"`
	}

	if err := k.searchCollection(keyword, "album", &resp); err != nil {
		return nil, err
	}

	albums := make([]model.Playlist, 0, len(resp.AlbumList))
	for _, item := range resp.AlbumList {
		albumID := firstNonEmpty(item.AlbumID, item.ID)
		if albumID == "" {
			continue
		}

		albums = append(albums, model.Playlist{
			Source:      "kuwo",
			ID:          albumID,
			Name:        normalizeKuwoText(item.Name),
			Cover:       normalizeKuwoImageURL(firstNonEmpty(item.HtsImg, item.Img)),
			TrackCount:  parseKuwoStringInt(item.MusicCnt),
			PlayCount:   parseKuwoStringInt(item.PlayCnt),
			Creator:     normalizeKuwoText(firstNonEmpty(item.AArtist, item.Artist)),
			Description: normalizeKuwoText(item.Info),
			Link:        fmt.Sprintf("http://www.kuwo.cn/album_detail/%s", albumID),
			Extra: map[string]string{
				"type":         "album",
				"album_id":     albumID,
				"company":      normalizeKuwoText(item.Company),
				"publish_time": strings.TrimSpace(item.Pub),
			},
		})
	}

	if len(albums) == 0 {
		return nil, errors.New("no albums found")
	}

	return albums, nil
}

// GetPlaylistSongs 获取歌单详情（解析歌曲列表）
func (k *Kuwo) GetAlbumSongs(id string) ([]model.Song, error) {
	_, songs, err := k.fetchAlbumDetail(id)
	return songs, err
}

func (k *Kuwo) ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`album_detail/(\d+)`),
		regexp.MustCompile(`album/(\d+)`),
		regexp.MustCompile(`albumid=(\d+)`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(link)
		if len(matches) >= 2 {
			return k.fetchAlbumDetail(matches[1])
		}
	}

	return nil, nil, errors.New("invalid kuwo album link")
}
