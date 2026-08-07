package kuwo

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"strconv"
	"strings"
)

func GetLyrics(s *model.Song) (string, error) { return defaultKuwo.GetLyrics(s) }

// GetLyrics 获取歌词
func (k *Kuwo) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "kuwo" {
		return "", errors.New("source mismatch")
	}

	rid := s.ID
	if s.Extra != nil && s.Extra["rid"] != "" {
		rid = s.Extra["rid"]
	}

	if lrc, err := k.fetchNewLyrics(rid); err == nil && strings.TrimSpace(lrc) != "" {
		return lrc, nil
	}

	params := url.Values{}
	params.Set("musicId", rid)
	params.Set("httpsStatus", "1")

	apiURL := "http://m.kuwo.cn/newh5/singles/songinfoandlrc?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return "", fmt.Errorf("failed to fetch kuwo lyric API: %w", err)
	}

	var resp struct {
		Data struct {
			Lrclist []struct {
				Time      string `json:"time"`
				LineLyric string `json:"lineLyric"`
			} `json:"lrclist"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse kuwo lyric JSON: %w", err)
	}

	if len(resp.Data.Lrclist) == 0 {
		return "", nil
	}

	var sb strings.Builder
	for _, line := range resp.Data.Lrclist {
		secs, _ := strconv.ParseFloat(line.Time, 64)
		m := int(secs) / 60
		s := int(secs) % 60
		ms := int((secs - float64(int(secs))) * 100)
		sb.WriteString(fmt.Sprintf("[%02d:%02d.%02d]%s\n", m, s, ms, line.LineLyric))
	}
	return sb.String(), nil
}
