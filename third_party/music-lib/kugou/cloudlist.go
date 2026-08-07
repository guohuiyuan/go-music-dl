package kugou

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

func (k *Kugou) fetchCloudlistDetail(listID string) (*model.Playlist, []model.Song, error) {
	cookie := parseKugouCookie(k.cookie)
	userID := strings.TrimSpace(cookie["userid"])
	token := strings.TrimSpace(cookie["token"])
	mid := strings.TrimSpace(cookie["KUGOU_API_MID"])
	if listID == "" || userID == "" || userID == "0" || token == "" || mid == "" {
		return nil, nil, fmt.Errorf("kugou cloudlist detail requires listid, userid, token and KUGOU_API_MID")
	}

	dfid := firstNonEmpty(cookie["dfid"], "-")
	clienttime := strconv.FormatInt(time.Now().Unix(), 10)
	data := struct {
		ListID          string `json:"listid"`
		UserID          string `json:"userid"`
		AreaCode        int    `json:"area_code"`
		ShowRelateGoods int    `json:"show_relate_goods"`
		PageSize        int    `json:"pagesize"`
		AllPlatform     int    `json:"allplatform"`
		ShowCover       int    `json:"show_cover"`
		Type            int    `json:"type"`
		Token           string `json:"token"`
		Page            int    `json:"page"`
	}{
		ListID:          listID,
		UserID:          userID,
		AreaCode:        1,
		ShowRelateGoods: 1,
		PageSize:        300,
		AllPlatform:     1,
		ShowCover:       1,
		Type:            0,
		Token:           token,
		Page:            1,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, nil, err
	}
	params := map[string]string{
		"dfid":       dfid,
		"mid":        mid,
		"uuid":       "-",
		"appid":      KugouLiteAppID,
		"clientver":  KugouLiteVer,
		"clienttime": clienttime,
		"token":      token,
		"userid":     userID,
	}
	apiURL := buildKugouAndroidURL("https://gateway.kugou.com/v4/get_list_all_file", params, string(jsonData))
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
		return nil, nil, err
	}

	var resp struct {
		Status    int    `json:"status"`
		ErrorCode int    `json:"error_code"`
		Errcode   int    `json:"errcode"`
		Error     string `json:"error"`
		Data      struct {
			Count interface{} `json:"count"`
			Info  []struct {
				ID            interface{} `json:"ID"`
				Hash          string      `json:"hash"`
				FileHash      string      `json:"FileHash"`
				SQFileHash    string      `json:"SQFileHash"`
				HQFileHash    string      `json:"HQFileHash"`
				ResFileHash   string      `json:"ResFileHash"`
				Name          string      `json:"name"`
				FileName      string      `json:"filename"`
				Timelen       int         `json:"timelen"`
				Size          int64       `json:"size"`
				FileSize      int64       `json:"filesize"`
				SQFileSize    int64       `json:"SQFileSize"`
				HQFileSize    int64       `json:"HQFileSize"`
				ResFileSize   int64       `json:"ResFileSize"`
				Bitrate       int         `json:"bitrate"`
				AlbumID       interface{} `json:"album_id"`
				AlbumIDLegacy interface{} `json:"AlbumID"`
				AlbumAudioID  interface{} `json:"album_audio_id"`
				MixSongID     interface{} `json:"MixSongID"`
				AudioID       interface{} `json:"audio_id"`
				Audioid       interface{} `json:"Audioid"`
				Cover         string      `json:"cover"`
				MVHash        string      `json:"mvhash"`
				Privilege     int         `json:"privilege"`
				RelateGoods   []struct {
					Hash      string `json:"hash"`
					Bitrate   int    `json:"bitrate"`
					Privilege int    `json:"privilege"`
					Size      int64  `json:"size"`
				} `json:"relate_goods"`
				AlbumInfo struct {
					ID   interface{} `json:"id"`
					Name string      `json:"name"`
				} `json:"albuminfo"`
				SingerInfo []struct {
					Name string `json:"name"`
				} `json:"singerinfo"`
				TransParam struct {
					UnionCover     string `json:"union_cover"`
					Ogg320Hash     string `json:"ogg_320_hash"`
					Ogg128Hash     string `json:"ogg_128_hash"`
					Ogg320FileSize int64  `json:"ogg_320_filesize"`
					Ogg128FileSize int64  `json:"ogg_128_filesize"`
				} `json:"trans_param"`
			} `json:"info"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("kugou cloudlist detail json error: %w", err)
	}
	if resp.Status != 1 || resp.ErrorCode != 0 || resp.Errcode != 0 {
		return nil, nil, fmt.Errorf("kugou cloudlist detail api error: status=%d error_code=%d errcode=%d error=%s", resp.Status, resp.ErrorCode, resp.Errcode, resp.Error)
	}

	playlist := &model.Playlist{
		Source:     "kugou",
		ID:         kugouCloudlistID(listID),
		TrackCount: kugouIntFromValue(resp.Data.Count),
	}
	songs := make([]model.Song, 0, len(resp.Data.Info))
	for _, item := range resp.Data.Info {
		baseSize := item.Size
		if baseSize == 0 {
			baseSize = item.FileSize
		}
		fileHash, hqHash, sqHash, size, bitrate := pickSonglistHashes(item.Hash, baseSize, item.Bitrate, item.RelateGoods)
		if isValidHash(item.FileHash) {
			fileHash = item.FileHash
		}
		if isValidHash(item.HQFileHash) {
			hqHash = item.HQFileHash
		}
		if isValidHash(item.SQFileHash) {
			sqHash = item.SQFileHash
		}

		finalHash := firstNonEmpty(sqHash, hqHash, item.ResFileHash, item.TransParam.Ogg320Hash, item.Hash, fileHash, item.TransParam.Ogg128Hash)
		if !isValidHash(finalHash) {
			continue
		}
		switch finalHash {
		case sqHash:
			if item.SQFileSize > 0 {
				size = item.SQFileSize
			}
		case hqHash:
			if item.HQFileSize > 0 {
				size = item.HQFileSize
			}
		case item.ResFileHash:
			if item.ResFileSize > 0 {
				size = item.ResFileSize
			}
		case item.TransParam.Ogg320Hash:
			if item.TransParam.Ogg320FileSize > 0 {
				size = item.TransParam.Ogg320FileSize
			}
		case item.TransParam.Ogg128Hash:
			if item.TransParam.Ogg128FileSize > 0 {
				size = item.TransParam.Ogg128FileSize
			}
		case item.Hash, fileHash:
			if baseSize > 0 {
				size = baseSize
			}
		}

		duration := normalizeKugouDuration(item.Timelen)
		if duration > 0 && size > 0 {
			bitrate = int(size * 8 / 1000 / int64(duration))
		}
		albumID := firstNonEmpty(formatKugouNumericString(item.AlbumID), formatKugouNumericString(item.AlbumIDLegacy), formatKugouNumericString(item.AlbumInfo.ID))
		audioID := firstNonEmpty(formatKugouNumericString(item.AudioID), formatKugouNumericString(item.Audioid))
		albumAudioID := firstNonEmpty(formatKugouNumericString(item.AlbumAudioID), formatKugouNumericString(item.MixSongID), formatKugouNumericString(item.ID), audioID)
		cover := strings.Replace(firstNonEmpty(item.Cover, item.TransParam.UnionCover), "{size}", "240", 1)
		songs = append(songs, model.Song{
			Source:   "kugou",
			ID:       finalHash,
			Name:     pickSonglistSongName(firstNonEmpty(item.Name, item.FileName)),
			Artist:   joinSonglistArtists(item.SingerInfo),
			Album:    item.AlbumInfo.Name,
			AlbumID:  albumID,
			Duration: duration,
			Size:     size,
			Bitrate:  bitrate,
			Cover:    cover,
			Link:     fmt.Sprintf("https://www.kugou.com/song/#hash=%s", finalHash),
			Extra: map[string]string{
				"hash":           finalHash,
				"ogg_320_hash":   item.TransParam.Ogg320Hash,
				"ogg_128_hash":   item.TransParam.Ogg128Hash,
				"sq_hash":        sqHash,
				"file_hash":      fileHash,
				"res_hash":       item.ResFileHash,
				"mv_hash":        item.MVHash,
				"hq_hash":        hqHash,
				"audio_id":       audioID,
				"album_audio_id": albumAudioID,
				"album_id":       albumID,
				"cloud_listid":   listID,
				"privilege":      strconv.Itoa(item.Privilege),
			},
		})
	}
	playlist.TrackCount = len(songs)
	return playlist, songs, nil
}
