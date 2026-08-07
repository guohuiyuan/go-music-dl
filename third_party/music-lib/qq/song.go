package qq

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

func Search(keyword string) ([]model.Song, error) { return defaultQQ.Search(keyword) }

func Parse(link string) (*model.Song, error) { return defaultQQ.Parse(link) }

// Search searches songs.
func (q *QQ) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("w", keyword)
	params.Set("format", "json")
	params.Set("p", "1")
	params.Set("n", "10")
	apiURL := "http://c.y.qq.com/soso/fcgi-bin/search_for_qq_cp?" + params.Encode()

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
		Data struct {
			Song struct {
				List []struct {
					SongID    int64  `json:"songid"`
					SongName  string `json:"songname"`
					SongMID   string `json:"songmid"`
					AlbumName string `json:"albumname"`
					AlbumMID  string `json:"albummid"`
					Interval  int    `json:"interval"`
					Size128   int64  `json:"size128"`
					Size320   int64  `json:"size320"`
					SizeFlac  int64  `json:"sizeflac"`
					Singer    []struct {
						Name string `json:"name"`
					} `json:"singer"`
					Pay struct {
						PayDownload   int `json:"paydownload"`
						PayPlay       int `json:"payplay"`
						PayTrackPrice int `json:"paytrackprice"`
					} `json:"pay"`
				} `json:"list"`
			} `json:"song"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq json parse error: %w", err)
	}

	isVip, _ := q.IsVipAccount()

	var songs []model.Song
	for _, item := range resp.Data.Song.List {
		// Hide VIP-only songs for non-VIP accounts.
		if !isVip && item.Pay.PayPlay == 1 {
			continue
		}

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
				"song_id": strconv.FormatInt(item.SongID, 10),
			},
		})
	}
	return songs, nil
}

// Parse parses a song link and enriches it with download info when possible.
func (q *QQ) Parse(link string) (*model.Song, error) {
	var songMID string

	// Try songDetail/xxx format
	re := regexp.MustCompile(`songDetail/(\w+)`)
	if matches := re.FindStringSubmatch(link); len(matches) >= 2 {
		songMID = matches[1]
	}

	// Try playsong.html?songid=xxx or songid query param
	if songMID == "" {
		if u, err := url.Parse(link); err == nil {
			if id := u.Query().Get("songid"); id != "" {
				// Numeric songid — fetch detail by ID
				song, err := q.fetchSongDetailByID(id)
				if err != nil {
					return nil, err
				}
				if downloadURL, dlErr := q.GetDownloadURL(song); dlErr == nil {
					song.URL = downloadURL
				}
				return song, nil
			}
			if mid := u.Query().Get("songmid"); mid != "" {
				songMID = mid
			}
		}
	}

	if songMID == "" {
		return nil, errors.New("invalid qq music link")
	}

	song, err := q.fetchSongDetail(songMID)
	if err != nil {
		return nil, err
	}

	downloadURL, err := q.GetDownloadURL(song)
	if err == nil {
		song.URL = downloadURL
	}

	return song, nil
}
