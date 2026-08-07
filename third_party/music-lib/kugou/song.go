package kugou

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

func Search(keyword string) ([]model.Song, error) { return defaultKugou.Search(keyword) }

func Parse(link string) (*model.Song, error) { return defaultKugou.Parse(link) }

type kugouSearchResponse struct {
	Data struct {
		Lists []kugouSearchItem `json:"lists"`
	} `json:"data"`
}

type kugouSearchItem struct {
	Scid        interface{} `json:"Scid"`
	ID          interface{} `json:"ID"`
	MixSongID   interface{} `json:"MixSongID"`
	SongName    string      `json:"SongName"`
	SingerName  string      `json:"SingerName"`
	AlbumName   string      `json:"AlbumName"`
	AlbumID     string      `json:"AlbumID"`
	Audioid     interface{} `json:"Audioid"`
	Duration    int         `json:"Duration"`
	FileHash    string      `json:"FileHash"`
	SQFileHash  string      `json:"SQFileHash"`
	HQFileHash  string      `json:"HQFileHash"`
	ResFileHash string      `json:"ResFileHash"`
	MvHash      string      `json:"MvHash"`
	SQFileSize  int64       `json:"SQFileSize"`
	HQFileSize  int64       `json:"HQFileSize"`
	ResFileSize int64       `json:"ResFileSize"`
	FileSize    interface{} `json:"FileSize"`
	Image       string      `json:"Image"`
	PayType     int         `json:"PayType"`
	Privilege   int         `json:"Privilege"`
	TransParam  struct {
		Ogg320Hash     string `json:"ogg_320_hash"`
		Ogg128Hash     string `json:"ogg_128_hash"`
		Ogg320FileSize int64  `json:"ogg_320_filesize"`
		Ogg128FileSize int64  `json:"ogg_128_filesize"`
	} `json:"trans_param"`
}

// Search 搜索歌曲
func (k *Kugou) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("keyword", keyword)
	params.Set("platform", "WebFilter")
	params.Set("format", "json")
	params.Set("page", "1")
	params.Set("pagesize", "10")
	params.Set("userid", "-1")
	params.Set("clientver", "")
	params.Set("tag", "em")
	params.Set("filter", "2")
	params.Set("iscorrection", "1")
	params.Set("privilege_filter", "0")
	params.Set("_", strconv.FormatInt(time.Now().UnixMilli(), 10))

	apiURL := "http://songsearch.kugou.com/song_search_v2?" + params.Encode()

	fetchSearch := func(withCookie bool) ([]byte, error) {
		options := []utils.RequestOption{
			utils.WithHeader("User-Agent", "Mozilla/5.0 (Linux; Android 10; SM-G981B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.162 Mobile Safari/537.36"),
			utils.WithRandomIPHeader(),
		}
		if withCookie && strings.TrimSpace(k.cookie) != "" {
			options = append(options, utils.WithHeader("Cookie", k.cookie))
		}
		return utils.Get(apiURL, options...)
	}

	body, err := fetchSearch(true)
	if err != nil {
		return nil, err
	}

	var resp kugouSearchResponse

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}
	if len(resp.Data.Lists) == 0 && strings.TrimSpace(k.cookie) != "" {
		if retryBody, retryErr := fetchSearch(false); retryErr == nil {
			var retryResp kugouSearchResponse
			if retryErr := json.Unmarshal(retryBody, &retryResp); retryErr == nil && len(retryResp.Data.Lists) > 0 {
				resp = retryResp
			}
		}
	}

	isVip, _ := k.IsVipAccount()

	var songs []model.Song
	for _, item := range resp.Data.Lists {
		finalHash := item.FileHash
		if item.Privilege != 10 && isVip {
			finalHash = item.SQFileHash
		}
		if strings.TrimSpace(finalHash) == "" {
			finalHash = firstNonEmpty(
				item.SQFileHash,
				item.HQFileHash,
				item.ResFileHash,
				item.TransParam.Ogg320Hash,
				item.FileHash,
				item.TransParam.Ogg128Hash,
			)
		}

		var size int64
		switch v := item.FileSize.(type) {
		case float64:
			size = int64(v)
		case int:
			size = int64(v)
		case string:
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				size = i
			}
		}

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

		bitrate := 0
		if item.Duration > 0 && size > 0 {
			bitrate = int(size * 8 / 1000 / int64(item.Duration))
		}

		coverURL := strings.Replace(item.Image, "{size}", "240", 1)

		songs = append(songs, model.Song{
			Source:   "kugou",
			ID:       finalHash,
			Name:     cleanKugouSearchText(item.SongName),
			Artist:   cleanKugouSearchText(item.SingerName),
			Album:    cleanKugouSearchText(item.AlbumName),
			AlbumID:  item.AlbumID,
			Duration: item.Duration,
			Size:     size,
			Bitrate:  bitrate,
			Cover:    coverURL,
			Link:     fmt.Sprintf("https://www.kugou.com/song/#hash=%s", finalHash),
			Extra: map[string]string{
				"hash":           finalHash,
				"ogg_320_hash":   item.TransParam.Ogg320Hash,
				"ogg_128_hash":   item.TransParam.Ogg128Hash,
				"sq_hash":        item.SQFileHash,
				"file_hash":      item.FileHash,
				"res_hash":       item.ResFileHash,
				"mv_hash":        item.MvHash,
				"hq_hash":        item.HQFileHash,
				"audio_id":       formatKugouNumericString(item.Audioid),
				"album_audio_id": firstNonEmpty(formatKugouNumericString(item.MixSongID), formatKugouNumericString(item.ID)),
				"album_id":       item.AlbumID,
				"privilege":      strconv.Itoa(item.Privilege),
			},
		})
	}
	return songs, nil
}

// Parse 解析链接
func (k *Kugou) Parse(link string) (*model.Song, error) {
	re := regexp.MustCompile(`(?i)hash=([a-f0-9]{32})`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, errors.New("invalid kugou link or hash not found")
	}
	hash := matches[1]
	return k.fetchSongInfo(hash)
}

func cleanKugouSearchText(value string) string {
	value = html.UnescapeString(value)
	value = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(value, "")
	return strings.TrimSpace(value)
}
