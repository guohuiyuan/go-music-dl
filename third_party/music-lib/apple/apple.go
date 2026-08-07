package apple

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	appleHomepageURL  = "https://music.apple.com"
	appleAmpAPIURL    = "https://amp-api.music.apple.com"
	defaultStorefront = "us"
)

type Apple struct {
	cookie     string // media-user-token
	token      string // bearer token
	storefront string
}

func New(cookie string) *Apple {
	a := &Apple{cookie: cookie, storefront: defaultStorefront}
	// cookie format: "media-user-token=xxx" or "token=xxx;media-user-token=xxx;storefront=cn"
	if cookie != "" {
		parts := parseCookieParts(cookie)
		if v := parts["media-user-token"]; v != "" {
			a.cookie = v
		}
		if v := parts["storefront"]; v != "" {
			a.storefront = v
		}
		if v := parts["token"]; v != "" {
			a.token = v
		}
	}
	return a
}

var defaultApple = New("")

// Package-level convenience functions delegating to defaultApple.

func Search(keyword string) ([]model.Song, error)                      { return defaultApple.Search(keyword) }
func GetDownloadURL(s *model.Song) (string, error)                     { return defaultApple.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error)                          { return defaultApple.GetLyrics(s) }
func Parse(link string) (*model.Song, error)                           { return defaultApple.Parse(link) }
func SearchAlbum(keyword string) ([]model.Playlist, error)             { return defaultApple.SearchAlbum(keyword) }
func SearchPlaylist(keyword string) ([]model.Playlist, error)          { return defaultApple.SearchPlaylist(keyword) }
func GetAlbumSongs(id string) ([]model.Song, error)                    { return defaultApple.GetAlbumSongs(id) }
func GetPlaylistSongs(id string) ([]model.Song, error)                 { return defaultApple.GetPlaylistSongs(id) }
func ParseAlbum(link string) (*model.Playlist, []model.Song, error)    { return defaultApple.ParseAlbum(link) }
func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) { return defaultApple.ParsePlaylist(link) }
func GetPlaylistCategories() ([]model.PlaylistCategory, error)         { return defaultApple.GetPlaylistCategories() }
func GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return defaultApple.GetCategoryPlaylists(categoryID, page, limit)
}

// Apple Music genre curators (extracted from public search-landing recommendations).
var appleCuratorCategories = []model.PlaylistCategory{
	{ID: "1526756058", Name: "热门", Source: "apple"},
	{ID: "1479949880", Name: "C-Pop", Source: "apple"},
	{ID: "1019400042", Name: "国语流行", Source: "apple"},
	{ID: "1019398918", Name: "粤语流行", Source: "apple"},
	{ID: "1019399540", Name: "国际流行", Source: "apple"},
	{ID: "1019399551", Name: "K-Pop", Source: "apple"},
	{ID: "1019399547", Name: "J-Pop", Source: "apple"},
	{ID: "989061415", Name: "嘻哈/说唱", Source: "apple"},
	{ID: "1019400044", Name: "R&B", Source: "apple"},
	{ID: "1019400046", Name: "摇滚", Source: "apple"},
	{ID: "1019397973", Name: "另类音乐", Source: "apple"},
	{ID: "976439535", Name: "舞曲", Source: "apple"},
	{ID: "976439536", Name: "电子", Source: "apple"},
	{ID: "976439541", Name: "独立音乐", Source: "apple"},
	{ID: "1019399549", Name: "爵士乐", Source: "apple"},
	{ID: "1019398924", Name: "古典音乐", Source: "apple"},
	{ID: "976439528", Name: "蓝调", Source: "apple"},
	{ID: "976439534", Name: "乡村音乐", Source: "apple"},
	{ID: "1531542847", Name: "拉丁音乐", Source: "apple"},
	{ID: "988656348", Name: "非洲音乐", Source: "apple"},
	{ID: "976439543", Name: "金属乐", Source: "apple"},
	{ID: "976439550", Name: "朋克乐", Source: "apple"},
	{ID: "1019400049", Name: "不插电", Source: "apple"},
	{ID: "1231181168", Name: "影视原声", Source: "apple"},
	{ID: "1441811365", Name: "DJ 混音精选", Source: "apple"},
	{ID: "1532467784", Name: "瞩目之星", Source: "apple"},
	{ID: "1554938339", Name: "年代之声", Source: "apple"},
	{ID: "1564180390", Name: "空间音频", Source: "apple"},
	{ID: "989010186", Name: "亲子", Source: "apple"},
}

// GetPlaylistCategories returns Apple Music genre categories.
func (a *Apple) GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return appleCuratorCategories, nil
}

// GetCategoryPlaylists returns playlists for a given Apple Music curator category.
func (a *Apple) GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	if limit <= 0 {
		limit = 20
	}
	const apiMax = 25
	offset := (page - 1) * limit

	// Curator IDs are from cn storefront; use cn for category playlists.
	storefront := "cn"

	var all []model.Playlist
	for len(all) < limit {
		batchSize := apiMax
		if limit-len(all) < batchSize {
			batchSize = limit - len(all)
		}

		params := url.Values{}
		params.Set("limit", strconv.Itoa(batchSize))
		params.Set("offset", strconv.Itoa(offset))
		params.Set("l", "zh-Hans-CN")

		uri := fmt.Sprintf("/v1/catalog/%s/apple-curators/%s/playlists", storefront, categoryID)
		body, err := a.ampGet(uri, params)
		if err != nil {
			if len(all) > 0 {
				break // return what we have
			}
			return nil, err
		}

		var resp appleResourceListResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("apple category playlists json error: %w", err)
		}

		for _, item := range resp.Data {
			all = append(all, applePlaylistToPlaylist(item))
		}

		// No more data or no next page
		if len(resp.Data) < batchSize || resp.Next == "" {
			break
		}
		offset += len(resp.Data)
	}
	return all, nil
}

func (a *Apple) ensureToken() error {
	if a.token != "" {
		return nil
	}
	token, err := fetchAppleToken()
	if err != nil {
		return err
	}
	a.token = token
	return nil
}

func (a *Apple) ampHeaders() []utils.RequestOption {
	opts := []utils.RequestOption{
		utils.WithHeader("Authorization", "Bearer "+a.token),
		utils.WithHeader("Origin", appleHomepageURL),
	}
	if a.cookie != "" {
		opts = append(opts, utils.WithHeader("Cookie", "media-user-token="+a.cookie))
	}
	return opts
}

func (a *Apple) ampGet(uri string, params url.Values) ([]byte, error) {
	fullURL := appleAmpAPIURL + uri
	if len(params) > 0 {
		fullURL += "?" + params.Encode()
	}
	return a.ampGetRaw(fullURL)
}

func (a *Apple) ampGetRaw(uri string) ([]byte, error) {
	if err := a.ensureToken(); err != nil {
		return nil, err
	}

	fullURL := uri
	if strings.HasPrefix(uri, "/") {
		fullURL = appleAmpAPIURL + uri
	}
	return utils.Get(fullURL, a.ampHeaders()...)
}

// Search searches Apple Music catalog for songs.
func (a *Apple) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("term", keyword)
	params.Set("types", "songs")
	params.Set("limit", "30")

	uri := fmt.Sprintf("/v1/catalog/%s/search", a.storefront)
	body, err := a.ampGet(uri, params)
	if err != nil {
		return nil, err
	}

	var resp appleSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("apple search json error: %w", err)
	}

	var songs []model.Song
	for _, item := range resp.Results.Songs.Data {
		songs = append(songs, appleSongFromCatalog(item))
	}
	return songs, nil
}

// SearchAlbum searches Apple Music catalog for albums.
func (a *Apple) SearchAlbum(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("term", keyword)
	params.Set("types", "albums")
	params.Set("limit", "20")

	uri := fmt.Sprintf("/v1/catalog/%s/search", a.storefront)
	body, err := a.ampGet(uri, params)
	if err != nil {
		return nil, err
	}

	var resp appleSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("apple search album json error: %w", err)
	}

	var playlists []model.Playlist
	for _, item := range resp.Results.Albums.Data {
		playlists = append(playlists, appleAlbumToPlaylist(item))
	}
	return playlists, nil
}

// SearchPlaylist searches Apple Music catalog for playlists.
func (a *Apple) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("term", keyword)
	params.Set("types", "playlists")
	params.Set("limit", "20")

	uri := fmt.Sprintf("/v1/catalog/%s/search", a.storefront)
	body, err := a.ampGet(uri, params)
	if err != nil {
		return nil, err
	}

	var resp appleSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("apple search playlist json error: %w", err)
	}

	var playlists []model.Playlist
	for _, item := range resp.Results.Playlists.Data {
		playlists = append(playlists, applePlaylistToPlaylist(item))
	}
	return playlists, nil
}

// GetAlbumSongs returns songs from an album.
func (a *Apple) GetAlbumSongs(id string) ([]model.Song, error) {
	_, songs, err := a.ParseAlbum(id)
	return songs, err
}

// GetPlaylistSongs returns songs from a playlist.
func (a *Apple) GetPlaylistSongs(id string) ([]model.Song, error) {
	_, songs, err := a.ParsePlaylist(id)
	return songs, err
}

// ParseAlbum fetches album details.
func (a *Apple) ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	albumID := extractAppleID(link, "album")
	if albumID == "" {
		return nil, nil, fmt.Errorf("invalid apple music album link: %s", link)
	}

	params := url.Values{}
	params.Set("extend", "extendedAssetUrls")

	uri := fmt.Sprintf("/v1/catalog/%s/albums/%s", a.storefront, albumID)
	body, err := a.ampGet(uri, params)
	if err != nil {
		return nil, nil, err
	}

	var resp appleResourceResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("apple album json error: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, nil, fmt.Errorf("apple album not found: %s", albumID)
	}

	album := resp.Data[0]
	pl := appleAlbumToPlaylist(album)

	var songs []model.Song
	for _, track := range album.Relationships.Tracks.Data {
		songs = append(songs, appleSongFromCatalog(track))
	}
	return &pl, songs, nil
}

// ParsePlaylist fetches playlist details.
func (a *Apple) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	playlistID := extractAppleID(link, "playlist")
	if playlistID == "" {
		return nil, nil, fmt.Errorf("invalid apple music playlist link: %s", link)
	}

	pl, songs, err := a.fetchPlaylistDetail(playlistID)
	if err != nil {
		return nil, nil, err
	}
	return &pl, songs, nil
}

func (a *Apple) fetchPlaylistDetail(playlistID string) (model.Playlist, []model.Song, error) {
	params := url.Values{}
	params.Set("limit[tracks]", "300")
	params.Set("extend", "extendedAssetUrls")

	uri := fmt.Sprintf("/v1/catalog/%s/playlists/%s", a.storefront, playlistID)
	body, err := a.ampGet(uri, params)
	if err != nil {
		return model.Playlist{}, nil, err
	}

	var resp appleResourceResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return model.Playlist{}, nil, fmt.Errorf("apple playlist json error: %w", err)
	}
	if len(resp.Data) == 0 {
		return model.Playlist{}, nil, fmt.Errorf("apple playlist not found: %s", playlistID)
	}

	item := resp.Data[0]
	tracks, err := a.fetchAllTrackResources(item.Relationships.Tracks)
	if err != nil {
		return model.Playlist{}, nil, err
	}
	item.Relationships.Tracks.Data = tracks

	pl := applePlaylistToPlaylist(item)

	var songs []model.Song
	for _, track := range item.Relationships.Tracks.Data {
		songs = append(songs, appleSongFromCatalog(track))
	}
	if pl.TrackCount == 0 || len(songs) > pl.TrackCount {
		pl.TrackCount = len(songs)
	}
	return pl, songs, nil
}

func (a *Apple) fetchAllTrackResources(tracks appleTrackRelationship) ([]appleResource, error) {
	all := append([]appleResource(nil), tracks.Data...)
	nextURI := strings.TrimSpace(tracks.Next)
	for nextURI != "" {
		body, err := a.ampGetRaw(nextURI)
		if err != nil {
			return nil, err
		}

		var page appleResourceListResponse
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("apple playlist tracks json error: %w", err)
		}
		all = append(all, page.Data...)
		nextURI = strings.TrimSpace(page.Next)
	}
	return all, nil
}

// Parse parses a single song link.
func (a *Apple) Parse(link string) (*model.Song, error) {
	songID := extractAppleID(link, "song")
	if songID == "" {
		return nil, fmt.Errorf("invalid apple music song link: %s", link)
	}

	params := url.Values{}
	params.Set("extend", "extendedAssetUrls")
	params.Set("include", "lyrics,albums")

	uri := fmt.Sprintf("/v1/catalog/%s/songs/%s", a.storefront, songID)
	body, err := a.ampGet(uri, params)
	if err != nil {
		return nil, err
	}

	var resp appleResourceResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("apple song json error: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("apple song not found: %s", songID)
	}

	song := appleSongFromCatalog(resp.Data[0])
	return &song, nil
}

// GetDownloadURL returns the preview URL (full download requires DRM decryption via gamdl).
func (a *Apple) GetDownloadURL(s *model.Song) (string, error) {
	if s == nil {
		return "", fmt.Errorf("song is nil")
	}
	if s.URL != "" {
		return s.URL, nil
	}
	// Fetch song to get preview URL
	song, err := a.Parse(s.ID)
	if err != nil {
		return "", err
	}
	if song.URL == "" {
		return "", fmt.Errorf("apple music: no preview URL available (full download requires gamdl)")
	}
	return song.URL, nil
}

// GetLyrics fetches lyrics for a song.
func (a *Apple) GetLyrics(s *model.Song) (string, error) {
	if s == nil {
		return "", fmt.Errorf("song is nil")
	}
	songID := s.ID
	if songID == "" {
		return "", fmt.Errorf("song id is empty")
	}

	params := url.Values{}
	params.Set("include", "lyrics")
	params.Set("extend", "extendedAssetUrls")

	uri := fmt.Sprintf("/v1/catalog/%s/songs/%s", a.storefront, songID)
	body, err := a.ampGet(uri, params)
	if err != nil {
		return "", err
	}

	var resp appleResourceResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if len(resp.Data) == 0 {
		return "", nil
	}

	for _, rel := range resp.Data[0].Relationships.Lyrics.Data {
		if rel.Attributes.Text != "" {
			return rel.Attributes.Text, nil
		}
	}
	return "", nil
}

// --- Token fetching ---

func fetchAppleToken() (string, error) {
	body, err := utils.Get(appleHomepageURL,
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
	)
	if err != nil {
		return "", fmt.Errorf("apple: fetch homepage error: %w", err)
	}

	// Find index.js URI
	re := regexp.MustCompile(`/(assets/index-legacy[~\-][^/"]+\.js)`)
	match := re.FindSubmatch(body)
	if len(match) < 2 {
		return "", fmt.Errorf("apple: index.js URI not found")
	}

	jsURL := appleHomepageURL + "/" + string(match[1])
	jsBody, err := utils.Get(jsURL,
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
	)
	if err != nil {
		return "", fmt.Errorf("apple: fetch index.js error: %w", err)
	}

	// Find token (starts with eyJh)
	tokenRe := regexp.MustCompile(`(?:=["'])(eyJh[^"']+)`)
	tokenMatch := tokenRe.FindSubmatch(jsBody)
	if len(tokenMatch) < 2 {
		return "", fmt.Errorf("apple: token not found in index.js")
	}

	return string(tokenMatch[1]), nil
}

// --- ID extraction ---

func extractAppleID(link string, mediaType string) string {
	link = strings.TrimSpace(link)
	if link == "" {
		return ""
	}

	if isAppleRawID(link, mediaType) {
		return link
	}

	if parsed, err := url.Parse(link); err == nil && parsed.Path != "" {
		if mediaType == "song" {
			if songID := strings.TrimSpace(parsed.Query().Get("i")); songID != "" {
				return songID
			}
		}
		if id := extractAppleIDFromPath(parsed.Path, mediaType); id != "" {
			return id
		}
	}

	patterns := []*regexp.Regexp{
		regexp.MustCompile(fmt.Sprintf(`%s/[^/]+/(\d+)`, mediaType)),
		regexp.MustCompile(fmt.Sprintf(`%ss?/[^/]+/[^/]+/(\d+)`, mediaType)),
		regexp.MustCompile(`/(\d+)$`),
		regexp.MustCompile(`[?&]i=(\d+)`),
	}
	if mediaType == "playlist" {
		patterns = append(patterns,
			regexp.MustCompile(`playlists?/[^/?#]+/([a-zA-Z0-9._-]+)`),
			regexp.MustCompile(`playlists?/([a-zA-Z0-9._-]+)(?:[?#]|$)`),
		)
	}

	for _, p := range patterns {
		if m := p.FindStringSubmatch(link); len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

func isAppleRawID(value string, mediaType string) bool {
	if strings.ContainsAny(value, "/?#") || strings.Contains(value, "://") {
		return false
	}
	switch mediaType {
	case "playlist":
		if strings.HasPrefix(value, "pl.") {
			return regexp.MustCompile(`^[a-zA-Z0-9._-]+$`).MatchString(value)
		}
		return regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(value)
	default:
		return regexp.MustCompile(`^\d+$`).MatchString(value)
	}
}

func extractAppleIDFromPath(path string, mediaType string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if segment != mediaType && segment != mediaType+"s" {
			continue
		}
		if i+2 < len(segments) {
			return segments[i+2]
		}
		if i+1 < len(segments) {
			return segments[i+1]
		}
	}
	return ""
}

// --- Response types ---

type appleArtwork struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func (a appleArtwork) CoverURL(size int) string {
	if a.URL == "" {
		return ""
	}
	s := strconv.Itoa(size)
	url := strings.ReplaceAll(a.URL, "{w}", s)
	url = strings.ReplaceAll(url, "{h}", s)
	return url
}

type appleSongAttributes struct {
	Name             string       `json:"name"`
	ArtistName       string       `json:"artistName"`
	AlbumName        string       `json:"albumName"`
	DurationInMillis int          `json:"durationInMillis"`
	Artwork          appleArtwork `json:"artwork"`
	URL              string       `json:"url"`
	Previews         []struct {
		URL string `json:"url"`
	} `json:"previews"`
	ExtendedAssetUrls struct {
		EnhancedHls string `json:"enhancedHls"`
	} `json:"extendedAssetUrls"`
	ISRC string `json:"isrc"`
}

type appleAlbumAttributes struct {
	Name           string       `json:"name"`
	ArtistName     string       `json:"artistName"`
	TrackCount     int          `json:"trackCount"`
	Artwork        appleArtwork `json:"artwork"`
	URL            string       `json:"url"`
	ReleaseDate    string       `json:"releaseDate"`
	RecordLabel    string       `json:"recordLabel"`
	EditorialNotes struct {
		Short string `json:"short"`
	} `json:"editorialNotes"`
}

type applePlaylistAttributes struct {
	Name        string       `json:"name"`
	CuratorName string       `json:"curatorName"`
	TrackCount  int          `json:"trackCount"`
	Artwork     appleArtwork `json:"artwork"`
	URL         string       `json:"url"`
	Description struct {
		Short    string `json:"short"`
		Standard string `json:"standard"`
	} `json:"description"`
	EditorialNotes struct {
		Short    string `json:"short"`
		Standard string `json:"standard"`
	} `json:"editorialNotes"`
	LastModifiedDate string `json:"lastModifiedDate"`
}

type appleLyricAttributes struct {
	Text string `json:"ttml"`
}

type appleResource struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Attributes    json.RawMessage `json:"attributes"`
	Relationships struct {
		Tracks appleTrackRelationship `json:"tracks"`
		Lyrics struct {
			Data []struct {
				Attributes appleLyricAttributes `json:"attributes"`
			} `json:"data"`
		} `json:"lyrics"`
	} `json:"relationships"`
}

type appleTrackRelationship struct {
	Data []appleResource `json:"data"`
	Href string          `json:"href"`
	Next string          `json:"next"`
}

type appleResourceResponse struct {
	Data []appleResource `json:"data"`
}

type appleResourceListResponse struct {
	Data []appleResource `json:"data"`
	Href string          `json:"href"`
	Next string          `json:"next"`
}

type appleSearchResponse struct {
	Results struct {
		Songs struct {
			Data []appleResource `json:"data"`
		} `json:"songs"`
		Albums struct {
			Data []appleResource `json:"data"`
		} `json:"albums"`
		Playlists struct {
			Data []appleResource `json:"data"`
		} `json:"playlists"`
	} `json:"results"`
}

// --- Converters ---

func appleSongFromCatalog(res appleResource) model.Song {
	var attr appleSongAttributes
	_ = json.Unmarshal(res.Attributes, &attr)

	previewURL := ""
	if len(attr.Previews) > 0 {
		previewURL = attr.Previews[0].URL
	}

	duration := attr.DurationInMillis / 1000

	return model.Song{
		Source:   "apple",
		ID:       res.ID,
		Name:     attr.Name,
		Artist:   attr.ArtistName,
		Album:    attr.AlbumName,
		Duration: duration,
		Cover:    attr.Artwork.CoverURL(600),
		URL:      previewURL,
		Link:     attr.URL,
		Ext:      "m4a",
		Extra: map[string]string{
			"isrc": attr.ISRC,
		},
	}
}

func appleAlbumToPlaylist(res appleResource) model.Playlist {
	var attr appleAlbumAttributes
	_ = json.Unmarshal(res.Attributes, &attr)

	return model.Playlist{
		Source:      "apple",
		ID:          res.ID,
		Name:        attr.Name,
		Cover:       attr.Artwork.CoverURL(300),
		TrackCount:  attr.TrackCount,
		Creator:     attr.ArtistName,
		Description: attr.EditorialNotes.Short,
		Link:        attr.URL,
		Extra: map[string]string{
			"release_date": attr.ReleaseDate,
			"record_label": attr.RecordLabel,
		},
	}
}

func applePlaylistToPlaylist(res appleResource) model.Playlist {
	var attr applePlaylistAttributes
	_ = json.Unmarshal(res.Attributes, &attr)

	description := attr.Description.Short
	if description == "" {
		description = attr.Description.Standard
	}
	if description == "" {
		description = attr.EditorialNotes.Short
	}
	if description == "" {
		description = attr.EditorialNotes.Standard
	}

	return model.Playlist{
		Source:      "apple",
		ID:          res.ID,
		Name:        attr.Name,
		Cover:       attr.Artwork.CoverURL(300),
		TrackCount:  attr.TrackCount,
		Creator:     attr.CuratorName,
		Description: description,
		Link:        attr.URL,
	}
}

func parseCookieParts(cookie string) map[string]string {
	result := make(map[string]string)
	for _, part := range strings.Split(cookie, ";") {
		part = strings.TrimSpace(part)
		if idx := strings.IndexByte(part, '='); idx > 0 {
			key := strings.TrimSpace(part[:idx])
			val := strings.TrimSpace(part[idx+1:])
			result[key] = val
		}
	}
	return result
}
