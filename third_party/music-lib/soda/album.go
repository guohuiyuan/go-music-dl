package soda

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"strconv"
	"strings"
)

func SearchAlbum(keyword string) ([]model.Playlist, error) {
	return defaultSoda.SearchAlbum(keyword)
}

func GetAlbumSongs(id string) ([]model.Song, error) {
	return defaultSoda.GetAlbumSongs(id)
}

func ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	return defaultSoda.ParseAlbum(link)
}

// SearchAlbum 搜索专辑 (PC API)
func (s *Soda) SearchAlbum(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("q", keyword)
	params.Set("cursor", "0")
	params.Set("search_method", "input")
	params.Set("aid", "386088")
	params.Set("device_platform", "web")
	params.Set("channel", "pc_web")

	apiURL := "https://api.qishui.com/luna/pc/search/album?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		ResultGroups []struct {
			Data []struct {
				Entity struct {
					Album struct {
						ID          string       `json:"id"`
						Name        string       `json:"name"`
						Artists     []sodaArtist `json:"artists"`
						Company     string       `json:"company"`
						CountTracks int          `json:"count_tracks"`
						URLCover    sodaImage    `json:"url_cover"`
						ReleaseDate int64        `json:"release_date"`
					} `json:"album"`
				} `json:"entity"`
			} `json:"data"`
		} `json:"result_groups"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda album search json parse error: %w", err)
	}
	if len(resp.ResultGroups) == 0 {
		return nil, nil
	}

	albums := make([]model.Playlist, 0, len(resp.ResultGroups[0].Data))
	for _, item := range resp.ResultGroups[0].Data {
		album := item.Entity.Album
		if album.ID == "" {
			continue
		}

		extra := map[string]string{
			"album_id": album.ID,
		}
		if album.ReleaseDate > 0 {
			extra["release_date"] = strconv.FormatInt(album.ReleaseDate, 10)
		}

		albums = append(albums, model.Playlist{
			Source:      "soda",
			ID:          album.ID,
			Name:        album.Name,
			Cover:       sodaBuildImageURL(album.URLCover, "~c5_300x300.jpg"),
			TrackCount:  album.CountTracks,
			Creator:     sodaJoinArtists(album.Artists),
			Description: strings.TrimSpace(album.Company),
			Link:        sodaAlbumLink(album.ID),
			Extra:       extra,
		})
	}

	return albums, nil
}

// GetAlbumSongs 获取专辑所有歌曲
func (s *Soda) GetAlbumSongs(id string) ([]model.Song, error) {
	_, songs, err := s.fetchAlbumDetail(id)
	return songs, err
}

// ParseAlbum 解析专辑链接
func (s *Soda) ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	albumID := sodaExtractAlbumID(link)
	if albumID == "" {
		return nil, nil, errors.New("invalid soda album link")
	}
	return s.fetchAlbumDetail(albumID)
}
