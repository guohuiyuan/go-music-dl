package core

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/guohuiyuan/music-lib/model"
)

type playlistCandidate struct {
	id   string
	link string
}

type playlistIntegrationCase struct {
	source         string
	keyword        string
	fallbackID     string
	fallbackLink   string
	searchOptional bool
}

func TestAlbumFactoriesAndSourceList(t *testing.T) {
	supported := []string{"netease", "qq", "kugou", "kuwo", "migu", "jamendo", "joox", "qianqian", "soda", "apple"}
	for _, source := range supported {
		if fn := GetAlbumSearchFunc(source); fn == nil {
			t.Fatalf("GetAlbumSearchFunc(%q) returned nil", source)
		}
		if fn := GetAlbumDetailFunc(source); fn == nil {
			t.Fatalf("GetAlbumDetailFunc(%q) returned nil", source)
		}
		if fn := GetParseAlbumFunc(source); fn == nil {
			t.Fatalf("GetParseAlbumFunc(%q) returned nil", source)
		}
	}

	if got := GetAlbumSourceNames(); !reflect.DeepEqual(got, supported) {
		t.Fatalf("GetAlbumSourceNames() = %v, want %v", got, supported)
	}
}

func TestPlaylistFactoriesAndSourceList(t *testing.T) {
	supported := []string{"netease", "qq", "kugou", "kuwo", "migu", "jamendo", "joox", "qianqian", "bilibili", "soda", "fivesing", "apple"}
	for _, source := range supported {
		if fn := GetPlaylistSearchFunc(source); fn == nil {
			t.Fatalf("GetPlaylistSearchFunc(%q) returned nil", source)
		}
		if fn := GetPlaylistDetailFunc(source); fn == nil {
			t.Fatalf("GetPlaylistDetailFunc(%q) returned nil", source)
		}
		if fn := GetParsePlaylistFunc(source); fn == nil {
			t.Fatalf("GetParsePlaylistFunc(%q) returned nil", source)
		}
	}

	if got := GetPlaylistSourceNames(); !reflect.DeepEqual(got, supported) {
		t.Fatalf("GetPlaylistSourceNames() = %v, want %v", got, supported)
	}
}

func TestUserPlaylistFactoriesAndSourceList(t *testing.T) {
	supported := []string{"netease", "qq", "kugou", "soda"}
	for _, source := range supported {
		if fn := GetUserPlaylistsFunc(source); fn == nil {
			t.Fatalf("GetUserPlaylistsFunc(%q) returned nil", source)
		}
	}

	if got := GetUserPlaylistSourceNames(); !reflect.DeepEqual(got, supported) {
		t.Fatalf("GetUserPlaylistSourceNames() = %v, want %v", got, supported)
	}
}

func TestJooxPlaylistIntegration(t *testing.T) {
	requireIntegration(t)
	runPlaylistIntegration(t, playlistIntegrationCase{
		source:       "joox",
		keyword:      "Taylor",
		fallbackID:   "YrcoxvVy7I2fJqO2sCzUaA==",
		fallbackLink: "https://www.joox.com/hk/playlist/YrcoxvVy7I2fJqO2sCzUaA==",
	})
}

func TestUpgradedSongParseIntegration(t *testing.T) {
	requireIntegration(t)

	tests := []struct {
		source string
		link   string
	}{
		{source: "qianqian", link: "https://music.91q.com/song/T10038909559"},
		{source: "joox", link: "https://www.joox.com/hk/single/12Q4rvA6Cj+vquLW8B8NDw=="},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			parseFn := GetParseFunc(tt.source)
			if parseFn == nil {
				t.Fatalf("%s parse func is not wired", tt.source)
			}

			var lastErr error
			for i := 0; i < 3; i++ {
				parsedSong, err := parseFn(tt.link)
				if err == nil && parsedSong != nil && parsedSong.ID != "" {
					return
				}
				if err != nil {
					lastErr = err
				}
				time.Sleep(time.Second)
			}
			if lastErr != nil {
				t.Fatalf("%s Parse(%q) failed: %v", tt.source, tt.link, lastErr)
			}
			t.Fatalf("%s Parse(%q) returned invalid song", tt.source, tt.link)
		})
	}
}

func TestUpgradedPlaylistIntegration(t *testing.T) {
	requireIntegration(t)

	tests := []playlistIntegrationCase{
		{
			source:       "migu",
			keyword:      "周杰伦",
			fallbackID:   "228114498",
			fallbackLink: "https://music.migu.cn/v5/#/playlist?playlistId=228114498&playlistType=ordinary",
		},
		{
			source:         "qianqian",
			keyword:        "周杰伦",
			fallbackID:     "309319",
			fallbackLink:   "https://music.91q.com/songlist/309319",
			searchOptional: true,
		},
		{
			source:         "jamendo",
			keyword:        "music",
			fallbackID:     "500608900",
			fallbackLink:   "https://www.jamendo.com/playlist/500608900/indie",
			searchOptional: true,
		},
		{
			source:       "joox",
			keyword:      "Taylor",
			fallbackID:   "YrcoxvVy7I2fJqO2sCzUaA==",
			fallbackLink: "https://www.joox.com/hk/playlist/YrcoxvVy7I2fJqO2sCzUaA==",
		},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			runPlaylistIntegration(t, tt)
		})
	}
}

func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("GO_MUSIC_DL_INTEGRATION") == "" {
		t.Skip("set GO_MUSIC_DL_INTEGRATION=1 to run network integration tests")
	}
}

func runPlaylistIntegration(t *testing.T, tt playlistIntegrationCase) {
	t.Helper()

	searchFn := GetPlaylistSearchFunc(tt.source)
	detailFn := GetPlaylistDetailFunc(tt.source)
	parseFn := GetParsePlaylistFunc(tt.source)
	if searchFn == nil || detailFn == nil || parseFn == nil {
		t.Fatalf("%s playlist funcs are not wired", tt.source)
	}

	candidates := make([]playlistCandidate, 0)
	seenCandidates := make(map[string]struct{})
	addCandidate := func(id, link string) {
		if id == "" {
			return
		}
		if link == "" {
			link = GetOriginalLink(tt.source, id, "playlist")
		}
		key := id + "|" + link
		if _, ok := seenCandidates[key]; ok {
			return
		}
		seenCandidates[key] = struct{}{}
		candidates = append(candidates, playlistCandidate{id: id, link: link})
	}

	searchSucceeded := false
	searchResultCount := 0
	var searchErr error
	for i := 0; i < 3; i++ {
		playlists, err := searchFn(tt.keyword)
		if err == nil {
			searchSucceeded = true
			searchResultCount = len(playlists)
			for _, playlist := range playlists {
				if playlist.ID == "" {
					continue
				}
				addCandidate(playlist.ID, playlist.Link)
			}
			break
		}
		searchErr = err
		time.Sleep(time.Second)
	}

	if !searchSucceeded {
		if !tt.searchOptional {
			t.Fatalf("%s SearchPlaylist(%q) failed: %v", tt.source, tt.keyword, searchErr)
		}
		t.Logf("%s SearchPlaylist(%q) skipped unstable search result: %v", tt.source, tt.keyword, searchErr)
	}
	if searchSucceeded && searchResultCount == 0 {
		if !tt.searchOptional {
			t.Fatalf("%s SearchPlaylist(%q) returned no playlists", tt.source, tt.keyword)
		}
		t.Logf("%s SearchPlaylist(%q) returned no playlists", tt.source, tt.keyword)
	}

	addCandidate(tt.fallbackID, tt.fallbackLink)
	if len(candidates) == 0 {
		t.Fatalf("%s playlist detail candidates are empty", tt.source)
	}

	var lastErr error
	for _, candidate := range candidates {
		err := verifyPlaylistCandidate(tt.source, candidate, detailFn, parseFn)
		if err == nil {
			return
		}
		lastErr = err
		t.Logf("%s playlist candidate %q failed: %v", tt.source, candidate.id, err)
	}
	if lastErr != nil {
		t.Fatalf("%s playlist integration failed: %v", tt.source, lastErr)
	}
	t.Fatalf("%s playlist integration failed", tt.source)
}

func verifyPlaylistCandidate(source string, candidate playlistCandidate, detailFn func(string) ([]model.Song, error), parseFn func(string) (*model.Playlist, []model.Song, error)) error {
	var songs []model.Song
	var err error
	for i := 0; i < 2; i++ {
		songs, err = detailFn(candidate.id)
		if err == nil && len(songs) > 0 {
			break
		}
		if err == nil {
			err = fmt.Errorf("%s GetPlaylistSongs(%q) returned no songs", source, candidate.id)
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		return fmt.Errorf("%s GetPlaylistSongs(%q) failed: %w", source, candidate.id, err)
	}
	if len(songs) == 0 {
		return fmt.Errorf("%s GetPlaylistSongs(%q) returned no songs", source, candidate.id)
	}

	var playlist *model.Playlist
	var parsedSongs []model.Song
	for i := 0; i < 2; i++ {
		playlist, parsedSongs, err = parseFn(candidate.link)
		if err == nil && playlist != nil && playlist.ID != "" && len(parsedSongs) > 0 {
			return nil
		}
		if err == nil {
			err = fmt.Errorf("%s ParsePlaylist(%q) returned invalid playlist", source, candidate.link)
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("%s ParsePlaylist(%q) failed: %w", source, candidate.link, err)
}

func TestGetOriginalLinkSupportsAlbums(t *testing.T) {
	tests := []struct {
		source string
		id     string
		want   string
	}{
		{source: "netease", id: "123", want: "https://music.163.com/#/album?id=123"},
		{source: "qq", id: "abc", want: "https://y.qq.com/n/ryqq/albumDetail/abc"},
		{source: "kugou", id: "456", want: "https://www.kugou.com/album/456.html"},
		{source: "kuwo", id: "789", want: "http://www.kuwo.cn/album_detail/789"},
		{source: "migu", id: "321", want: "https://music.migu.cn/v3/music/album/321"},
		{source: "jamendo", id: "654", want: "https://www.jamendo.com/album/654"},
		{source: "joox", id: "album-id", want: "https://www.joox.com/hk/album/album-id"},
		{source: "qianqian", id: "PS1000000001", want: "https://music.91q.com/album/PS1000000001"},
		{source: "soda", id: "852", want: "https://www.qishui.com/share/album?album_id=852"},
	}

	for _, tt := range tests {
		if got := GetOriginalLink(tt.source, tt.id, "album"); got != tt.want {
			t.Fatalf("GetOriginalLink(%q, %q, album) = %q, want %q", tt.source, tt.id, got, tt.want)
		}
	}
}

func TestGetOriginalLinkSupportsPlaylists(t *testing.T) {
	tests := []struct {
		source string
		id     string
		want   string
	}{
		{source: "netease", id: "123", want: "https://music.163.com/#/playlist?id=123"},
		{source: "qq", id: "abc", want: "https://y.qq.com/n/ryqq/playlist/abc"},
		{source: "kugou", id: "456", want: "https://www.kugou.com/yy/special/single/456.html"},
		{source: "kuwo", id: "789", want: "http://www.kuwo.cn/playlist_detail/789"},
		{source: "migu", id: "321", want: "https://music.migu.cn/v5/#/playlist?playlistId=321&playlistType=ordinary"},
		{source: "jamendo", id: "654", want: "https://www.jamendo.com/playlist/654"},
		{source: "joox", id: "playlist-id", want: "https://www.joox.com/hk/playlist/playlist-id"},
		{source: "qianqian", id: "309319", want: "https://music.91q.com/songlist/309319"},
		{source: "soda", id: "852", want: "https://www.qishui.com/playlist/852"},
		{source: "fivesing", id: "abc123", want: "http://5sing.kugou.com/dj/abc123.html"},
	}

	for _, tt := range tests {
		if got := GetOriginalLink(tt.source, tt.id, "playlist"); got != tt.want {
			t.Fatalf("GetOriginalLink(%q, %q, playlist) = %q, want %q", tt.source, tt.id, got, tt.want)
		}
	}
}

func TestDetectSourceSupportsAlbumCapableNewSources(t *testing.T) {
	tests := []struct {
		link string
		want string
	}{
		{link: "https://music.migu.cn/v3/music/album/123", want: "migu"},
		{link: "https://www.jamendo.com/album/456", want: "jamendo"},
		{link: "https://www.joox.com/hk/album/abc", want: "joox"},
		{link: "https://music.91q.com/album/PS0001", want: "qianqian"},
		{link: "https://www.qishui.com/share/album?album_id=777", want: "soda"},
	}

	for _, tt := range tests {
		if got := DetectSource(tt.link); got != tt.want {
			t.Fatalf("DetectSource(%q) = %q, want %q", tt.link, got, tt.want)
		}
	}
}
