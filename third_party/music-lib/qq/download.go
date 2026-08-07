package qq

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"math/rand"
	"time"
)

func GetDownloadURL(s *model.Song) (string, error) { return defaultQQ.GetDownloadURL(s) }

// GetDownloadURL returns a download URL.
func (q *QQ) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "qq" {
		return "", errors.New("source mismatch")
	}

	songMID := s.ID
	if s.Extra != nil && s.Extra["songmid"] != "" {
		songMID = s.Extra["songmid"]
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	guid := fmt.Sprintf("%d", r.Int63n(9000000000)+1000000000)

	// Request qualities from best to worst and use the first successful one.
	var prefixes []string
	var exts []string

	isVip, _ := q.IsVipAccount()
	if isVip {
		prefixes = []string{"AI00", "Q001", "Q000", "F000", "O801", "M800", "M500"} // Master, Atmos5.1, Atmos2.0, FLAC, 640k, 320k, 128k
		exts = []string{"flac", "flac", "flac", "flac", "ogg", "mp3", "mp3"}
	} else {
		prefixes = []string{"M800", "M500"} // Non-VIPs typically only reach 128kbps natively unless the track is free 320k
		exts = []string{"mp3", "mp3"}
	}

	var filenames []string
	var songmids []string
	var songtypes []int

	for i := range prefixes {
		filename := fmt.Sprintf("%s%s%s.%s", prefixes[i], songMID, songMID, exts[i])
		filenames = append(filenames, filename)
		songmids = append(songmids, songMID)
		songtypes = append(songtypes, 0)
	}

	reqData := map[string]interface{}{
		"comm": map[string]interface{}{
			"cv":          4747474,
			"ct":          24,
			"format":      "json",
			"inCharset":   "utf-8",
			"outCharset":  "utf-8",
			"notice":      0,
			"platform":    "yqq.json",
			"needNewCode": 1,
			"uin":         0,
		},
		"req_1": map[string]interface{}{
			"module": "music.vkey.GetVkey",
			"method": "UrlGetVkey",
			"param": map[string]interface{}{
				"guid":      guid,
				"songmid":   songmids,
				"songtype":  songtypes,
				"uin":       "0",
				"loginflag": 1,
				"platform":  "20",
				"filename":  filenames,
			},
		},
	}

	jsonData, _ := json.Marshal(reqData)
	headers := []utils.RequestOption{
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", DownloadReferer),
		utils.WithHeader("Content-Type", "application/json"),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post("https://u.y.qq.com/cgi-bin/musicu.fcg", bytes.NewReader(jsonData), headers...)
	if err != nil {
		return "", err
	}

	var result struct {
		Req1 struct {
			Data struct {
				MidUrlInfo []struct {
					Filename string `json:"filename"`
					Purl     string `json:"purl"`
				} `json:"midurlinfo"`
			} `json:"data"`
		} `json:"req_1"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("qq geturl json parse error: %w", err)
	}

	// Because we passed the filenames cleanly prioritized down from best to worst, the mapped return array technically aligns 1:1.
	// We'll iterate the initial array order we asked for and grab the first `Filename` that successfully gave a `Purl`.
	for _, expectedFilename := range filenames {
		for _, info := range result.Req1.Data.MidUrlInfo {
			if info.Filename == expectedFilename && info.Purl != "" {
				return "https://ws.stream.qqmusic.qq.com/" + info.Purl, nil
			}
		}
	}

	return "", errors.New("no valid download url found or vip required")
}
