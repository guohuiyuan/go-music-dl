package fivesing

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"html"
	"net/url"
	"regexp"
	"strconv"
)

func Search(keyword string) ([]model.Song, error) { return defaultFivesing.Search(keyword) }

func Parse(link string) (*model.Song, error) { return defaultFivesing.Parse(link) }

// Search 搜索歌曲
func (f *Fivesing) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("keyword", keyword)
	params.Set("sort", "1")
	params.Set("page", "1")
	params.Set("filter", "0")
	params.Set("type", "0")

	apiURL := "http://search.5sing.kugou.com/home/json?" + params.Encode()
	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Cookie", f.cookie))
	if err != nil {
		return nil, err
	}

	var resp struct {
		List []struct {
			SongID    int64  `json:"songId"`
			SongName  string `json:"songName"`
			Singer    string `json:"singer"`
			SongSize  int64  `json:"songSize"`
			TypeEname string `json:"typeEname"`
		} `json:"list"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("fivesing json parse error: %w", err)
	}

	var songs []model.Song
	for _, item := range resp.List {
		name := removeEmTags(html.UnescapeString(item.SongName))
		artist := removeEmTags(html.UnescapeString(item.Singer))

		duration := 0
		if item.SongSize > 0 {
			duration = int((item.SongSize * 8) / 320000)
		}

		songs = append(songs, model.Song{
			Source:   "fivesing",
			ID:       fmt.Sprintf("%d|%s", item.SongID, item.TypeEname),
			Name:     name,
			Artist:   artist,
			Duration: duration,
			Size:     item.SongSize,
			Link:     fmt.Sprintf("http://5sing.kugou.com/%s/%d.html", item.TypeEname, item.SongID),
			Extra: map[string]string{
				"songid":   strconv.FormatInt(item.SongID, 10),
				"songtype": item.TypeEname,
			},
		})
	}
	return songs, nil
}

// Parse 解析链接并获取完整信息
func (f *Fivesing) Parse(link string) (*model.Song, error) {
	re := regexp.MustCompile(`5sing\.kugou\.com/(\w+)/(\d+)\.html`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 3 {
		return nil, errors.New("invalid 5sing link")
	}
	songType := matches[1]
	songID := matches[2]

	return f.fetchSongInfo(songID, songType)
}
