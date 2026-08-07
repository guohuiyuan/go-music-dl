package joox

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
	return defaultJoox.SearchAlbum(keyword)
}

func GetAlbumSongs(id string) ([]model.Song, error) { return defaultJoox.GetAlbumSongs(id) }

func ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	return defaultJoox.ParseAlbum(link)
}

// SearchPlaylist 搜索歌单
func (j *Joox) SearchAlbum(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("country", "sg")
	params.Set("lang", "zh_cn")
	params.Set("keyword", keyword)
	apiURL := "https://cache.api.joox.com/openjoox/v3/search?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		SectionList []struct {
			SectionType int `json:"section_type"`
			ItemList    []struct {
				Type  int `json:"type"`
				Album struct {
					ID          string       `json:"id"`
					Name        string       `json:"name"`
					Images      []jooxImage  `json:"images"`
					PublishDate string       `json:"publish_date"`
					ArtistList  []jooxArtist `json:"artist_list"`
				} `json:"album"`
			} `json:"item_list"`
		} `json:"section_list"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("joox album search json error: %w", err)
	}

	albums := make([]model.Playlist, 0)
	seen := make(map[string]struct{})
	for _, section := range resp.SectionList {
		if section.SectionType != 1 {
			continue
		}
		for _, item := range section.ItemList {
			if item.Type != 2 {
				continue
			}
			albumID := normalizeJooxID(item.Album.ID)
			if albumID == "" {
				continue
			}
			if _, ok := seen[albumID]; ok {
				continue
			}
			seen[albumID] = struct{}{}

			albums = append(albums, model.Playlist{
				Source:      "joox",
				ID:          albumID,
				Name:        item.Album.Name,
				Cover:       pickJooxImage(item.Album.Images),
				Creator:     joinJooxArtists(item.Album.ArtistList),
				Description: strings.TrimSpace(item.Album.PublishDate),
				Link:        jooxAlbumLink(albumID),
				Extra: map[string]string{
					"type":         "album",
					"album_id":     albumID,
					"publish_date": strings.TrimSpace(item.Album.PublishDate),
				},
			})
		}
	}

	if len(albums) == 0 {
		return nil, errors.New("no albums found")
	}

	return albums, nil
}

// GetPlaylistSongs 获取歌单详情 (Updated to use OpenJoox v3 API)
func (j *Joox) GetAlbumSongs(id string) ([]model.Song, error) {
	_, songs, err := j.fetchAlbumDetail(id)
	return songs, err
}

func (j *Joox) ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`joox\.com/.*/album/([^/?#]+)`),
		regexp.MustCompile(`h_activity_id=([^&]+)`),
		regexp.MustCompile(`albumid=([^&]+)`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(link)
		if len(matches) >= 2 {
			return j.fetchAlbumDetail(matches[1])
		}
	}

	if len(link) > 10 && !strings.Contains(link, "/") {
		return j.fetchAlbumDetail(link)
	}

	return nil, nil, errors.New("invalid joox album link")
}
