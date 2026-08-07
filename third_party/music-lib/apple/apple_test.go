package apple

import (
	"os"
	"strings"
	"testing"
)

func TestExtractAppleID(t *testing.T) {
	const playlistID = "pl.d467987f72384448b2bebe52c0b212d6"

	tests := []struct {
		name      string
		link      string
		mediaType string
		want      string
	}{
		{
			name:      "raw playlist id with dot",
			link:      playlistID,
			mediaType: "playlist",
			want:      playlistID,
		},
		{
			name:      "playlist url",
			link:      "https://music.apple.com/us/playlist/jay-chou-essentials/" + playlistID + "?l=en-US",
			mediaType: "playlist",
			want:      playlistID,
		},
		{
			name:      "raw album id",
			link:      "1887230874",
			mediaType: "album",
			want:      "1887230874",
		},
		{
			name:      "album url",
			link:      "https://music.apple.com/us/album/children-of-the-sun/1887230874",
			mediaType: "album",
			want:      "1887230874",
		},
		{
			name:      "song url from album query",
			link:      "https://music.apple.com/us/album/the-day-it-rained/1887230874?i=1887230879",
			mediaType: "song",
			want:      "1887230879",
		},
		{
			name:      "domain is not a playlist id",
			link:      "music.apple.com",
			mediaType: "playlist",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractAppleID(tt.link, tt.mediaType); got != tt.want {
				t.Fatalf("extractAppleID(%q, %q) = %q, want %q", tt.link, tt.mediaType, got, tt.want)
			}
		})
	}
}

func TestApplePlaylistSearchAndDetailIntegration(t *testing.T) {
	if os.Getenv("MUSIC_LIB_INTEGRATION") == "" {
		t.Skip("set MUSIC_LIB_INTEGRATION=1 to run Apple Music network integration")
	}

	client := New("storefront=us")
	playlists, err := client.SearchPlaylist("\u5468\u6770\u4f26")
	if err != nil {
		t.Fatalf("SearchPlaylist failed: %v", err)
	}
	if len(playlists) == 0 {
		t.Fatal("SearchPlaylist returned no playlists")
	}

	var playlistID string
	var playlistLink string
	for _, playlist := range playlists {
		if strings.HasPrefix(playlist.ID, "pl.") {
			playlistID = playlist.ID
			playlistLink = playlist.Link
			break
		}
	}
	if playlistID == "" {
		t.Fatalf("SearchPlaylist returned no playlist with a pl.* id: %+v", playlists)
	}
	if playlistLink == "" {
		t.Fatal("SearchPlaylist returned playlist without link")
	}

	songs, err := client.GetPlaylistSongs(playlistID)
	if err != nil {
		t.Fatalf("GetPlaylistSongs(%q) failed: %v", playlistID, err)
	}
	if len(songs) == 0 {
		t.Fatalf("GetPlaylistSongs(%q) returned no songs", playlistID)
	}

	playlist, parsedSongs, err := client.ParsePlaylist(playlistLink)
	if err != nil {
		t.Fatalf("ParsePlaylist(%q) failed: %v", playlistLink, err)
	}
	if playlist == nil || playlist.ID != playlistID {
		t.Fatalf("ParsePlaylist returned playlist=%+v, want id %q", playlist, playlistID)
	}
	if len(parsedSongs) == 0 {
		t.Fatalf("ParsePlaylist(%q) returned no songs", playlistLink)
	}
	if playlist.TrackCount == 0 {
		t.Fatalf("ParsePlaylist(%q) returned zero track count", playlistLink)
	}
}
