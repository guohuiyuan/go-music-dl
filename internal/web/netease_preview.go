package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

const neteasePrivilegeBatchSize = 100

var neteasePrivilegeAPI = "https://music.163.com/api/song/enhance/privilege"

type neteasePrivilege struct {
	ID  int64 `json:"id"`
	Fee int   `json:"fee"`
	Pl  int   `json:"pl"`
}

// markNeteasePreviewSongs 在 go-music-dl 侧补齐网易云歌曲的播放权限标记。
// 歌单/搜索接口返回的 dt 是完整时长，试听曲的元数据仍然是完整时长，
// 只有 privileges.pl 能反映当前 Cookie 是否只能播放试听片段。
func markNeteasePreviewSongs(songs []model.Song) []model.Song {
	ids := make([]string, 0, len(songs))
	seen := make(map[string]struct{}, len(songs))
	for _, song := range songs {
		if song.Source != "netease" {
			continue
		}
		id := strings.TrimSpace(song.ID)
		if _, err := strconv.ParseInt(id, 10, 64); err != nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return songs
	}

	cookie := core.CM.Get("netease")
	for start := 0; start < len(ids); start += neteasePrivilegeBatchSize {
		end := start + neteasePrivilegeBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		privileges, err := fetchNeteasePrivileges(ids[start:end], cookie)
		if err != nil {
			// 权限接口失败时保留原始列表，避免误删歌曲。
			continue
		}
		applyNeteasePrivileges(songs, privileges)
	}
	return songs
}

func fetchNeteasePrivileges(ids []string, cookie string) ([]neteasePrivilege, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query := url.Values{}
	query.Set("ids", "["+strings.Join(ids, ",")+"]")
	requestURL := neteasePrivilegeAPI + "?" + query.Encode()
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", core.UA_Common)
	req.Header.Set("Referer", "https://music.163.com/")
	if strings.TrimSpace(cookie) != "" {
		req.Header.Set("Cookie", cookie)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Hostname() != "music.163.com" {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("netease privilege status: %d", resp.StatusCode)
	}

	var payload struct {
		Code int                `json:"code"`
		Data []neteasePrivilege `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Code != 0 && payload.Code != http.StatusOK {
		return nil, fmt.Errorf("netease privilege code: %d", payload.Code)
	}
	return payload.Data, nil
}

func applyNeteasePrivileges(songs []model.Song, privileges []neteasePrivilege) {
	if len(songs) == 0 || len(privileges) == 0 {
		return
	}
	privilegeByID := make(map[int64]neteasePrivilege, len(privileges))
	for _, privilege := range privileges {
		privilegeByID[privilege.ID] = privilege
	}
	for i := range songs {
		if songs[i].Source != "netease" {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimSpace(songs[i].ID), 10, 64)
		if err != nil {
			continue
		}
		privilege, ok := privilegeByID[id]
		if !ok {
			continue
		}
		if songs[i].Extra == nil {
			songs[i].Extra = make(map[string]string)
		}
		songs[i].Extra["fee"] = strconv.Itoa(privilege.Fee)
		songs[i].Extra["pl"] = strconv.Itoa(privilege.Pl)
		songs[i].IsVIP = privilege.Fee > 0
		if privilege.Pl <= 0 {
			songs[i].Extra["preview_only"] = "true"
		}
	}
}
