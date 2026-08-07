package netease

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/lyrics"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"strings"
)

func GetLyrics(s *model.Song) (string, error) { return defaultNetease.GetLyrics(s) }

// GetLyrics fetches lyrics.
func (n *Netease) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "netease" {
		return "", errors.New("source mismatch")
	}

	songID := s.ID
	if s.Extra != nil && s.Extra["song_id"] != "" {
		songID = s.Extra["song_id"]
	}

	reqData := map[string]interface{}{
		"csrf_token": "",
		"id":         songID,
		"lv":         -1,
		"tv":         -1,
		"rv":         -1,
		"yv":         -1,
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	}

	lyricAPI := "https://music.163.com/weapi/song/lyric"
	body, err := utils.Post(lyricAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return "", err
	}

	var resp struct {
		Code int `json:"code"`
		Lrc  struct {
			Lyric string `json:"lyric"`
		} `json:"lrc"`
		Yrc struct {
			Lyric string `json:"lyric"`
		} `json:"yrc"`
		TLyric struct {
			Lyric string `json:"lyric"`
		} `json:"tlyric"`
		RomaLrc struct {
			Lyric string `json:"lyric"`
		} `json:"romalrc"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("json parse error: %w", err)
	}
	if resp.Code != 200 {
		return "", fmt.Errorf("netease api error code: %d", resp.Code)
	}
	tags := map[string]string{
		"ti": s.Name,
		"ar": s.Artist,
		"al": s.Album,
	}
	data := lyrics.MultiData{}
	if strings.TrimSpace(resp.Yrc.Lyric) != "" {
		data["orig"] = lyrics.ParseYRC(resp.Yrc.Lyric)
	} else if strings.TrimSpace(resp.Lrc.Lyric) != "" {
		lrcTags, lrcData := lyrics.ParseLRC(resp.Lrc.Lyric)
		for k, v := range lrcTags {
			if strings.TrimSpace(tags[k]) == "" {
				tags[k] = v
			}
		}
		data["orig"] = lrcData
	}
	if strings.TrimSpace(resp.TLyric.Lyric) != "" {
		_, data["ts"] = lyrics.ParseLRC(resp.TLyric.Lyric)
	}
	if strings.TrimSpace(resp.RomaLrc.Lyric) != "" {
		_, data["roma"] = lyrics.ParseLRC(resp.RomaLrc.Lyric)
	}
	if len(data["orig"]) == 0 {
		return "", errors.New("lyric is empty or not found")
	}
	return lyrics.ConvertVerbatimLRC(tags, data, lyrics.DefaultDisplayOrder()), nil
}
