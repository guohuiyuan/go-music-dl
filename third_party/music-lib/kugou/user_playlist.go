package kugou

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

func GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	return defaultKugou.GetUserPlaylists(page, limit)
}

func (k *Kugou) GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	cookie := parseKugouCookie(k.cookie)
	userID := firstNonEmpty(cookie["userid"], cookie["KugooID"])
	if strings.TrimSpace(userID) == "" || userID == "0" {
		return nil, fmt.Errorf("kugou user playlists require userid cookie")
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

	if kugouHasAppCookie(cookie) {
		return k.getUserPlaylistsGateway(cookie, page, limit)
	}

	params := url.Values{}
	params.Set("json", "true")
	params.Set("page", strconv.Itoa(page))
	params.Set("pagesize", strconv.Itoa(limit))
	apiURL := "http://m.kugou.com/plist/index/" + url.PathEscape(userID) + "?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		if strings.TrimSpace(cookie["token"]) == "" || strings.TrimSpace(cookie["KUGOU_API_MID"]) == "" {
			return nil, err
		}
		return k.getUserPlaylistsGateway(cookie, page, limit)
	}
	playlists, parseErr := parseKugouUserPlaylists(body, userID)
	if parseErr != nil {
		if strings.TrimSpace(cookie["token"]) == "" || strings.TrimSpace(cookie["KUGOU_API_MID"]) == "" {
			return nil, parseErr
		}
		return k.getUserPlaylistsGateway(cookie, page, limit)
	}
	return playlists, nil
}

func (k *Kugou) getUserPlaylistsGateway(cookie map[string]string, page, limit int) ([]model.Playlist, error) {
	userID := firstNonEmpty(cookie["userid"], cookie["KugooID"])
	mid := firstNonEmpty(cookie["KUGOU_API_MID"], "-")
	dfid := firstNonEmpty(cookie["dfid"], "-")
	clienttime := strconv.FormatInt(time.Now().Unix(), 10)
	data := struct {
		UserID   string `json:"userid"`
		Token    string `json:"token"`
		TotalVer int    `json:"total_ver"`
		Type     int    `json:"type"`
		Page     int    `json:"page"`
		PageSize int    `json:"pagesize"`
	}{
		UserID:   userID,
		Token:    cookie["token"],
		TotalVer: 979,
		Type:     2,
		Page:     page,
		PageSize: limit,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	params := map[string]string{
		"dfid":       dfid,
		"mid":        mid,
		"uuid":       "-",
		"appid":      KugouLiteAppID,
		"clientver":  KugouLiteVer,
		"clienttime": clienttime,
		"token":      cookie["token"],
		"userid":     userID,
		"plat":       "1",
	}
	apiURL := buildKugouAndroidURL("https://gateway.kugou.com/v7/get_all_list", params, string(jsonData))
	body, err := utils.Post(apiURL, strings.NewReader(string(jsonData)),
		utils.WithHeader("User-Agent", "Android15-1070-11083-46-0-DiscoveryDRADProtocol-wifi"),
		utils.WithHeader("Content-Type", "application/json"),
		utils.WithHeader("x-router", "cloudlist.service.kugou.com"),
		utils.WithHeader("dfid", dfid),
		utils.WithHeader("clienttime", clienttime),
		utils.WithHeader("mid", mid),
		utils.WithHeader("kg-rc", "1"),
		utils.WithHeader("kg-thash", "5d816a0"),
		utils.WithHeader("kg-rec", "1"),
		utils.WithHeader("kg-rf", "B9EDA08A64250DEFFBCADDEE00F8F25F"),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}
	return parseKugouUserPlaylists(body, userID)
}

func kugouHasAppCookie(cookie map[string]string) bool {
	return strings.TrimSpace(cookie["token"]) != "" &&
		strings.TrimSpace(cookie["userid"]) != "" &&
		strings.TrimSpace(cookie["userid"]) != "0" &&
		strings.TrimSpace(cookie["KUGOU_API_MID"]) != ""
}

func kugouCloudlistID(listID string) string {
	listID = strings.TrimSpace(listID)
	if listID == "" || strings.HasPrefix(listID, "cloudlist:") {
		return listID
	}
	return "cloudlist:" + listID
}

func parseKugouCloudlistID(id string) (string, bool) {
	id = strings.TrimSpace(id)
	listID, ok := strings.CutPrefix(id, "cloudlist:")
	if !ok {
		return "", false
	}
	listID = strings.TrimSpace(listID)
	return listID, listID != ""
}

func kugouIntFromValue(value interface{}) int {
	text := formatKugouNumericString(value)
	if text == "" {
		return 0
	}
	n, _ := strconv.Atoi(text)
	return n
}

func parseKugouUserPlaylists(body []byte, userID string) ([]model.Playlist, error) {
	var resp struct {
		Status  int    `json:"status"`
		Errcode int    `json:"errcode"`
		Error   string `json:"error"`
		Data    struct {
			Info []struct {
				ListID             interface{} `json:"listid"`
				SpecialID          int         `json:"specialid"`
				GlobalSpecialID    string      `json:"global_specialid"`
				GlobalCollectionID string      `json:"global_collection_id"`
				SpecialName        string      `json:"specialname"`
				Name               string      `json:"name"`
				ImgURL             string      `json:"imgurl"`
				Pic                string      `json:"pic"`
				Intro              string      `json:"intro"`
				PlayCount          int         `json:"playcount"`
				SongCount          int         `json:"songcount"`
				Count              interface{} `json:"count"`
				CollectCount       int         `json:"collectcount"`
				Username           string      `json:"username"`
				NickName           string      `json:"nickname"`
				ListCreateUsername string      `json:"list_create_username"`
				ListCreateUserID   interface{} `json:"list_create_userid"`
				ListCreateListID   interface{} `json:"list_create_listid"`
				ListCreateGID      string      `json:"list_create_gid"`
				PubTime            string      `json:"publishtime"`
				UpdateTime         interface{} `json:"update_time"`
			} `json:"info"`
			List []struct {
				SpecialID   int    `json:"specialid"`
				SpecialName string `json:"specialname"`
				ImgURL      string `json:"imgurl"`
				Intro       string `json:"intro"`
				PlayCount   int    `json:"playcount"`
				SongCount   int    `json:"songcount"`
				NickName    string `json:"nickname"`
			} `json:"list"`
		} `json:"data"`
		Plist struct {
			List struct {
				Info []struct {
					SpecialID    int    `json:"specialid"`
					SpecialName  string `json:"specialname"`
					ImgURL       string `json:"imgurl"`
					Intro        string `json:"intro"`
					PlayCount    int    `json:"playcount"`
					SongCount    int    `json:"songcount"`
					CollectCount int    `json:"collectcount"`
					NickName     string `json:"nickname"`
					PubTime      string `json:"publishtime"`
				} `json:"info"`
			} `json:"list"`
		} `json:"plist"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kugou user playlist json parse error: %w", err)
	}
	if resp.Status != 0 && resp.Status != 1 {
		return nil, fmt.Errorf("kugou user playlist api error: status=%d errcode=%d error=%s", resp.Status, resp.Errcode, resp.Error)
	}

	playlists := make([]model.Playlist, 0, len(resp.Data.Info)+len(resp.Data.List))
	for _, item := range resp.Plist.List.Info {
		if item.SpecialID == 0 || strings.TrimSpace(item.SpecialName) == "" {
			continue
		}
		playlistID := strconv.Itoa(item.SpecialID)
		playlists = append(playlists, model.Playlist{
			Source:      "kugou",
			ID:          playlistID,
			Name:        item.SpecialName,
			Cover:       strings.Replace(item.ImgURL, "{size}", "240", 1),
			TrackCount:  item.SongCount,
			PlayCount:   item.PlayCount,
			Creator:     firstNonEmpty(item.NickName, userID),
			Description: item.Intro,
			Link:        fmt.Sprintf("https://www.kugou.com/yy/special/single/%s.html", playlistID),
			Extra: map[string]string{
				"user_id":       userID,
				"collect_count": strconv.Itoa(item.CollectCount),
				"publish_time":  item.PubTime,
			},
		})
	}
	for _, item := range resp.Data.Info {
		if listID := formatKugouNumericString(item.ListID); listID != "" && listID != "0" {
			name := firstNonEmpty(item.Name, item.SpecialName)
			if name == "" {
				continue
			}
			trackCount := kugouIntFromValue(item.Count)
			if trackCount == 0 {
				trackCount = item.SongCount
			}
			playlistID := kugouCloudlistID(listID)
			globalCollectionID := strings.TrimSpace(item.GlobalCollectionID)
			link := ""
			if globalCollectionID != "" {
				link = fmt.Sprintf("https://www.kugou.com/songlist/%s/", globalCollectionID)
			}
			playlists = append(playlists, model.Playlist{
				Source:      "kugou",
				ID:          playlistID,
				Name:        name,
				Cover:       strings.Replace(firstNonEmpty(item.Pic, item.ImgURL), "{size}", "240", 1),
				TrackCount:  trackCount,
				PlayCount:   item.PlayCount,
				Creator:     firstNonEmpty(item.ListCreateUsername, item.Username, item.NickName, userID),
				Description: item.Intro,
				Link:        link,
				Extra: map[string]string{
					"user_id":              userID,
					"cloud_listid":         listID,
					"global_collection_id": globalCollectionID,
					"list_create_userid":   formatKugouNumericString(item.ListCreateUserID),
					"list_create_listid":   formatKugouNumericString(item.ListCreateListID),
					"list_create_gid":      item.ListCreateGID,
					"update_time":          formatKugouNumericString(item.UpdateTime),
				},
			})
			continue
		}
		playlistID := ""
		if item.SpecialID > 0 {
			playlistID = strconv.Itoa(item.SpecialID)
		} else {
			playlistID = strings.TrimSpace(item.GlobalSpecialID)
		}
		name := firstNonEmpty(item.SpecialName, item.Name)
		if playlistID == "" || name == "" {
			continue
		}
		playlists = append(playlists, model.Playlist{
			Source:      "kugou",
			ID:          playlistID,
			Name:        name,
			Cover:       strings.Replace(firstNonEmpty(item.ImgURL, item.Pic), "{size}", "240", 1),
			TrackCount:  item.SongCount,
			PlayCount:   item.PlayCount,
			Creator:     firstNonEmpty(item.Username, item.NickName, userID),
			Description: item.Intro,
			Link:        fmt.Sprintf("https://www.kugou.com/yy/special/single/%s.html", playlistID),
			Extra: map[string]string{
				"user_id":          userID,
				"global_specialid": item.GlobalSpecialID,
				"collect_count":    strconv.Itoa(item.CollectCount),
				"publish_time":     item.PubTime,
			},
		})
	}
	for _, item := range resp.Data.List {
		if item.SpecialID == 0 || strings.TrimSpace(item.SpecialName) == "" {
			continue
		}
		playlistID := strconv.Itoa(item.SpecialID)
		playlists = append(playlists, model.Playlist{
			Source:      "kugou",
			ID:          playlistID,
			Name:        item.SpecialName,
			Cover:       strings.Replace(item.ImgURL, "{size}", "240", 1),
			TrackCount:  item.SongCount,
			PlayCount:   item.PlayCount,
			Creator:     firstNonEmpty(item.NickName, userID),
			Description: item.Intro,
			Link:        fmt.Sprintf("https://www.kugou.com/yy/special/single/%s.html", playlistID),
			Extra: map[string]string{
				"user_id": userID,
			},
		})
	}
	return playlists, nil
}
