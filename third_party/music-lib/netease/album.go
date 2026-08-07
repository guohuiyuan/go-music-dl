package netease

import (
	"encoding/json"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"strconv"
)

func SearchAlbum(keyword string) ([]model.Playlist, error) {
	return defaultNetease.SearchAlbum(keyword)
}

func GetAlbumSongs(albumID string) ([]model.Song, error) {
	return defaultNetease.GetAlbumSongs(albumID)
}

func ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	return defaultNetease.ParseAlbum(link)
}

// SearchAlbum searches albums.
func (n *Netease) SearchAlbum(keyword string) ([]model.Playlist, error) {
	body, err := n.cloudSearch(keyword, 10, 10)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code   int `json:"code"`
		Result struct {
			Albums []struct {
				ID          int    `json:"id"`
				Name        string `json:"name"`
				PicURL      string `json:"picUrl"`
				Size        int    `json:"size"`
				Company     string `json:"company"`
				Description string `json:"description"`
				BriefDesc   string `json:"briefDesc"`
				PublishTime int64  `json:"publishTime"`
				Artist      struct {
					Name string `json:"name"`
				} `json:"artist"`
				Artists []struct {
					Name string `json:"name"`
				} `json:"artists"`
			} `json:"albums"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease album json parse error: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("netease api error code: %d", resp.Code)
	}

	albums := make([]model.Playlist, 0, len(resp.Result.Albums))
	for _, item := range resp.Result.Albums {
		artistName := item.Artist.Name
		if artistName == "" && len(item.Artists) > 0 {
			names := make([]string, 0, len(item.Artists))
			for _, artist := range item.Artists {
				if artist.Name != "" {
					names = append(names, artist.Name)
				}
			}
			artistName = joinArtistNames(names)
		}

		description := item.Description
		if description == "" {
			description = item.BriefDesc
		}

		albums = append(albums, model.Playlist{
			Source:      "netease",
			ID:          strconv.Itoa(item.ID),
			Name:        item.Name,
			Cover:       item.PicURL,
			TrackCount:  item.Size,
			Creator:     artistName,
			Description: description,
			Link:        fmt.Sprintf("https://music.163.com/#/album?id=%d", item.ID),
			Extra: map[string]string{
				"type":         "album",
				"company":      item.Company,
				"publish_time": strconv.FormatInt(item.PublishTime, 10),
			},
		})
	}

	return albums, nil
}

// GetAlbumSongs returns songs in an album.
func (n *Netease) GetAlbumSongs(albumID string) ([]model.Song, error) {
	_, songs, err := n.fetchAlbumDetail(albumID)
	return songs, err
}

// ParseAlbum parses an album link.
func (n *Netease) ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	kind, albumID, err := parseNeteaseLink(link)
	if err != nil || kind != neteaseLinkAlbum {
		return nil, nil, errNeteaseInvalidAlbumLink
	}
	return n.fetchAlbumDetail(albumID)
}
