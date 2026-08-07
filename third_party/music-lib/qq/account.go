package qq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/guohuiyuan/music-lib/utils"
	"math/rand"
	"time"
)

func (q *QQ) IsVipAccount() (bool, error) {
	if q.isVipCache != nil {
		return *q.isVipCache, nil
	}

	if q.cookie == "" {
		isVip := false
		q.isVipCache = &isVip
		return false, nil
	}

	// Use a random GUID to reduce the chance of rate limiting.
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	guid := fmt.Sprintf("%d", r.Int63n(9000000000)+1000000000)

	// Probe a VIP-only song to detect account capability.
	songMID := "004YZbkL2MNHoY"
	// Prefer M500 here because standard VIP accounts may not have FLAC access.
	filename := fmt.Sprintf("M500%s%s.mp3", songMID, songMID)

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
				"songmid":   []string{songMID},
				"songtype":  []int{0},
				"uin":       "0",
				"loginflag": 1,
				"platform":  "20",
				"filename":  []string{filename},
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
		return false, err
	}

	var result struct {
		Req1 struct {
			Code int `json:"code"`
			Data struct {
				MidUrlInfo []struct {
					Purl string `json:"purl"`
				} `json:"midurlinfo"`
			} `json:"data"`
		} `json:"req_1"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}

	// Cache only when the probe result is conclusive.
	isVip := false
	if len(result.Req1.Data.MidUrlInfo) > 0 && result.Req1.Data.MidUrlInfo[0].Purl != "" {
		isVip = true
	} else if result.Req1.Code != 0 {
		return false, fmt.Errorf("api returned error code: %d", result.Req1.Code)
	}

	q.isVipCache = &isVip
	return isVip, nil
}
