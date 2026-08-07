package kugou

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	MobileUserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1"
	MobileReferer   = "http://m.kugou.com"
	PCUserAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
	VIPInfoAPI      = "https://vip.kugou.com/recharge/roleinfo"
	KugouSignKey    = "NVPh5oo715z5DIWAeQlhMDsWXXQV4hwt"
	KugouLiteSign   = "LnT6xpN3khm36zse0QzvmgTZ3waWdRSA"
	KugouLiteAppID  = "3116"
	KugouLiteVer    = "11440"
	KugouLiteKey    = "185672dd44712f60bb1736df5a377e82"
)

type Kugou struct {
	cookie     string
	isVipCache *bool
}

func New(cookie string) *Kugou { return &Kugou{cookie: cookie} }

var defaultKugou = New("")

// fetchPlaylistDetail [内部复用] 获取歌单详情 (Metadata + Songs)
// fetchAlbumDetail returns album metadata and songs.
func (k *Kugou) fetchAlbumDetail(id string) (*model.Playlist, []model.Song, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil, errors.New("album id is empty")
	}

	infoURL := fmt.Sprintf("http://mobilecdn.kugou.com/api/v3/album/info?albumid=%s&version=9108&area_code=1", id)
	infoBody, err := utils.Get(infoURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, nil, err
	}

	var infoResp struct {
		Status  int    `json:"status"`
		Errcode int    `json:"errcode"`
		Error   string `json:"error"`
		Data    struct {
			AlbumID     int    `json:"albumid"`
			AlbumName   string `json:"albumname"`
			SingerName  string `json:"singername"`
			Intro       string `json:"intro"`
			ImgURL      string `json:"imgurl"`
			PublishTime string `json:"publishtime"`
			PlayCount   int    `json:"play_count"`
			SongCount   int    `json:"songcount"`
		} `json:"data"`
	}

	if err := json.Unmarshal(infoBody, &infoResp); err != nil {
		return nil, nil, fmt.Errorf("kugou album info json error: %w", err)
	}
	if infoResp.Errcode != 0 || infoResp.Status != 1 {
		return nil, nil, fmt.Errorf("kugou album info api error: status=%d errcode=%d error=%s", infoResp.Status, infoResp.Errcode, infoResp.Error)
	}

	const pageSize = 300
	total := 0
	songs := make([]model.Song, 0)

	for page := 1; ; page++ {
		songURL := fmt.Sprintf("http://mobilecdn.kugou.com/api/v3/album/song?albumid=%s&page=%d&pagesize=%d&version=9108&area_code=1", id, page, pageSize)
		body, err := utils.Get(songURL,
			utils.WithHeader("User-Agent", MobileUserAgent),
			utils.WithHeader("Cookie", k.cookie),
			utils.WithRandomIPHeader(),
		)
		if err != nil {
			return nil, nil, err
		}

		var resp struct {
			Status  int    `json:"status"`
			Errcode int    `json:"errcode"`
			Error   string `json:"error"`
			Data    struct {
				Total int `json:"total"`
				Info  []struct {
					Hash        string `json:"hash"`
					FileHash    string `json:"origin_hash"`
					SQFileHash  string `json:"sqhash"`
					HQFileHash  string `json:"320hash"`
					ResFileHash string `json:"res_hash"`
					MvHash      string `json:"mvhash"`
					FileName    string `json:"filename"`
					SongName    string `json:"songname"`
					SingerName  string `json:"singername"`
					AlbumName   string `json:"album_name"`
					AlbumID     string `json:"album_id"`
					Duration    int    `json:"duration"`
					FileSize    int64  `json:"filesize"`
					SQFileSize  int64  `json:"sqfilesize"`
					HQFileSize  int64  `json:"320filesize"`
					AudioID     int64  `json:"audio_id"`
					Privilege   int    `json:"privilege"`
					Remark      string `json:"remark"`
					TransParam  struct {
						UnionCover     string `json:"union_cover"`
						Ogg320Hash     string `json:"ogg_320_hash"`
						Ogg128Hash     string `json:"ogg_128_hash"`
						Ogg320FileSize int64  `json:"ogg_320_filesize"`
						Ogg128FileSize int64  `json:"ogg_128_filesize"`
					} `json:"trans_param"`
				} `json:"info"`
			} `json:"data"`
		}

		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, nil, fmt.Errorf("kugou album songs json error: %w", err)
		}
		if resp.Errcode != 0 || resp.Status != 1 {
			return nil, nil, fmt.Errorf("kugou album songs api error: status=%d errcode=%d error=%s", resp.Status, resp.Errcode, resp.Error)
		}

		if total == 0 {
			total = resp.Data.Total
		}
		if len(resp.Data.Info) == 0 {
			break
		}

		for _, item := range resp.Data.Info {
			finalHash := firstNonEmpty(
				item.Hash,
				item.SQFileHash,
				item.HQFileHash,
				item.ResFileHash,
				item.TransParam.Ogg320Hash,
				item.FileHash,
				item.TransParam.Ogg128Hash,
			)
			if !isValidHash(finalHash) {
				continue
			}

			name := strings.TrimSpace(item.SongName)
			artist := strings.TrimSpace(item.SingerName)
			if name == "" || artist == "" {
				parts := strings.Split(item.FileName, " - ")
				if len(parts) >= 2 {
					artist = strings.TrimSpace(parts[0])
					name = strings.TrimSpace(strings.Join(parts[1:], " - "))
				} else if name == "" {
					name = strings.TrimSpace(item.FileName)
				}
			}

			albumName := strings.TrimSpace(item.AlbumName)
			if albumName == "" {
				albumName = strings.TrimSpace(infoResp.Data.AlbumName)
			}
			if albumName == "" {
				albumName = strings.TrimSpace(item.Remark)
			}

			size := item.FileSize
			switch finalHash {
			case item.SQFileHash:
				if item.SQFileSize > 0 {
					size = item.SQFileSize
				}
			case item.HQFileHash:
				if item.HQFileSize > 0 {
					size = item.HQFileSize
				}
			case item.ResFileHash:
				if item.SQFileSize > 0 {
					size = item.SQFileSize
				}
			case item.TransParam.Ogg320Hash:
				if item.TransParam.Ogg320FileSize > 0 {
					size = item.TransParam.Ogg320FileSize
				}
			case item.TransParam.Ogg128Hash:
				if item.TransParam.Ogg128FileSize > 0 {
					size = item.TransParam.Ogg128FileSize
				}
			}

			bitrate := 0
			if item.Duration > 0 && size > 0 {
				bitrate = int(size * 8 / 1000 / int64(item.Duration))
			}

			cover := strings.Replace(firstNonEmpty(item.TransParam.UnionCover, infoResp.Data.ImgURL), "{size}", "240", 1)
			albumID := firstNonEmpty(item.AlbumID, id)

			songs = append(songs, model.Song{
				Source:   "kugou",
				ID:       finalHash,
				Name:     name,
				Artist:   artist,
				Album:    albumName,
				AlbumID:  albumID,
				Duration: item.Duration,
				Size:     size,
				Bitrate:  bitrate,
				Cover:    cover,
				Link:     fmt.Sprintf("https://www.kugou.com/song/#hash=%s", finalHash),
				Extra: map[string]string{
					"hash":         finalHash,
					"ogg_320_hash": item.TransParam.Ogg320Hash,
					"ogg_128_hash": item.TransParam.Ogg128Hash,
					"sq_hash":      item.SQFileHash,
					"file_hash":    item.FileHash,
					"res_hash":     item.ResFileHash,
					"mv_hash":      item.MvHash,
					"hq_hash":      item.HQFileHash,
					"audio_id":     strconv.FormatInt(item.AudioID, 10),
					"album_id":     albumID,
					"privilege":    strconv.Itoa(item.Privilege),
				},
			})
		}

		if len(resp.Data.Info) < pageSize {
			break
		}
		if total > 0 && len(songs) >= total {
			break
		}
	}

	trackCount := total
	if trackCount == 0 {
		trackCount = infoResp.Data.SongCount
	}
	if trackCount == 0 {
		trackCount = len(songs)
	}

	album := &model.Playlist{
		Source:      "kugou",
		ID:          firstNonEmpty(strconv.Itoa(infoResp.Data.AlbumID), id),
		Name:        infoResp.Data.AlbumName,
		Cover:       strings.Replace(infoResp.Data.ImgURL, "{size}", "240", 1),
		TrackCount:  trackCount,
		PlayCount:   infoResp.Data.PlayCount,
		Creator:     infoResp.Data.SingerName,
		Description: infoResp.Data.Intro,
		Link:        fmt.Sprintf("https://www.kugou.com/album/%s.html", id),
		Extra: map[string]string{
			"type":         "album",
			"album_id":     firstNonEmpty(strconv.Itoa(infoResp.Data.AlbumID), id),
			"publish_time": infoResp.Data.PublishTime,
		},
	}

	return album, songs, nil
}

func (k *Kugou) fetchPlaylistDetail(id string) (*model.Playlist, []model.Song, error) {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(id)), "gcid_") {
		return k.fetchSonglistDetail(id)
	}

	apiURL := fmt.Sprintf("http://mobilecdn.kugou.com/api/v3/special/song?specialid=%s&page=1&pagesize=300&version=9108&area_code=1", id)

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, nil, err
	}

	var resp struct {
		Data struct {
			Info []struct {
				Hash        string      `json:"hash"`
				FileHash    string      `json:"FileHash"`
				SQFileHash  string      `json:"SQFileHash"`
				HQFileHash  string      `json:"HQFileHash"`
				ResFileHash string      `json:"ResFileHash"`
				MvHash      string      `json:"MvHash"`
				FileName    string      `json:"filename"`
				Duration    int         `json:"duration"`
				FileSize    int64       `json:"filesize"`
				SQFileSize  int64       `json:"SQFileSize"`
				HQFileSize  int64       `json:"HQFileSize"`
				ResFileSize int64       `json:"ResFileSize"`
				AlbumName   string      `json:"album_name"`
				AlbumID     string      `json:"AlbumID"`
				Remark      string      `json:"remark"`
				SingerName  string      `json:"singername"`
				SongName    string      `json:"songname"`
				Audioid     interface{} `json:"Audioid"`
				Privilege   int         `json:"Privilege"`
				TransParam  struct {
					UnionCover     string `json:"union_cover"`
					Ogg320Hash     string `json:"ogg_320_hash"`
					Ogg128Hash     string `json:"ogg_128_hash"`
					Ogg320FileSize int64  `json:"ogg_320_filesize"`
					Ogg128FileSize int64  `json:"ogg_128_filesize"`
				} `json:"trans_param"`
			} `json:"info"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("kugou playlist detail json error: %w", err)
	}

	playlist := &model.Playlist{
		Source: "kugou",
		ID:     id,
		Link:   fmt.Sprintf("https://www.kugou.com/yy/special/single/%s.html", id),
	}

	var songs []model.Song
	for _, item := range resp.Data.Info {
		finalHash := firstNonEmpty(
			item.Hash,
			item.SQFileHash,
			item.HQFileHash,
			item.ResFileHash,
			item.TransParam.Ogg320Hash,
			item.FileHash,
			item.TransParam.Ogg128Hash,
		)
		if !isValidHash(finalHash) {
			continue
		}

		name := item.SongName
		artist := item.SingerName
		if name == "" || artist == "" {
			parts := strings.Split(item.FileName, " - ")
			if len(parts) >= 2 {
				artist = strings.TrimSpace(parts[0])
				name = strings.TrimSpace(strings.Join(parts[1:], " - "))
			} else {
				name = item.FileName
			}
		}

		cover := ""
		if item.TransParam.UnionCover != "" {
			cover = strings.Replace(item.TransParam.UnionCover, "{size}", "240", 1)
		}

		albumName := item.AlbumName
		if albumName == "" {
			albumName = item.Remark
		}

		size := item.FileSize
		switch finalHash {
		case item.SQFileHash:
			if item.SQFileSize > 0 {
				size = item.SQFileSize
			}
		case item.HQFileHash:
			if item.HQFileSize > 0 {
				size = item.HQFileSize
			}
		case item.ResFileHash:
			if item.ResFileSize > 0 {
				size = item.ResFileSize
			}
		case item.TransParam.Ogg320Hash:
			if item.TransParam.Ogg320FileSize > 0 {
				size = item.TransParam.Ogg320FileSize
			}
		case item.TransParam.Ogg128Hash:
			if item.TransParam.Ogg128FileSize > 0 {
				size = item.TransParam.Ogg128FileSize
			}
		}

		songs = append(songs, model.Song{
			Source:   "kugou",
			ID:       finalHash,
			Name:     name,
			Artist:   artist,
			Album:    albumName,
			AlbumID:  item.AlbumID,
			Duration: item.Duration,
			Size:     size,
			Cover:    cover,
			Link:     fmt.Sprintf("https://www.kugou.com/song/#hash=%s", finalHash),
			Extra: map[string]string{
				"hash":         finalHash,
				"ogg_320_hash": item.TransParam.Ogg320Hash,
				"ogg_128_hash": item.TransParam.Ogg128Hash,
				"sq_hash":      item.SQFileHash,
				"file_hash":    item.FileHash,
				"res_hash":     item.ResFileHash,
				"mv_hash":      item.MvHash,
				"hq_hash":      item.HQFileHash,
				"audio_id":     formatKugouNumericString(item.Audioid),
				"album_id":     item.AlbumID,
				"privilege":    strconv.Itoa(item.Privilege),
			},
		})
	}

	playlist.TrackCount = len(songs)

	return playlist, songs, nil
}

func (k *Kugou) fetchSonglistDetail(id string) (*model.Playlist, []model.Song, error) {
	id = strings.TrimSpace(id)
	apiURL := fmt.Sprintf("https://www.kugou.com/songlist/%s/", id)

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, nil, err
	}

	matches := regexp.MustCompile(`window\.\$output\s*=\s*({.*?})\s*;\s*</script>`).FindSubmatch(body)
	if len(matches) < 2 {
		return nil, nil, errors.New("kugou songlist payload not found")
	}

	var resp struct {
		EncodeGIC string `json:"encode_gic"`
		Info      struct {
			ListInfo struct {
				Name               string `json:"name"`
				Pic                string `json:"pic"`
				Intro              string `json:"intro"`
				ListCreateUsername string `json:"list_create_username"`
				Count              int    `json:"count"`
				Heat               int    `json:"heat"`
			} `json:"listinfo"`
			Songs []struct {
				Hash        string      `json:"hash"`
				FileHash    string      `json:"FileHash"`
				SQFileHash  string      `json:"SQFileHash"`
				HQFileHash  string      `json:"HQFileHash"`
				ResFileHash string      `json:"ResFileHash"`
				MvHash      string      `json:"MvHash"`
				Name        string      `json:"name"`
				Bitrate     int         `json:"bitrate"`
				Size        int64       `json:"size"`
				Timelen     int         `json:"timelen"`
				Cover       string      `json:"cover"`
				Privilege   int         `json:"privilege"`
				AlbumID     string      `json:"AlbumID"`
				Audioid     interface{} `json:"Audioid"`
				RelateGoods []struct {
					Hash      string `json:"hash"`
					Bitrate   int    `json:"bitrate"`
					Privilege int    `json:"privilege"`
					Size      int64  `json:"size"`
				} `json:"relate_goods"`
				SingerInfo []struct {
					Name string `json:"name"`
				} `json:"singerinfo"`
				AlbumInfo struct {
					Name string `json:"name"`
				} `json:"albuminfo"`
				TransParam struct {
					UnionCover     string `json:"union_cover"`
					Ogg320Hash     string `json:"ogg_320_hash"`
					Ogg128Hash     string `json:"ogg_128_hash"`
					Ogg320FileSize int64  `json:"ogg_320_filesize"`
					Ogg128FileSize int64  `json:"ogg_128_filesize"`
				} `json:"trans_param"`
			} `json:"songs"`
		} `json:"info"`
	}
	if err := json.Unmarshal(matches[1], &resp); err != nil {
		return nil, nil, fmt.Errorf("kugou songlist json parse error: %w", err)
	}

	playlistID := resp.EncodeGIC
	if playlistID == "" {
		playlistID = id
	}
	cover := strings.Replace(resp.Info.ListInfo.Pic, "{size}", "240", 1)
	playlist := &model.Playlist{
		Source:      "kugou",
		ID:          playlistID,
		Name:        resp.Info.ListInfo.Name,
		Cover:       cover,
		TrackCount:  resp.Info.ListInfo.Count,
		PlayCount:   resp.Info.ListInfo.Heat,
		Creator:     resp.Info.ListInfo.ListCreateUsername,
		Description: resp.Info.ListInfo.Intro,
		Link:        fmt.Sprintf("https://www.kugou.com/songlist/%s/", playlistID),
	}

	isVip, _ := k.IsVipAccount()
	songs := make([]model.Song, 0, len(resp.Info.Songs))
	for _, item := range resp.Info.Songs {
		if !isVip && item.Privilege == 10 {
			continue
		}

		fileHash, hqHash, sqHash, size, bitrate := pickSonglistHashes(item.Hash, item.Size, item.Bitrate, item.RelateGoods)
		if isValidHash(item.FileHash) {
			fileHash = item.FileHash
		}
		if isValidHash(item.HQFileHash) {
			hqHash = item.HQFileHash
		}
		if isValidHash(item.SQFileHash) {
			sqHash = item.SQFileHash
		}

		finalHash := firstNonEmpty(
			item.Hash,
			sqHash,
			hqHash,
			item.ResFileHash,
			item.TransParam.Ogg320Hash,
			fileHash,
			item.TransParam.Ogg128Hash,
		)
		if !isValidHash(finalHash) {
			continue
		}

		switch finalHash {
		case sqHash:
		case hqHash:
		case item.ResFileHash:
			if item.Size > 0 {
				size = item.Size
			}
		case item.TransParam.Ogg320Hash:
			if item.TransParam.Ogg320FileSize > 0 {
				size = item.TransParam.Ogg320FileSize
			}
		case item.TransParam.Ogg128Hash:
			if item.TransParam.Ogg128FileSize > 0 {
				size = item.TransParam.Ogg128FileSize
			}
		case fileHash, item.Hash:
			if item.Size > 0 {
				size = item.Size
			}
		}

		duration := normalizeKugouDuration(item.Timelen)
		if duration > 0 && size > 0 {
			bitrate = int(size * 8 / 1000 / int64(duration))
		}

		coverURL := item.Cover
		if item.TransParam.UnionCover != "" {
			coverURL = item.TransParam.UnionCover
		}
		coverURL = strings.Replace(coverURL, "{size}", "240", 1)

		songs = append(songs, model.Song{
			Source:   "kugou",
			ID:       finalHash,
			Name:     pickSonglistSongName(item.Name),
			Artist:   joinSonglistArtists(item.SingerInfo),
			Album:    item.AlbumInfo.Name,
			AlbumID:  item.AlbumID,
			Duration: duration,
			Size:     size,
			Bitrate:  bitrate,
			Cover:    coverURL,
			Link:     fmt.Sprintf("https://www.kugou.com/song/#hash=%s", finalHash),
			Extra: map[string]string{
				"hash":         finalHash,
				"ogg_320_hash": item.TransParam.Ogg320Hash,
				"ogg_128_hash": item.TransParam.Ogg128Hash,
				"sq_hash":      sqHash,
				"file_hash":    fileHash,
				"res_hash":     item.ResFileHash,
				"mv_hash":      item.MvHash,
				"hq_hash":      hqHash,
				"audio_id":     formatKugouNumericString(item.Audioid),
				"album_id":     item.AlbumID,
				"privilege":    strconv.Itoa(item.Privilege),
			},
		})
	}

	if playlist.TrackCount == 0 {
		playlist.TrackCount = len(songs)
	}

	return playlist, songs, nil
}

func (k *Kugou) fetchVIPSongInfo(s *model.Song) (*model.Song, error) {
	if strings.TrimSpace(k.cookie) == "" {
		return nil, errors.New("cookie required for kugou vip download")
	}

	var fallback *model.Song
	var lastErr error
	for _, hash := range collectCandidateHashes(s) {
		info, err := k.fetchURLV5(s, hash)
		if err == nil && info != nil && info.URL != "" {
			if looksLossless(info.Ext, info.Bitrate, info.Size) {
				isVip := true
				k.isVipCache = &isVip
				return info, nil
			}
			if fallback == nil {
				fallback = info
			}
		} else if err != nil {
			lastErr = err
		}

		info, err = k.fetchPrivURLV6(s, hash)
		if err == nil && info != nil && info.URL != "" {
			if looksLossless(info.Ext, info.Bitrate, info.Size) {
				isVip := true
				k.isVipCache = &isVip
				return info, nil
			}
			if fallback == nil {
				fallback = info
			}
		} else if err != nil {
			lastErr = err
		}

		info, err = k.fetchSonginfoV2(hash)
		if err == nil && info != nil && info.URL != "" {
			if looksLossless(info.Ext, info.Bitrate, info.Size) {
				isVip := true
				k.isVipCache = &isVip
				return info, nil
			}
			if fallback == nil {
				fallback = info
			}
		} else if err != nil {
			lastErr = err
		}

		info, err = k.fetchTrackerSongInfo(hash)
		if err == nil && info != nil && info.URL != "" {
			if looksLossless(info.Ext, info.Bitrate, info.Size) {
				isVip := true
				k.isVipCache = &isVip
				return info, nil
			}
			if fallback == nil {
				fallback = info
			}
		} else if err != nil {
			lastErr = err
		}
	}

	if fallback != nil {
		return fallback, nil
	}

	isVip := false
	k.isVipCache = &isVip
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("kugou vip download url not found")
}

func (k *Kugou) fetchURLV5(s *model.Song, hash string) (*model.Song, error) {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if !isValidHash(hash) {
		return nil, errors.New("invalid kugou hash")
	}

	cookie := parseKugouCookie(k.cookie)
	token := strings.TrimSpace(cookie["token"])
	userID := strings.TrimSpace(cookie["userid"])
	mid := strings.TrimSpace(cookie["KUGOU_API_MID"])
	if token == "" || userID == "" || userID == "0" || mid == "" {
		return nil, errors.New("kugou v5 url requires app token, userid and KUGOU_API_MID")
	}

	albumAudioID := "0"
	albumID := "0"
	name, artist, album := "", "", ""
	if s != nil {
		name = s.Name
		artist = s.Artist
		album = s.Album
		albumID = firstNonEmpty(s.AlbumID, albumID)
		if s.Extra != nil {
			albumAudioID = firstNonEmpty(s.Extra["album_audio_id"], s.Extra["audio_id"], albumAudioID)
			albumID = firstNonEmpty(s.Extra["album_id"], albumID)
		}
	}

	clienttime := strconv.FormatInt(time.Now().Unix(), 10)
	dfid := firstNonEmpty(cookie["dfid"], randomKugouDFID())
	params := map[string]string{
		"dfid":           dfid,
		"mid":            mid,
		"uuid":           "-",
		"appid":          KugouLiteAppID,
		"clientver":      KugouLiteVer,
		"clienttime":     clienttime,
		"token":          token,
		"userid":         userID,
		"album_id":       albumID,
		"area_code":      "1",
		"hash":           hash,
		"ssa_flag":       "is_fromtrack",
		"version":        "11436",
		"page_id":        "967177915",
		"quality":        "flac",
		"album_audio_id": albumAudioID,
		"behavior":       "play",
		"pid":            "411",
		"cmd":            "26",
		"pidversion":     "3001",
		"IsFreePart":     "0",
		"ppage_id":       "356753938,823673182,967485191",
		"cdnBackup":      "1",
		"kcard":          "0",
		"module":         "",
	}
	params["key"] = utils.MD5(hash + KugouLiteKey + KugouLiteAppID + mid + userID)
	apiURL := buildKugouAndroidURL("https://gateway.kugou.com/v5/url", params, "")

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", "Android15-1070-11083-46-0-DiscoveryDRADProtocol-wifi"),
		utils.WithHeader("x-router", "trackercdn.kugou.com"),
		utils.WithHeader("dfid", dfid),
		utils.WithHeader("clienttime", clienttime),
		utils.WithHeader("mid", mid),
		utils.WithHeader("kg-rc", "1"),
		utils.WithHeader("kg-thash", "5d816a0"),
		utils.WithHeader("kg-rec", "1"),
		utils.WithHeader("kg-rf", "B9EDA08A64250DEFFBCADDEE00F8F25F"),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kugou v5 url json parse error: %w", err)
	}
	downloadURL := pickKugouResponseURL(resp)
	if strings.TrimSpace(downloadURL) == "" {
		return nil, fmt.Errorf("kugou v5 url unavailable, status=%d error_code=%d", findKugouInt(resp, "status"), findKugouInt(resp, "error_code", "errcode"))
	}

	ext := normalizeKugouExt(findKugouString(resp, "fileType", "extName", "extname"), downloadURL)
	if ext == "" {
		ext = "flac"
	}
	return &model.Song{
		Source:  "kugou",
		ID:      hash,
		Name:    firstNonEmpty(findKugouString(resp, "songName", "song_name", "audio_name"), name),
		Artist:  firstNonEmpty(findKugouString(resp, "authorName", "author_name", "singerName"), artist),
		Album:   album,
		AlbumID: albumID,
		Size:    findKugouInt64(resp, "fileSize", "filesize"),
		Bitrate: normalizeKugouBitrate(findKugouInt(resp, "bitRate", "bitrate")),
		Ext:     ext,
		URL:     downloadURL,
		Link:    fmt.Sprintf("https://www.kugou.com/song/#hash=%s", hash),
		Extra: map[string]string{
			"hash":           hash,
			"album_audio_id": albumAudioID,
			"album_id":       albumID,
		},
	}, nil
}

func (k *Kugou) fetchSonginfoV2(hash string) (*model.Song, error) {
	cookie := parseKugouCookie(k.cookie)
	if strings.TrimSpace(cookie["t"]) == "" || strings.TrimSpace(cookie["KugooID"]) == "" {
		return nil, errors.New("kugou songinfo v2 requires cookie t and KugooID")
	}

	baseParams := map[string]string{
		"srcappid":   "2919",
		"clientver":  "20000",
		"clienttime": strconv.FormatInt(time.Now().UnixMilli(), 10),
		"mid":        firstNonEmpty(cookie["mid"], cookie["kg_mid"]),
		"uuid":       firstNonEmpty(cookie["uuid"], cookie["mid"], cookie["kg_mid"]),
		"dfid":       firstNonEmpty(cookie["dfid"], cookie["kg_dfid"]),
		"appid":      "1014",
		"platid":     "4",
		"token":      cookie["t"],
		"userid":     cookie["KugooID"],
	}

	step1Params := cloneStringMap(baseParams)
	step1Params["hash"] = hash

	var step1Resp struct {
		Data struct {
			Hash               string      `json:"hash"`
			SongName           string      `json:"song_name"`
			AuthorName         string      `json:"author_name"`
			Img                string      `json:"img"`
			AlbumName          string      `json:"album_name"`
			EncodeAlbumAudioID string      `json:"encode_album_audio_id"`
			AlbumAudioID       interface{} `json:"album_audio_id"`
		} `json:"data"`
		Status int `json:"status"`
	}
	if err := k.getSonginfoV2(step1Params, &step1Resp); err != nil {
		return nil, err
	}
	if strings.TrimSpace(step1Resp.Data.EncodeAlbumAudioID) == "" {
		return nil, errors.New("kugou songinfo v2 missing encode_album_audio_id")
	}

	step2Params := cloneStringMap(baseParams)
	step2Params["encode_album_audio_id"] = step1Resp.Data.EncodeAlbumAudioID

	var step2Resp struct {
		Data struct {
			PlayURL       string `json:"play_url"`
			PlayBackupURL string `json:"play_backup_url"`
			FileSize      int64  `json:"filesize"`
			Bitrate       int    `json:"bitrate"`
			FileName      string `json:"file_name"`
			SongName      string `json:"song_name"`
			AuthorName    string `json:"author_name"`
			Img           string `json:"img"`
			TimeLength    int    `json:"timelength"`
			ExtName       string `json:"extname"`
		} `json:"data"`
		Status int `json:"status"`
	}
	if err := k.getSonginfoV2(step2Params, &step2Resp); err != nil {
		return nil, err
	}

	downloadURL := strings.TrimSpace(step2Resp.Data.PlayURL)
	if downloadURL == "" {
		downloadURL = strings.TrimSpace(step2Resp.Data.PlayBackupURL)
	}
	if downloadURL == "" {
		return nil, errors.New("kugou songinfo v2 missing play url")
	}

	info := &model.Song{
		Source:   "kugou",
		ID:       hash,
		Name:     firstNonEmpty(step2Resp.Data.SongName, step1Resp.Data.SongName),
		Artist:   firstNonEmpty(step2Resp.Data.AuthorName, step1Resp.Data.AuthorName),
		Album:    step1Resp.Data.AlbumName,
		Duration: normalizeKugouDuration(step2Resp.Data.TimeLength),
		Size:     step2Resp.Data.FileSize,
		Bitrate:  normalizeKugouBitrate(step2Resp.Data.Bitrate),
		Ext:      normalizeKugouExt(step2Resp.Data.ExtName, downloadURL),
		Cover:    strings.Replace(firstNonEmpty(step2Resp.Data.Img, step1Resp.Data.Img), "{size}", "240", 1),
		URL:      downloadURL,
		Link:     fmt.Sprintf("https://www.kugou.com/song/#hash=%s", hash),
		Extra: map[string]string{
			"hash":                  hash,
			"encode_album_audio_id": step1Resp.Data.EncodeAlbumAudioID,
			"album_audio_id":        formatKugouNumericString(step1Resp.Data.AlbumAudioID),
		},
	}
	return info, nil
}

func (k *Kugou) fetchPrivURLV6(s *model.Song, hash string) (*model.Song, error) {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if !isValidHash(hash) {
		return nil, errors.New("invalid kugou hash")
	}

	cookie := parseKugouCookie(k.cookie)
	token := strings.TrimSpace(cookie["token"])
	userID := strings.TrimSpace(cookie["userid"])
	mid := strings.TrimSpace(cookie["KUGOU_API_MID"])
	if token == "" || userID == "" || userID == "0" || mid == "" {
		return nil, errors.New("kugou priv_url v6 requires app token, userid and KUGOU_API_MID")
	}

	albumAudioID := ""
	if s != nil && s.Extra != nil {
		albumAudioID = firstNonEmpty(s.Extra["album_audio_id"], s.Extra["audio_id"])
	}
	if albumAudioID == "" {
		if info, err := k.fetchSonginfoV2(hash); err == nil && info != nil && info.Extra != nil {
			albumAudioID = firstNonEmpty(info.Extra["album_audio_id"], info.Extra["audio_id"])
		}
	}
	if albumAudioID == "" {
		albumAudioID = "0"
	}

	dfid := firstNonEmpty(cookie["dfid"], randomKugouDFID())
	vipToken := firstNonEmpty(cookie["vip_token"], cookie["viptoken"])
	vipType := firstNonEmpty(cookie["vip_type"], cookie["vipType"], "6")
	clienttimeMS := time.Now().UnixMilli()

	bodyData := map[string]interface{}{
		"area_code": "1",
		"behavior":  "play",
		"qualities": []string{"128", "320", "flac", "high", "multitrack", "viper_atmos", "viper_tape", "viper_clear", "super"},
		"resource": map[string]interface{}{
			"album_audio_id":  albumAudioID,
			"collect_list_id": "3",
			"collect_time":    clienttimeMS,
			"hash":            hash,
			"id":              0,
			"page_id":         1,
			"type":            "audio",
		},
		"token": token,
		"tracker_param": map[string]interface{}{
			"all_m":         1,
			"auth":          "",
			"is_free_part":  0,
			"key":           utils.MD5(hash + KugouLiteKey + KugouLiteAppID + mid + userID),
			"module_id":     0,
			"need_climax":   1,
			"need_xcdn":     1,
			"open_time":     "",
			"pid":           "411",
			"pidversion":    "3001",
			"priv_vip_type": "6",
			"viptoken":      vipToken,
		},
		"userid": userID,
		"vip":    vipType,
	}
	jsonData, err := json.Marshal(bodyData)
	if err != nil {
		return nil, err
	}

	clienttime := strconv.FormatInt(time.Now().Unix(), 10)
	params := map[string]string{
		"dfid":       dfid,
		"mid":        mid,
		"uuid":       "-",
		"appid":      KugouLiteAppID,
		"clientver":  KugouLiteVer,
		"clienttime": clienttime,
		"token":      token,
		"userid":     userID,
	}
	apiURL := buildKugouAndroidURL("http://tracker.kugou.com/v6/priv_url", params, string(jsonData))

	body, err := utils.Post(apiURL, strings.NewReader(string(jsonData)),
		utils.WithHeader("User-Agent", "Android15-1070-11083-46-0-DiscoveryDRADProtocol-wifi"),
		utils.WithHeader("Content-Type", "application/json"),
		utils.WithHeader("dfid", dfid),
		utils.WithHeader("clienttime", clienttime),
		utils.WithHeader("mid", mid),
		utils.WithHeader("kg-rc", "1"),
		utils.WithHeader("kg-thash", "5d816a0"),
		utils.WithHeader("kg-rec", "1"),
		utils.WithHeader("kg-rf", "B9EDA08A64250DEFFBCADDEE00F8F25F"),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kugou priv_url v6 json parse error: %w", err)
	}

	downloadURL := pickKugouResponseURL(resp)
	if strings.TrimSpace(downloadURL) == "" {
		return nil, fmt.Errorf("kugou priv_url v6 unavailable, status=%d error_code=%d", findKugouInt(resp, "status"), findKugouInt(resp, "error_code", "errcode"))
	}

	ext := normalizeKugouExt(findKugouString(resp, "fileType", "extName", "extname"), downloadURL)
	bitrate := normalizeKugouBitrate(findKugouInt(resp, "bitRate", "bitrate"))
	size := findKugouInt64(resp, "fileSize", "filesize")
	name, artist, album, albumID := "", "", "", ""
	if s != nil {
		name = s.Name
		artist = s.Artist
		album = s.Album
		albumID = s.AlbumID
	}

	return &model.Song{
		Source:  "kugou",
		ID:      hash,
		Name:    firstNonEmpty(findKugouString(resp, "songName", "song_name", "audio_name"), name),
		Artist:  firstNonEmpty(findKugouString(resp, "authorName", "author_name", "singerName"), artist),
		Album:   album,
		AlbumID: albumID,
		Size:    size,
		Bitrate: bitrate,
		Ext:     ext,
		URL:     downloadURL,
		Link:    fmt.Sprintf("https://www.kugou.com/song/#hash=%s", hash),
		Extra: map[string]string{
			"hash":           hash,
			"album_audio_id": albumAudioID,
		},
	}, nil
}

func (k *Kugou) getSonginfoV2(params map[string]string, out interface{}) error {
	apiURL := buildKugouSonginfoURL(params)
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", PCUserAgent),
		utils.WithHeader("Referer", "https://www.kugou.com/"),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithHeader("Accept", "application/json, text/plain, */*"),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("kugou songinfo v2 json parse error: %w", err)
	}
	return nil
}

// fetchSongInfo 内部核心逻辑：获取详情和 URL
func (k *Kugou) fetchSongInfo(hash string) (*model.Song, error) {
	params := url.Values{}
	params.Set("cmd", "playInfo")
	params.Set("hash", hash)

	apiURL := "http://m.kugou.com/app/i/getSongInfo.php?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		URL        string      `json:"url"`
		BitRate    int         `json:"bitRate"`
		ExtName    string      `json:"extName"`
		AlbumImg   string      `json:"album_img"`
		SongName   string      `json:"songName"`
		AuthorName string      `json:"author_name"`
		TimeLength int         `json:"timeLength"`
		FileSize   int64       `json:"fileSize"`
		Error      interface{} `json:"error"`
		Errcode    int         `json:"errcode"` // [新增] 检查酷狗是否返回风控错误
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kugou song info json parse error: %w", err)
	}

	// errcode 1002 代表操作太频繁 (风控)
	if resp.Errcode != 0 || resp.URL == "" {
		return nil, fmt.Errorf("kugou song info unavailable, errcode=%d", resp.Errcode)
	}

	if resp.URL == "" {
		return nil, errors.New("download url not found (might be paid song)")
	}

	cover := strings.Replace(resp.AlbumImg, "{size}", "240", 1)

	return &model.Song{
		Source:   "kugou",
		ID:       hash,
		Name:     resp.SongName,
		Artist:   resp.AuthorName,
		Duration: resp.TimeLength,
		Size:     resp.FileSize,
		Bitrate:  resp.BitRate / 1000,
		Ext:      resp.ExtName,
		Cover:    cover,
		URL:      resp.URL,
		Link:     fmt.Sprintf("https://www.kugou.com/song/#hash=%s", hash),
		Extra: map[string]string{
			"hash": hash,
		},
	}, nil
}

// fallbackFetchSongInfo 备用 API：如果原接口被风控 (1002)，使用 PC 网页端 API 进行 Fallback
func (k *Kugou) fetchTrackerSongInfo(hash string) (*model.Song, error) {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if !isValidHash(hash) {
		return nil, errors.New("invalid kugou hash")
	}

	apiURLs := []string{
		fmt.Sprintf("https://trackercdn.kugou.com/i/v2/?cdnBackup=1&behavior=download&pid=1&cmd=21&appid=1001&hash=%s&key=%s", hash, utils.MD5(hash+"kgcloudv2")),
		fmt.Sprintf("http://trackercdnbj.kugou.com/i/v2/?cmd=23&pid=1&behavior=download&hash=%s&key=%s", hash, utils.MD5(hash+"kgcloudv2")),
		fmt.Sprintf("http://trackercdn.kugou.com/i/?cmd=4&pid=1&forceDown=0&vip=1&hash=%s&key=%s", hash, utils.MD5(hash+"kgcloud")),
	}

	for _, apiURL := range apiURLs {
		body, err := utils.Get(apiURL,
			utils.WithHeader("User-Agent", PCUserAgent),
			utils.WithHeader("Referer", "https://www.kugou.com/"),
			utils.WithHeader("Cookie", k.cookie),
			utils.WithRandomIPHeader(),
		)
		if err != nil {
			continue
		}

		var resp struct {
			Status    int         `json:"status"`
			Errcode   int         `json:"errcode"`
			URL       interface{} `json:"url"`
			BackupURL interface{} `json:"backup_url"`
			BitRate   int         `json:"bitRate"`
			ExtName   string      `json:"extName"`
			AlbumImg  string      `json:"album_img"`
			SongName  string      `json:"songName"`
			Author    string      `json:"author_name"`
			FileSize  int64       `json:"fileSize"`
			TimeLen   int         `json:"timeLength"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}

		downloadURL := pickKugouURL(resp.URL)
		if downloadURL == "" {
			downloadURL = pickKugouURL(resp.BackupURL)
		}
		if downloadURL == "" || resp.Errcode != 0 {
			continue
		}

		return &model.Song{
			Source:   "kugou",
			ID:       hash,
			Name:     resp.SongName,
			Artist:   resp.Author,
			Duration: normalizeKugouDuration(resp.TimeLen),
			Size:     resp.FileSize,
			Bitrate:  normalizeKugouBitrate(resp.BitRate),
			Ext:      normalizeKugouExt(resp.ExtName, downloadURL),
			Cover:    strings.Replace(resp.AlbumImg, "{size}", "240", 1),
			URL:      downloadURL,
			Link:     fmt.Sprintf("https://www.kugou.com/song/#hash=%s", hash),
			Extra: map[string]string{
				"hash": hash,
			},
		}, nil
	}

	return nil, errors.New("tracker kugou download url not found")
}

func isValidHash(h string) bool {
	return h != "" && h != "00000000000000000000000000000000"
}

func collectCandidateHashes(s *model.Song) []string {
	var hashes []string
	if s.Extra != nil {
		hashes = append(
			hashes,
			s.Extra["hash"],
			s.Extra["sq_hash"],
			s.Extra["hq_hash"],
			s.Extra["res_hash"],
			s.Extra["ogg_320_hash"],
			s.Extra["file_hash"],
			s.Extra["ogg_128_hash"],
		)
	}
	hashes = append(hashes, s.ID)

	seen := make(map[string]struct{}, len(hashes))
	result := make([]string, 0, len(hashes))
	for _, hash := range hashes {
		hash = strings.ToLower(strings.TrimSpace(hash))
		if !isValidHash(hash) {
			continue
		}
		if _, ok := seen[hash]; ok {
			continue
		}
		seen[hash] = struct{}{}
		result = append(result, hash)
	}
	return result
}

func parseKugouCookie(cookie string) map[string]string {
	result := map[string]string{}
	for _, pair := range strings.Split(cookie, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return result
}

func buildKugouSonginfoURL(params map[string]string) string {
	query := url.Values{}
	for key, value := range params {
		query.Set(key, value)
	}
	query.Set("signature", signKugouSonginfoParams(params))
	return "https://wwwapi.kugou.com/play/songinfo?" + query.Encode()
}

func buildKugouAndroidURL(baseURL string, params map[string]string, data string) string {
	query := url.Values{}
	for key, value := range params {
		query.Set(key, value)
	}
	query.Set("signature", signKugouAndroidParams(params, data))
	return baseURL + "?" + query.Encode()
}

func signKugouSonginfoParams(params map[string]string) string {
	pairs := make([]string, 0, len(params))
	for key, value := range params {
		pairs = append(pairs, key+"="+value)
	}
	sort.Strings(pairs)
	return utils.MD5(KugouSignKey + strings.Join(pairs, "") + KugouSignKey)
}

func signKugouAndroidParams(params map[string]string, data string) string {
	pairs := make([]string, 0, len(params))
	for key, value := range params {
		pairs = append(pairs, key+"="+value)
	}
	sort.Strings(pairs)
	return utils.MD5(KugouLiteSign + strings.Join(pairs, "") + data + KugouLiteSign)
}

func cloneStringMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func formatKugouNumericString(v interface{}) string {
	switch n := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(n)
	case int:
		return strconv.Itoa(n)
	case int32:
		return strconv.FormatInt(int64(n), 10)
	case int64:
		return strconv.FormatInt(n, 10)
	case float32:
		return strconv.FormatFloat(float64(n), 'f', 0, 32)
	case float64:
		return strconv.FormatFloat(n, 'f', 0, 64)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func getKugouPrivilege(s *model.Song) int {
	if s == nil || s.Extra == nil {
		return -1
	}
	privilege, err := strconv.Atoi(strings.TrimSpace(s.Extra["privilege"]))
	if err != nil {
		return -1
	}
	return privilege
}

func pickKugouURL(v interface{}) string {
	switch u := v.(type) {
	case string:
		return u
	case []interface{}:
		for _, item := range u {
			if s, ok := item.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func randomKugouDFID() string {
	value := strings.ToUpper(utils.MD5(strconv.FormatInt(time.Now().UnixNano(), 10)))
	if len(value) > 24 {
		return value[:24]
	}
	return value
}

func pickKugouResponseURL(resp map[string]interface{}) string {
	if downloadURL := pickKugouURL(resp["url"]); downloadURL != "" {
		return strings.ReplaceAll(downloadURL, "\\/", "/")
	}
	if downloadURL := pickKugouURL(resp["backup_url"]); downloadURL != "" {
		return strings.ReplaceAll(downloadURL, "\\/", "/")
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return ""
	}
	if downloadURL := pickKugouURL(data["url"]); downloadURL != "" {
		return strings.ReplaceAll(downloadURL, "\\/", "/")
	}
	if downloadURL := pickKugouURL(data["backup_url"]); downloadURL != "" {
		return strings.ReplaceAll(downloadURL, "\\/", "/")
	}
	for _, key := range []string{"flac", "high", "320", "128", "super"} {
		item, ok := data[key].(map[string]interface{})
		if !ok {
			continue
		}
		if downloadURL := pickKugouURL(item["url"]); downloadURL != "" {
			return strings.ReplaceAll(downloadURL, "\\/", "/")
		}
		if downloadURL := pickKugouURL(item["backup_url"]); downloadURL != "" {
			return strings.ReplaceAll(downloadURL, "\\/", "/")
		}
	}
	return ""
}

func findKugouString(resp map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := resp[key]; ok {
			if s := strings.TrimSpace(formatKugouNumericString(value)); s != "" {
				return s
			}
		}
	}
	if data, ok := resp["data"].(map[string]interface{}); ok {
		return findKugouString(data, keys...)
	}
	return ""
}

func findKugouInt(resp map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		if value, ok := resp[key]; ok {
			switch n := value.(type) {
			case int:
				return n
			case int64:
				return int(n)
			case float64:
				return int(n)
			case string:
				if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
					return i
				}
			}
		}
	}
	if data, ok := resp["data"].(map[string]interface{}); ok {
		return findKugouInt(data, keys...)
	}
	return 0
}

func findKugouInt64(resp map[string]interface{}, keys ...string) int64 {
	for _, key := range keys {
		if value, ok := resp[key]; ok {
			switch n := value.(type) {
			case int:
				return int64(n)
			case int64:
				return n
			case float64:
				return int64(n)
			case string:
				if i, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64); err == nil {
					return i
				}
			}
		}
	}
	if data, ok := resp["data"].(map[string]interface{}); ok {
		return findKugouInt64(data, keys...)
	}
	return 0
}

func normalizeKugouDuration(v int) int {
	if v > 1000 {
		return v / 1000
	}
	return v
}

func normalizeKugouBitrate(v int) int {
	if v > 1000 {
		return v / 1000
	}
	return v
}

func normalizeKugouExt(extName, downloadURL string) string {
	extName = strings.ToLower(strings.TrimSpace(extName))
	if extName != "" {
		return extName
	}

	parsed, err := url.Parse(downloadURL)
	if err != nil {
		return ""
	}
	path := strings.ToLower(parsed.Path)
	switch {
	case strings.HasSuffix(path, ".flac"):
		return "flac"
	case strings.HasSuffix(path, ".ape"):
		return "ape"
	case strings.HasSuffix(path, ".mp3"):
		return "mp3"
	}
	return ""
}

func looksLossless(ext string, bitrate int, size int64) bool {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "flac" || ext == "ape" || ext == "wav" {
		return true
	}
	return bitrate >= 700 || size >= 20*1024*1024
}

func pickSonglistHashes(defaultHash string, defaultSize int64, defaultBitrate int, goods []struct {
	Hash      string `json:"hash"`
	Bitrate   int    `json:"bitrate"`
	Privilege int    `json:"privilege"`
	Size      int64  `json:"size"`
}) (fileHash, hqHash, sqHash string, size int64, bitrate int) {
	fileHash = strings.TrimSpace(defaultHash)
	size = defaultSize
	bitrate = defaultBitrate

	for _, item := range goods {
		hash := strings.TrimSpace(item.Hash)
		if !isValidHash(hash) {
			continue
		}

		switch {
		case item.Bitrate >= 700:
			sqHash = hash
			size = item.Size
			bitrate = item.Bitrate
		case item.Bitrate >= 320:
			if sqHash == "" {
				size = item.Size
				bitrate = item.Bitrate
			}
			hqHash = hash
		default:
			if fileHash == "" {
				fileHash = hash
			}
		}
	}

	if fileHash == "" {
		if hqHash != "" {
			fileHash = hqHash
		} else if sqHash != "" {
			fileHash = sqHash
		}
	}
	return fileHash, hqHash, sqHash, size, bitrate
}

func joinSonglistArtists(artists []struct {
	Name string `json:"name"`
}) string {
	if len(artists) == 0 {
		return ""
	}

	names := make([]string, 0, len(artists))
	for _, artist := range artists {
		name := strings.TrimSpace(artist.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return strings.Join(names, "/")
}

func pickSonglistSongName(name string) string {
	parts := strings.SplitN(name, " - ", 2)
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		return strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(name)
}
