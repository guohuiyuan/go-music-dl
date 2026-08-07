package qianqian

import (
	"errors"
	"github.com/guohuiyuan/music-lib/model"
	"regexp"
	"strings"
)

func SearchAlbum(keyword string) ([]model.Playlist, error) {
	return defaultQianqian.SearchAlbum(keyword)
}

func GetAlbumSongs(id string) ([]model.Song, error) {
	return defaultQianqian.GetAlbumSongs(id)
}

func ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	return defaultQianqian.ParseAlbum(link)
}

// SearchAlbum 搜索专辑
func (q *Qianqian) SearchAlbum(keyword string) ([]model.Playlist, error) {
	items, err := q.searchAlbumItems(keyword)
	if err != nil {
		return nil, err
	}

	albums := make([]model.Playlist, 0, len(items))
	for _, item := range items {
		albumID := normalizeQianqianAlbumAssetCode(item.AlbumAssetCode)
		if albumID == "" {
			continue
		}

		albums = append(albums, model.Playlist{
			Source:      "qianqian",
			ID:          albumID,
			Name:        item.Title,
			Cover:       item.Pic,
			TrackCount:  len(item.TrackList),
			Creator:     joinQianqianArtists(item.Artist),
			Description: item.Introduce,
			Link:        qianqianAlbumLink(albumID),
			Extra: map[string]string{
				"type":         "album",
				"album_id":     albumID,
				"release_date": qianqianReleaseDate(item.ReleaseDate),
				"genre":        strings.TrimSpace(item.Genre),
				"lang":         strings.TrimSpace(item.Lang),
			},
		})
	}
	if len(albums) == 0 {
		return nil, errors.New("no albums found")
	}

	return albums, nil
}

// GetAlbumSongs 获取专辑歌曲列表
func (q *Qianqian) GetAlbumSongs(id string) ([]model.Song, error) {
	_, songs, err := q.fetchAlbumDetail(id)
	return songs, err
}

// ParseAlbum 解析专辑链接
func (q *Qianqian) ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`music\.91q\.com/album/([A-Za-z0-9]+)`),
		regexp.MustCompile(`albumAssetCode=([A-Za-z0-9]+)`),
		regexp.MustCompile(`albumid=(\d+)`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(link)
		if len(matches) >= 2 {
			return q.fetchAlbumDetail(matches[1])
		}
	}

	return nil, nil, errors.New("invalid qianqian album link")
}
