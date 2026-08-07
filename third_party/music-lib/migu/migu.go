package migu

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const (
	UserAgent   = "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13B143 Safari/601.1"
	Referer     = "http://music.migu.cn/"
	MagicUserID = "15548614588710179085069"
)

type Migu struct {
	cookie string
}

func New(cookie string) *Migu { return &Migu{cookie: cookie} }

var defaultMigu = New("")

func (m *Migu) fetchPlaylistInfo(id string) (*model.Playlist, error) {
	playlistID := strings.TrimSpace(id)
	if playlistID == "" {
		return nil, errors.New("playlist id is empty")
	}

	params := url.Values{}
	params.Set("needSimple", "00")
	params.Set("resourceType", "2021")
	params.Set("resourceId", playlistID)

	apiURL := "https://app.c.nf.migu.cn/MIGUM2.0/v1.0/content/resourceinfo.do?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", m.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code     string `json:"code"`
		Info     string `json:"info"`
		Resource []struct {
			MusicListID    string `json:"musicListId"`
			Title          string `json:"title"`
			Summary        string `json:"summary"`
			MusicNum       int    `json:"musicNum"`
			OriginalImgURL string `json:"originalImgUrl"`
			OwnerName      string `json:"ownerName"`
			ResourceType   string `json:"resourceType"`
			ImgItem        struct {
				Img     string `json:"img"`
				ImgOri  string `json:"imgOri"`
				WebpImg string `json:"webpImg"`
			} `json:"imgItem"`
			OpNumItem struct {
				PlayNum int `json:"playNum"`
			} `json:"opNumItem"`
		} `json:"resource"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("migu playlist info json parse error: %w", err)
	}
	if resp.Code != "" && resp.Code != "000000" {
		return nil, fmt.Errorf("migu api error: %s (code %s)", resp.Info, resp.Code)
	}
	if len(resp.Resource) == 0 {
		return nil, errors.New("migu playlist info not found")
	}

	info := resp.Resource[0]
	playlistID = firstNonEmpty(info.MusicListID, playlistID)
	playlist := &model.Playlist{
		Source:      "migu",
		ID:          playlistID,
		Name:        firstNonEmpty(info.Title, playlistID),
		Cover:       firstNonEmpty(info.OriginalImgURL, info.ImgItem.Img, info.ImgItem.WebpImg, info.ImgItem.ImgOri),
		TrackCount:  info.MusicNum,
		PlayCount:   info.OpNumItem.PlayNum,
		Creator:     info.OwnerName,
		Description: info.Summary,
		Link:        miguPlaylistLink(playlistID),
		Extra: map[string]string{
			"type":          "playlist",
			"playlist_id":   playlistID,
			"resource_type": firstNonEmpty(info.ResourceType, "2021"),
		},
	}

	return playlist, nil
}

func (m *Migu) fetchPlaylistDetail(id string) (*model.Playlist, []model.Song, error) {
	playlistID := strings.TrimSpace(id)
	if playlistID == "" {
		return nil, nil, errors.New("playlist id is empty")
	}

	songs, err := m.GetPlaylistSongs(playlistID)
	if err != nil {
		return nil, nil, err
	}

	playlist, infoErr := m.fetchPlaylistInfo(playlistID)
	if infoErr != nil {
		playlist = &model.Playlist{
			Source:     "migu",
			ID:         playlistID,
			Name:       playlistID,
			TrackCount: len(songs),
			Link:       miguPlaylistLink(playlistID),
			Extra: map[string]string{
				"type":        "playlist",
				"playlist_id": playlistID,
			},
		}
	}
	if playlist.TrackCount == 0 {
		playlist.TrackCount = len(songs)
	}
	if playlist.Cover == "" && len(songs) > 0 {
		playlist.Cover = songs[0].Cover
	}

	return playlist, songs, nil
}

// Parse 解析链接并获取完整信息
func (m *Migu) fetchAlbumDetail(id string) (*model.Playlist, []model.Song, error) {
	albumID := strings.TrimSpace(id)
	if albumID == "" {
		return nil, nil, errors.New("album id is empty")
	}

	songs, totalSongs, err := m.fetchAlbumSongs(albumID)
	if err != nil {
		return nil, nil, err
	}

	params := url.Values{}
	params.Set("needSimple", "00")
	params.Set("resourceType", "2003")
	params.Set("resourceId", albumID)

	apiURL := "https://app.c.nf.migu.cn/MIGUM2.0/v1.0/content/resourceinfo.do?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", m.cookie),
	)
	if err != nil {
		return nil, nil, err
	}

	var resp struct {
		Code     string `json:"code"`
		Info     string `json:"info"`
		Resource []struct {
			ResourceType   string          `json:"resourceType"`
			AlbumID        string          `json:"albumId"`
			ImgItems       []miguImageItem `json:"imgItems"`
			Title          string          `json:"title"`
			Singer         string          `json:"singer"`
			Summary        string          `json:"summary"`
			TotalCount     string          `json:"totalCount"`
			PublishTime    string          `json:"publishTime"`
			PublishCorp    string          `json:"publishCorp"`
			AlbumAliasName string          `json:"albumAliasName"`
			AlbumClass     string          `json:"albumClass"`
			Language       string          `json:"language"`
			PublishCompany string          `json:"publishCompany"`
			PublishDate    string          `json:"publishDate"`
			TranslateName  string          `json:"translateName"`
		} `json:"resource"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("migu album detail json parse error: %w", err)
	}
	if resp.Code != "" && resp.Code != "000000" {
		return nil, nil, fmt.Errorf("migu api error: %s (code %s)", resp.Info, resp.Code)
	}
	if len(resp.Resource) == 0 {
		return nil, nil, errors.New("album not found")
	}

	info := resp.Resource[0]
	albumID = firstNonEmpty(strings.TrimSpace(info.AlbumID), albumID)
	trackCount, _ := strconv.Atoi(strings.TrimSpace(info.TotalCount))
	if trackCount == 0 {
		if totalSongs > 0 {
			trackCount = totalSongs
		} else {
			trackCount = len(songs)
		}
	}

	album := &model.Playlist{
		Source:      "migu",
		ID:          albumID,
		Name:        strings.TrimSpace(info.Title),
		Cover:       pickMiguImage(info.ImgItems),
		TrackCount:  trackCount,
		Creator:     strings.TrimSpace(info.Singer),
		Description: strings.TrimSpace(info.Summary),
		Link:        miguAlbumLink(albumID),
		Extra: map[string]string{
			"type":            "album",
			"album_id":        albumID,
			"resource_type":   firstNonEmpty(strings.TrimSpace(info.ResourceType), "2003"),
			"publish_time":    strings.TrimSpace(info.PublishTime),
			"publish_date":    strings.TrimSpace(info.PublishDate),
			"publish_corp":    strings.TrimSpace(info.PublishCorp),
			"publish_company": strings.TrimSpace(info.PublishCompany),
			"album_alias":     strings.TrimSpace(info.AlbumAliasName),
			"album_class":     strings.TrimSpace(info.AlbumClass),
			"language":        strings.TrimSpace(info.Language),
			"translate_name":  strings.TrimSpace(info.TranslateName),
		},
	}

	return album, songs, nil
}

func (m *Migu) fetchAlbumSongs(id string) ([]model.Song, int, error) {
	albumID := strings.TrimSpace(id)
	if albumID == "" {
		return nil, 0, errors.New("album id is empty")
	}

	const pageSize = 50

	seen := make(map[string]struct{})
	songs := make([]model.Song, 0, pageSize)
	totalCount := 0

	for pageNo := 1; ; pageNo++ {
		params := url.Values{}
		params.Set("albumId", albumID)
		params.Set("pageNo", strconv.Itoa(pageNo))
		params.Set("pageSize", strconv.Itoa(pageSize))

		apiURL := "https://app.c.nf.migu.cn/MIGUM2.0/v1.0/content/queryAlbumSong?" + params.Encode()
		body, err := utils.Get(apiURL,
			utils.WithHeader("User-Agent", UserAgent),
			utils.WithHeader("Referer", Referer),
			utils.WithHeader("Cookie", m.cookie),
		)
		if err != nil {
			return nil, 0, err
		}

		var resp struct {
			Code string `json:"code"`
			Info string `json:"info"`
			Data struct {
				SongList   []MiguSongItem `json:"songList"`
				TotalCount int            `json:"totalCount"`
			} `json:"data"`
		}

		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, 0, fmt.Errorf("migu album songs json parse error: %w", err)
		}
		if resp.Code != "" && resp.Code != "000000" {
			return nil, 0, fmt.Errorf("migu api error: %s (code %s)", resp.Info, resp.Code)
		}

		if totalCount == 0 {
			totalCount = resp.Data.TotalCount
		}
		if len(resp.Data.SongList) == 0 {
			break
		}

		before := len(songs)
		for _, item := range resp.Data.SongList {
			song := m.convertItemToSong(item)
			if song == nil {
				continue
			}

			key := firstNonEmpty(song.Extra["content_id"], item.ContentID, song.ID, item.CopyrightID)
			if key == "" {
				key = song.Name + "|" + song.Artist
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			songs = append(songs, *song)
		}

		if len(resp.Data.SongList) < pageSize {
			break
		}
		if totalCount > 0 && len(songs) >= totalCount {
			break
		}
		if len(songs) == before {
			break
		}
	}

	if len(songs) == 0 {
		return nil, totalCount, errors.New("album has no playable songs")
	}

	return songs, totalCount, nil
}

// 内部结构体定义，用于 Search 和 Parse 复用
type miguImageItem struct {
	ImgSizeType string `json:"imgSizeType"`
	Img         string `json:"img"`
}

type miguArtistItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type miguAlbumItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type miguRateFormat struct {
	FormatType      string   `json:"formatType"`
	ResourceType    string   `json:"resourceType"`
	Size            string   `json:"size"`
	AndroidSize     string   `json:"androidSize"`
	ISize           string   `json:"isize"`
	ASize           string   `json:"asize"`
	FileType        string   `json:"fileType"`
	AndroidFileType string   `json:"androidFileType"`
	IFormat         string   `json:"iformat"`
	AFormat         string   `json:"aformat"`
	Price           string   `json:"price"`
	ShowTag         []string `json:"showTag"`
	ShowTags        []string `json:"showTags"`
}

type MiguSongItem struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	SongName        string           `json:"songName"`
	SongID          string           `json:"songId"`
	Singers         []miguArtistItem `json:"singers"`
	Artists         []miguArtistItem `json:"artists"`
	SingerList      []miguArtistItem `json:"singerList"`
	Albums          []miguAlbumItem  `json:"albums"`
	AlbumID         string           `json:"albumId"`
	Album           string           `json:"album"`
	Singer          string           `json:"singer"`
	ContentID       string           `json:"contentId"`
	CopyrightID     string           `json:"copyrightId"`
	ChargeAuditions string           `json:"chargeAuditions"`
	ImgItems        []miguImageItem  `json:"imgItems"`
	AlbumImgs       []miguImageItem  `json:"albumImgs"`
	RateFormats     []miguRateFormat `json:"rateFormats"`
	AudioFormats    []miguRateFormat `json:"audioFormats"`
	Img1            string           `json:"img1"`
	Img2            string           `json:"img2"`
	Img3            string           `json:"img3"`
	Duration        int              `json:"duration"`
}

// fetchSongDetail 通过 contentId 获取歌曲详情
func (m *Migu) fetchSongDetail(contentID string) (*model.Song, error) {
	params := url.Values{}
	params.Set("resourceType", "2")
	params.Set("resourceId", contentID)

	apiURL := "http://c.musicapp.migu.cn/MIGUM2.0/v1.0/content/resourceinfo.do?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", m.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Resource []MiguSongItem `json:"resource"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp.Resource) == 0 {
		return nil, errors.New("song detail not found")
	}

	song := m.convertItemToSong(resp.Resource[0])
	if song == nil {
		return nil, errors.New("no valid format found for this song")
	}
	return song, nil
}

// convertItemToSong 将 API 返回的 Item 转换为 Song 模型 (复用 Search 中的逻辑)
func (m *Migu) convertItemToSong(item MiguSongItem) *model.Song {
	return m.convertItemToSongWithOption(item, false)
}

func (m *Migu) convertItemToSongAllowPaid(item MiguSongItem) *model.Song {
	return m.convertItemToSongWithOption(item, true)
}

func (m *Migu) convertItemToSongWithOption(item MiguSongItem, allowPaid bool) *model.Song {
	artistNames := collectMiguArtistNames(item)
	songName := firstNonEmpty(strings.TrimSpace(item.Name), strings.TrimSpace(item.SongName))
	albumName := strings.TrimSpace(item.Album)
	if len(item.Albums) > 0 && strings.TrimSpace(item.Albums[0].Name) != "" {
		albumName = strings.TrimSpace(item.Albums[0].Name)
	}

	rateFormats := item.RateFormats
	if len(rateFormats) == 0 {
		rateFormats = item.AudioFormats
	}
	if len(rateFormats) == 0 {
		return nil
	}

	type validFormat struct {
		index int
		size  int64
		ext   string
	}
	var candidates []validFormat
	var duration int64 = int64(item.Duration)
	var pqSize int64 = 0

	for i, fmtItem := range rateFormats {
		sizeStr := firstNonZeroString(fmtItem.AndroidSize, fmtItem.ASize, fmtItem.Size, fmtItem.ISize)
		sizeVal, _ := strconv.ParseInt(sizeStr, 10, 64)

		ext := firstNonEmpty(fmtItem.AndroidFileType, fmtItem.FileType)
		if ext == "" {
			ext = miguFormatExt(fmtItem.FormatType, firstNonEmpty(fmtItem.AFormat, fmtItem.IFormat))
		}

		if fmtItem.FormatType == "PQ" {
			pqSize = sizeVal
		}

		if duration == 0 && sizeVal > 0 {
			var bitrate int64 = 0
			switch fmtItem.FormatType {
			case "PQ":
				bitrate = 128000
			case "HQ":
				bitrate = 320000
			case "LQ":
				bitrate = 64000
			}
			if bitrate > 0 {
				duration = (sizeVal * 8) / bitrate
			}
		}

		priceVal, _ := strconv.Atoi(fmtItem.Price)
		tags := fmtItem.ShowTag
		if len(tags) == 0 {
			tags = fmtItem.ShowTags
		}
		isVipTag := false
		for _, tag := range tags {
			if tag == "vip" {
				isVipTag = true
				break
			}
		}
		isHiddenPaid := (item.ChargeAuditions == "1" && priceVal >= 200)

		if allowPaid || (!isVipTag && !isHiddenPaid) {
			candidates = append(candidates, validFormat{index: i, size: sizeVal, ext: ext})
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].size > candidates[j].size })
	bestInfo := candidates[0]
	bestFormat := rateFormats[bestInfo.index]

	displaySize := bestInfo.size
	if pqSize > 0 {
		displaySize = pqSize
	}

	bitrate := 0
	if duration > 0 && bestInfo.size > 0 {
		bitrate = int(bestInfo.size * 8 / 1000 / duration)
	}

	coverURL := normalizeMiguImageURL(firstNonEmpty(pickMiguImage(item.ImgItems), pickMiguImage(item.AlbumImgs), item.Img1, item.Img2, item.Img3))

	linkID := firstNonEmpty(item.ContentID, item.CopyrightID)
	extra := map[string]string{
		"content_id":    item.ContentID,
		"resource_type": bestFormat.ResourceType,
		"format_type":   bestFormat.FormatType,
	}
	if item.CopyrightID != "" {
		extra["copyright_id"] = item.CopyrightID
	}

	return &model.Song{
		Source:   "migu",
		ID:       fmt.Sprintf("%s|%s|%s", item.ContentID, bestFormat.ResourceType, bestFormat.FormatType),
		Name:     songName,
		Artist:   strings.Join(artistNames, " / "),
		Album:    albumName,
		Size:     displaySize,
		Duration: int(duration),
		Bitrate:  bitrate,
		Cover:    coverURL,
		Ext:      bestInfo.ext,
		Link:     fmt.Sprintf("https://music.migu.cn/v3/music/song/%s", linkID),
		Extra:    extra,
	}
}

// GetLyrics 获取歌词
func collectMiguArtistNames(item MiguSongItem) []string {
	names := make([]string, 0, len(item.Singers)+len(item.Artists)+1)
	seen := make(map[string]struct{})

	appendName := func(name string) {
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

	for _, singer := range item.Singers {
		appendName(singer.Name)
	}
	for _, singer := range item.SingerList {
		appendName(singer.Name)
	}
	for _, artist := range item.Artists {
		appendName(artist.Name)
	}
	if len(names) == 0 {
		for _, name := range strings.Split(item.Singer, "|") {
			appendName(name)
		}
	}

	return names
}

func pickMiguImage(items []miguImageItem) string {
	for _, preferred := range []string{"02", "01", "03"} {
		for _, item := range items {
			if item.ImgSizeType == preferred && strings.TrimSpace(item.Img) != "" {
				return item.Img
			}
		}
	}
	for _, item := range items {
		if strings.TrimSpace(item.Img) != "" {
			return item.Img
		}
	}
	return ""
}

func firstNonZeroString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "0" {
			return value
		}
	}
	return ""
}

func normalizeMiguImageURL(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}
	if strings.HasPrefix(image, "http://") || strings.HasPrefix(image, "https://") {
		return image
	}
	if strings.HasPrefix(image, "//") {
		return "https:" + image
	}
	if strings.HasPrefix(image, "/") {
		return "https://d.musicapp.migu.cn" + image
	}
	return image
}

func miguFormatExt(formatType, formatCode string) string {
	formatType = strings.ToUpper(strings.TrimSpace(formatType))
	formatCode = strings.TrimSpace(formatCode)
	if strings.Contains(formatType, "SQ") || strings.HasPrefix(formatCode, "011") {
		return "flac"
	}
	return "mp3"
}

func miguAlbumLink(id string) string {
	return fmt.Sprintf("https://music.migu.cn/v3/music/album/%s", id)
}

func miguPlaylistLink(id string) string {
	return fmt.Sprintf("https://music.migu.cn/v5/#/playlist?playlistId=%s&playlistType=ordinary", id)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
