package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/guohuiyuan/music-lib/model"
)

const (
	sodaShortPlaylistLink = "https://qishui.douyin.com/s/iQJQNPDh/"
	sodaShortTrackLink    = "https://qishui.douyin.com/s/iQJx6Qo4/"
)

func TestDetectSourceSupportsSodaDouyinShortLinks(t *testing.T) {
	for _, link := range []string{sodaShortPlaylistLink, sodaShortTrackLink} {
		if got := DetectSource(link); got != "soda" {
			t.Fatalf("DetectSource(%q) = %q, want %q", link, got, "soda")
		}
	}

	if parseFn := GetParseFunc("soda"); parseFn == nil {
		t.Fatal("GetParseFunc(\"soda\") returned nil")
	}
	if parsePlaylistFn := GetParsePlaylistFunc("soda"); parsePlaylistFn == nil {
		t.Fatal("GetParsePlaylistFunc(\"soda\") returned nil")
	}
}

func TestSodaShortTrackParseIntegration(t *testing.T) {
	requireIntegration(t)

	parseFn := GetParseFunc("soda")
	if parseFn == nil {
		t.Fatal("soda parse func is not wired")
	}

	song, err := parseSongWithRetry(parseFn, sodaShortTrackLink)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("parsed soda short track link: id=%s name=%q artist=%q vip=%v", song.ID, song.Name, song.Artist, song.IsVIP)
}

func TestSodaShortPlaylistParseIntegration(t *testing.T) {
	requireIntegration(t)

	parsePlaylistFn := GetParsePlaylistFunc("soda")
	if parsePlaylistFn == nil {
		t.Fatal("soda playlist parse func is not wired")
	}

	playlist, songs, err := parsePlaylistWithRetry(parsePlaylistFn, sodaShortPlaylistLink)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("parsed soda short playlist link: id=%s name=%q tracks=%d", playlist.ID, playlist.Name, len(songs))
}

func parseSongWithRetry(parseFn func(string) (*model.Song, error), link string) (*model.Song, error) {
	var lastErr error
	for i := 0; i < 3; i++ {
		song, err := parseFn(link)
		if err == nil && song != nil && song.ID != "" && song.Source == "soda" && song.Name != "" {
			return song, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("Parse(%q) returned invalid soda song", link)
		}
		time.Sleep(time.Second)
	}
	return nil, fmt.Errorf("Parse(%q) failed: %w", link, lastErr)
}

func parsePlaylistWithRetry(parseFn func(string) (*model.Playlist, []model.Song, error), link string) (*model.Playlist, []model.Song, error) {
	var lastErr error
	for i := 0; i < 3; i++ {
		playlist, songs, err := parseFn(link)
		if err == nil && playlist != nil && playlist.ID != "" && playlist.Source == "soda" && len(songs) > 0 {
			return playlist, songs, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("ParsePlaylist(%q) returned invalid soda playlist", link)
		}
		time.Sleep(time.Second)
	}
	return nil, nil, fmt.Errorf("ParsePlaylist(%q) failed: %w", link, lastErr)
}
