package netease

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"strings"
)

func GetDownloadURL(s *model.Song) (string, error) { return defaultNetease.GetDownloadURL(s) }

// GetDownloadURL returns a download URL.
func (n *Netease) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "netease" {
		return "", errors.New("source mismatch")
	}

	songID := s.ID
	if s.Extra != nil && s.Extra["song_id"] != "" {
		songID = s.Extra["song_id"]
	}

	levels := preferredDownloadLevels(s)

	if n.cookie != "" {
		isVip, _ := n.IsVipAccount()
		if isVip {
			if cached, ok := n.getCachedDownloadURL(songID, strings.Join(levels, ",")); ok {
				if cached.ext != "" {
					s.Ext = cached.ext
				}
				return cached.url, nil
			}

			for _, level := range levels {
				if url, ext, err := n.getEAPIDownloadURL(songID, level); err == nil && url != "" {
					s.Ext = ext
					n.setCachedDownloadURL(songID, strings.Join(levels, ","), url, s.Ext)
					return url, nil
				}
			}
		}
	}

	// Fall back to the original weapi route.
	reqData := map[string]interface{}{
		"ids": []string{songID},
		"br":  320000,
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

	body, err := utils.Post(DownloadAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return "", err
	}

	var resp struct {
		Data []struct {
			URL  string `json:"url"`
			Code int    `json:"code"`
			Br   int    `json:"br"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("json parse error: %w", err)
	}
	if len(resp.Data) == 0 || resp.Data[0].URL == "" {
		return "", errors.New("download url not found (might be vip or copyright restricted)")
	}
	n.setCachedDownloadURL(songID, strings.Join(levels, ","), resp.Data[0].URL, s.Ext)
	return resp.Data[0].URL, nil
}
