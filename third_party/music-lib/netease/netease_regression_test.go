package netease

import (
	"errors"
	"testing"
)

const regressionPlaylistLink = "https://music.163.com/#/playlist?id=7507256008"

func TestParsePlaylistRegression(t *testing.T) {
	playlist, songs, err := ParsePlaylist(regressionPlaylistLink)
	if err != nil {
		t.Fatalf("ParsePlaylist failed for %s: %v", regressionPlaylistLink, err)
	}
	if playlist == nil {
		t.Fatalf("ParsePlaylist returned nil playlist for %s", regressionPlaylistLink)
	}
	if playlist.ID != "7507256008" {
		t.Fatalf("ParsePlaylist returned unexpected playlist id: got %q", playlist.ID)
	}
	if len(songs) == 0 {
		t.Fatalf("ParsePlaylist returned no songs for %s", regressionPlaylistLink)
	}
}

func TestParseNeteaseLinkRegression(t *testing.T) {
	tests := []struct {
		name     string
		link     string
		wantKind neteaseLinkKind
		wantID   string
	}{
		{
			name:     "hash playlist link",
			link:     regressionPlaylistLink,
			wantKind: neteaseLinkPlaylist,
			wantID:   "7507256008",
		},
		{
			name:     "direct song link",
			link:     "https://music.163.com/song?id=29732995",
			wantKind: neteaseLinkSong,
			wantID:   "29732995",
		},
		{
			name:     "direct album link",
			link:     "https://music.163.com/album?id=32311",
			wantKind: neteaseLinkAlbum,
			wantID:   "32311",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			kind, id, err := parseNeteaseLink(tc.link)
			if err != nil {
				t.Fatalf("parseNeteaseLink(%q) returned error: %v", tc.link, err)
			}
			if kind != tc.wantKind {
				t.Fatalf("parseNeteaseLink(%q) kind mismatch: got %q want %q", tc.link, kind, tc.wantKind)
			}
			if id != tc.wantID {
				t.Fatalf("parseNeteaseLink(%q) id mismatch: got %q want %q", tc.link, id, tc.wantID)
			}
		})
	}
}

func TestParseWithPlaylistLinkRegression(t *testing.T) {
	song, err := Parse(regressionPlaylistLink)
	if !errors.Is(err, errNeteasePlaylistLink) {
		t.Fatalf("Parse(%q) error mismatch: got %v", regressionPlaylistLink, err)
	}
	if song != nil {
		t.Fatalf("Parse(%q) returned unexpected song: %#v", regressionPlaylistLink, song)
	}
}
