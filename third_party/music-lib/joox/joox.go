package joox

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	UserAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36"
	Cookie        = "wmid=142420656; user_type=1; country=id; session_key=2a5d97d05dc8fe238150184eaf3519ad;"
	XForwardedFor = "36.73.34.109"
)

type Joox struct {
	cookie string
}

func New(cookie string) *Joox {
	if cookie == "" {
		cookie = Cookie
	}
	return &Joox{cookie: cookie}
}

var defaultJoox = New(Cookie)

// [新增]
// [新增]

func (j *Joox) fetchPlaylistDetail(id string) (*model.Playlist, []model.Song, error) {
	playlist, songs, err := j.fetchPlaylistPageData(id)
	if err == nil {
		return playlist, songs, nil
	}

	playlistID := normalizeJooxID(id)
	if playlistID == "" {
		return nil, nil, errors.New("playlist id is empty")
	}

	songs, songsErr := j.GetPlaylistSongs(playlistID)
	if songsErr != nil {
		return nil, nil, err
	}

	playlist = &model.Playlist{
		Source:     "joox",
		ID:         playlistID,
		Name:       playlistID,
		TrackCount: len(songs),
		Link:       jooxPlaylistLink(playlistID),
		Extra: map[string]string{
			"type":        "playlist",
			"playlist_id": playlistID,
		},
	}
	if len(songs) > 0 {
		playlist.Cover = songs[0].Cover
	}

	return playlist, songs, nil
}

func (j *Joox) fetchPlaylistSongsFromPage(id string) ([]model.Song, error) {
	_, songs, err := j.fetchPlaylistPageData(id)
	return songs, err
}

func (j *Joox) fetchPlaylistPageData(id string) (*model.Playlist, []model.Song, error) {
	playlistID := normalizeJooxID(id)
	if playlistID == "" {
		return nil, nil, errors.New("playlist id is empty")
	}

	var lastErr error
	for _, pageURL := range jooxPlaylistLinks(playlistID) {
		playlist, songs, err := j.fetchPlaylistPageDataFromURL(playlistID, pageURL)
		if err == nil {
			return playlist, songs, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, nil, lastErr
	}
	return nil, nil, errors.New("joox playlist page data not found")
}

func (j *Joox) fetchPlaylistPageDataFromURL(playlistID string, pageURL string) (*model.Playlist, []model.Song, error) {
	body, err := utils.Get(pageURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return nil, nil, err
	}

	matches := regexp.MustCompile(`(?s)<script[^>]*id="__NEXT_DATA__"[^>]*>(.*?)</script>`).FindSubmatch(body)
	if len(matches) < 2 {
		return nil, nil, errors.New("joox playlist page data not found")
	}

	var nextData struct {
		Props struct {
			PageProps struct {
				AllPlaylistTracks jooxAllPlaylistTracks `json:"allPlaylistTracks"`
				PlaylistDetail    jooxPlaylistPageData  `json:"playlistDetailList"`
				Content           struct {
					Page struct {
						AllPlaylistTracks jooxAllPlaylistTracks `json:"allPlaylistTracks"`
						PlaylistDetail    jooxPlaylistPageData  `json:"playlistDetailList"`
					} `json:"page"`
				} `json:"content"`
			} `json:"pageProps"`
		} `json:"props"`
	}

	if err := json.Unmarshal(matches[1], &nextData); err != nil {
		return nil, nil, fmt.Errorf("joox playlist page json error: %w", err)
	}

	detail := nextData.Props.PageProps.PlaylistDetail
	tracks := nextData.Props.PageProps.AllPlaylistTracks.Tracks.Items
	trackCount := firstNonZero(
		nextData.Props.PageProps.AllPlaylistTracks.Tracks.TotalCount,
		nextData.Props.PageProps.AllPlaylistTracks.Tracks.TotalCountCamel,
		nextData.Props.PageProps.AllPlaylistTracks.Tracks.ListCount,
	)
	if len(tracks) == 0 {
		tracks = detail.TrackList
	}
	if len(tracks) == 0 {
		detail = nextData.Props.PageProps.Content.Page.PlaylistDetail
		tracks = nextData.Props.PageProps.Content.Page.AllPlaylistTracks.Tracks.Items
		trackCount = firstNonZero(
			nextData.Props.PageProps.Content.Page.AllPlaylistTracks.Tracks.TotalCount,
			nextData.Props.PageProps.Content.Page.AllPlaylistTracks.Tracks.TotalCountCamel,
			nextData.Props.PageProps.Content.Page.AllPlaylistTracks.Tracks.ListCount,
		)
		if len(tracks) == 0 {
			tracks = detail.TrackList
		}
	}
	if len(tracks) == 0 {
		return nil, nil, errors.New("playlist has no songs")
	}

	playlistID = normalizeJooxID(firstNonEmpty(detail.ID, detail.ListID, detail.PlaylistID, playlistID))
	name := firstNonEmpty(detail.Name, detail.Title, detail.PlaylistName, playlistID)
	cover := firstNonEmpty(detail.ImgSrc, detail.Cover, detail.Image, detail.Pic)
	trackCount = firstNonZero(trackCount, detail.TrackCount, detail.TotalCount, detail.TotalCountCamel, detail.ListCount, len(tracks))

	playlist := &model.Playlist{
		Source:      "joox",
		ID:          playlistID,
		Name:        name,
		Cover:       cover,
		TrackCount:  trackCount,
		Creator:     firstNonEmpty(detail.Creator, detail.CreatorName, detail.UserName, detail.NickName, detail.OwnerName, detail.Author, detail.AuthorName, detail.User.Name, detail.User.UserName, detail.User.NickName, detail.User.Nick, detail.Owner.Name, detail.Owner.UserName, detail.Owner.NickName, detail.Owner.Nick, detail.CreatorInfo.Name, detail.CreatorInfo.UserName, detail.CreatorInfo.NickName, detail.CreatorInfo.Nick),
		Description: firstNonEmpty(detail.Description, detail.Intro, detail.Desc),
		Link:        pageURL,
		Extra: map[string]string{
			"type":        "playlist",
			"playlist_id": playlistID,
		},
	}

	songs := make([]model.Song, 0, len(tracks))
	for _, item := range tracks {
		song := jooxSongFromTrack(item, name, cover, "")
		if song != nil {
			songs = append(songs, *song)
		}
	}
	if len(songs) == 0 {
		return nil, nil, errors.New("playlist has no playable songs")
	}

	return playlist, songs, nil
}

// Parse 解析链接并获取完整信息
func (j *Joox) fetchAlbumDetail(id string) (*model.Playlist, []model.Song, error) {
	albumData, err := j.fetchAlbumPageData(id)
	if err != nil {
		return nil, nil, err
	}

	albumID := normalizeJooxID(albumData.ID)
	if albumID == "" {
		return nil, nil, errors.New("album not found")
	}

	trackCount := albumData.TrackList.TotalCount
	if trackCount == 0 {
		if albumData.TrackList.ListCount > 0 {
			trackCount = albumData.TrackList.ListCount
		} else {
			trackCount = len(albumData.TrackList.Items)
		}
	}

	cover := strings.TrimSpace(albumData.ImgSrc)
	if cover == "" && len(albumData.TrackList.Items) > 0 {
		cover = pickJooxImage(albumData.TrackList.Items[0].Images)
	}

	album := &model.Playlist{
		Source:      "joox",
		ID:          albumID,
		Name:        albumData.Title,
		Cover:       cover,
		TrackCount:  trackCount,
		Creator:     joinJooxArtists(albumData.ArtistList),
		Description: strings.TrimSpace(albumData.Description),
		Link:        jooxAlbumLink(albumID),
		Extra: map[string]string{
			"type":         "album",
			"album_id":     albumID,
			"publish_date": strings.TrimSpace(albumData.PublishDate),
		},
	}

	songs := make([]model.Song, 0, len(albumData.TrackList.Items))
	for _, item := range albumData.TrackList.Items {
		songID := normalizeJooxID(item.ID)
		if songID == "" {
			continue
		}
		songs = append(songs, model.Song{
			Source:   "joox",
			ID:       songID,
			Name:     item.Name,
			Artist:   joinJooxArtists(item.ArtistList),
			Album:    firstNonEmpty(item.AlbumName, albumData.Title),
			Duration: item.PlayDuration,
			Cover:    firstNonEmpty(pickJooxImage(item.Images), strings.TrimSpace(albumData.ImgSrc)),
			Link:     fmt.Sprintf("https://www.joox.com/hk/single/%s", songID),
			Extra: map[string]string{
				"songid":   songID,
				"album_id": albumID,
			},
		})
	}

	if len(songs) == 0 {
		return nil, nil, errors.New("album has no songs")
	}

	return album, songs, nil
}

func (j *Joox) fetchAlbumPageData(id string) (*jooxAlbumPageData, error) {
	albumID := normalizeJooxID(id)
	if albumID == "" {
		return nil, errors.New("album id is empty")
	}

	pageURL := jooxAlbumLink(albumID)
	body, err := utils.Get(pageURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return nil, err
	}

	matches := regexp.MustCompile(`(?s)<script[^>]*id="__NEXT_DATA__"[^>]*>(.*?)</script>`).FindSubmatch(body)
	if len(matches) < 2 {
		return nil, errors.New("joox album page data not found")
	}

	var nextData struct {
		Props struct {
			PageProps struct {
				AlbumData jooxAlbumPageData `json:"albumData"`
				Content   struct {
					Page struct {
						AlbumData jooxAlbumPageData `json:"albumData"`
					} `json:"page"`
				} `json:"content"`
			} `json:"pageProps"`
		} `json:"props"`
	}

	if err := json.Unmarshal(matches[1], &nextData); err != nil {
		return nil, fmt.Errorf("joox album page json error: %w", err)
	}

	albumData := nextData.Props.PageProps.AlbumData
	if normalizeJooxID(albumData.ID) == "" {
		albumData = nextData.Props.PageProps.Content.Page.AlbumData
	}
	if normalizeJooxID(albumData.ID) == "" {
		return nil, errors.New("album data not found")
	}

	return &albumData, nil
}

type jooxArtist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type jooxImage struct {
	Width int    `json:"width"`
	URL   string `json:"url"`
}

type jooxAlbumTrack struct {
	ID                string       `json:"id"`
	Name              string       `json:"name"`
	Title             string       `json:"title"`
	AlbumID           string       `json:"album_id"`
	AlbumIDCamel      string       `json:"albumId"`
	AlbumName         string       `json:"album_name"`
	AlbumNameCamel    string       `json:"albumName"`
	ArtistList        []jooxArtist `json:"artist_list"`
	ArtistListCamel   []jooxArtist `json:"artistList"`
	PlayDuration      int          `json:"play_duration"`
	PlayDurationCamel int          `json:"playDuration"`
	Duration          int          `json:"duration"`
	Images            []jooxImage  `json:"images"`
	ImgSrc            string       `json:"imgSrc"`
	Cover             string       `json:"cover"`
	Image             string       `json:"image"`
	Pic               string       `json:"pic"`
}

type jooxAlbumPageData struct {
	ID          string       `json:"id"`
	ImgSrc      string       `json:"imgSrc"`
	Title       string       `json:"title"`
	ArtistList  []jooxArtist `json:"artistList"`
	PublishDate string       `json:"publishDate"`
	Description string       `json:"description"`
	TrackList   struct {
		Items      []jooxAlbumTrack `json:"items"`
		ListCount  int              `json:"list_count"`
		TotalCount int              `json:"total_count"`
	} `json:"trackList"`
}

type jooxAllPlaylistTracks struct {
	Tracks struct {
		Items           []jooxAlbumTrack `json:"items"`
		ListCount       int              `json:"list_count"`
		TotalCount      int              `json:"total_count"`
		TotalCountCamel int              `json:"totalCount"`
	} `json:"tracks"`
}

type jooxPlaylistPageData struct {
	ID              string           `json:"id"`
	ListID          string           `json:"listId"`
	PlaylistID      string           `json:"playlistId"`
	Name            string           `json:"name"`
	Title           string           `json:"title"`
	PlaylistName    string           `json:"playlistName"`
	ImgSrc          string           `json:"imgSrc"`
	Cover           string           `json:"cover"`
	Image           string           `json:"image"`
	Pic             string           `json:"pic"`
	Creator         string           `json:"creator"`
	CreatorName     string           `json:"creatorName"`
	UserName        string           `json:"userName"`
	NickName        string           `json:"nickName"`
	OwnerName       string           `json:"ownerName"`
	Author          string           `json:"author"`
	AuthorName      string           `json:"authorName"`
	User            jooxNamedUser    `json:"user"`
	Owner           jooxNamedUser    `json:"owner"`
	CreatorInfo     jooxNamedUser    `json:"creatorInfo"`
	Description     string           `json:"description"`
	Intro           string           `json:"intro"`
	Desc            string           `json:"desc"`
	TrackCount      int              `json:"trackCount"`
	TotalCount      int              `json:"total_count"`
	TotalCountCamel int              `json:"totalCount"`
	ListCount       int              `json:"list_count"`
	TrackList       []jooxAlbumTrack `json:"trackList"`
}

type jooxNamedUser struct {
	Name     string `json:"name"`
	UserName string `json:"userName"`
	NickName string `json:"nickName"`
	Nick     string `json:"nick"`
}

func joinJooxArtists(artists []jooxArtist) string {
	names := make([]string, 0, len(artists))
	for _, artist := range artists {
		name := strings.TrimSpace(artist.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, " / ")
}

func pickJooxImage(images []jooxImage) string {
	for _, preferred := range []int{300, 1000, 100} {
		for _, image := range images {
			if image.Width == preferred && strings.TrimSpace(image.URL) != "" {
				return image.URL
			}
		}
	}
	for _, image := range images {
		if strings.TrimSpace(image.URL) != "" {
			return image.URL
		}
	}
	return ""
}

func normalizeJooxID(raw string) string {
	id := strings.TrimSpace(raw)
	if id == "" {
		return ""
	}
	if strings.Contains(id, "%") {
		if decoded, err := url.PathUnescape(id); err == nil {
			id = decoded
		}
		if decoded, err := url.QueryUnescape(id); err == nil {
			id = strings.ReplaceAll(decoded, " ", "+")
		}
	}
	return id
}

func jooxAlbumLink(id string) string {
	return fmt.Sprintf("https://www.joox.com/hk/album/%s", normalizeJooxID(id))
}

func jooxPlaylistLink(id string) string {
	return jooxRegionalPlaylistLink("id", id)
}

func jooxRegionalPlaylistLink(region string, id string) string {
	return fmt.Sprintf("https://www.joox.com/%s/playlist/%s", region, normalizeJooxID(id))
}

func jooxPlaylistLinks(id string) []string {
	return []string{
		jooxRegionalPlaylistLink("id", id),
		jooxRegionalPlaylistLink("sg", id),
		jooxRegionalPlaylistLink("hk", id),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func jooxSongFromTrack(item jooxAlbumTrack, fallbackAlbum, fallbackCover, fallbackAlbumID string) *model.Song {
	songID := normalizeJooxID(item.ID)
	if songID == "" {
		return nil
	}

	artists := item.ArtistList
	if len(artists) == 0 {
		artists = item.ArtistListCamel
	}

	albumID := normalizeJooxID(firstNonEmpty(item.AlbumID, item.AlbumIDCamel, fallbackAlbumID))
	extra := map[string]string{
		"songid": songID,
	}
	if albumID != "" {
		extra["album_id"] = albumID
	}

	return &model.Song{
		Source:   "joox",
		ID:       songID,
		Name:     firstNonEmpty(item.Name, item.Title, "Unknown"),
		Artist:   firstNonEmpty(joinJooxArtists(artists), "Unknown"),
		Album:    firstNonEmpty(item.AlbumName, item.AlbumNameCamel, fallbackAlbum),
		AlbumID:  albumID,
		Duration: firstNonZero(item.PlayDuration, item.PlayDurationCamel, item.Duration),
		Cover:    firstNonEmpty(pickJooxImage(item.Images), item.ImgSrc, item.Cover, item.Image, item.Pic, fallbackCover),
		Link:     fmt.Sprintf("https://www.joox.com/hk/single/%s", songID),
		Extra:    extra,
	}
}

// fetchSongInfo 内部函数：获取歌曲详情和下载链接
func (j *Joox) fetchSongInfo(songID string) (*model.Song, error) {
	params := url.Values{}
	params.Set("songid", songID)
	params.Set("lang", "zh_cn")
	params.Set("country", "sg")

	apiURL := "https://api.joox.com/web-fcgi-bin/web_get_songinfo?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return nil, err
	}

	bodyStr := string(body)
	if strings.HasPrefix(bodyStr, "MusicInfoCallback(") {
		bodyStr = strings.TrimPrefix(bodyStr, "MusicInfoCallback(")
		bodyStr = strings.TrimSuffix(bodyStr, ")")
	}

	var resp struct {
		Msong     string      `json:"msong"`   // 歌名
		Msinger   string      `json:"msinger"` // 歌手
		Malbum    string      `json:"malbum"`  // 专辑
		Img       string      `json:"img"`     // 封面
		MInterval int         `json:"minterval"`
		R320Url   string      `json:"r320Url"`
		R192Url   string      `json:"r192Url"`
		Mp3Url    string      `json:"mp3Url"`
		M4aUrl    string      `json:"m4aUrl"`
		KbpsMap   interface{} `json:"kbps_map"`
	}

	if err := json.Unmarshal([]byte(bodyStr), &resp); err != nil {
		return nil, fmt.Errorf("joox detail json error: %w", err)
	}

	// 解析下载链接
	availableQualities := make(map[string]interface{})
	if kbpsMapStr, ok := resp.KbpsMap.(string); ok {
		json.Unmarshal([]byte(kbpsMapStr), &availableQualities)
	} else if kbpsMapObj, ok := resp.KbpsMap.(map[string]interface{}); ok {
		availableQualities = kbpsMapObj
	}

	type Candidate struct {
		MapKey string
		URL    string
	}
	candidates := []Candidate{
		{"320", resp.R320Url}, {"192", resp.R192Url}, {"128", resp.Mp3Url}, {"96", resp.M4aUrl},
	}

	var downloadURL string
	for _, c := range candidates {
		if val, ok := availableQualities[c.MapKey]; ok {
			hasSize := false
			switch v := val.(type) {
			case string:
				hasSize = v != "0" && v != ""
			case float64:
				hasSize = v > 0
			case int:
				hasSize = v > 0
			}
			if hasSize && c.URL != "" {
				downloadURL = c.URL
				break
			}
		}
	}

	if downloadURL == "" {
		downloadURL = firstNonEmpty(resp.R320Url, resp.R192Url, resp.Mp3Url, resp.M4aUrl)
	}

	if downloadURL == "" {
		return nil, errors.New("no valid download url found")
	}

	return &model.Song{
		Source:   "joox",
		ID:       songID,
		Name:     resp.Msong,
		Artist:   resp.Msinger,
		Album:    resp.Malbum,
		Duration: resp.MInterval,
		Cover:    resp.Img,
		URL:      downloadURL,
		Link:     fmt.Sprintf("https://www.joox.com/hk/single/%s", songID),
		Extra: map[string]string{
			"songid": songID,
		},
	}, nil
}
