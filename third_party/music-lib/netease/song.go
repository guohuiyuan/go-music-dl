package netease

import (
	"encoding/json"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"strconv"
	"strings"
)

func Search(keyword string) ([]model.Song, error) { return defaultNetease.Search(keyword) }

func Parse(link string) (*model.Song, error) { return defaultNetease.Parse(link) }

// Search searches songs.
func (n *Netease) Search(keyword string) ([]model.Song, error) {
	body, err := n.cloudSearch(keyword, 1, 10)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			Songs []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
				Ar   []struct {
					Name string `json:"name"`
				} `json:"ar"`
				Al struct {
					Name   string `json:"name"`
					PicURL string `json:"picUrl"`
				} `json:"al"`
				Dt        int `json:"dt"`
				Privilege struct {
					Fl  int `json:"fl"`
					Pl  int `json:"pl"`
					Fee int `json:"fee"`
				} `json:"privilege"`
				H struct {
					Size int64 `json:"size"`
				} `json:"h"`
				M struct {
					Size int64 `json:"size"`
				} `json:"m"`
				L struct {
					Size int64 `json:"size"`
				} `json:"l"`
			} `json:"songs"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease json parse error: %w", err)
	}

	var songs []model.Song
	isVip, _ := n.IsVipAccount()

	for _, item := range resp.Result.Songs {
		// Skip unavailable songs for non-VIP accounts.
		if !isVip && item.Privilege.Fl == 0 {
			continue
		}

		var size int64
		// Prefer the highest available bitrate.
		if item.Privilege.Fl >= 320000 && item.H.Size > 0 {
			size = item.H.Size
		} else if item.Privilege.Fl >= 192000 && item.M.Size > 0 {
			size = item.M.Size
		} else {
			size = item.L.Size
		}

		duration := item.Dt / 1000
		bitrate := 128
		if duration > 0 && size > 0 {
			bitrate = int(size * 8 / 1000 / int64(duration))
		}

		var artistNames []string
		for _, ar := range item.Ar {
			artistNames = append(artistNames, ar.Name)
		}

		songs = append(songs, model.Song{
			Source:   "netease",
			ID:       strconv.Itoa(item.ID),
			Name:     item.Name,
			Artist:   strings.Join(artistNames, "、"),
			Album:    item.Al.Name,
			Duration: duration,
			Size:     size,
			Bitrate:  bitrate,
			Cover:    item.Al.PicURL,
			Link:     fmt.Sprintf("https://music.163.com/#/song?id=%d", item.ID),
			Extra: map[string]string{
				"song_id": strconv.Itoa(item.ID),
			},
		})
	}
	return songs, nil
}

// Parse parses a song link.
func (n *Netease) Parse(link string) (*model.Song, error) {
	kind, songID, err := parseNeteaseLink(link)
	if err != nil {
		return nil, errInvalidNeteaseLink
	}
	switch kind {
	case neteaseLinkPlaylist:
		return nil, errNeteasePlaylistLink
	case neteaseLinkAlbum:
		return nil, errNeteaseAlbumLink
	case neteaseLinkSong:
	default:
		return nil, errInvalidNeteaseLink
	}

	songs, err := n.fetchSongsBatch([]string{songID})
	if err != nil {
		return nil, fmt.Errorf("fetch song detail failed: %w", err)
	}
	if len(songs) == 0 {
		return nil, errNeteaseSongNotFound
	}
	song := &songs[0]

	downloadURL, err := n.GetDownloadURL(song)
	if err == nil {
		song.URL = downloadURL
	}

	return song, nil
}
