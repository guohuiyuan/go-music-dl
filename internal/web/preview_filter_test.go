package web

import (
	"testing"

	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

func TestFilterSongsForDisplayRemovesPreviewMetadata(t *testing.T) {
	songs := []model.Song{
		{ID: "short", Duration: 30},
		{ID: "sixty", Duration: 60},
		{ID: "spotify", Duration: 210, Extra: map[string]string{"preview_only": "true"}},
		{
			ID:       "soda-vip-playback",
			Duration: 145,
			IsVIP:    true,
			Extra: map[string]string{
				"is_vip":             "true",
				"only_vip_playable":  "true",
				"only_vip_download":  "true",
				"vip_play_qualities": "lossless",
			},
		},
		{
			ID:       "vip-download-only",
			Duration: 180,
			IsVIP:    true,
			Extra:    map[string]string{"only_vip_download": "true"},
		},
		{ID: "unknown-duration", Duration: 0},
		{ID: "full", Duration: 181},
	}

	got := filterSongsForDisplay(songs, core.WebSettings{FilterShortTracks: true})
	want := []string{"vip-download-only", "unknown-duration", "full"}
	if len(got) != len(want) {
		t.Fatalf("filtered songs = %#v, want IDs %v", got, want)
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("filtered song %d = %q, want %q", i, got[i].ID, id)
		}
	}
}

func TestFilterSongsForDisplayLeavesSongsWhenDisabled(t *testing.T) {
	songs := []model.Song{
		{ID: "short", Duration: 30},
		{ID: "soda-vip-playback", Duration: 145, Extra: map[string]string{"only_vip_playable": "true"}},
	}

	got := filterSongsForDisplay(songs, core.WebSettings{FilterShortTracks: false})
	if len(got) != len(songs) {
		t.Fatalf("filtered songs = %#v, want both songs preserved", got)
	}
}
