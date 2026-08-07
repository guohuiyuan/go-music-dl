package qq

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

func GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	return defaultQQ.GetUserPlaylists(page, limit)
}

const qqFavoriteSongsPlaylistID = "profile:favorites"
const qqProfileDirPlaylistPrefix = "profile:dir:"

func (q *QQ) GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	if strings.TrimSpace(q.cookie) == "" {
		return nil, fmt.Errorf("qq user playlists require cookie")
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
	uin := normalizeQQUIN(q.cookie)
	if uin == "" {
		return nil, fmt.Errorf("qq user playlists require uin cookie")
	}
	playlists := make([]model.Playlist, 0)
	seen := make(map[string]bool)
	addPlaylist := func(playlist model.Playlist) {
		playlist.ID = strings.TrimSpace(playlist.ID)
		playlist.Name = strings.TrimSpace(playlist.Name)
		if playlist.ID == "" || playlist.Name == "" || seen[playlist.ID] {
			return
		}
		seen[playlist.ID] = true
		playlists = append(playlists, playlist)
	}

	if favorite, err := q.fetchFavoriteSongsPlaylistSummary(uin); err == nil && favorite.TrackCount > 0 {
		addPlaylist(favorite)
	}

	params := url.Values{}
	params.Set("hostuin", uin)
	params.Set("sin", strconv.Itoa((page-1)*limit))
	params.Set("size", strconv.Itoa(limit))
	params.Set("format", "json")
	params.Set("inCharset", "utf8")
	params.Set("outCharset", "utf-8")
	apiURL := "https://c.y.qq.com/rsc/fcgi-bin/fcg_user_created_diss?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
		utils.WithHeader("Referer", "https://y.qq.com/"),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			DissList []struct {
				DirID      int64  `json:"dirid"`
				DissID     int64  `json:"dissid"`
				Tid        int64  `json:"tid"`
				DissName   string `json:"diss_name"`
				Title      string `json:"title"`
				DissCover  string `json:"diss_cover"`
				Cover      string `json:"cover"`
				SongCnt    int    `json:"song_cnt"`
				SongNum    int    `json:"song_num"`
				SongCount  int    `json:"song_count"`
				ListenNum  int    `json:"listen_num"`
				VisitNum   int    `json:"visitnum"`
				DissDesc   string `json:"diss_desc"`
				Desc       string `json:"desc"`
				CommitTime string `json:"commit_time"`
			} `json:"disslist"`
			List []struct {
				DissID       string `json:"dissid"`
				DissName     string `json:"dissname"`
				ImgURL       string `json:"imgurl"`
				SongCount    int    `json:"song_count"`
				SongNum      int    `json:"song_num"`
				ListenNum    int    `json:"listennum"`
				Introduction string `json:"introduction"`
			} `json:"list"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq user playlist json parse error: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("qq user playlist api error: %s (code %d)", resp.Message, resp.Code)
	}

	for _, item := range resp.Data.DissList {
		playlistID := ""
		if item.DissID > 0 {
			playlistID = strconv.FormatInt(item.DissID, 10)
		} else if item.Tid > 0 {
			playlistID = strconv.FormatInt(item.Tid, 10)
		} else if item.DirID > 0 {
			playlistID = qqProfileDirPlaylistPrefix + strconv.FormatInt(item.DirID, 10)
		}
		if playlistID == "" {
			continue
		}
		name := firstNonEmptyQQ(item.DissName, item.Title)
		if playlistID == "" || name == "" {
			continue
		}
		trackCount := item.SongCount
		if trackCount == 0 {
			trackCount = item.SongNum
		}
		if trackCount == 0 {
			trackCount = item.SongCnt
		}
		dirID := ""
		if item.DirID > 0 {
			dirID = strconv.FormatInt(item.DirID, 10)
		}
		playCount := item.ListenNum
		if playCount == 0 {
			playCount = item.VisitNum
		}
		addPlaylist(model.Playlist{
			Source:      "qq",
			ID:          playlistID,
			Name:        name,
			Cover:       normalizeQQCover(firstNonEmptyQQ(item.DissCover, item.Cover)),
			TrackCount:  trackCount,
			PlayCount:   playCount,
			Creator:     uin,
			Description: firstNonEmptyQQ(item.DissDesc, item.Desc),
			Link:        qqPlaylistLink(playlistID, firstNonEmptyQQ(qqCookieValue(q.cookie, "euin"), uin), dirID),
			Extra: map[string]string{
				"uin":         uin,
				"dirid":       dirID,
				"commit_time": item.CommitTime,
			},
		})
	}
	for _, item := range resp.Data.List {
		playlistID := strings.TrimSpace(item.DissID)
		name := strings.TrimSpace(item.DissName)
		if playlistID == "" || name == "" {
			continue
		}
		trackCount := item.SongCount
		if trackCount == 0 {
			trackCount = item.SongNum
		}
		addPlaylist(model.Playlist{
			Source:      "qq",
			ID:          playlistID,
			Name:        name,
			Cover:       normalizeQQCover(item.ImgURL),
			TrackCount:  trackCount,
			PlayCount:   item.ListenNum,
			Creator:     uin,
			Description: item.Introduction,
			Link:        fmt.Sprintf("https://y.qq.com/n/ryqq/playlist/%s", playlistID),
			Extra: map[string]string{
				"uin": uin,
			},
		})
	}
	if collected, err := q.fetchProfileOrderPlaylists(uin, page, limit); err == nil {
		for _, playlist := range collected {
			addPlaylist(playlist)
		}
	}
	return playlists, nil
}

func (q *QQ) fetchFavoriteSongsPlaylistSummary(uin string) (model.Playlist, error) {
	total, songs, err := q.fetchProfileOrderSongs(uin, 1, 1)
	if err != nil {
		return model.Playlist{}, err
	}
	cover := ""
	if len(songs) > 0 {
		cover = songs[0].Cover
	}
	return model.Playlist{
		Source:      "qq",
		ID:          qqFavoriteSongsPlaylistID,
		Name:        "我喜欢的歌曲",
		Cover:       cover,
		TrackCount:  total,
		Creator:     uin,
		Description: "QQ音乐我喜欢的歌曲",
		Link:        "https://y.qq.com/n/ryqq/profile",
		Extra: map[string]string{
			"uin":     uin,
			"virtual": "favorite_songs",
		},
	}, nil
}

func (q *QQ) fetchProfileOrderPlaylists(uin string, page, limit int) ([]model.Playlist, error) {
	params := q.profileOrderAssetParams(uin, "3", page, limit)
	apiURL := "https://c.y.qq.com/fav/fcgi-bin/fcg_get_profile_order_asset.fcg?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
		utils.WithHeader("Referer", "https://y.qq.com/"),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			CDList []struct {
				DissID    int64  `json:"dissid"`
				DissName  string `json:"dissname"`
				SongNum   int    `json:"songnum"`
				ListenNum int    `json:"listennum"`
				Logo      string `json:"logo"`
				Nickname  string `json:"nickname"`
				Uin       int64  `json:"uin"`
			} `json:"cdlist"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq profile playlist json parse error: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("qq profile playlist api error: %s (code %d)", resp.Msg, resp.Code)
	}

	playlists := make([]model.Playlist, 0, len(resp.Data.CDList))
	for _, item := range resp.Data.CDList {
		if item.DissID <= 0 || strings.TrimSpace(item.DissName) == "" {
			continue
		}
		playlistID := strconv.FormatInt(item.DissID, 10)
		creator := strings.TrimSpace(item.Nickname)
		if creator == "" && item.Uin > 0 {
			creator = strconv.FormatInt(item.Uin, 10)
		}
		playlists = append(playlists, model.Playlist{
			Source:     "qq",
			ID:         playlistID,
			Name:       item.DissName,
			Cover:      normalizeQQCover(item.Logo),
			TrackCount: item.SongNum,
			PlayCount:  item.ListenNum,
			Creator:    creator,
			Link:       fmt.Sprintf("https://y.qq.com/n/ryqq/playlist/%s", playlistID),
			Extra: map[string]string{
				"uin":       uin,
				"collected": "true",
			},
		})
	}
	return playlists, nil
}

func (q *QQ) fetchProfileOrderSongs(uin string, page, limit int) (int, []model.Song, error) {
	params := q.profileOrderAssetParams(uin, "1", page, limit)
	apiURL := "https://c.y.qq.com/fav/fcgi-bin/fcg_get_profile_order_asset.fcg?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
		utils.WithHeader("Referer", "https://y.qq.com/"),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return 0, nil, err
	}

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			TotalSong int `json:"totalsong"`
			SongList  []struct {
				Data struct {
					SongID    int64  `json:"songid"`
					SongName  string `json:"songname"`
					SongMid   string `json:"songmid"`
					AlbumName string `json:"albumname"`
					AlbumMid  string `json:"albummid"`
					Interval  int    `json:"interval"`
					Size128   int64  `json:"size128"`
					Size320   int64  `json:"size320"`
					SizeFlac  int64  `json:"sizeflac"`
					Singer    []struct {
						Name string `json:"name"`
					} `json:"singer"`
				} `json:"data"`
			} `json:"songlist"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, nil, fmt.Errorf("qq profile songs json parse error: %w", err)
	}
	if resp.Code != 0 {
		return 0, nil, fmt.Errorf("qq profile songs api error: %s (code %d)", resp.Msg, resp.Code)
	}

	songs := make([]model.Song, 0, len(resp.Data.SongList))
	for _, item := range resp.Data.SongList {
		song := qqProfileSongToModel(item.Data.SongID, item.Data.SongName, item.Data.SongMid, item.Data.AlbumName, item.Data.AlbumMid, item.Data.Interval, item.Data.Size128, item.Data.Size320, item.Data.SizeFlac, item.Data.Singer)
		if song.ID != "" && song.Name != "" {
			songs = append(songs, song)
		}
	}
	return resp.Data.TotalSong, songs, nil
}

func (q *QQ) fetchProfileDirPlaylistSongs(dirID string) ([]model.Song, error) {
	dirID = strings.TrimSpace(dirID)
	if dirID == "" {
		return nil, fmt.Errorf("qq profile dir playlist require dirid")
	}
	uin := firstNonEmptyQQ(qqCookieValue(q.cookie, "euin"), qqCookieValue(q.cookie, "wxuin"), normalizeQQUIN(q.cookie))
	if uin == "" {
		return nil, fmt.Errorf("qq profile dir playlist require uin cookie")
	}

	params := url.Values{}
	params.Set("uin", uin)
	params.Set("dirid", dirID)
	params.Set("new", "0")
	params.Set("dirinfo", "1")
	params.Set("miniportal", "1")
	params.Set("fromDir2Diss", "1")
	params.Set("mobile", "1")
	params.Set("from", "0")
	params.Set("to", "500")
	params.Set("format", "json")
	params.Set("g_tk", "5381")
	apiURL := "http://s.plcloud.music.qq.com/fcgi-bin/fcg_musiclist_getinfo.fcg?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148"),
		utils.WithHeader("Referer", "https://y.qq.com/w/myalbum.html"),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}
	body = stripQQJSONPBody(body)

	var resp struct {
		Code           interface{}              `json:"code"`
		Msg            string                   `json:"msg"`
		SongList       []map[string]interface{} `json:"SongList"`
		SongCount      int                      `json:"SongCount"`
		TotalSongNum   int                      `json:"TotalSongNum"`
		Title          string                   `json:"Title"`
		NickName       string                   `json:"NickName"`
		PicURL         string                   `json:"PicUrl"`
		Desc           string                   `json:"Desc"`
		DirID          int64                    `json:"DirID"`
		DissID         int64                    `json:"dissID"`
		DissCreateTime int64                    `json:"dissCreTime"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq profile dir playlist json parse error: %w", err)
	}
	if !qqJSONCodeOK(resp.Code) {
		return nil, fmt.Errorf("qq profile dir playlist api error: %s (code %v)", resp.Msg, resp.Code)
	}

	songs := make([]model.Song, 0, len(resp.SongList))
	for _, item := range resp.SongList {
		song := qqProfileDirSongToModel(item)
		if song.ID != "" && song.Name != "" {
			songs = append(songs, song)
		}
	}
	return songs, nil
}

func (q *QQ) profileOrderAssetParams(uin, reqtype string, page, limit int) url.Values {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	offset := (page - 1) * limit
	params := url.Values{}
	params.Set("format", "json")
	params.Set("inCharset", "utf8")
	params.Set("outCharset", "utf-8")
	params.Set("platform", "yqq.json")
	params.Set("needNewCode", "0")
	params.Set("loginUin", uin)
	params.Set("hostUin", "0")
	params.Set("notice", "0")
	params.Set("g_tk", "5381")
	params.Set("ct", "20")
	params.Set("cid", "205360956")
	params.Set("userid", uin)
	params.Set("reqtype", reqtype)
	params.Set("sin", strconv.Itoa(offset))
	params.Set("ein", strconv.Itoa(offset+limit-1))
	return params
}

func qqProfileSongToModel(songID int64, name, songMID, albumName, albumMID string, interval int, size128, size320, sizeFlac int64, singers []struct {
	Name string `json:"name"`
}) model.Song {
	artistNames := make([]string, 0, len(singers))
	for _, singer := range singers {
		if strings.TrimSpace(singer.Name) != "" {
			artistNames = append(artistNames, strings.TrimSpace(singer.Name))
		}
	}
	fileSize := size128
	bitrate := 128
	if sizeFlac > 0 {
		fileSize = sizeFlac
		if interval > 0 {
			bitrate = int(fileSize * 8 / 1000 / int64(interval))
		} else {
			bitrate = 800
		}
	} else if size320 > 0 {
		fileSize = size320
		bitrate = 320
	}
	coverURL := ""
	if albumMID != "" {
		coverURL = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", albumMID)
	}
	if songMID == "" && songID > 0 {
		songMID = strconv.FormatInt(songID, 10)
	}
	return model.Song{
		Source:   "qq",
		ID:       songMID,
		Name:     name,
		Artist:   strings.Join(artistNames, "、"),
		Album:    albumName,
		Duration: interval,
		Size:     fileSize,
		Bitrate:  bitrate,
		Cover:    coverURL,
		Link:     fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", songMID),
		Extra: map[string]string{
			"songmid": songMID,
		},
	}
}

func qqProfileDirSongToModel(item map[string]interface{}) model.Song {
	songType := qqMapInt(item, "type")
	if data := qqMapString(item, "data"); data != "" && songType%10 >= 2 && songType%10 <= 4 {
		parts := strings.Split(data, "|")
		value := func(index int) string {
			if index >= 0 && index < len(parts) {
				return strings.TrimSpace(parts[index])
			}
			return ""
		}
		songMID := value(0)
		name := value(1)
		albumMID := value(4)
		albumName := value(5)
		interval := int(qqParseInt64(value(7)))
		size320 := qqParseInt64(value(11))
		size128 := qqParseInt64(value(12))
		sizeFlac := qqParseInt64(value(16))
		coverURL := ""
		if albumMID != "" {
			coverURL = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", albumMID)
		}
		fileSize := size128
		bitrate := 128
		if sizeFlac > 0 {
			fileSize = sizeFlac
			if interval > 0 {
				bitrate = int(fileSize * 8 / 1000 / int64(interval))
			} else {
				bitrate = 800
			}
		} else if size320 > 0 {
			fileSize = size320
			bitrate = 320
		}
		return model.Song{
			Source:   "qq",
			ID:       songMID,
			Name:     name,
			Artist:   value(3),
			Album:    albumName,
			Duration: interval,
			Size:     fileSize,
			Bitrate:  bitrate,
			Cover:    coverURL,
			Link:     fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", songMID),
			Extra: map[string]string{
				"songmid": songMID,
			},
		}
	}

	songMID := firstNonEmptyQQ(qqMapString(item, "songmid"), qqMapString(item, "mid"), qqMapString(item, "id"))
	name := firstNonEmptyQQ(qqMapString(item, "songname"), qqMapString(item, "name"))
	albumMID := firstNonEmptyQQ(qqMapString(item, "albummid"), qqMapString(item, "diskid"))
	coverURL := ""
	if albumMID != "" {
		coverURL = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", albumMID)
	}
	return model.Song{
		Source:   "qq",
		ID:       songMID,
		Name:     name,
		Artist:   firstNonEmptyQQ(qqMapString(item, "singername"), qqMapString(item, "singer")),
		Album:    firstNonEmptyQQ(qqMapString(item, "albumname"), qqMapString(item, "diskname")),
		Duration: qqMapInt(item, "playtime"),
		Cover:    coverURL,
		Link:     fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", songMID),
		Extra: map[string]string{
			"songmid": songMID,
		},
	}
}

func stripQQJSONPBody(body []byte) []byte {
	s := strings.TrimSpace(string(body))
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end >= start {
		return []byte(s[start : end+1])
	}
	return body
}

func qqJSONCodeOK(code interface{}) bool {
	switch v := code.(type) {
	case nil:
		return true
	case float64:
		return int(v) == 0
	case string:
		return strings.TrimSpace(v) == "" || strings.TrimSpace(v) == "0"
	default:
		return false
	}
}

func qqMapString(item map[string]interface{}, key string) string {
	switch v := item[key].(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return ""
	}
}

func qqMapInt(item map[string]interface{}, key string) int {
	return int(qqParseInt64(qqMapString(item, key)))
}

func qqParseInt64(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	n, _ := strconv.ParseInt(value, 10, 64)
	return n
}

func qqPlaylistLink(playlistID, uin, dirID string) string {
	playlistID = strings.TrimSpace(playlistID)
	if strings.HasPrefix(playlistID, qqProfileDirPlaylistPrefix) && strings.TrimSpace(dirID) != "" {
		params := url.Values{}
		params.Set("dirid", strings.TrimSpace(dirID))
		if strings.TrimSpace(uin) != "" {
			params.Set("bu", fmt.Sprintf("%X", []byte(strings.TrimSpace(uin))))
		}
		return "https://y.qq.com/w/myalbum.html?" + params.Encode()
	}
	return fmt.Sprintf("https://y.qq.com/n/ryqq/playlist/%s", playlistID)
}

func qqCookieValue(cookie, key string) string {
	for _, part := range strings.Split(cookie, ";") {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 && strings.TrimSpace(kv[0]) == key {
			return strings.TrimSpace(kv[1])
		}
	}
	return ""
}

func normalizeQQUIN(cookie string) string {
	uin := firstNonEmptyQQ(
		qqCookieValue(cookie, "wxuin"),
		qqCookieValue(cookie, "uin"),
		qqCookieValue(cookie, "ptui_loginuin"),
		qqCookieValue(cookie, "luin"),
		qqCookieValue(cookie, "pt2gguin"),
		qqCookieValue(cookie, "superuin"),
		qqCookieValue(cookie, "p_uin"),
		qqCookieValue(cookie, "musicid"),
		qqCookieValue(cookie, "userid"),
	)
	uin = strings.TrimLeft(strings.TrimPrefix(uin, "o"), "0")
	return strings.TrimSpace(uin)
}

func normalizeQQCover(cover string) string {
	cover = strings.TrimSpace(cover)
	if strings.HasPrefix(cover, "//") {
		return "https:" + cover
	}
	if strings.HasPrefix(cover, "http://") {
		return strings.Replace(cover, "http://", "https://", 1)
	}
	return cover
}
