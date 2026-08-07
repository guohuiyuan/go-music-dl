package qianqian

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
)

func GetLyrics(s *model.Song) (string, error) { return defaultQianqian.GetLyrics(s) }

// GetLyrics 获取歌词
func (q *Qianqian) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "qianqian" {
		return "", errors.New("source mismatch")
	}

	tsid := s.ID
	if s.Extra != nil && s.Extra["tsid"] != "" {
		tsid = s.Extra["tsid"]
	}

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
		return "", err
	}

	var resp struct {
		Data []struct {
			Lyric string `json:"lyric"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("qianqian song info parse error: %w", err)
	}
	if len(resp.Data) == 0 || resp.Data[0].Lyric == "" {
		return "", errors.New("lyric url not found")
	}

	lyricURL := resp.Data[0].Lyric
	lrcBody, err := utils.Get(lyricURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", q.cookie),
	)
	if err != nil {
		return "", fmt.Errorf("download lyric failed: %w", err)
	}
	return string(lrcBody), nil
}
