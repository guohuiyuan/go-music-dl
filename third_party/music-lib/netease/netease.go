package netease

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	Referer                = "http://music.163.com/"
	SearchAPI              = "http://music.163.com/api/linux/forward"
	DownloadAPI            = "http://music.163.com/weapi/song/enhance/player/url"
	DownloadEAPI           = "https://interface3.music.163.com/eapi/song/enhance/player/url/v1"
	DetailAPI              = "https://music.163.com/weapi/v3/song/detail"
	PlaylistAPI            = "https://music.163.com/weapi/v3/playlist/detail"
	PlaylistCategoryAPI    = "https://music.163.com/weapi/playlist/catalogue"
	CategoryPlaylistAPI    = "https://music.163.com/weapi/playlist/list"
	UserPlaylistAPI        = "https://music.163.com/weapi/user/playlist"
	AlbumAPI               = "https://music.163.com/weapi/v1/album/%s"
	UserAccountAPI         = "https://music.163.com/weapi/nuser/account/get"
	RecommendedPlaylistAPI = "https://music.163.com/weapi/personalized/playlist"
)

type Netease struct {
	cookie     string
	isVipCache *bool
}

type neteaseLinkKind string

const (
	neteaseLinkUnknown  neteaseLinkKind = ""
	neteaseLinkSong     neteaseLinkKind = "song"
	neteaseLinkAlbum    neteaseLinkKind = "album"
	neteaseLinkPlaylist neteaseLinkKind = "playlist"
)

var (
	errInvalidNeteaseLink      = errors.New("invalid netease link")
	errNeteasePlaylistLink     = errors.New("netease playlist link detected, use ParsePlaylist")
	errNeteaseAlbumLink        = errors.New("netease album link detected, use ParseAlbum")
	errNeteaseInvalidAlbumLink = errors.New("invalid netease album link")
	errNeteaseInvalidListLink  = errors.New("invalid netease playlist link")
	errNeteaseSongNotFound     = errors.New("netease song not found")
)

type cachedDownloadURL struct {
	url       string
	ext       string
	expiresAt time.Time
}

type cachedVIPStatus struct {
	isVip     bool
	expiresAt time.Time
}

var downloadURLCache = struct {
	sync.Mutex
	items map[string]cachedDownloadURL
}{items: make(map[string]cachedDownloadURL)}

var vipStatusCache = struct {
	sync.Mutex
	items map[string]cachedVIPStatus
}{items: make(map[string]cachedVIPStatus)}

const downloadURLCacheTTL = 10 * time.Minute
const vipStatusCacheTTL = 10 * time.Minute

func New(cookie string) *Netease { return &Netease{cookie: cookie} }

var defaultNetease = New("")

func (n *Netease) vipStatusCacheKey() string {
	return utils.MD5(n.cookie)
}

func (n *Netease) getCachedVIPStatus() (bool, bool) {
	key := n.vipStatusCacheKey()
	now := time.Now()

	vipStatusCache.Lock()
	defer vipStatusCache.Unlock()

	item, ok := vipStatusCache.items[key]
	if !ok {
		return false, false
	}
	if now.After(item.expiresAt) {
		delete(vipStatusCache.items, key)
		return false, false
	}
	return item.isVip, true
}

func (n *Netease) setCachedVIPStatus(isVip bool) {
	key := n.vipStatusCacheKey()

	vipStatusCache.Lock()
	defer vipStatusCache.Unlock()
	vipStatusCache.items[key] = cachedVIPStatus{
		isVip:     isVip,
		expiresAt: time.Now().Add(vipStatusCacheTTL),
	}
}

// cloudSearch calls the shared Netease cloud search route.
func (n *Netease) cloudSearch(keyword string, searchType int, limit int) ([]byte, error) {
	eparams := map[string]interface{}{
		"method": "POST",
		"url":    "http://music.163.com/api/cloudsearch/pc",
		"params": map[string]interface{}{
			"s":      keyword,
			"type":   searchType,
			"offset": 0,
			"limit":  limit,
		},
	}
	eparamsJSON, _ := json.Marshal(eparams)
	encryptedParam := EncryptLinux(string(eparamsJSON))
	form := url.Values{}
	form.Set("eparams", encryptedParam)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	}

	return utils.Post(SearchAPI, strings.NewReader(form.Encode()), headers...)
}

// joinArtistNames joins artist names for display.
func joinArtistNames(names []string) string {
	return strings.Join(names, ", ")
}

// fetchAlbumDetail returns album metadata and songs.
func (n *Netease) fetchAlbumDetail(albumID string) (*model.Playlist, []model.Song, error) {
	reqData := map[string]interface{}{
		"csrf_token": "",
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post(fmt.Sprintf(AlbumAPI, albumID), strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return nil, nil, err
	}

	var resp struct {
		Code  int `json:"code"`
		Album struct {
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
		} `json:"album"`
		Songs []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Ar   []struct {
				Name string `json:"name"`
			} `json:"ar"`
			Al struct {
				ID     int    `json:"id"`
				Name   string `json:"name"`
				PicURL string `json:"picUrl"`
			} `json:"al"`
			Dt        int `json:"dt"`
			Privilege struct {
				Fl int `json:"fl"`
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
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("netease album detail json parse error: %w", err)
	}
	if resp.Code != 200 {
		return nil, nil, fmt.Errorf("netease api error code: %d", resp.Code)
	}

	artistName := resp.Album.Artist.Name
	if artistName == "" && len(resp.Album.Artists) > 0 {
		names := make([]string, 0, len(resp.Album.Artists))
		for _, artist := range resp.Album.Artists {
			if artist.Name != "" {
				names = append(names, artist.Name)
			}
		}
		artistName = joinArtistNames(names)
	}

	description := resp.Album.Description
	if description == "" {
		description = resp.Album.BriefDesc
	}

	album := &model.Playlist{
		Source:      "netease",
		ID:          strconv.Itoa(resp.Album.ID),
		Name:        resp.Album.Name,
		Cover:       resp.Album.PicURL,
		TrackCount:  resp.Album.Size,
		Creator:     artistName,
		Description: description,
		Link:        fmt.Sprintf("https://music.163.com/#/album?id=%d", resp.Album.ID),
		Extra: map[string]string{
			"type":         "album",
			"company":      resp.Album.Company,
			"publish_time": strconv.FormatInt(resp.Album.PublishTime, 10),
		},
	}

	songs := make([]model.Song, 0, len(resp.Songs))
	for _, item := range resp.Songs {
		artistNames := make([]string, 0, len(item.Ar))
		for _, artist := range item.Ar {
			if artist.Name != "" {
				artistNames = append(artistNames, artist.Name)
			}
		}

		var size int64
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

		songs = append(songs, model.Song{
			Source:   "netease",
			ID:       strconv.Itoa(item.ID),
			Name:     item.Name,
			Artist:   joinArtistNames(artistNames),
			Album:    item.Al.Name,
			AlbumID:  strconv.Itoa(item.Al.ID),
			Duration: duration,
			Size:     size,
			Bitrate:  bitrate,
			Cover:    item.Al.PicURL,
			Link:     fmt.Sprintf("https://music.163.com/#/song?id=%d", item.ID),
			Extra: map[string]string{
				"song_id":  strconv.Itoa(item.ID),
				"album_id": strconv.Itoa(item.Al.ID),
			},
		})
	}

	return album, songs, nil
}

func (n *Netease) fetchPlaylistDetail(playlistID string) (*model.Playlist, []model.Song, error) {
	reqData := map[string]interface{}{
		"id":         playlistID,
		"n":          0, // 0表示不直接返回详情，我们只需要ID列表
		"csrf_token": "",
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post(PlaylistAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return nil, nil, err
	}

	var resp struct {
		Code     int `json:"code"`
		Playlist struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			CoverImgURL string `json:"coverImgUrl"`
			Description string `json:"description"`
			PlayCount   int    `json:"playCount"`
			TrackCount  int    `json:"trackCount"`
			Creator     struct {
				Nickname string `json:"nickname"`
			} `json:"creator"`
			// Use trackIds so we can fetch the full list separately.
			TrackIds []struct {
				ID int `json:"id"`
			} `json:"trackIds"`
		} `json:"playlist"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("netease playlist detail json parse error: %w", err)
	}
	if resp.Code != 200 {
		return nil, nil, fmt.Errorf("netease api error code: %d", resp.Code)
	}

	// Build playlist metadata.
	playlist := &model.Playlist{
		Source:      "netease",
		ID:          strconv.Itoa(resp.Playlist.ID),
		Name:        resp.Playlist.Name,
		Cover:       resp.Playlist.CoverImgURL,
		TrackCount:  resp.Playlist.TrackCount,
		PlayCount:   resp.Playlist.PlayCount,
		Creator:     resp.Playlist.Creator.Nickname,
		Description: resp.Playlist.Description,
		Link:        fmt.Sprintf("https://music.163.com/#/playlist?id=%d", resp.Playlist.ID),
	}

	// Collect all song IDs first.
	var allIDs []string
	for _, tid := range resp.Playlist.TrackIds {
		allIDs = append(allIDs, strconv.Itoa(tid.ID))
	}

	// Fetch song details in batches.
	var allSongs []model.Song
	batchSize := 500
	for i := 0; i < len(allIDs); i += batchSize {
		end := i + batchSize
		if end > len(allIDs) {
			end = len(allIDs)
		}

		batchIDs := allIDs[i:end]
		batchSongs, err := n.fetchSongsBatch(batchIDs)
		if err == nil {
			allSongs = append(allSongs, batchSongs...)
		}
	}

	return playlist, allSongs, nil
}

// fetchSongsBatch fetches song details in batches.
func (n *Netease) fetchSongsBatch(songIDs []string) ([]model.Song, error) {
	if len(songIDs) == 0 {
		return nil, nil
	}

	// Build the c payload: [{"id":123},{"id":456},...]
	var cList []map[string]interface{}
	for _, id := range songIDs {
		cList = append(cList, map[string]interface{}{"id": id})
	}
	cJSON, _ := json.Marshal(cList)
	idsJSON, _ := json.Marshal(songIDs)

	reqData := map[string]interface{}{
		"c":   string(cJSON),
		"ids": string(idsJSON),
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))

	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post(DetailAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return nil, err
	}

	var resp struct {
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
			Dt int `json:"dt"`
		} `json:"songs"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	var songs []model.Song
	for _, item := range resp.Songs {
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
			Duration: item.Dt / 1000,
			Cover:    item.Al.PicURL,
			Link:     fmt.Sprintf("https://music.163.com/#/song?id=%d", item.ID),
			Extra: map[string]string{
				"song_id": strconv.Itoa(item.ID),
			},
		})
	}
	return songs, nil
}

func parseNeteaseLink(link string) (neteaseLinkKind, string, error) {
	candidates := []string{link}

	if parsed, err := url.Parse(link); err == nil {
		if parsed.Path != "" && parsed.Path != "/" {
			pathCandidate := parsed.Path
			if parsed.RawQuery != "" {
				pathCandidate += "?" + parsed.RawQuery
			}
			candidates = append(candidates, pathCandidate)
		}
		if fragment := strings.TrimSpace(strings.TrimPrefix(parsed.Fragment, "!")); fragment != "" {
			candidates = append(candidates, fragment)
		}
	}

	for _, candidate := range candidates {
		if kind, id, ok := parseNeteaseLinkCandidate(candidate); ok {
			return kind, id, nil
		}
	}

	return neteaseLinkUnknown, "", errInvalidNeteaseLink
}

func parseNeteaseLinkCandidate(candidate string) (neteaseLinkKind, string, bool) {
	parsed, err := url.Parse(candidate)
	if err != nil {
		return neteaseLinkUnknown, "", false
	}

	var kind neteaseLinkKind
	segments := strings.FieldsFunc(strings.ToLower(parsed.Path), func(r rune) bool {
		return r == '/'
	})
	for _, segment := range segments {
		switch segment {
		case string(neteaseLinkSong):
			kind = neteaseLinkSong
		case string(neteaseLinkAlbum):
			kind = neteaseLinkAlbum
		case string(neteaseLinkPlaylist):
			kind = neteaseLinkPlaylist
		}
	}

	id := parsed.Query().Get("id")
	if !isDigits(id) {
		id = ""
	}

	if kind == neteaseLinkUnknown && len(segments) >= 2 {
		last := segments[len(segments)-1]
		prev := segments[len(segments)-2]
		if isDigits(last) {
			switch prev {
			case string(neteaseLinkSong):
				kind = neteaseLinkSong
			case string(neteaseLinkAlbum):
				kind = neteaseLinkAlbum
			case string(neteaseLinkPlaylist):
				kind = neteaseLinkPlaylist
			}
			if kind != neteaseLinkUnknown {
				id = last
			}
		}
	}

	if kind == neteaseLinkUnknown || id == "" {
		return neteaseLinkUnknown, "", false
	}

	return kind, id, true
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func (n *Netease) downloadURLCacheKey(songID string, levels string) string {
	return songID + ":" + levels + ":" + utils.MD5(n.cookie)
}

func (n *Netease) getCachedDownloadURL(songID string, levels string) (cachedDownloadURL, bool) {
	key := n.downloadURLCacheKey(songID, levels)
	now := time.Now()

	downloadURLCache.Lock()
	defer downloadURLCache.Unlock()

	item, ok := downloadURLCache.items[key]
	if !ok {
		return cachedDownloadURL{}, false
	}
	if now.After(item.expiresAt) {
		delete(downloadURLCache.items, key)
		return cachedDownloadURL{}, false
	}
	return item, true
}

func (n *Netease) setCachedDownloadURL(songID string, levels string, url string, ext string) {
	if strings.TrimSpace(url) == "" {
		return
	}
	key := n.downloadURLCacheKey(songID, levels)

	downloadURLCache.Lock()
	defer downloadURLCache.Unlock()
	downloadURLCache.items[key] = cachedDownloadURL{
		url:       url,
		ext:       ext,
		expiresAt: time.Now().Add(downloadURLCacheTTL),
	}
}

func preferredDownloadLevels(s *model.Song) []string {
	if s != nil && s.Extra != nil {
		level := strings.ToLower(strings.TrimSpace(s.Extra["netease_level"]))
		if level == "" {
			level = strings.ToLower(strings.TrimSpace(s.Extra["level"]))
		}
		switch level {
		case "standard", "exhigh", "lossless", "hires":
			return []string{level}
		}
	}
	return []string{"lossless", "hires", "exhigh"}
}

// getEAPIDownloadURL fetches a high-quality download URL via eapi.
func (n *Netease) getEAPIDownloadURL(songID string, quality string) (string, string, error) {
	idNum, err := strconv.Atoi(songID)
	if err != nil {
		return "", "", fmt.Errorf("invalid song id: %v", err)
	}

	headerJSON := `{"os":"pc","appver":"","osver":"","deviceId":"pyncm!","requestId":"12345678"}`

	payload := map[string]interface{}{
		"ids":        []int{idNum},
		"level":      quality,
		"encodeType": "flac",
		"header":     headerJSON,
	}

	payloadBytes, _ := json.Marshal(payload)
	params := EncryptEApi(DownloadEAPI, string(payloadBytes))

	form := url.Values{}
	form.Set("params", params)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post(DownloadEAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return "", "", err
	}

	var resp struct {
		Data []struct {
			URL  string `json:"url"`
			Code int    `json:"code"`
			Type string `json:"type"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", "", fmt.Errorf("eapi json parse error: %w", err)
	}
	if len(resp.Data) == 0 || resp.Data[0].URL == "" {
		return "", "", errors.New("eapi download url not found")
	}
	return resp.Data[0].URL, normalizeNeteaseAudioType(resp.Data[0].Type, quality), nil
}

func normalizeNeteaseAudioType(audioType string, quality string) string {
	audioType = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(audioType, ".")))
	switch audioType {
	case "flac", "mp3", "m4a":
		return audioType
	}
	switch quality {
	case "lossless", "hires":
		return "flac"
	default:
		return ""
	}
}
