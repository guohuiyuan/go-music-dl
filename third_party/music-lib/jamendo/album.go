package jamendo

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"regexp"
	"strconv"
)

func SearchAlbum(keyword string) ([]model.Playlist, error) {
	return defaultJamendo.SearchAlbum(keyword)
}

func GetAlbumSongs(id string) ([]model.Song, error) { return defaultJamendo.GetAlbumSongs(id) }

func ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	return defaultJamendo.ParseAlbum(link)
}

func (j *Jamendo) SearchAlbum(keyword string) ([]model.Playlist, error) {
	body, err := j.searchByType(keyword, "album")
	if err != nil {
		return nil, err
	}

	var results []jamendoAlbumSearchItem
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("jamendo album json parse error: %w", err)
	}

	albums := make([]model.Playlist, 0, len(results))
	for _, item := range results {
		if item.ID == 0 {
			continue
		}

		albumID := strconv.Itoa(item.ID)
		extra := map[string]string{
			"album_id": albumID,
		}
		if item.Artist.ID > 0 {
			extra["artist_id"] = strconv.Itoa(item.Artist.ID)
		}

		albums = append(albums, model.Playlist{
			Source:  "jamendo",
			ID:      albumID,
			Name:    item.Name,
			Cover:   item.Cover.Big.Size300,
			Creator: item.Artist.Name,
			Link:    albumLink(albumID),
			Extra:   extra,
		})
	}

	if len(albums) == 0 {
		return nil, errors.New("no albums found")
	}

	return albums, nil
}

func (j *Jamendo) GetAlbumSongs(id string) ([]model.Song, error) {
	_, songs, err := j.fetchAlbumDetail(id)
	return songs, err
}

func (j *Jamendo) ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	re := regexp.MustCompile(`jamendo\.com/album/(\d+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, nil, errors.New("invalid jamendo album link")
	}

	return j.fetchAlbumDetail(matches[1])
}
