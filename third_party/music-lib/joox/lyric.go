package joox

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"strings"
)

func GetLyrics(s *model.Song) (string, error) { return defaultJoox.GetLyrics(s) }

// GetLyrics 获取歌词
func (j *Joox) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "joox" {
		return "", errors.New("source mismatch")
	}

	songID := s.ID
	if s.Extra != nil && s.Extra["songid"] != "" {
		songID = s.Extra["songid"]
	}

	params := url.Values{}
	params.Set("musicid", songID)
	params.Set("country", "sg")
	params.Set("lang", "zh_cn")
	apiURL := "https://api.joox.com/web-fcgi-bin/web_lyric?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return "", err
	}

	bodyStr := string(body)
	if idx := strings.Index(bodyStr, "MusicJsonCallback("); idx >= 0 {
		bodyStr = strings.TrimPrefix(bodyStr[idx:], "MusicJsonCallback(")
		bodyStr = strings.TrimSuffix(bodyStr, ")")
	}

	var resp struct {
		Lyric string `json:"lyric"`
	}
	if err := json.Unmarshal([]byte(bodyStr), &resp); err != nil {
		return "", fmt.Errorf("joox lyric json parse error: %w", err)
	}
	if resp.Lyric == "" {
		return "", errors.New("lyric not found or empty")
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(resp.Lyric)
	if err != nil {
		return "", fmt.Errorf("base64 decode error: %w", err)
	}

	return string(decodedBytes), nil
}
