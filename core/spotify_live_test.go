package core

import (
	"os"
	"testing"
)

func TestSpotifyAnonymousSearchFactoriesLive(t *testing.T) {
	if os.Getenv("SPOTIFY_LIVE_TEST") != "1" {
		t.Skip("set SPOTIFY_LIVE_TEST=1 to run against live Spotify")
	}
	old := CM.Get("spotify")
	CM.SetAll(map[string]string{"spotify": ""})
	t.Cleanup(func() {
		CM.SetAll(map[string]string{"spotify": old})
	})

	songs, err := GetSearchFunc("spotify")("hello")
	if err != nil {
		t.Fatalf("song search error = %v", err)
	}
	if len(songs) == 0 {
		t.Fatal("song search returned no songs")
	}
	t.Logf("song: %s - %s", songs[0].Name, songs[0].Artist)

	albums, err := GetAlbumSearchFunc("spotify")("hello")
	if err != nil {
		t.Fatalf("album search error = %v", err)
	}
	if len(albums) == 0 {
		t.Fatal("album search returned no albums")
	}
	t.Logf("album: %s", albums[0].Name)
	albumDetail, albumSongs, err := GetParseAlbumFunc("spotify")(albums[0].Link)
	if err != nil {
		t.Fatalf("album detail error = %v", err)
	}
	if albumDetail == nil || len(albumSongs) == 0 {
		t.Fatal("album detail returned no tracks")
	}
	t.Logf("album detail: %s (%d tracks)", albumDetail.Name, len(albumSongs))

	playlists, err := GetPlaylistSearchFunc("spotify")("hello")
	if err != nil {
		t.Fatalf("playlist search error = %v", err)
	}
	if len(playlists) == 0 {
		t.Fatal("playlist search returned no playlists")
	}
	t.Logf("playlist: %s", playlists[0].Name)
}
