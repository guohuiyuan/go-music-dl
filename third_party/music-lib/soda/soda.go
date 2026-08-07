package soda

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	// PC端 UserAgent
	UserAgent        = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
	pcAppUserAgent   = "LunaPC/3.3.0(359450208)"
	vipProbeTrackID  = "7304719759323564095"
	vipProbeTrackURL = "https://qishui.douyin.com/s/iQeFw9cE/"
)

type Soda struct {
	cookie     string
	isVipCache *bool
}

type sodaArtist struct {
	Name string `json:"name"`
}

type sodaImage struct {
	Urls []string `json:"urls"`
	Uri  string   `json:"uri"`
}

type sodaBitRate struct {
	BR      int    `json:"br"`
	Quality string `json:"quality"`
	Size    int64  `json:"size"`
}

type sodaTrackPlayInfo struct {
	MainPlayURL   string `json:"main_play_url"`
	BackupPlayURL string `json:"backup_play_url"`
	PlayAuth      string `json:"play_auth"`
	Size          int64  `json:"size"`
	Format        string `json:"format"`
	Bitrate       int    `json:"bitrate"`
	Quality       string `json:"quality"`
	Duration      int    `json:"duration"`
}

type sodaTrackAudioInfo struct {
	PlayInfoList []sodaTrackPlayInfo `json:"play_info_list"`
}

type sodaVideoModelEntry struct {
	MainPlayURL   string
	BackupPlayURL string
	PlayAuth      string
	Size          int64
	Format        string
	Bitrate       int
	Quality       string
	Duration      float64
}

type sodaAlbum struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	URLCover sodaImage `json:"url_cover"`
}

type sodaPreview struct {
	VID      string        `json:"vid"`
	Start    int           `json:"start"`
	Duration int           `json:"duration"`
	BitRates []sodaBitRate `json:"bit_rates"`
}

type sodaQualityBenefit struct {
	Condition    string `json:"condition"`
	NeedVIP      bool   `json:"need_vip"`
	NeedPurchase bool   `json:"need_purchase"`
}

type sodaQualityPolicy struct {
	PlayDetail     *sodaQualityBenefit `json:"play_detail"`
	DownloadDetail *sodaQualityBenefit `json:"download_detail"`
}

type sodaLabelInfo struct {
	OnlyVIPDownload           bool                         `json:"only_vip_download"`
	OnlyVIPPlayable           bool                         `json:"only_vip_playable"`
	QualityOnlyVIPCanDownload []string                     `json:"quality_only_vip_can_download"`
	QualityOnlyVIPCanPlay     []string                     `json:"quality_only_vip_can_play"`
	QualityMap                map[string]sodaQualityPolicy `json:"quality_map"`
}

type sodaTrack struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Duration  int                `json:"duration"`
	VID       string             `json:"vid"`
	Artists   []sodaArtist       `json:"artists"`
	Album     sodaAlbum          `json:"album"`
	BitRates  []sodaBitRate      `json:"bit_rates"`
	Preview   sodaPreview        `json:"preview"`
	LabelInfo sodaLabelInfo      `json:"label_info"`
	AudioInfo sodaTrackAudioInfo `json:"audio_info"`
}

type sodaAPIStatusInfo struct {
	StatusMsg string `json:"status_msg"`
}

type sodaUserPlaylistOwner struct {
	ID         string `json:"id"`
	Nickname   string `json:"nickname"`
	PublicName string `json:"public_name"`
}

type sodaUserPlaylistItem struct {
	ID           string                `json:"id"`
	Title        string                `json:"title"`
	PublicTitle  string                `json:"public_title"`
	Desc         string                `json:"desc"`
	URLCover     sodaImage             `json:"url_cover"`
	CountTracks  int                   `json:"count_tracks"`
	PlayCount    int                   `json:"play_count"`
	Owner        sodaUserPlaylistOwner `json:"owner"`
	ReviewStatus string                `json:"review_status"`
	Type         int                   `json:"type"`
	ResourceCnt  struct {
		TrackCnt int `json:"track_cnt"`
	} `json:"resource_cnt"`
	Stats struct {
		CountPlayed    int `json:"count_played"`
		CountCollected int `json:"count_collected"`
	} `json:"stats"`
}

type sodaPlaylistDetailResponse struct {
	StatusCode int               `json:"status_code"`
	StatusInfo sodaAPIStatusInfo `json:"status_info"`
	NextCursor string            `json:"next_cursor"`
	HasMore    bool              `json:"has_more"`
	Playlist   sodaUserPlaylistItem

	MediaResources []struct {
		Type   string `json:"type"`
		Entity struct {
			TrackWrapper struct {
				Track sodaTrack `json:"track"`
			} `json:"track_wrapper"`
		} `json:"entity"`
	} `json:"media_resources"`
}

type sodaTrackV2Response struct {
	StatusCode  int               `json:"status_code"`
	StatusInfo  sodaAPIStatusInfo `json:"status_info"`
	Track       sodaTrack         `json:"track"`
	TrackInfo   sodaTrack         `json:"track_info"`
	TrackPlayer sodaTrackPlayer   `json:"track_player"`
	Lyric       struct {
		Content string `json:"content"`
	} `json:"lyric"`
}

type sodaTrackPlayer struct {
	MediaID       string          `json:"media_id"`
	URLPlayerInfo string          `json:"url_player_info"`
	VideoModel    json.RawMessage `json:"video_model"`
}

type sodaShareAlbumPage struct {
	LoaderData struct {
		AlbumPage struct {
			AlbumInfo struct {
				ID          string       `json:"id"`
				Name        string       `json:"name"`
				Artists     []sodaArtist `json:"artists"`
				Company     string       `json:"company"`
				CountTracks int          `json:"count_tracks"`
				URLCover    sodaImage    `json:"url_cover"`
				ReleaseDate int64        `json:"release_date"`
				PCLines     []string     `json:"pclines"`
			} `json:"albumInfo"`
			TrackList []sodaTrack `json:"trackList"`
		} `json:"album_page"`
	} `json:"loaderData"`
}

func New(cookie string) *Soda { return &Soda{cookie: cookie} }

var defaultSoda = New("")

func (l sodaLabelInfo) IsVIP() bool {
	if l.OnlyVIPDownload || l.OnlyVIPPlayable {
		return true
	}
	if len(l.QualityOnlyVIPCanDownload) > 0 || len(l.QualityOnlyVIPCanPlay) > 0 {
		return true
	}
	for _, policy := range l.QualityMap {
		if policy.PlayDetail != nil && policy.PlayDetail.NeedVIP {
			return true
		}
		if policy.DownloadDetail != nil && policy.DownloadDetail.NeedVIP {
			return true
		}
	}
	return false
}

func sodaTrackExtra(trackID string, label sodaLabelInfo, values map[string]string) map[string]string {
	extra := map[string]string{
		"track_id": trackID,
		"is_vip":   strconv.FormatBool(label.IsVIP()),
	}
	if label.OnlyVIPDownload {
		extra["only_vip_download"] = "true"
	}
	if label.OnlyVIPPlayable {
		extra["only_vip_playable"] = "true"
	}
	if len(label.QualityOnlyVIPCanDownload) > 0 {
		extra["vip_download_qualities"] = strings.Join(label.QualityOnlyVIPCanDownload, ",")
	}
	if len(label.QualityOnlyVIPCanPlay) > 0 {
		extra["vip_play_qualities"] = strings.Join(label.QualityOnlyVIPCanPlay, ",")
	}
	for key, value := range values {
		if strings.TrimSpace(value) != "" {
			extra[key] = value
		}
	}
	return extra
}

func (r *sodaTrackV2Response) primaryTrack() sodaTrack {
	if r == nil {
		return sodaTrack{}
	}
	if r.Track.ID != "" {
		return r.Track
	}
	return r.TrackInfo
}

func parseSodaTrackV2Response(body []byte) (*sodaTrackV2Response, error) {
	var resp sodaTrackV2Response
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda track_v2 json parse error: %w", err)
	}
	if resp.StatusCode != 0 {
		msg := strings.TrimSpace(resp.StatusInfo.StatusMsg)
		if msg == "" {
			msg = "unknown error"
		}
		return nil, fmt.Errorf("soda track_v2 api error: status_code=%d status_msg=%s", resp.StatusCode, msg)
	}
	return &resp, nil
}

// fetchPlaylistDetail [内部通用] 获取歌单详情
func (s *Soda) fetchAlbumDetail(id string) (*model.Playlist, []model.Song, error) {
	body, err := utils.Get(sodaAlbumLink(id),
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil {
		return nil, nil, err
	}

	pageData, err := parseSodaShareAlbumPage(body)
	if err != nil {
		return nil, nil, err
	}

	info := pageData.LoaderData.AlbumPage.AlbumInfo
	if info.ID == "" {
		return nil, nil, errors.New("album not found")
	}

	description := strings.TrimSpace(strings.Join(info.PCLines, " "))
	if description == "" {
		description = strings.TrimSpace(info.Company)
	}

	album := &model.Playlist{
		Source:      "soda",
		ID:          info.ID,
		Name:        info.Name,
		Cover:       sodaBuildImageURL(info.URLCover, "~c5_300x300.jpg"),
		TrackCount:  info.CountTracks,
		Creator:     sodaJoinArtists(info.Artists),
		Description: description,
		Link:        sodaAlbumLink(info.ID),
		Extra: map[string]string{
			"album_id": info.ID,
		},
	}
	if info.ReleaseDate > 0 {
		album.Extra["release_date"] = strconv.FormatInt(info.ReleaseDate, 10)
	}
	if album.TrackCount == 0 {
		album.TrackCount = len(pageData.LoaderData.AlbumPage.TrackList)
	}

	songs := make([]model.Song, 0, len(pageData.LoaderData.AlbumPage.TrackList))
	for _, track := range pageData.LoaderData.AlbumPage.TrackList {
		if track.ID == "" {
			continue
		}

		displaySize := sodaMaxBitRateSize(track.BitRates)
		if previewSize := sodaMaxBitRateSize(track.Preview.BitRates); previewSize > displaySize {
			displaySize = previewSize
		}

		artist := sodaJoinArtists(track.Artists)
		if artist == "" {
			artist = album.Creator
		}

		cover := sodaBuildImageURL(track.Album.URLCover, "~c5_375x375.jpg")
		if cover == "" {
			cover = sodaBuildImageURL(info.URLCover, "~c5_375x375.jpg")
		}

		albumID := track.Album.ID
		if albumID == "" {
			albumID = info.ID
		}
		albumName := strings.TrimSpace(track.Album.Name)
		if albumName == "" {
			albumName = info.Name
		}

		duration := track.Duration / 1000
		bitrate := 0
		if duration > 0 && displaySize > 0 {
			bitrate = int(displaySize * 8 / 1000 / int64(duration))
		}

		songs = append(songs, model.Song{
			Source:   "soda",
			ID:       track.ID,
			Name:     track.Name,
			Artist:   artist,
			Album:    albumName,
			AlbumID:  albumID,
			Duration: duration,
			Size:     displaySize,
			Bitrate:  bitrate,
			Cover:    cover,
			Link:     fmt.Sprintf("https://www.qishui.com/track/%s", track.ID),
			Extra:    sodaTrackExtra(track.ID, track.LabelInfo, map[string]string{"album_id": albumID}),
			IsVIP:    track.LabelInfo.IsVIP(),
		})
	}

	if len(songs) == 0 {
		return nil, nil, errors.New("album has no songs")
	}

	return album, songs, nil
}

func (s *Soda) fetchPlaylistDetail(id string) (*model.Playlist, []model.Song, error) {
	return s.fetchPlaylistDetailPaged(id)
}

func (s *Soda) fetchPlaylistDetailWeb(id string) (*model.Playlist, []model.Song, error) {
	params := url.Values{}
	params.Set("playlist_id", id)
	params.Set("cursor", "0")
	params.Set("cnt", "20")
	params.Set("aid", "386088")
	params.Set("device_platform", "web")
	params.Set("channel", "pc_web")

	apiURL := "https://api.qishui.com/luna/pc/playlist/detail?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil {
		return nil, nil, err
	}

	var resp struct {
		Playlist struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Desc  string `json:"desc"`
			Owner struct {
				Nickname string `json:"nickname"`
			} `json:"owner"`
			CountTracks int `json:"count_tracks"`
			UrlCover    struct {
				Urls []string `json:"urls"`
				Uri  string   `json:"uri"`
			} `json:"url_cover"`
		} `json:"playlist"`

		MediaResources []struct {
			Type   string `json:"type"`
			Entity struct {
				TrackWrapper struct {
					Track struct {
						ID       string `json:"id"`
						Name     string `json:"name"`
						Duration int    `json:"duration"`
						Artists  []struct {
							Name string `json:"name"`
						} `json:"artists"`
						Album struct {
							Name     string `json:"name"`
							UrlCover struct {
								Urls []string `json:"urls"`
								Uri  string   `json:"uri"`
							} `json:"url_cover"`
						} `json:"album"`
						BitRates []struct {
							Size    int64  `json:"size"`
							Quality string `json:"quality"`
						} `json:"bit_rates"`
						AudioInfo struct {
							PlayInfoList []struct {
								MainPlayUrl string `json:"main_play_url"`
								PlayAuth    string `json:"play_auth"`
								Size        int64  `json:"size"`
								Format      string `json:"format"`
								Bitrate     int    `json:"bitrate"`
							} `json:"play_info_list"`
						} `json:"audio_info"`
					} `json:"track"`
				} `json:"track_wrapper"`
			} `json:"entity"`
		} `json:"media_resources"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("soda playlist detail json error: %w", err)
	}

	pl := &model.Playlist{
		Source:      "soda",
		ID:          id,
		Name:        resp.Playlist.Title,
		Creator:     resp.Playlist.Owner.Nickname,
		Description: resp.Playlist.Desc,
		TrackCount:  resp.Playlist.CountTracks,
		Link:        fmt.Sprintf("https://www.qishui.com/playlist/%s", id),
	}
	if len(resp.Playlist.UrlCover.Urls) > 0 {
		cover := resp.Playlist.UrlCover.Urls[0]
		if resp.Playlist.UrlCover.Uri != "" && !strings.Contains(cover, resp.Playlist.UrlCover.Uri) {
			cover += resp.Playlist.UrlCover.Uri
		}
		if !strings.Contains(cover, "~") {
			cover += "~c5_300x300.jpg"
		}
		pl.Cover = cover
	}

	var songs []model.Song
	for _, item := range resp.MediaResources {
		if item.Type != "track" {
			continue
		}
		track := item.Entity.TrackWrapper.Track
		if track.ID == "" {
			continue
		}

		var displaySize int64
		for _, br := range track.BitRates {
			if br.Size > displaySize {
				displaySize = br.Size
			}
		}
		for _, pi := range track.AudioInfo.PlayInfoList {
			if pi.Size > displaySize {
				displaySize = pi.Size
			}
		}

		var artistNames []string
		for _, ar := range track.Artists {
			artistNames = append(artistNames, ar.Name)
		}

		var cover string
		if len(track.Album.UrlCover.Urls) > 0 {
			domain := track.Album.UrlCover.Urls[0]
			uri := track.Album.UrlCover.Uri
			if domain != "" && uri != "" && !strings.Contains(domain, uri) {
				cover = domain + uri + "~c5_375x375.jpg"
			} else if domain != "" {
				cover = domain + "~c5_375x375.jpg"
			}
		}

		bitrate := 0
		seconds := track.Duration / 1000
		if seconds > 0 && displaySize > 0 {
			bitrate = int(displaySize * 8 / 1000 / int64(seconds))
		}

		song := model.Song{
			Source:   "soda",
			ID:       track.ID,
			Name:     track.Name,
			Artist:   strings.Join(artistNames, "、"),
			Album:    track.Album.Name,
			Duration: track.Duration / 1000,
			Size:     displaySize,
			Bitrate:  bitrate,
			Cover:    cover,
			Link:     fmt.Sprintf("https://www.qishui.com/track/%s", track.ID),
			Extra: map[string]string{
				"track_id": track.ID,
			},
		}

		if len(track.AudioInfo.PlayInfoList) > 0 {
			best := track.AudioInfo.PlayInfoList[0]
			for _, info := range track.AudioInfo.PlayInfoList {
				if info.Size > best.Size {
					best = info
				}
			}
			if best.MainPlayUrl != "" {
				song.URL = best.MainPlayUrl + "#auth=" + url.QueryEscape(best.PlayAuth)
				if song.Size == 0 {
					song.Size = best.Size
				}
				song.Ext = best.Format
				song.Bitrate = normalizeSodaBitrate(best.Bitrate)
			}
		}

		songs = append(songs, song)
	}
	return pl, songs, nil
}

// GetDownloadInfo 获取下载信息
func (s *Soda) fetchPlaylistDetailPaged(id string) (*model.Playlist, []model.Song, error) {
	playlistID := strings.TrimSpace(id)
	if playlistID == "" {
		return nil, nil, errors.New("playlist id is empty")
	}

	const pageSize = 100
	cursor := ""
	seenCursors := make(map[string]bool)
	seenTracks := make(map[string]bool)
	var playlist *model.Playlist
	songs := make([]model.Song, 0)

	for page := 0; page < 20; page++ {
		resp, err := s.fetchPlaylistDetailPage(playlistID, cursor, pageSize)
		if err != nil {
			if page == 0 {
				return s.fetchPlaylistDetailWeb(playlistID)
			}
			return nil, nil, err
		}
		if playlist == nil {
			pl := sodaBuildPlaylistFromUserItem(resp.Playlist, "", "")
			if pl.ID == "" {
				pl.ID = playlistID
				pl.Source = "soda"
				pl.Link = fmt.Sprintf("https://www.qishui.com/playlist/%s", playlistID)
			}
			playlist = &pl
		}

		for _, item := range resp.MediaResources {
			if item.Type != "track" {
				continue
			}
			track := item.Entity.TrackWrapper.Track
			if track.ID == "" || seenTracks[track.ID] {
				continue
			}
			seenTracks[track.ID] = true
			song := sodaBuildSongFromTrack(track)
			if song.Cover == "" && playlist != nil {
				song.Cover = playlist.Cover
			}
			songs = append(songs, song)
		}

		nextCursor := strings.TrimSpace(resp.NextCursor)
		if nextCursor == "" || nextCursor == cursor || seenCursors[nextCursor] {
			break
		}
		if !resp.HasMore && len(resp.MediaResources) < pageSize {
			break
		}
		seenCursors[nextCursor] = true
		cursor = nextCursor
	}

	if playlist == nil || playlist.ID == "" {
		return nil, nil, errors.New("playlist not found")
	}
	if playlist.TrackCount == 0 {
		playlist.TrackCount = len(songs)
	}
	return playlist, songs, nil
}

func (s *Soda) fetchPlaylistDetailPage(playlistID, cursor string, count int) (*sodaPlaylistDetailResponse, error) {
	body, err := utils.Get(sodaPCPlaylistDetailURL(playlistID, cursor, count), s.pcRequestOptions()...)
	if err != nil {
		return nil, err
	}

	var resp sodaPlaylistDetailResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda playlist detail json error: %w", err)
	}
	if resp.StatusCode != 0 {
		msg := strings.TrimSpace(resp.StatusInfo.StatusMsg)
		if msg == "" {
			msg = "unknown error"
		}
		return nil, fmt.Errorf("soda playlist detail api error: status_code=%d status_msg=%s", resp.StatusCode, msg)
	}
	return &resp, nil
}

func parseSodaShareAlbumPage(body []byte) (*sodaShareAlbumPage, error) {
	routerJSON, err := extractSodaJSONBlock(string(body), "_ROUTER_DATA = ")
	if err != nil {
		return nil, err
	}

	var page sodaShareAlbumPage
	if err := json.Unmarshal([]byte(routerJSON), &page); err != nil {
		return nil, fmt.Errorf("soda album page json error: %w", err)
	}

	return &page, nil
}

func extractSodaJSONBlock(page string, marker string) (string, error) {
	start := strings.Index(page, marker)
	if start < 0 {
		return "", errors.New("soda router data not found")
	}
	start += len(marker)

	depth := 0
	inString := false
	escaped := false
	started := false

	for i := start; i < len(page); i++ {
		ch := page[i]

		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
			started = true
		case '}':
			depth--
			if started && depth == 0 {
				return page[start : i+1], nil
			}
		}
	}

	return "", errors.New("soda router data is incomplete")
}

func sodaJoinArtists(artists []sodaArtist) string {
	names := make([]string, 0, len(artists))
	for _, artist := range artists {
		name := strings.TrimSpace(artist.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, " / ")
}

func sodaBuildImageURL(img sodaImage, suffix string) string {
	if len(img.Urls) == 0 {
		return ""
	}

	cover := strings.TrimSpace(img.Urls[0])
	uri := strings.TrimSpace(img.Uri)
	if uri != "" && !strings.Contains(cover, uri) {
		cover += uri
	}
	if cover == "" {
		return ""
	}
	if suffix != "" && !strings.Contains(cover, "~") {
		cover += suffix
	}
	return cover
}

func sodaMaxBitRateSize(bitRates []sodaBitRate) int64 {
	var size int64
	for _, br := range bitRates {
		if br.Size > size {
			size = br.Size
		}
	}
	return size
}

func normalizeSodaBitrate(bitrate int) int {
	if bitrate > 1000 {
		return bitrate / 1000
	}
	return bitrate
}

func sodaQualityRank(quality, format string, bitrate int) int {
	q := strings.ToLower(strings.TrimSpace(quality))
	q = strings.NewReplacer("-", "", "_", "", " ", "").Replace(q)
	f := strings.ToLower(strings.TrimSpace(format))
	br := normalizeSodaBitrate(bitrate)
	isLosslessFormat := strings.Contains(f, "flac") || strings.Contains(f, "alac") || strings.Contains(f, "wav")
	isLosslessLabel := strings.Contains(q, "lossless") || strings.Contains(q, "flac") ||
		strings.Contains(q, "sq") || strings.Contains(q, "svip")
	isHiResLabel := strings.Contains(q, "hires") || strings.Contains(q, "master")

	switch {
	case isHiResLabel && (isLosslessFormat || br >= 900):
		return 110
	case isLosslessLabel || isLosslessFormat || br >= 900:
		return 100
	case isHiResLabel:
		return 90
	case strings.Contains(q, "atmos") || strings.Contains(q, "dolby") ||
		strings.Contains(q, "spatial"):
		return 88
	case strings.Contains(q, "highest") || strings.Contains(q, "excellent") ||
		strings.Contains(q, "superhigh") || strings.Contains(q, "hq"):
		return 80
	case strings.Contains(q, "higher") || q == "high" || strings.Contains(q, "320"):
		return 70
	case strings.Contains(q, "standard") || strings.Contains(q, "medium") ||
		strings.Contains(q, "normal") || strings.Contains(q, "128"):
		return 50
	case strings.Contains(q, "low") || strings.Contains(q, "preview"):
		return 10
	}

	switch {
	case br >= 900:
		return 100
	case br >= 320:
		return 70
	case br >= 256:
		return 65
	case br >= 192:
		return 55
	case br >= 128:
		return 50
	case br > 0:
		return 20
	default:
		return 0
	}
}

func sodaBetterStreamCandidate(
	aDuration float64, aQuality, aFormat string, aBitrate int, aSize int64,
	bDuration float64, bQuality, bFormat string, bBitrate int, bSize int64,
) bool {
	if aDuration > 0 || bDuration > 0 {
		if aDuration > bDuration+1 {
			return true
		}
		if bDuration > aDuration+1 {
			return false
		}
	}

	aRank := sodaQualityRank(aQuality, aFormat, aBitrate)
	bRank := sodaQualityRank(bQuality, bFormat, bBitrate)
	if aRank != bRank {
		return aRank > bRank
	}

	aBR := normalizeSodaBitrate(aBitrate)
	bBR := normalizeSodaBitrate(bBitrate)
	if aBR != bBR {
		return aBR > bBR
	}
	if aSize != bSize {
		return aSize > bSize
	}
	return strings.TrimSpace(aQuality) > strings.TrimSpace(bQuality)
}

func sodaBestTrackPlayInfo(list []sodaTrackPlayInfo) (sodaTrackPlayInfo, bool) {
	var best sodaTrackPlayInfo
	ok := false
	for _, info := range list {
		if strings.TrimSpace(info.MainPlayURL) == "" && strings.TrimSpace(info.BackupPlayURL) == "" {
			continue
		}
		if !ok || sodaBetterStreamCandidate(
			float64(info.Duration), info.Quality, info.Format, info.Bitrate, info.Size,
			float64(best.Duration), best.Quality, best.Format, best.Bitrate, best.Size,
		) {
			best = info
			ok = true
		}
	}
	return best, ok
}

func sodaTrackDurationSeconds(track sodaTrack) int {
	if track.Duration > 1000 {
		return track.Duration / 1000
	}
	return track.Duration
}

func sodaBuildSongFromTrack(track sodaTrack) model.Song {
	displaySize := sodaMaxBitRateSize(track.BitRates)
	if previewSize := sodaMaxBitRateSize(track.Preview.BitRates); previewSize > displaySize {
		displaySize = previewSize
	}

	duration := sodaTrackDurationSeconds(track)
	bitrate := 0
	if duration > 0 && displaySize > 0 {
		bitrate = int(displaySize * 8 / 1000 / int64(duration))
	}

	albumID := strings.TrimSpace(track.Album.ID)
	artist := sodaJoinArtists(track.Artists)

	song := model.Song{
		Source:   "soda",
		ID:       track.ID,
		Name:     track.Name,
		Artist:   artist,
		Album:    track.Album.Name,
		AlbumID:  albumID,
		Duration: duration,
		Size:     displaySize,
		Bitrate:  bitrate,
		Cover:    sodaBuildImageURL(track.Album.URLCover, "~c5_375x375.jpg"),
		Link:     fmt.Sprintf("https://www.qishui.com/track/%s", track.ID),
		Extra:    sodaTrackExtra(track.ID, track.LabelInfo, map[string]string{"album_id": albumID}),
		IsVIP:    track.LabelInfo.IsVIP(),
	}

	if best, ok := sodaBestTrackPlayInfo(track.AudioInfo.PlayInfoList); ok {
		downloadURL := strings.TrimSpace(best.MainPlayURL)
		if downloadURL == "" {
			downloadURL = strings.TrimSpace(best.BackupPlayURL)
		}
		if downloadURL != "" {
			song.URL = downloadURL
			if strings.TrimSpace(best.PlayAuth) != "" {
				song.URL += "#auth=" + url.QueryEscape(best.PlayAuth)
			}
			if best.Size > song.Size {
				song.Size = best.Size
			}
			if best.Format != "" {
				song.Ext = best.Format
			}
			if best.Bitrate > 0 {
				song.Bitrate = normalizeSodaBitrate(best.Bitrate)
			}
			if strings.TrimSpace(best.Quality) != "" {
				song.Extra["quality"] = strings.TrimSpace(best.Quality)
			}
		}
	}

	return song
}

func sodaDownloadInfoURL(info *DownloadInfo) string {
	if info == nil {
		return ""
	}
	if strings.TrimSpace(info.PlayAuth) == "" {
		return info.URL
	}
	return info.URL + "#auth=" + url.QueryEscape(info.PlayAuth)
}

func applySodaDownloadInfo(song *model.Song, info *DownloadInfo) {
	if song == nil || info == nil {
		return
	}
	if downloadURL := sodaDownloadInfoURL(info); downloadURL != "" {
		song.URL = downloadURL
	}
	if info.Size > 0 {
		song.Size = info.Size
	}
	if info.Format != "" {
		song.Ext = info.Format
	}
	if info.Bitrate > 0 {
		song.Bitrate = normalizeSodaBitrate(info.Bitrate)
	} else if song.Duration > 0 && info.Size > 0 {
		song.Bitrate = int(info.Size * 8 / 1000 / int64(song.Duration))
	}
	if info.Duration > 0 && song.Duration == 0 {
		song.Duration = int(info.Duration + 0.5)
	}
	if strings.TrimSpace(info.Quality) != "" {
		if song.Extra == nil {
			song.Extra = map[string]string{}
		}
		song.Extra["quality"] = strings.TrimSpace(info.Quality)
		song.Extra["download_quality"] = strings.TrimSpace(info.Quality)
	}
}

func sodaDownloadInfoIsPreview(info *DownloadInfo, fullDurationSeconds int) bool {
	if info == nil || info.Duration <= 0 || fullDurationSeconds <= 0 {
		return false
	}
	return info.Duration+5 < float64(fullDurationSeconds)
}

func sodaAlbumLink(id string) string {
	return fmt.Sprintf("https://www.qishui.com/share/album?album_id=%s", strings.TrimSpace(id))
}

func sodaExtractAlbumID(link string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`album_id=(\d+)`),
		regexp.MustCompile(`(?:^|[?&])id=(\d+)`),
		regexp.MustCompile(`album/(\d+)`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(link)
		if len(matches) >= 2 {
			return matches[1]
		}
	}

	link = strings.TrimSpace(link)
	if len(link) > 10 && !strings.Contains(link, "/") {
		return link
	}
	return ""
}

func sodaExtractPlaylistIDFromText(text string) string {
	text = strings.TrimSpace(text)
	if text != "" && isSodaDigits(text) && !strings.Contains(text, "/") {
		return text
	}

	candidates := []string{text}
	if decoded, err := url.QueryUnescape(text); err == nil && decoded != text {
		candidates = append(candidates, decoded)
	}

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:^|[?&])playlist_id=(\d+)`),
		regexp.MustCompile(`(?:^|[?&])playlistId=(\d+)`),
		regexp.MustCompile(`"playlist_id"\s*:\s*"?(\d+)"?`),
		regexp.MustCompile(`"playlistId"\s*:\s*"?(\d+)"?`),
		regexp.MustCompile(`(?i)(?:/|%2f)playlist(?:/|%2f)(\d+)`),
		regexp.MustCompile(`playlist/(\d+)`),
	}
	for _, candidate := range candidates {
		for _, pattern := range patterns {
			matches := pattern.FindStringSubmatch(candidate)
			if len(matches) >= 2 {
				return matches[1]
			}
		}
	}
	return ""
}

func (s *Soda) extractPlaylistID(link string) (string, error) {
	if playlistID := sodaExtractPlaylistIDFromText(link); playlistID != "" {
		return playlistID, nil
	}

	finalURL, body, err := s.fetchSharePage(link)
	if err != nil {
		return "", err
	}
	if playlistID := sodaExtractPlaylistIDFromText(finalURL); playlistID != "" {
		return playlistID, nil
	}
	if playlistID := sodaExtractPlaylistIDFromText(string(body)); playlistID != "" {
		return playlistID, nil
	}
	return "", errors.New("soda playlist id not found")
}

func (s *Soda) extractTrackID(link string) (string, error) {
	if trackID := sodaExtractTrackIDFromText(link); trackID != "" {
		return trackID, nil
	}

	finalURL, body, err := s.fetchSharePage(link)
	if err != nil {
		return "", err
	}
	if trackID := sodaExtractTrackIDFromText(finalURL); trackID != "" {
		return trackID, nil
	}
	if trackID := sodaExtractTrackIDFromText(string(body)); trackID != "" {
		return trackID, nil
	}
	return "", errors.New("soda track id not found")
}

func (s *Soda) fetchSharePage(link string) (string, []byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	if strings.TrimSpace(s.cookie) != "" {
		req.Header.Set("Cookie", s.cookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("http request failed: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}
	finalURL := ""
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return finalURL, body, nil
}

func sodaExtractTrackIDFromText(text string) string {
	text = strings.TrimSpace(text)
	if len(text) > 10 && isSodaDigits(text) && !strings.Contains(text, "/") {
		return text
	}

	candidates := []string{text}
	if decoded, err := url.QueryUnescape(text); err == nil && decoded != text {
		candidates = append(candidates, decoded)
	}

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:^|[?&])track_id=(\d{10,})`),
		regexp.MustCompile(`"track_id"\s*:\s*"(\d{10,})"`),
		regexp.MustCompile(`(?:/|%2F)(?:track|song)(?:/|%2F)(\d{10,})`),
		regexp.MustCompile(`(?:track|song)/(\d{10,})`),
	}
	for _, candidate := range candidates {
		for _, pattern := range patterns {
			matches := pattern.FindStringSubmatch(candidate)
			if len(matches) >= 2 {
				return matches[1]
			}
		}
	}
	return ""
}

func isSodaDigits(value string) bool {
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return value != ""
}

type DownloadInfo struct {
	URL      string
	PlayAuth string
	Format   string
	Size     int64
	Duration float64
	Bitrate  int
	Quality  string
}

type sodaPlayerInfo struct {
	MainPlayURL   string  `json:"MainPlayUrl"`
	BackupPlayURL string  `json:"BackupPlayUrl"`
	PlayAuth      string  `json:"PlayAuth"`
	Size          int64   `json:"Size"`
	Bitrate       int     `json:"Bitrate"`
	Format        string  `json:"Format"`
	Duration      float64 `json:"Duration"`
	Quality       string  `json:"Quality"`
}

func sodaBestPlayerInfo(list []sodaPlayerInfo) (sodaPlayerInfo, bool) {
	var best sodaPlayerInfo
	ok := false
	for _, info := range list {
		if strings.TrimSpace(info.MainPlayURL) == "" && strings.TrimSpace(info.BackupPlayURL) == "" {
			continue
		}
		if !ok || sodaBetterStreamCandidate(
			info.Duration, info.Quality, info.Format, info.Bitrate, info.Size,
			best.Duration, best.Quality, best.Format, best.Bitrate, best.Size,
		) {
			best = info
			ok = true
		}
	}
	return best, ok
}

func sodaBestFromVideoModel(raw json.RawMessage) (*DownloadInfo, bool) {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return nil, false
	}
	for i := 0; i < 3 && strings.HasPrefix(text, "\""); i++ {
		var nested string
		if err := json.Unmarshal([]byte(text), &nested); err != nil {
			break
		}
		text = strings.TrimSpace(nested)
	}

	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return nil, false
	}

	entries := make([]sodaVideoModelEntry, 0)
	sodaCollectVideoModelEntries(value, "", "", 0, &entries)

	var best sodaVideoModelEntry
	ok := false
	for _, entry := range entries {
		if strings.TrimSpace(entry.MainPlayURL) == "" && strings.TrimSpace(entry.BackupPlayURL) == "" {
			continue
		}
		if !ok || sodaBetterStreamCandidate(
			entry.Duration, entry.Quality, entry.Format, entry.Bitrate, entry.Size,
			best.Duration, best.Quality, best.Format, best.Bitrate, best.Size,
		) {
			best = entry
			ok = true
		}
	}
	if !ok {
		return nil, false
	}

	downloadURL := strings.TrimSpace(best.MainPlayURL)
	if downloadURL == "" {
		downloadURL = strings.TrimSpace(best.BackupPlayURL)
	}
	if downloadURL == "" {
		return nil, false
	}
	return &DownloadInfo{
		URL:      downloadURL,
		PlayAuth: strings.TrimSpace(best.PlayAuth),
		Format:   strings.TrimSpace(best.Format),
		Size:     best.Size,
		Duration: best.Duration,
		Bitrate:  best.Bitrate,
		Quality:  strings.TrimSpace(best.Quality),
	}, true
}

func sodaCollectVideoModelEntries(value any, keyHint string, inheritedAuth string, inheritedDuration float64, entries *[]sodaVideoModelEntry) {
	switch v := value.(type) {
	case map[string]any:
		auth := strings.TrimSpace(inheritedAuth)
		if ownAuth := sodaVideoModelPlayAuth(v); ownAuth != "" {
			auth = ownAuth
		}
		duration := inheritedDuration
		if ownDuration := sodaJSONFloat(v, "video_duration", "duration", "Duration"); ownDuration > 0 {
			duration = normalizeSodaDuration(ownDuration)
		}
		if entry, ok := sodaVideoModelEntryFromMap(v, keyHint, auth, duration); ok {
			*entries = append(*entries, entry)
		}
		for key, child := range v {
			sodaCollectVideoModelEntries(child, key, auth, duration, entries)
		}
	case []any:
		for _, child := range v {
			sodaCollectVideoModelEntries(child, keyHint, inheritedAuth, inheritedDuration, entries)
		}
	}
}

func sodaVideoModelEntryFromMap(values map[string]any, keyHint string, inheritedAuth string, inheritedDuration float64) (sodaVideoModelEntry, bool) {
	entry := sodaVideoModelEntry{
		MainPlayURL:   sodaJSONString(values, "main_play_url", "MainPlayUrl", "main_url", "MainUrl", "url", "URL", "play_url", "PlayURL"),
		BackupPlayURL: sodaJSONString(values, "backup_play_url", "BackupPlayUrl", "backup_url", "BackupUrl", "backup_url_1", "backup_url_2", "backup_url_3"),
		PlayAuth:      sodaJSONString(values, "play_auth", "PlayAuth"),
		Size:          sodaJSONInt64(values, "size", "Size", "file_size", "FileSize", "data_size", "DataSize"),
		Format:        sodaJSONString(values, "format", "Format", "vtype", "VType", "file_format", "FileFormat"),
		Bitrate:       sodaJSONInt(values, "bitrate", "Bitrate", "br", "BR", "bit_rate", "BitRate"),
		Quality:       sodaJSONString(values, "quality", "Quality", "definition", "Definition", "quality_type", "QualityType"),
		Duration:      sodaJSONFloat(values, "duration", "Duration"),
	}
	if meta, ok := values["video_meta"].(map[string]any); ok {
		if entry.Size == 0 {
			entry.Size = sodaJSONInt64(meta, "size", "Size", "file_size", "FileSize")
		}
		if entry.Format == "" {
			entry.Format = sodaJSONString(meta, "format", "Format", "vtype", "VType", "codec_type", "CodecType")
		}
		if entry.Bitrate == 0 {
			entry.Bitrate = sodaJSONInt(meta, "bitrate", "Bitrate", "real_bitrate", "RealBitrate", "bit_rate", "BitRate")
		}
		if entry.Quality == "" {
			entry.Quality = sodaJSONString(meta, "quality", "Quality", "definition", "Definition", "quality_type", "QualityType")
		}
		if entry.Duration == 0 {
			entry.Duration = sodaJSONFloat(meta, "duration", "Duration")
		}
	}
	if entry.BackupPlayURL == "" {
		entry.BackupPlayURL = sodaJSONFirstString(values, "backup_urls", "backupUrls", "url_list", "UrlList")
	}
	if entry.PlayAuth == "" {
		entry.PlayAuth = sodaVideoModelPlayAuth(values)
	}
	if entry.PlayAuth == "" {
		entry.PlayAuth = strings.TrimSpace(inheritedAuth)
	}
	if entry.Quality == "" {
		entry.Quality = sodaVideoModelQualityHint(sodaJSONString(values, "gear_des_key", "GearDesKey"))
	}
	if entry.Quality == "" {
		entry.Quality = sodaVideoModelQualityHint(keyHint)
	}
	if entry.Duration == 0 {
		entry.Duration = inheritedDuration
	}
	return entry, strings.TrimSpace(entry.MainPlayURL) != "" || strings.TrimSpace(entry.BackupPlayURL) != ""
}

func sodaVideoModelPlayAuth(values map[string]any) string {
	for _, key := range []string{"encrypt_info", "EncryptInfo", "encryptInfo"} {
		child, ok := values[key].(map[string]any)
		if !ok {
			continue
		}
		if auth := sodaJSONString(child, "spade_a", "SpadeA", "spadeA", "play_auth", "PlayAuth"); auth != "" {
			return auth
		}
	}
	return ""
}

func sodaVideoModelQualityHint(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	normalized := strings.ToLower(strings.NewReplacer("-", "", "_", "", " ", "").Replace(key))
	for _, token := range []string{"hires", "lossless", "sq", "flac", "highest", "higher", "standard", "normal"} {
		if strings.Contains(normalized, token) {
			return token
		}
	}
	return ""
}

func sodaJSONString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			if text := sodaAnyString(value); text != "" {
				return text
			}
		}
	}
	return ""
}

func sodaJSONFirstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok {
			continue
		}
		list, ok := raw.([]any)
		if !ok {
			continue
		}
		for _, item := range list {
			if text := sodaAnyString(item); text != "" {
				return text
			}
		}
	}
	return ""
}

func sodaAnyString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		for _, item := range v {
			if text := sodaAnyString(item); text != "" {
				return text
			}
		}
	}
	return ""
}

func sodaJSONInt(values map[string]any, keys ...string) int {
	return int(sodaJSONFloat(values, keys...) + 0.5)
}

func sodaJSONInt64(values map[string]any, keys ...string) int64 {
	return int64(sodaJSONFloat(values, keys...) + 0.5)
}

func normalizeSodaDuration(duration float64) float64 {
	if duration > 1000 {
		return duration / 1000
	}
	return duration
}

func sodaJSONFloat(values map[string]any, keys ...string) float64 {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case float64:
			return v
		case string:
			n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err == nil {
				return n
			}
		}
	}
	return 0
}

func sodaWebTrackV2URL(trackID string) string {
	params := url.Values{}
	params.Set("track_id", trackID)
	params.Set("media_type", "track")
	params.Set("aid", "386088")
	params.Set("device_platform", "web")
	params.Set("channel", "pc_web")
	return "https://api.qishui.com/luna/pc/track_v2?" + params.Encode()
}

func sodaPCAppParams() url.Values {
	now := time.Now().UnixMilli()
	deviceID := strconv.FormatInt(now, 10)
	iid := strconv.FormatInt(now+1, 10)

	params := url.Values{}
	params.Set("aid", "386088")
	params.Set("app_name", "luna_pc")
	params.Set("region", "cn")
	params.Set("geo_region", "cn")
	params.Set("os_region", "cn")
	params.Set("sim_region", "")
	params.Set("device_id", deviceID)
	params.Set("cdid", "")
	params.Set("iid", iid)
	params.Set("version_name", "3.3.0")
	params.Set("version_code", "30030000")
	params.Set("channel", "official")
	params.Set("build_mode", "master")
	params.Set("network_carrier", "")
	params.Set("ac", "wifi")
	params.Set("tz_name", "Asia/Shanghai")
	params.Set("resolution", "")
	params.Set("device_platform", "windows")
	params.Set("device_type", "Windows")
	params.Set("os_version", "Windows 11")
	params.Set("fp", deviceID)
	return params
}

func sodaPCTrackV2URL() string {
	params := sodaPCAppParams()
	return "https://api.qishui.com/luna/pc/track_v2?" + params.Encode()
}

func sodaPCMeURL() string {
	params := sodaPCAppParams()
	return "https://api.qishui.com/luna/pc/me?" + params.Encode()
}

func sodaPCUserPlaylistURL(userID, cursor string, count int) string {
	if count <= 0 {
		count = 50
	}
	params := sodaPCAppParams()
	params.Set("user_id", strings.TrimSpace(userID))
	params.Set("cursor", strings.TrimSpace(cursor))
	params.Set("count", strconv.Itoa(count))
	return "https://api.qishui.com/luna/pc/user/playlist?" + params.Encode()
}

func sodaPCPlaylistDetailURL(playlistID, cursor string, count int) string {
	if count <= 0 {
		count = 100
	}
	params := sodaPCAppParams()
	params.Set("playlist_id", strings.TrimSpace(playlistID))
	params.Set("cursor", strings.TrimSpace(cursor))
	params.Set("count", strconv.Itoa(count))
	return "https://api.qishui.com/luna/pc/playlist/detail?" + params.Encode()
}

func (s *Soda) fetchWebTrackV2(trackID string) (*sodaTrackV2Response, error) {
	body, err := utils.Get(sodaWebTrackV2URL(trackID),
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil {
		return nil, err
	}
	return parseSodaTrackV2Response(body)
}

func (s *Soda) pcRequestOptions(extra ...utils.RequestOption) []utils.RequestOption {
	opts := []utils.RequestOption{
		utils.WithHeader("User-Agent", pcAppUserAgent),
		utils.WithHeader("x-luna-background-type", "foreground"),
		utils.WithHeader("x-luna-is-background-req", "0"),
		utils.WithHeader("x-luna-is-local-user", "1"),
	}
	if strings.TrimSpace(s.cookie) != "" {
		opts = append(opts, utils.WithHeader("Cookie", s.cookie))
	}
	opts = append(opts, extra...)
	return opts
}

func (s *Soda) fetchPCTrackV2(trackID string) (*sodaTrackV2Response, error) {
	if strings.TrimSpace(s.cookie) == "" {
		return nil, errors.New("soda pc track_v2 requires cookie")
	}

	reqData := map[string]string{
		"track_id":   trackID,
		"media_type": "track",
		"queue_type": "favorite_track_playlist",
		"scene_name": "library",
	}
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return nil, err
	}

	body, err := utils.Post(sodaPCTrackV2URL(), bytes.NewReader(jsonData),
		s.pcRequestOptions(utils.WithHeader("Content-Type", "application/json; charset=utf-8"))...,
	)
	if err != nil {
		return nil, err
	}
	return parseSodaTrackV2Response(body)
}

func (s *Soda) fetchPlayerInfo(playerInfoURL string) (*DownloadInfo, error) {
	infoBody, err := utils.Get(playerInfoURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil {
		return nil, err
	}

	var infoResp struct {
		ResponseMetadata struct {
			Error struct {
				Message string `json:"Message"`
				Code    string `json:"Code"`
			} `json:"Error"`
		} `json:"ResponseMetadata"`
		Result struct {
			Data struct {
				PlayInfoList []sodaPlayerInfo `json:"PlayInfoList"`
			} `json:"Data"`
		} `json:"Result"`
	}
	if err := json.Unmarshal(infoBody, &infoResp); err != nil {
		return nil, fmt.Errorf("parse play info response error: %w", err)
	}

	list := infoResp.Result.Data.PlayInfoList
	if len(list) == 0 {
		if infoResp.ResponseMetadata.Error.Message != "" {
			return nil, errors.New(infoResp.ResponseMetadata.Error.Message)
		}
		return nil, errors.New("no audio stream found")
	}

	best, ok := sodaBestPlayerInfo(list)
	if !ok {
		return nil, errors.New("invalid download url")
	}
	downloadURL := best.MainPlayURL
	if downloadURL == "" {
		downloadURL = best.BackupPlayURL
	}
	if downloadURL == "" {
		return nil, errors.New("invalid download url")
	}

	return &DownloadInfo{
		URL:      downloadURL,
		PlayAuth: best.PlayAuth,
		Format:   best.Format,
		Size:     best.Size,
		Duration: best.Duration,
		Bitrate:  best.Bitrate,
		Quality:  best.Quality,
	}, nil
}

func (s *Soda) fetchSongDetail(trackID string) (*model.Song, error) {
	v2Resp, err := s.fetchWebTrackV2(trackID)
	if err != nil {
		return nil, err
	}

	track := v2Resp.primaryTrack()
	if track.ID == "" {
		return nil, errors.New("track info not found")
	}

	song := sodaBuildSongFromTrack(track)
	if info, err := s.resolveDownloadInfo(track.ID, v2Resp); err == nil {
		applySodaDownloadInfo(&song, info)
	}

	return &song, nil
}

func parseSodaLyric(raw string) string {
	var sb strings.Builder
	lineRegex := regexp.MustCompile(`^\[(\d+),(\d+)\](.*)$`)
	wordRegex := regexp.MustCompile(`<[^>]+>`)

	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := lineRegex.FindStringSubmatch(line)
		if len(matches) >= 4 {
			startTimeStr := matches[1]
			content := matches[3]
			cleanContent := wordRegex.ReplaceAllString(content, "")
			startTime, _ := strconv.Atoi(startTimeStr)
			minutes := startTime / 60000
			seconds := (startTime % 60000) / 1000
			millis := (startTime % 1000) / 10
			sb.WriteString(fmt.Sprintf("[%02d:%02d.%02d]%s\n", minutes, seconds, millis, cleanContent))
		}
	}
	return sb.String()
}
