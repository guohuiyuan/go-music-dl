package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestApplyNeteasePrivilegesMarksPreviewTracks(t *testing.T) {
	songs := []model.Song{
		{ID: "41654821", Source: "netease", Extra: map[string]string{"song_id": "41654821"}},
		{ID: "1", Source: "netease"},
		{ID: "2", Source: "qq"},
	}
	applyNeteasePrivileges(songs, []neteasePrivilege{
		{ID: 41654821, Fee: 1, Pl: 0},
		{ID: 1, Fee: 1, Pl: 999000},
	})

	if songs[0].Extra["preview_only"] != "true" || !songs[0].IsVIP {
		t.Fatalf("preview track flags = %#v, VIP=%t", songs[0].Extra, songs[0].IsVIP)
	}
	if songs[0].Extra["song_id"] != "41654821" {
		t.Fatalf("existing extra lost: %#v", songs[0].Extra)
	}
	if songs[1].Extra["preview_only"] != "" || songs[1].IsVIP != true {
		t.Fatalf("playable VIP track flags = %#v, VIP=%t", songs[1].Extra, songs[1].IsVIP)
	}
	if len(songs[2].Extra) != 0 || songs[2].IsVIP {
		t.Fatalf("non-NetEase song was modified: %#v", songs[2])
	}
}

func TestMarkNeteasePreviewSongsFetchesPrivileges(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("ids"); got != "[41654821]" {
			t.Errorf("ids query = %q, want [41654821]", got)
		}
		if got := r.Header.Get("Referer"); got != "https://music.163.com/" {
			t.Errorf("referer = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":[{"id":41654821,"fee":1,"pl":0}]}`))
	}))
	defer server.Close()

	oldAPI := neteasePrivilegeAPI
	neteasePrivilegeAPI = server.URL
	t.Cleanup(func() { neteasePrivilegeAPI = oldAPI })

	songs := []model.Song{{ID: "41654821", Source: "netease"}}
	songs = markNeteasePreviewSongs(songs)
	if strings.TrimSpace(songs[0].Extra["preview_only"]) != "true" {
		t.Fatalf("preview flag = %#v", songs[0].Extra)
	}
}
