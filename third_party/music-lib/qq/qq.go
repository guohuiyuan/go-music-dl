package qq

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	UserAgent       = "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13B143 Safari/601.1"
	SearchReferer   = "http://m.y.qq.com"
	DownloadReferer = "http://y.qq.com"
	LyricReferer    = "https://y.qq.com/portal/player.html"
)

type QQ struct {
	cookie     string
	isVipCache *bool
}

func New(cookie string) *QQ { return &QQ{cookie: cookie} }

var defaultQQ = New("")

// joinQQNames joins artist names for display.
func joinQQNames(names []string) string {
	return strings.Join(names, ", ")
}

// fetchAlbumDetail returns album metadata and songs.
func (q *QQ) fetchAlbumDetail(id string) (*model.Playlist, []model.Song, error) {
	albumMID := strings.TrimSpace(id)
	if albumMID == "" {
		return nil, nil, errors.New("album id is empty")
	}

	headers := []utils.RequestOption{
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
		utils.WithHeader("Referer", "https://y.qq.com/"),
		utils.WithHeader("Content-Type", "application/json"),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	}

	detailReq := map[string]interface{}{
		"comm": map[string]interface{}{
			"ct": 24,
			"cv": 0,
		},
		"album": map[string]interface{}{
			"module": "music.musichallAlbum.AlbumInfoServer",
			"method": "GetAlbumDetail",
			"param": map[string]interface{}{
				"albumMid": albumMID,
			},
		},
	}
	detailJSON, _ := json.Marshal(detailReq)

	detailBody, err := utils.Post("https://u.y.qq.com/cgi-bin/musicu.fcg", bytes.NewReader(detailJSON), headers...)
	if err != nil {
		return nil, nil, err
	}

	var detailResp struct {
		Code  int `json:"code"`
		Album struct {
			Code int `json:"code"`
			Data struct {
				BasicInfo struct {
					AlbumID     int64  `json:"albumID"`
					AlbumMid    string `json:"albumMid"`
					AlbumName   string `json:"albumName"`
					PublishDate string `json:"publishDate"`
					Desc        string `json:"desc"`
				} `json:"basicInfo"`
				Company struct {
					Name string `json:"name"`
				} `json:"company"`
				Singer struct {
					SingerList []struct {
						Name string `json:"name"`
					} `json:"singerList"`
				} `json:"singer"`
			} `json:"data"`
		} `json:"album"`
	}

	if err := json.Unmarshal(detailBody, &detailResp); err != nil {
		return nil, nil, fmt.Errorf("qq album detail json parse error: %w", err)
	}
	if detailResp.Album.Code != 0 {
		return nil, nil, fmt.Errorf("qq album detail api error code: %d", detailResp.Album.Code)
	}

	info := detailResp.Album.Data.BasicInfo
	if info.AlbumMid != "" {
		albumMID = info.AlbumMid
	}
	if info.AlbumName == "" {
		return nil, nil, errors.New("album not found")
	}

	artistNames := make([]string, 0, len(detailResp.Album.Data.Singer.SingerList))
	for _, singer := range detailResp.Album.Data.Singer.SingerList {
		if singer.Name != "" {
			artistNames = append(artistNames, singer.Name)
		}
	}

	const batchSize = 100
	totalNum := 0
	songs := make([]model.Song, 0)

	for begin := 0; ; begin += batchSize {
		songReq := map[string]interface{}{
			"comm": map[string]interface{}{
				"ct": 24,
				"cv": 0,
			},
			"album": map[string]interface{}{
				"module": "music.musichallAlbum.AlbumSongList",
				"method": "GetAlbumSongList",
				"param": map[string]interface{}{
					"albumMid": albumMID,
					"begin":    begin,
					"num":      batchSize,
					"order":    2,
				},
			},
		}
		songJSON, _ := json.Marshal(songReq)

		songBody, err := utils.Post("https://u.y.qq.com/cgi-bin/musicu.fcg", bytes.NewReader(songJSON), headers...)
		if err != nil {
			return nil, nil, err
		}

		var songResp struct {
			Code  int `json:"code"`
			Album struct {
				Code int `json:"code"`
				Data struct {
					TotalNum int `json:"totalNum"`
					SongList []struct {
						SongInfo struct {
							Mid      string `json:"mid"`
							Name     string `json:"name"`
							Interval int    `json:"interval"`
							Singer   []struct {
								Name string `json:"name"`
							} `json:"singer"`
							Album struct {
								ID   int64  `json:"id"`
								Mid  string `json:"mid"`
								Name string `json:"name"`
							} `json:"album"`
							File struct {
								Size128MP3 int64 `json:"size_128mp3"`
								Size320MP3 int64 `json:"size_320mp3"`
								SizeFlac   int64 `json:"size_flac"`
							} `json:"file"`
							Pay struct {
								PayPlay int `json:"pay_play"`
							} `json:"pay"`
						} `json:"songInfo"`
					} `json:"songList"`
				} `json:"data"`
			} `json:"album"`
		}

		if err := json.Unmarshal(songBody, &songResp); err != nil {
			return nil, nil, fmt.Errorf("qq album songs json parse error: %w", err)
		}
		if songResp.Album.Code != 0 {
			return nil, nil, fmt.Errorf("qq album songs api error code: %d", songResp.Album.Code)
		}

		if totalNum == 0 {
			totalNum = songResp.Album.Data.TotalNum
		}

		pageSongs := songResp.Album.Data.SongList
		if len(pageSongs) == 0 {
			break
		}

		for _, item := range pageSongs {
			songInfo := item.SongInfo
			if songInfo.Mid == "" {
				continue
			}

			pageArtistNames := make([]string, 0, len(songInfo.Singer))
			for _, singer := range songInfo.Singer {
				if singer.Name != "" {
					pageArtistNames = append(pageArtistNames, singer.Name)
				}
			}

			fileSize := songInfo.File.Size128MP3
			bitrate := 128
			if songInfo.File.SizeFlac > 0 {
				fileSize = songInfo.File.SizeFlac
				if songInfo.Interval > 0 {
					bitrate = int(fileSize * 8 / 1000 / int64(songInfo.Interval))
				} else {
					bitrate = 800
				}
			} else if songInfo.File.Size320MP3 > 0 {
				fileSize = songInfo.File.Size320MP3
				bitrate = 320
			}

			cover := ""
			if songInfo.Album.Mid != "" {
				cover = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", songInfo.Album.Mid)
			}

			songs = append(songs, model.Song{
				Source:   "qq",
				ID:       songInfo.Mid,
				Name:     songInfo.Name,
				Artist:   joinQQNames(pageArtistNames),
				Album:    songInfo.Album.Name,
				AlbumID:  songInfo.Album.Mid,
				Duration: songInfo.Interval,
				Size:     fileSize,
				Bitrate:  bitrate,
				Cover:    cover,
				Link:     fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", songInfo.Mid),
				Extra: map[string]string{
					"songmid":   songInfo.Mid,
					"album_mid": songInfo.Album.Mid,
					"album_id":  strconv.FormatInt(songInfo.Album.ID, 10),
				},
			})
		}

		if len(pageSongs) < batchSize {
			break
		}
		if totalNum > 0 && begin+len(pageSongs) >= totalNum {
			break
		}
	}

	trackCount := totalNum
	if trackCount == 0 {
		trackCount = len(songs)
	}

	album := &model.Playlist{
		Source:      "qq",
		ID:          albumMID,
		Name:        info.AlbumName,
		Cover:       fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", albumMID),
		TrackCount:  trackCount,
		Creator:     joinQQNames(artistNames),
		Description: info.Desc,
		Link:        fmt.Sprintf("https://y.qq.com/n/ryqq/albumDetail/%s", albumMID),
		Extra: map[string]string{
			"type":         "album",
			"album_id":     strconv.FormatInt(info.AlbumID, 10),
			"album_mid":    albumMID,
			"company":      detailResp.Album.Data.Company.Name,
			"publish_time": info.PublishDate,
		},
	}

	return album, songs, nil
}

// fetchPlaylistDetail returns playlist metadata and songs.
func (q *QQ) fetchPlaylistDetail(id string) (*model.Playlist, []model.Song, error) {
	params := url.Values{}
	params.Set("type", "1")
	params.Set("json", "1")
	params.Set("utf8", "1")
	params.Set("onlysong", "0")
	params.Set("disstid", id)
	params.Set("format", "json")
	params.Set("g_tk", "5381")
	params.Set("loginUin", "0")
	params.Set("hostUin", "0")
	params.Set("inCharset", "utf8")
	params.Set("outCharset", "utf-8")
	params.Set("notice", "0")
	params.Set("platform", "yqq")
	params.Set("needNewCode", "0")

	var resp struct {
		Code    int    `json:"code"`
		Subcode int    `json:"subcode"`
		Msg     string `json:"msg"`
		Cdlist  []struct {
			Dissname string `json:"dissname"`
			Logo     string `json:"logo"`
			Nickname string `json:"nickname"`
			Desc     string `json:"desc"`
			Visitnum int    `json:"visitnum"`
			Songnum  int    `json:"songnum"`
			Songlist []struct {
				SongID    int64  `json:"songid"`
				SongName  string `json:"songname"`
				SongMID   string `json:"songmid"`
				AlbumName string `json:"albumname"`
				AlbumMID  string `json:"albummid"`
				Interval  int    `json:"interval"`
				Size128   int64  `json:"size128"`
				Size320   int64  `json:"size320"`
				SizeFlac  int64  `json:"sizeflac"`
				Pay       struct {
					PayPlay int `json:"payplay"`
				} `json:"pay"`
				Singer []struct {
					Name string `json:"name"`
				} `json:"singer"`
			} `json:"songlist"`
		} `json:"cdlist"`
	}

	endpoints := []string{
		"https://i.y.qq.com/qzone-music/fcg-bin/fcg_ucc_getcdinfo_byids_cp.fcg",
		"http://c.y.qq.com/qzone/fcg-bin/fcg_ucc_getcdinfo_byids_cp.fcg",
	}
	var lastErr error
	for _, endpoint := range endpoints {
		apiURL := endpoint + "?" + params.Encode()
		body, err := utils.Get(apiURL,
			utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
			utils.WithHeader("Referer", "https://y.qq.com/"),
			utils.WithHeader("Cookie", q.cookie),
			utils.WithRandomIPHeader(),
		)
		if err != nil {
			lastErr = err
			continue
		}

		sBody := string(body)
		if idx := strings.Index(sBody, "("); idx >= 0 && strings.HasSuffix(strings.TrimSpace(sBody), ")") {
			sBody = sBody[idx+1 : len(sBody)-1]
			body = []byte(sBody)
		}

		resp = struct {
			Code    int    `json:"code"`
			Subcode int    `json:"subcode"`
			Msg     string `json:"msg"`
			Cdlist  []struct {
				Dissname string `json:"dissname"`
				Logo     string `json:"logo"`
				Nickname string `json:"nickname"`
				Desc     string `json:"desc"`
				Visitnum int    `json:"visitnum"`
				Songnum  int    `json:"songnum"`
				Songlist []struct {
					SongID    int64  `json:"songid"`
					SongName  string `json:"songname"`
					SongMID   string `json:"songmid"`
					AlbumName string `json:"albumname"`
					AlbumMID  string `json:"albummid"`
					Interval  int    `json:"interval"`
					Size128   int64  `json:"size128"`
					Size320   int64  `json:"size320"`
					SizeFlac  int64  `json:"sizeflac"`
					Pay       struct {
						PayPlay int `json:"payplay"`
					} `json:"pay"`
					Singer []struct {
						Name string `json:"name"`
					} `json:"singer"`
				} `json:"songlist"`
			} `json:"cdlist"`
		}{}
		if err := json.Unmarshal(body, &resp); err != nil {
			lastErr = fmt.Errorf("qq playlist detail json error: %w", err)
			continue
		}
		if len(resp.Cdlist) > 0 && resp.Subcode == 0 {
			lastErr = nil
			break
		}
		lastErr = fmt.Errorf("qq playlist detail api error: subcode=%d msg=%s", resp.Subcode, resp.Msg)
	}

	if len(resp.Cdlist) == 0 {
		if lastErr != nil {
			return nil, nil, lastErr
		}
		return nil, nil, errors.New("playlist not found (empty cdlist)")
	}

	info := resp.Cdlist[0]

	// Build playlist metadata.
	playlist := &model.Playlist{
		Source:      "qq",
		ID:          id,
		Name:        info.Dissname,
		Cover:       info.Logo,
		Creator:     info.Nickname,
		Description: info.Desc,
		PlayCount:   info.Visitnum,
		TrackCount:  info.Songnum,
		Link:        fmt.Sprintf("https://y.qq.com/n/ryqq/playlist/%s", id),
	}

	var songs []model.Song
	for _, item := range info.Songlist {

		var artistNames []string
		for _, s := range item.Singer {
			artistNames = append(artistNames, s.Name)
		}

		var coverURL string
		if item.AlbumMID != "" {
			coverURL = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", item.AlbumMID)
		}

		fileSize := item.Size128
		bitrate := 128
		if item.SizeFlac > 0 {
			fileSize = item.SizeFlac
			if item.Interval > 0 {
				bitrate = int(fileSize * 8 / 1000 / int64(item.Interval))
			} else {
				bitrate = 800
			}
		} else if item.Size320 > 0 {
			fileSize = item.Size320
			bitrate = 320
		}

		songs = append(songs, model.Song{
			Source:   "qq",
			ID:       item.SongMID,
			Name:     item.SongName,
			Artist:   strings.Join(artistNames, "、"),
			Album:    item.AlbumName,
			Duration: item.Interval,
			Size:     fileSize,
			Bitrate:  bitrate,
			Cover:    coverURL,
			Link:     fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", item.SongMID),
			Extra: map[string]string{
				"songmid": item.SongMID,
			},
		})
	}
	return playlist, songs, nil
}

// fetchSongDetail loads song metadata by songmid.
func (q *QQ) fetchSongDetail(songMID string) (*model.Song, error) {
	params := url.Values{}
	params.Set("songmid", songMID)
	params.Set("format", "json")

	apiURL := "https://c.y.qq.com/v8/fcg-bin/fcg_play_single_song.fcg?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", SearchReferer),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			ID    int64  `json:"id"`
			Name  string `json:"name"`
			Mid   string `json:"mid"`
			Album struct {
				Name string `json:"name"`
				Mid  string `json:"mid"`
			} `json:"album"`
			Singer []struct {
				Name string `json:"name"`
			} `json:"singer"`
			Interval int `json:"interval"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq detail json parse error: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, errors.New("song detail not found")
	}

	item := resp.Data[0]
	var artistNames []string
	for _, s := range item.Singer {
		artistNames = append(artistNames, s.Name)
	}

	var coverURL string
	if item.Album.Mid != "" {
		coverURL = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", item.Album.Mid)
	}

	return &model.Song{
		Source:   "qq",
		ID:       item.Mid,
		Name:     item.Name,
		Artist:   strings.Join(artistNames, "、"),
		Album:    item.Album.Name,
		Duration: item.Interval,
		Cover:    coverURL,
		Link:     fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", item.Mid),
		Extra: map[string]string{
			"songmid": item.Mid,
			"song_id": strconv.FormatInt(item.ID, 10),
		},
	}, nil
}

func (q *QQ) fetchSongDetailByID(songID string) (*model.Song, error) {
	params := url.Values{}
	params.Set("songid", songID)
	params.Set("format", "json")

	apiURL := "https://c.y.qq.com/v8/fcg-bin/fcg_play_single_song.fcg?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", SearchReferer),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			ID    int64  `json:"id"`
			Name  string `json:"name"`
			Mid   string `json:"mid"`
			Album struct {
				Name string `json:"name"`
				Mid  string `json:"mid"`
			} `json:"album"`
			Singer []struct {
				Name string `json:"name"`
			} `json:"singer"`
			Interval int `json:"interval"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq detail json parse error: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, errors.New("song detail not found")
	}

	item := resp.Data[0]
	var artistNames []string
	for _, s := range item.Singer {
		artistNames = append(artistNames, s.Name)
	}
	var coverURL string
	if item.Album.Mid != "" {
		coverURL = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", item.Album.Mid)
	}

	return &model.Song{
		Source:   "qq",
		ID:       item.Mid,
		Name:     item.Name,
		Artist:   strings.Join(artistNames, "、"),
		Album:    item.Album.Name,
		Duration: item.Interval,
		Cover:    coverURL,
		Link:     fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", item.Mid),
		Extra: map[string]string{
			"songmid": item.Mid,
			"song_id": strconv.FormatInt(item.ID, 10),
		},
	}, nil
}
