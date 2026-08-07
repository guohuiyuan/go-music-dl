package netease

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

func (n *Netease) GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	if strings.TrimSpace(n.cookie) == "" {
		return nil, fmt.Errorf("netease user playlists require cookie")
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	accountReq := map[string]interface{}{
		"csrf_token": "",
	}
	accountJSON, _ := json.Marshal(accountReq)
	accountParams, accountEncSecKey := EncryptWeApi(string(accountJSON))
	accountForm := url.Values{}
	accountForm.Set("params", accountParams)
	accountForm.Set("encSecKey", accountEncSecKey)
	accountBody, err := utils.Post(UserAccountAPI, strings.NewReader(accountForm.Encode()),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}
	var accountResp struct {
		Code    int `json:"code"`
		Profile struct {
			UserID   int64  `json:"userId"`
			Nickname string `json:"nickname"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(accountBody, &accountResp); err != nil {
		return nil, fmt.Errorf("netease account json parse error: %w", err)
	}
	if accountResp.Code != 200 || accountResp.Profile.UserID == 0 {
		return nil, fmt.Errorf("netease account api error code: %d", accountResp.Code)
	}

	reqData := map[string]interface{}{
		"uid":          accountResp.Profile.UserID,
		"limit":        limit,
		"offset":       (page - 1) * limit,
		"includeVideo": true,
		"csrf_token":   "",
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)
	body, err := utils.Post(UserPlaylistAPI, strings.NewReader(form.Encode()),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code     int `json:"code"`
		Playlist []struct {
			ID          int64   `json:"id"`
			Name        string  `json:"name"`
			CoverImgURL string  `json:"coverImgUrl"`
			TrackCount  int     `json:"trackCount"`
			PlayCount   float64 `json:"playCount"`
			Description string  `json:"description"`
			Subscribed  bool    `json:"subscribed"`
			Creator     struct {
				Nickname string `json:"nickname"`
			} `json:"creator"`
		} `json:"playlist"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease user playlist json parse error: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("netease user playlist api error code: %d", resp.Code)
	}

	playlists := make([]model.Playlist, 0, len(resp.Playlist))
	for _, item := range resp.Playlist {
		playlistID := strconv.FormatInt(item.ID, 10)
		creator := strings.TrimSpace(item.Creator.Nickname)
		if creator == "" {
			creator = accountResp.Profile.Nickname
		}
		playlists = append(playlists, model.Playlist{
			Source:      "netease",
			ID:          playlistID,
			Name:        item.Name,
			Cover:       item.CoverImgURL,
			TrackCount:  item.TrackCount,
			PlayCount:   int(item.PlayCount),
			Creator:     creator,
			Description: item.Description,
			Link:        fmt.Sprintf("https://music.163.com/#/playlist?id=%s", playlistID),
			Extra: map[string]string{
				"user_id":    strconv.FormatInt(accountResp.Profile.UserID, 10),
				"subscribed": strconv.FormatBool(item.Subscribed),
			},
		})
	}
	return playlists, nil
}
