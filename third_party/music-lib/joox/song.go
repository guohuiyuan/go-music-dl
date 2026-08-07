package joox

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"regexp"
	"strings"
)

func Search(keyword string) ([]model.Song, error) { return defaultJoox.Search(keyword) }

func Parse(link string) (*model.Song, error) { return defaultJoox.Parse(link) }

// Search 搜索歌曲
func (j *Joox) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("country", "sg")
	params.Set("lang", "zh_cn")
	params.Set("keyword", keyword)
	apiURL := "https://cache.api.joox.com/openjoox/v3/search?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		SectionList []struct {
			ItemList []struct {
				Song []struct {
					SongInfo struct {
						ID         string `json:"id"`
						Name       string `json:"name"`
						AlbumName  string `json:"album_name"`
						ArtistList []struct {
							Name string `json:"name"`
						} `json:"artist_list"`
						PlayDuration int `json:"play_duration"`
						Images       []struct {
							Width int    `json:"width"`
							URL   string `json:"url"`
						} `json:"images"`
						VipFlag int `json:"vip_flag"`
					} `json:"song_info"`
				} `json:"song"`
			} `json:"item_list"`
		} `json:"section_list"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("joox search json error: %w", err)
	}

	var songs []model.Song
	for _, section := range resp.SectionList {
		for _, items := range section.ItemList {
			for _, songItem := range items.Song {
				info := songItem.SongInfo
				if info.ID == "" {
					continue
				}

				var artistNames []string
				for _, ar := range info.ArtistList {
					artistNames = append(artistNames, ar.Name)
				}

				var cover string
				for _, img := range info.Images {
					if img.Width == 300 {
						cover = img.URL
						break
					}
				}
				if cover == "" && len(info.Images) > 0 {
					cover = info.Images[0].URL
				}

				songs = append(songs, model.Song{
					Source:   "joox",
					ID:       info.ID,
					Name:     info.Name,
					Artist:   strings.Join(artistNames, " / "),
					Album:    info.AlbumName,
					Duration: info.PlayDuration,
					Cover:    cover,
					Link:     fmt.Sprintf("https://www.joox.com/hk/single/%s", info.ID),
					Extra: map[string]string{
						"songid": info.ID,
					},
				})
			}
		}
	}
	return songs, nil
}

func (j *Joox) Parse(link string) (*model.Song, error) {
	// 1. 提取 ID
	// 支持格式: https://www.joox.com/hk/single/C+Q0... 或纯 ID
	re := regexp.MustCompile(`joox\.com/.*/single/([^/?#]+)`)
	matches := re.FindStringSubmatch(link)
	var songID string
	if len(matches) >= 2 {
		songID = normalizeJooxID(matches[1])
	} else {
		// 尝试直接匹配 ID (如果是纯 ID 字符串)
		if len(link) > 10 && !strings.Contains(link, "/") {
			songID = link
		} else {
			return nil, errors.New("invalid joox link")
		}
	}

	// 2. 调用核心逻辑获取详情
	return j.fetchSongInfo(songID)
}
