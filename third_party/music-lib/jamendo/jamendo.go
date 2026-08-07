package jamendo

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"math/rand"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

const (
	UserAgent          = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"
	Referer            = "https://www.jamendo.com/search?q=musicdl"
	XJamVersion        = "4gvfvv"
	SearchAPI          = "https://www.jamendo.com/api/search"
	SearchApiPath      = "/api/search"
	TrackAPI           = "https://www.jamendo.com/api/tracks"
	TrackApiPath       = "/api/tracks"
	AlbumAPI           = "https://www.jamendo.com/api/albums"
	AlbumApiPath       = "/api/albums"
	ArtistAPI          = "https://www.jamendo.com/api/artists"
	ArtistApiPath      = "/api/artists"
	PlaylistAPI        = "https://www.jamendo.com/api/playlists"
	PlaylistApiPath    = "/api/playlists"
	PlaylistTracksAPI  = "https://www.jamendo.com/api/playlists/tracks"
	PlaylistTracksPath = "/api/playlists/tracks"
	ClientID           = "9873ff31"
)

type Jamendo struct {
	cookie string
}

type jamendoTrackItem struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Duration int    `json:"duration"`
	ArtistID int    `json:"artistId"`
	AlbumID  int    `json:"albumId"`
	Artist   struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"artist"`
	Album struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"album"`
	Cover struct {
		Big struct {
			Size300 string `json:"size300"`
		} `json:"big"`
	} `json:"cover"`
	Download map[string]string `json:"download"`
	Stream   map[string]string `json:"stream"`
}

type jamendoAlbumSearchItem struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Artist struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"artist"`
	Cover struct {
		Big struct {
			Size300 string `json:"size300"`
		} `json:"big"`
	} `json:"cover"`
}

type jamendoAlbumItem struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	ArtistID     int               `json:"artistId"`
	DateReleased int64             `json:"dateReleased"`
	Description  map[string]string `json:"description"`
	Cover        struct {
		Big struct {
			Size300 string `json:"size300"`
		} `json:"big"`
	} `json:"cover"`
	Tracks []struct {
		Position int `json:"position"`
		ID       int `json:"id"`
	} `json:"tracks"`
}

type jamendoPlaylistItem struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	UserName    string `json:"user_name"`
	Image       string `json:"image"`
	Description string `json:"description"`
	Tracks      []struct {
		Position int `json:"position"`
		ID       int `json:"id"`
	} `json:"tracks"`
}

type jamendoArtistItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type jamendoTrackMeta struct {
	ArtistName string
	AlbumName  string
	AlbumID    string
}

func New(cookie string) *Jamendo {
	return &Jamendo{cookie: cookie}
}

var defaultJamendo = New("")

func (j *Jamendo) fetchAlbumDetail(id string) (*model.Playlist, []model.Song, error) {
	albumItem, err := j.getAlbumByID(id)
	if err != nil {
		return nil, nil, err
	}

	creator, err := j.getArtistNameByID(albumItem.ArtistID)
	if err != nil {
		creator = ""
	}

	album := &model.Playlist{
		Source:      "jamendo",
		ID:          strconv.Itoa(albumItem.ID),
		Name:        albumItem.Name,
		Cover:       albumItem.Cover.Big.Size300,
		TrackCount:  len(albumItem.Tracks),
		Creator:     creator,
		Description: pickJamendoDescription(albumItem.Description),
		Link:        albumLink(strconv.Itoa(albumItem.ID)),
		Extra: map[string]string{
			"album_id": strconv.Itoa(albumItem.ID),
		},
	}
	if albumItem.ArtistID > 0 {
		album.Extra["artist_id"] = strconv.Itoa(albumItem.ArtistID)
	}

	songs, err := j.fetchAlbumTracks(albumItem, creator)
	if err != nil {
		return nil, nil, err
	}
	if album.Creator == "" && len(songs) > 0 {
		album.Creator = songs[0].Artist
	}

	return album, songs, nil
}

func (j *Jamendo) fetchPlaylistDetail(id string) (*model.Playlist, []model.Song, error) {
	playlistID := strings.TrimSpace(id)
	if playlistID == "" {
		return nil, nil, errors.New("playlist id is empty")
	}

	playlistItem, err := j.getPlaylistByID(playlistID)
	if err != nil {
		return nil, nil, err
	}

	songs, err := j.GetPlaylistSongs(playlistID)
	if err != nil {
		return nil, nil, err
	}

	playlist := &model.Playlist{
		Source:      "jamendo",
		ID:          strconv.Itoa(playlistItem.ID),
		Name:        playlistItem.Name,
		Cover:       playlistItem.Image,
		TrackCount:  len(songs),
		Creator:     playlistItem.UserName,
		Description: strings.TrimSpace(playlistItem.Description),
		Link:        playlistLink(strconv.Itoa(playlistItem.ID)),
		Extra: map[string]string{
			"type":        "playlist",
			"playlist_id": strconv.Itoa(playlistItem.ID),
		},
	}

	return playlist, songs, nil
}

func (j *Jamendo) fetchPlaylistTracks(playlistItem *jamendoPlaylistItem) ([]model.Song, error) {
	if playlistItem == nil || len(playlistItem.Tracks) == 0 {
		return nil, errors.New("playlist is empty or invalid")
	}

	songs := make([]model.Song, len(playlistItem.Tracks))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)

	var firstErr error
	var errMu sync.Mutex

	for idx, track := range playlistItem.Tracks {
		idx := idx
		trackID := track.ID
		if trackID == 0 {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			song, err := j.getTrackByID(strconv.Itoa(trackID), jamendoTrackMeta{})
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
				return
			}
			if song == nil {
				return
			}

			songs[idx] = *song
		}()
	}

	wg.Wait()

	filtered := make([]model.Song, 0, len(songs))
	for _, song := range songs {
		if song.ID == "" {
			continue
		}
		filtered = append(filtered, song)
	}

	if len(filtered) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, errors.New("playlist has no playable tracks")
	}

	return filtered, nil
}

func (j *Jamendo) fetchAlbumTracks(albumItem *jamendoAlbumItem, creator string) ([]model.Song, error) {
	if albumItem == nil || len(albumItem.Tracks) == 0 {
		return nil, errors.New("album is empty or invalid")
	}

	songs := make([]model.Song, len(albumItem.Tracks))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)

	var firstErr error
	var errMu sync.Mutex

	meta := jamendoTrackMeta{
		ArtistName: creator,
		AlbumName:  albumItem.Name,
		AlbumID:    strconv.Itoa(albumItem.ID),
	}

	for idx, track := range albumItem.Tracks {
		idx := idx
		trackID := track.ID
		if trackID == 0 {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			song, err := j.getTrackByID(strconv.Itoa(trackID), meta)
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
				return
			}
			if song == nil {
				return
			}

			songs[idx] = *song
		}()
	}

	wg.Wait()

	filtered := make([]model.Song, 0, len(songs))
	for _, song := range songs {
		if song.ID == "" {
			continue
		}
		filtered = append(filtered, song)
	}

	if len(filtered) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, errors.New("album has no playable tracks")
	}

	return filtered, nil
}

func (j *Jamendo) getTrackByID(id string, meta jamendoTrackMeta) (*model.Song, error) {
	params := url.Values{}
	params.Set("id", id)

	body, err := j.apiGet(TrackAPI+"?"+params.Encode(), TrackApiPath)
	if err != nil {
		return nil, err
	}

	var results []jamendoTrackItem
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("jamendo track json error: %w", err)
	}
	if len(results) == 0 {
		return nil, errors.New("track not found")
	}

	item := results[0]
	resolvedMeta, err := j.resolveTrackMeta(item, meta)
	if err != nil {
		return nil, err
	}

	song := buildSong(item, resolvedMeta)
	if song == nil {
		return nil, errors.New("no valid stream found")
	}
	return song, nil
}

func (j *Jamendo) resolveTrackMeta(item jamendoTrackItem, meta jamendoTrackMeta) (jamendoTrackMeta, error) {
	if meta.AlbumID == "" && item.AlbumID > 0 {
		meta.AlbumID = strconv.Itoa(item.AlbumID)
	}
	if meta.AlbumName == "" {
		meta.AlbumName = strings.TrimSpace(item.Album.Name)
	}
	if meta.ArtistName == "" {
		meta.ArtistName = strings.TrimSpace(item.Artist.Name)
	}

	if meta.AlbumName == "" && item.AlbumID > 0 {
		albumItem, err := j.getAlbumByID(strconv.Itoa(item.AlbumID))
		if err != nil {
			return meta, err
		}
		meta.AlbumName = albumItem.Name
		if meta.AlbumID == "" {
			meta.AlbumID = strconv.Itoa(albumItem.ID)
		}
	}

	if meta.ArtistName == "" && item.ArtistID > 0 {
		artistName, err := j.getArtistNameByID(item.ArtistID)
		if err != nil {
			return meta, err
		}
		meta.ArtistName = artistName
	}

	return meta, nil
}

func (j *Jamendo) getAlbumByID(id string) (*jamendoAlbumItem, error) {
	params := url.Values{}
	params.Set("id", id)

	body, err := j.apiGet(AlbumAPI+"?"+params.Encode(), AlbumApiPath)
	if err != nil {
		return nil, err
	}

	var results []jamendoAlbumItem
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("jamendo album json error: %w", err)
	}
	if len(results) == 0 {
		return nil, errors.New("album not found")
	}

	return &results[0], nil
}

func (j *Jamendo) getPlaylistByID(id string) (*jamendoPlaylistItem, error) {
	params := url.Values{}
	params.Set("id", id)

	body, err := j.apiGet(PlaylistAPI+"?"+params.Encode(), PlaylistApiPath)
	if err != nil {
		return nil, err
	}

	var results []jamendoPlaylistItem
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("jamendo playlist json error: %w", err)
	}
	if len(results) == 0 {
		return nil, errors.New("playlist not found")
	}

	return &results[0], nil
}

func (j *Jamendo) getArtistNameByID(id int) (string, error) {
	if id == 0 {
		return "", nil
	}

	params := url.Values{}
	params.Set("id", strconv.Itoa(id))

	body, err := j.apiGet(ArtistAPI+"?"+params.Encode(), ArtistApiPath)
	if err != nil {
		return "", err
	}

	var results []jamendoArtistItem
	if err := json.Unmarshal(body, &results); err != nil {
		return "", fmt.Errorf("jamendo artist json error: %w", err)
	}
	if len(results) == 0 {
		return "", errors.New("artist not found")
	}

	return results[0].Name, nil
}

func (j *Jamendo) searchByType(keyword, searchType string) ([]byte, error) {
	params := url.Values{}
	params.Set("query", keyword)
	params.Set("type", searchType)
	params.Set("limit", "20")
	params.Set("identities", "www")

	return j.apiGet(SearchAPI+"?"+params.Encode(), SearchApiPath)
}

func (j *Jamendo) apiGet(apiURL, path string) ([]byte, error) {
	return utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("x-jam-call", makeXJamCall(path)),
		utils.WithHeader("x-jam-version", XJamVersion),
		utils.WithHeader("x-requested-with", "XMLHttpRequest"),
		utils.WithHeader("Cookie", j.cookie),
	)
}

func buildSong(item jamendoTrackItem, meta jamendoTrackMeta) *model.Song {
	streams := item.Download
	if len(streams) == 0 {
		streams = item.Stream
	}

	downloadURL, ext := pickBestQuality(streams)
	if downloadURL == "" {
		return nil
	}

	trackID := strconv.Itoa(item.ID)
	albumID := meta.AlbumID
	if albumID == "" && item.AlbumID > 0 {
		albumID = strconv.Itoa(item.AlbumID)
	}

	song := &model.Song{
		Source:   "jamendo",
		ID:       trackID,
		Name:     item.Name,
		Artist:   firstNonEmpty(strings.TrimSpace(item.Artist.Name), strings.TrimSpace(meta.ArtistName)),
		Album:    firstNonEmpty(strings.TrimSpace(item.Album.Name), strings.TrimSpace(meta.AlbumName)),
		AlbumID:  albumID,
		Duration: item.Duration,
		Ext:      ext,
		Cover:    item.Cover.Big.Size300,
		URL:      downloadURL,
		Link:     trackLink(trackID),
		Extra: map[string]string{
			"track_id": trackID,
		},
	}

	if albumID != "" {
		song.Extra["album_id"] = albumID
	}

	return song
}

func pickBestQuality(streams map[string]string) (string, string) {
	for _, key := range []string{"flac", "mp33", "mp32", "mp3", "ogg"} {
		if url := streams[key]; url != "" {
			switch key {
			case "mp33", "mp32":
				return url, "mp3"
			default:
				return url, key
			}
		}
	}
	return "", ""
}

func pickJamendoDescription(desc map[string]string) string {
	for _, key := range []string{"en", "fr", "de", "es", "it", "ru", "pt", "pl"} {
		if value := strings.TrimSpace(desc[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func albumLink(id string) string {
	return fmt.Sprintf("https://www.jamendo.com/album/%s", id)
}

func playlistLink(id string) string {
	return fmt.Sprintf("https://www.jamendo.com/playlist/%s", id)
}

func trackLink(id string) string {
	return fmt.Sprintf("https://www.jamendo.com/track/%s", id)
}

func makeXJamCall(path string) string {
	r := rand.Float64()
	randStr := fmt.Sprintf("%v", r)
	data := path + randStr
	hash := sha1.Sum([]byte(data))
	digest := hex.EncodeToString(hash[:])
	return fmt.Sprintf("$%s*%s~", digest, randStr)
}
