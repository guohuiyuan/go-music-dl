package qq

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/lyrics"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"strconv"
	"strings"
)

func GetLyrics(s *model.Song) (string, error) { return defaultQQ.GetLyrics(s) }

// GetLyrics fetches lyrics.
func (q *QQ) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "qq" {
		return "", errors.New("source mismatch")
	}

	songMID := s.ID
	if s.Extra != nil && s.Extra["songmid"] != "" {
		songMID = s.Extra["songmid"]
	}

	songID, _ := strconv.Atoi(s.ID)
	if s.Extra != nil && s.Extra["song_id"] != "" {
		songID, _ = strconv.Atoi(s.Extra["song_id"])
	}
	if songID == 0 {
		if parsed, err := q.fetchSongDetail(songMID); err == nil && parsed != nil {
			if parsed.Extra != nil && parsed.Extra["song_id"] != "" {
				songID, _ = strconv.Atoi(parsed.Extra["song_id"])
			}
			if s.Name == "" {
				s.Name = parsed.Name
			}
			if s.Artist == "" {
				s.Artist = parsed.Artist
			}
			if s.Album == "" {
				s.Album = parsed.Album
			}
			if s.Duration == 0 {
				s.Duration = parsed.Duration
			}
		}
	}
	if songID == 0 {
		return "", errors.New("qq song id not found")
	}

	reqData := map[string]interface{}{
		"comm": map[string]interface{}{
			"ct":          11,
			"cv":          "1003006",
			"v":           "1003006",
			"os_ver":      "15",
			"phonetype":   "24122RKC7C",
			"rom":         "Redmi/miro/miro:15/AE3A.240806.005/OS2.0.105.0.VOMCNXM:user/release-keys",
			"tmeAppID":    "qqmusiclight",
			"nettype":     "NETWORK_WIFI",
			"udid":        "0",
			"uid":         "0",
			"sid":         "",
			"loginUin":    "0",
			"platform":    "yqq.json",
			"needNewCode": 0,
		},
		"request": map[string]interface{}{
			"method": "GetPlayLyricInfo",
			"module": "music.musichallSong.PlayLyricInfo",
			"param": map[string]interface{}{
				"albumName":  base64.StdEncoding.EncodeToString([]byte(s.Album)),
				"crypt":      1,
				"ct":         19,
				"cv":         2111,
				"interval":   s.Duration,
				"lrc_t":      0,
				"qrc":        1,
				"qrc_t":      0,
				"roma":       1,
				"roma_t":     0,
				"singerName": base64.StdEncoding.EncodeToString([]byte(s.Artist)),
				"songID":     songID,
				"songName":   base64.StdEncoding.EncodeToString([]byte(s.Name)),
				"trans":      1,
				"trans_t":    0,
				"type":       0,
			},
		},
	}
	jsonData, _ := json.Marshal(reqData)
	headers := []utils.RequestOption{
		utils.WithHeader("Referer", LyricReferer),
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Content-Type", "application/json"),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post("https://u.y.qq.com/cgi-bin/musicu.fcg", bytes.NewReader(jsonData), headers...)
	if err != nil {
		return "", err
	}

	var resp struct {
		Code    int `json:"code"`
		Request struct {
			Code int `json:"code"`
			Data struct {
				Lyric  string      `json:"lyric"`
				LrcT   interface{} `json:"lrc_t"`
				QrcT   interface{} `json:"qrc_t"`
				Trans  string      `json:"trans"`
				TransT interface{} `json:"trans_t"`
				Roma   string      `json:"roma"`
				RomaT  interface{} `json:"roma_t"`
			} `json:"data"`
		} `json:"request"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("qq lyric json parse error: %w", err)
	}
	if resp.Code != 0 || resp.Request.Code != 0 {
		return "", fmt.Errorf("qq lyric api error code: %d/%d", resp.Code, resp.Request.Code)
	}
	if resp.Request.Data.Lyric == "" {
		return "", errors.New("lyric is empty or not found")
	}

	tags := map[string]string{"ti": s.Name, "ar": s.Artist, "al": s.Album}
	data := lyrics.MultiData{}
	for _, item := range []struct {
		key string
		raw string
	}{
		{"orig", resp.Request.Data.Lyric},
		{"ts", resp.Request.Data.Trans},
		{"roma", resp.Request.Data.Roma},
	} {
		if strings.TrimSpace(item.raw) == "" {
			continue
		}
		decrypted, err := lyrics.DecryptQRCHex(item.raw)
		if err != nil {
			continue
		}
		qrcTags, qrcData := lyrics.ParseQRC(decrypted)
		if item.key == "orig" {
			for k, v := range qrcTags {
				if strings.TrimSpace(tags[k]) == "" {
					tags[k] = v
				}
			}
		}
		data[item.key] = qrcData
	}
	if len(data["orig"]) == 0 {
		return "", errors.New("lyric is empty or qrc decrypt failed")
	}
	return lyrics.ConvertVerbatimLRC(tags, data, lyrics.DefaultDisplayOrder()), nil
}
