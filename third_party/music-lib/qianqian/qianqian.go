package qianqian

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	AppID     = "16073360"
	Secret    = "0b50b02fd0d73a9c4c8c3a781c30845f"
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"
	Referer   = "https://music.91q.com/player"
)

type Qianqian struct {
	cookie string
}

func New(cookie string) *Qianqian { return &Qianqian{cookie: cookie} }

var defaultQianqian = New("")

type qianqianArtist struct {
	Name       string `json:"name"`
	ArtistType int    `json:"artistType"`
}

type qianqianRateFileInfo struct {
	Size   int64  `json:"size"`
	Format string `json:"format"`
}

type qianqianAlbumSearchItem struct {
	AlbumAssetCode string            `json:"albumAssetCode"`
	Title          string            `json:"title"`
	Pic            string            `json:"pic"`
	Introduce      string            `json:"introduce"`
	ReleaseDate    string            `json:"releaseDate"`
	Genre          string            `json:"genre"`
	Lang           string            `json:"lang"`
	Artist         []qianqianArtist  `json:"artist"`
	TrackList      []json.RawMessage `json:"trackList"`
}

// [新增]

// SearchPlaylist 搜索歌单
func (q *Qianqian) searchAlbumItems(keyword string) ([]qianqianAlbumSearchItem, error) {
	keywords := []string{keyword}
	if sanitized := sanitizeQianqianAlbumKeyword(keyword); sanitized != "" && sanitized != keyword {
		keywords = append(keywords, sanitized)
	}

	var lastErr error
	for _, currentKeyword := range keywords {
		items, retryWithSanitized, err := q.searchAlbumItemsOnce(currentKeyword)
		if err == nil {
			return items, nil
		}
		lastErr = err
		if retryWithSanitized && currentKeyword == keyword {
			continue
		}
		break
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("no albums found")
}

func (q *Qianqian) searchAlbumItemsOnce(keyword string) ([]qianqianAlbumSearchItem, bool, error) {
	params := url.Values{}
	params.Set("word", keyword)
	params.Set("type", "3")
	params.Set("pageNo", "1")
	params.Set("pageSize", "10")
	params.Set("appid", AppID)
	signParams(params)

	apiURL := "https://music.91q.com/v1/search?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", q.cookie),
	)
	if err != nil {
		return nil, false, err
	}

	var rawResp struct {
		State bool            `json:"state"`
		Errno int             `json:"errno"`
		Msg   string          `json:"errmsg"`
		Data  json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, false, fmt.Errorf("qianqian album json parse error: %w", err)
	}

	if !rawResp.State {
		if rawResp.Errno == 23001 {
			return nil, true, fmt.Errorf("api error: %s (code %d)", rawResp.Msg, rawResp.Errno)
		}
		if rawResp.Errno != 22000 {
			return nil, false, fmt.Errorf("api error: %s (code %d)", rawResp.Msg, rawResp.Errno)
		}
	}

	if len(rawResp.Data) == 0 || string(rawResp.Data) == "[]" || string(rawResp.Data) == "null" {
		return nil, false, errors.New("no albums found")
	}

	var dataObj struct {
		TypeAlbum []qianqianAlbumSearchItem `json:"typeAlbum"`
	}
	if err := json.Unmarshal(rawResp.Data, &dataObj); err != nil {
		return nil, false, fmt.Errorf("qianqian album json parse error: %w", err)
	}
	if len(dataObj.TypeAlbum) == 0 {
		return nil, false, errors.New("no albums found")
	}

	return dataObj.TypeAlbum, false, nil
}

func (q *Qianqian) fetchPlaylistDetail(id string) (*model.Playlist, []model.Song, error) {
	playlistID := strings.TrimSpace(id)
	if playlistID == "" {
		return nil, nil, errors.New("playlist id is empty")
	}

	params := url.Values{}
	params.Set("id", playlistID)
	params.Set("appid", AppID)
	params.Set("type", "0")
	signParams(params)

	apiURL := "https://music.91q.com/v1/tracklist/info?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", q.cookie),
	)
	if err != nil {
		return nil, nil, err
	}

	var resp struct {
		Data struct {
			ID          interface{} `json:"id"`
			Title       string      `json:"title"`
			Pic         string      `json:"pic"`
			Desc        string      `json:"desc"`
			Description string      `json:"description"`
			TrackCount  int         `json:"trackCount"`
			TagList     []string    `json:"tagList"`
			Creator     string      `json:"creator"`
			Author      string      `json:"author"`
			UserName    string      `json:"userName"`
			NickName    string      `json:"nickName"`
			OwnerName   string      `json:"ownerName"`
			TrackList   []struct {
				TSID       string           `json:"TSID"`
				Title      string           `json:"title"`
				AlbumTitle string           `json:"albumTitle"`
				Pic        string           `json:"pic"`
				Duration   int              `json:"duration"`
				Artist     []qianqianArtist `json:"artist"`
				IsVip      int              `json:"isVip"`
			} `json:"trackList"`
		} `json:"data"`
		Errno  int    `json:"errno"`
		ErrMsg string `json:"errmsg"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("qianqian playlist detail json error: %w", err)
	}

	if resp.Errno != 0 && resp.Errno != 22000 {
		return nil, nil, fmt.Errorf("api error: %s (code %d)", resp.ErrMsg, resp.Errno)
	}

	var songs []model.Song
	for _, item := range resp.Data.TrackList {
		cover := strings.TrimSpace(item.Pic)
		if cover == "" {
			cover = strings.TrimSpace(resp.Data.Pic)
		}
		songs = append(songs, model.Song{
			Source:   "qianqian",
			ID:       item.TSID,
			Name:     item.Title,
			Artist:   joinQianqianArtists(item.Artist),
			Album:    item.AlbumTitle,
			Duration: item.Duration,
			Cover:    cover,
			Link:     fmt.Sprintf("https://music.91q.com/song/%s", item.TSID),
			Extra: map[string]string{
				"tsid": item.TSID,
			},
		})
	}

	if len(songs) == 0 {
		return nil, nil, errors.New("playlist is empty or invalid")
	}

	trackCount := resp.Data.TrackCount
	if trackCount == 0 {
		trackCount = len(songs)
	}

	description := qianqianFirstNonEmpty(resp.Data.Desc, resp.Data.Description)
	if description == "" && len(resp.Data.TagList) > 0 {
		description = strings.Join(resp.Data.TagList, "、")
	}

	playlist := &model.Playlist{
		Source:      "qianqian",
		ID:          playlistID,
		Name:        qianqianFirstNonEmpty(resp.Data.Title, playlistID),
		Cover:       qianqianFirstNonEmpty(resp.Data.Pic, songs[0].Cover),
		TrackCount:  trackCount,
		Creator:     qianqianFirstNonEmpty(resp.Data.Creator, resp.Data.Author, resp.Data.UserName, resp.Data.NickName, resp.Data.OwnerName),
		Description: description,
		Link:        qianqianPlaylistLink(playlistID),
		Extra: map[string]string{
			"type":        "playlist",
			"playlist_id": playlistID,
		},
	}

	return playlist, songs, nil
}

func (q *Qianqian) fetchAlbumDetail(id string) (*model.Playlist, []model.Song, error) {
	albumID, err := q.resolveAlbumAssetCode(id)
	if err != nil {
		return nil, nil, err
	}

	params := url.Values{}
	params.Set("albumAssetCode", albumID)
	params.Set("appid", AppID)
	signParams(params)

	apiURL := "https://music.91q.com/v1/album/info?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", q.cookie),
	)
	if err != nil {
		return nil, nil, err
	}

	var resp struct {
		State  bool   `json:"state"`
		Errno  int    `json:"errno"`
		ErrMsg string `json:"errmsg"`
		Data   struct {
			AlbumAssetCode string           `json:"albumAssetCode"`
			Title          string           `json:"title"`
			Pic            string           `json:"pic"`
			Introduce      string           `json:"introduce"`
			ReleaseDate    string           `json:"releaseDate"`
			Genre          string           `json:"genre"`
			Lang           string           `json:"lang"`
			Artist         []qianqianArtist `json:"artist"`
			TrackList      []struct {
				Duration     int                             `json:"duration"`
				Artist       []qianqianArtist                `json:"artist"`
				AssetID      string                          `json:"assetId"`
				Sort         int                             `json:"sort"`
				Title        string                          `json:"title"`
				IsVip        int                             `json:"isVip"`
				IsPaid       int                             `json:"isPaid"`
				RateFileInfo map[string]qianqianRateFileInfo `json:"rateFileInfo"`
			} `json:"trackList"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("qianqian album detail json error: %w", err)
	}
	if !resp.State && resp.Errno != 22000 {
		return nil, nil, fmt.Errorf("api error: %s (code %d)", resp.ErrMsg, resp.Errno)
	}
	if normalizeQianqianAlbumAssetCode(resp.Data.AlbumAssetCode) == "" {
		return nil, nil, errors.New("album not found")
	}

	album := &model.Playlist{
		Source:      "qianqian",
		ID:          albumID,
		Name:        resp.Data.Title,
		Cover:       resp.Data.Pic,
		TrackCount:  len(resp.Data.TrackList),
		Creator:     joinQianqianArtists(resp.Data.Artist),
		Description: resp.Data.Introduce,
		Link:        qianqianAlbumLink(albumID),
		Extra: map[string]string{
			"type":         "album",
			"album_id":     albumID,
			"release_date": qianqianReleaseDate(resp.Data.ReleaseDate),
			"genre":        strings.TrimSpace(resp.Data.Genre),
			"lang":         strings.TrimSpace(resp.Data.Lang),
		},
	}

	songs := make([]model.Song, 0, len(resp.Data.TrackList))
	for _, item := range resp.Data.TrackList {
		songID := strings.TrimSpace(item.AssetID)
		if songID == "" {
			continue
		}

		artist := joinQianqianArtists(item.Artist)
		if artist == "" {
			artist = album.Creator
		}
		size, bitrate := qianqianRateStats(item.RateFileInfo, item.Duration)

		song := model.Song{
			Source:   "qianqian",
			ID:       songID,
			Name:     item.Title,
			Artist:   artist,
			Album:    album.Name,
			AlbumID:  album.ID,
			Duration: item.Duration,
			Size:     size,
			Bitrate:  bitrate,
			Cover:    album.Cover,
			Link:     fmt.Sprintf("https://music.91q.com/song/%s", songID),
			Extra: map[string]string{
				"tsid":     songID,
				"album_id": album.ID,
			},
		}
		if item.Sort > 0 {
			song.Extra["track"] = strconv.Itoa(item.Sort)
		}

		songs = append(songs, song)
	}
	if len(songs) == 0 {
		return nil, nil, errors.New("album is empty or invalid")
	}
	if album.TrackCount == 0 {
		album.TrackCount = len(songs)
	}

	return album, songs, nil
}

// fetchDownloadURL 内部方法：获取下载链接
func (q *Qianqian) fetchDownloadURL(tsid string) (string, error) {
	qualities := []string{"3000", "320", "128", "64"}
	for _, rate := range qualities {
		params := url.Values{}
		params.Set("TSID", tsid)
		params.Set("appid", AppID)
		params.Set("rate", rate)
		signParams(params)
		apiURL := "https://music.91q.com/v1/song/tracklink?" + params.Encode()

		body, err := utils.Get(apiURL,
			utils.WithHeader("User-Agent", UserAgent),
			utils.WithHeader("Referer", Referer),
			utils.WithHeader("Cookie", q.cookie),
		)
		if err != nil {
			continue
		}

		var resp struct {
			Data struct {
				Path           string `json:"path"`
				Format         string `json:"format"`
				Size           int64  `json:"size"`
				Duration       int    `json:"duration"`
				TrailAudioInfo struct {
					Path string `json:"path"`
				} `json:"trail_audio_info"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}
		downloadURL := resp.Data.Path
		if downloadURL == "" {
			downloadURL = resp.Data.TrailAudioInfo.Path
		}
		if downloadURL != "" {
			return downloadURL, nil
		}
	}
	return "", errors.New("download url not found")
}

// fetchSongInfo 内部方法：获取元数据
func (q *Qianqian) fetchSongInfo(tsid string) (*model.Song, error) {
	params := url.Values{}
	params.Set("TSID", tsid)
	params.Set("appid", AppID)
	signParams(params)
	apiURL := "https://music.91q.com/v1/song/info?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", q.cookie),
	)
	if err != nil {
		return nil, err
	}

	// 该接口不仅返回 lyric，还返回歌曲基本信息，虽然结构体需要扩展
	var resp struct {
		Data []struct {
			Title      string           `json:"title"`
			AlbumTitle string           `json:"albumTitle"`
			Pic        string           `json:"pic"`
			Duration   int              `json:"duration"`
			Artist     []qianqianArtist `json:"artist"`
			Lyric      string           `json:"lyric"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qianqian song info parse error: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, errors.New("song info not found")
	}

	item := resp.Data[0]

	return &model.Song{
		Source:   "qianqian",
		ID:       tsid,
		Name:     item.Title,
		Artist:   joinQianqianArtists(item.Artist),
		Album:    item.AlbumTitle,
		Duration: item.Duration,
		Cover:    item.Pic,
		Link:     fmt.Sprintf("https://music.91q.com/song/%s", tsid),
		Extra: map[string]string{
			"tsid": tsid,
		},
	}, nil
}

func (q *Qianqian) resolveAlbumAssetCode(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", errors.New("album id is empty")
	}

	if normalized := normalizeQianqianAlbumAssetCode(id); normalized != "" {
		return normalized, nil
	}

	params := url.Values{}
	params.Set("albumid", id)
	params.Set("appid", AppID)
	signParams(params)

	apiURL := "https://music.91q.com/v1/album/albumid2psid?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", q.cookie),
	)
	if err != nil {
		return "", err
	}

	var resp struct {
		State  bool   `json:"state"`
		Errno  int    `json:"errno"`
		ErrMsg string `json:"errmsg"`
		Data   []struct {
			PSID string `json:"psid"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("qianqian album id json parse error: %w", err)
	}
	if !resp.State && resp.Errno != 22000 {
		return "", fmt.Errorf("api error: %s (code %d)", resp.ErrMsg, resp.Errno)
	}

	for _, item := range resp.Data {
		if psid := normalizeQianqianAlbumAssetCode(item.PSID); psid != "" {
			return psid, nil
		}
	}

	return "", errors.New("album asset code not found")
}

func normalizeQianqianAlbumAssetCode(id string) string {
	id = strings.TrimSpace(id)
	if len(id) < 2 {
		return ""
	}
	if id[0] == 'p' || id[0] == 'P' {
		return "P" + id[1:]
	}
	return ""
}

func sanitizeQianqianAlbumKeyword(keyword string) string {
	replacer := strings.NewReplacer(
		":", " ",
		"：", " ",
		"\"", " ",
		"'", " ",
		"“", " ",
		"”", " ",
		"‘", " ",
		"’", " ",
		"(", " ",
		")", " ",
		"（", " ",
		"）", " ",
		"[", " ",
		"]", " ",
		"【", " ",
		"】", " ",
		",", " ",
		"，", " ",
		"/", " ",
		"\\", " ",
		"-", " ",
		".", " ",
	)
	return strings.Join(strings.Fields(replacer.Replace(strings.TrimSpace(keyword))), " ")
}

func qianqianAlbumLink(id string) string {
	return fmt.Sprintf("https://music.91q.com/album/%s", id)
}

func qianqianPlaylistLink(id string) string {
	return fmt.Sprintf("https://music.91q.com/songlist/%s", id)
}

func qianqianValueString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

func qianqianReleaseDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 10 {
		return raw[:10]
	}
	return raw
}

func qianqianFirstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func joinQianqianArtists(artists []qianqianArtist) string {
	if len(artists) == 0 {
		return ""
	}

	names := make([]string, 0, len(artists))
	seen := make(map[string]struct{}, len(artists))
	addName := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	for _, artist := range artists {
		if artist.ArtistType == 38 {
			addName(artist.Name)
		}
	}
	if len(names) == 0 {
		for _, artist := range artists {
			addName(artist.Name)
		}
	}

	return strings.Join(names, "、")
}

func qianqianRateStats(rateFileInfo map[string]qianqianRateFileInfo, duration int) (int64, int) {
	for _, rate := range []string{"3000", "320", "128", "64"} {
		info, ok := rateFileInfo[rate]
		if !ok || info.Size <= 0 {
			continue
		}

		if duration > 0 {
			return info.Size, int(info.Size * 8 / 1000 / int64(duration))
		}

		if rate == "3000" {
			return info.Size, 800
		}

		bitrate, _ := strconv.Atoi(rate)
		return info.Size, bitrate
	}

	return 0, 0
}

func signParams(v url.Values) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	v.Set("timestamp", timestamp)
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			buf.WriteString("&")
		}
		buf.WriteString(k)
		buf.WriteString("=")
		buf.WriteString(v.Get(k))
	}
	buf.WriteString(Secret)
	hash := md5.Sum([]byte(buf.String()))
	sign := hex.EncodeToString(hash[:])
	v.Set("sign", sign)
}
