package migu

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"regexp"
)

func Search(keyword string) ([]model.Song, error) { return defaultMigu.Search(keyword) }

func Parse(link string) (*model.Song, error) { return defaultMigu.Parse(link) }

// Search 搜索歌曲
func (m *Migu) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("ua", "Android_migu")
	params.Set("version", "5.0.1")
	params.Set("text", keyword)
	params.Set("pageNo", "1")
	params.Set("pageSize", "10")
	params.Set("searchSwitch", `{"song":1,"album":0,"singer":0,"tagSong":0,"mvSong":0,"songlist":0,"bestShow":1}`)

	apiURL := "http://pd.musicapp.migu.cn/MIGUM2.0/v1.0/content/search_all.do?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", m.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		SongResultData struct {
			Result []MiguSongItem `json:"result"`
		} `json:"songResultData"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("migu json parse error: %w", err)
	}

	var songs []model.Song
	for _, item := range resp.SongResultData.Result {
		song := m.convertItemToSong(item)
		if song != nil {
			songs = append(songs, *song)
		}
	}
	return songs, nil
}

func (m *Migu) Parse(link string) (*model.Song, error) {
	// 1. 提取 ContentID
	// 支持格式: https://music.migu.cn/v3/music/song/60054701934
	re := regexp.MustCompile(`music\.migu\.cn/v3/music/song/(\d+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, errors.New("invalid migu link")
	}
	contentID := matches[1]

	// 2. 获取歌曲详情 (为了拿到 resourceType 和 formatType)
	song, err := m.fetchSongDetail(contentID)
	if err != nil {
		return nil, err
	}

	// 3. 获取下载链接
	// 因为 convertItemToSong 已经填充了 Extra，所以可以直接调用 GetDownloadURL
	downloadURL, err := m.GetDownloadURL(song)
	if err == nil {
		song.URL = downloadURL
	}

	return song, nil
}
