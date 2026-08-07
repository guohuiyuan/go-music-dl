package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/soda"
)

func getSodaUserPlaylistCookie() string {
	for _, key := range []string{"SODA_USER_PLAYLIST_COOKIE", "SODA_COOKIE"} {
		if cookie := strings.TrimSpace(os.Getenv(key)); cookie != "" {
			return cookie
		}
	}

	for _, name := range []string{"汽水个人歌单.txt", "qishui.md"} {
		raw, err := os.ReadFile(filepath.Join("..", name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToLower(line), "cookie:") {
				return strings.TrimSpace(line[len("cookie:"):])
			}
		}
	}
	return ""
}

func TestSodaUserPlaylistsAndDetail(t *testing.T) {
	cookie := getSodaUserPlaylistCookie()
	if cookie == "" {
		t.Skip("SODA_USER_PLAYLIST_COOKIE, SODA_COOKIE, ../汽水个人歌单.txt, or ../qishui.md cookie not set")
	}

	s := soda.New(cookie)
	playlists, err := s.GetUserPlaylists(1, 50)
	if err != nil {
		t.Fatalf("GetUserPlaylists error: %v", err)
	}
	if len(playlists) == 0 {
		t.Fatal("GetUserPlaylists returned no playlists")
	}

	playlist := playlists[0]
	if playlist.ID == "" || playlist.Source != "soda" || playlist.Name == "" {
		t.Fatalf("invalid playlist: %#v", playlist)
	}

	songs, err := s.GetPlaylistSongs(playlist.ID)
	if err != nil {
		t.Fatalf("GetPlaylistSongs(%q) error: %v", playlist.ID, err)
	}
	if len(songs) == 0 {
		t.Fatalf("GetPlaylistSongs(%q) returned no songs", playlist.ID)
	}
	t.Logf("loaded soda user playlist: id=%s name=%q playlists=%d songs=%d", playlist.ID, playlist.Name, len(playlists), len(songs))
}
