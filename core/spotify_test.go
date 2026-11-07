package core

import (
	"reflect"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/spotify"
)

func TestSpotifyFactoriesAndSourceLists(t *testing.T) {
	checks := []struct {
		name string
		fn   any
	}{
		{name: "search", fn: GetSearchFunc("spotify")},
		{name: "search album", fn: GetAlbumSearchFunc("spotify")},
		{name: "album songs", fn: GetAlbumDetailFunc("spotify")},
		{name: "parse album", fn: GetParseAlbumFunc("spotify")},
		{name: "search playlist", fn: GetPlaylistSearchFunc("spotify")},
		{name: "playlist songs", fn: GetPlaylistDetailFunc("spotify")},
		{name: "parse playlist", fn: GetParsePlaylistFunc("spotify")},
		{name: "parse song", fn: GetParseFunc("spotify")},
		{name: "download", fn: GetDownloadFunc("spotify")},
		{name: "lyrics", fn: GetLyricFunc("spotify")},
		{name: "recommend", fn: GetRecommendFunc("spotify")},
		{name: "categories", fn: GetPlaylistCategoriesFunc("spotify")},
		{name: "category playlists", fn: GetCategoryPlaylistsFunc("spotify")},
		{name: "top lists", fn: GetTopListsFunc("spotify")},
		{name: "top list songs", fn: GetTopListSongsFunc("spotify")},
		{name: "user playlists", fn: GetUserPlaylistsFunc("spotify")},
	}
	for _, check := range checks {
		if check.fn == nil {
			t.Fatalf("spotify %s factory returned nil", check.name)
		}
	}

	for name, sources := range map[string][]string{
		"all":       GetAllSourceNames(),
		"playlist":  GetPlaylistSourceNames(),
		"album":     GetAlbumSourceNames(),
		"category":  GetPlaylistCategorySourceNames(),
		"top list":  GetTopListSourceNames(),
		"recommend": GetRecommendSourceNames(),
		"user":      GetUserPlaylistSourceNames(),
	} {
		if !containsSource(sources, "spotify") {
			t.Fatalf("%s sources missing spotify: %v", name, sources)
		}
	}
	if containsSource(GetDefaultSourceNames(), "spotify") {
		t.Fatalf("spotify requires credentials and must not be a default search source: %v", GetDefaultSourceNames())
	}
}

func TestSpotifyLinkDetectionAndGeneration(t *testing.T) {
	for _, link := range []string{
		"https://open.spotify.com/track/abc123",
		"https://open.spotify.com/intl-zh/album/album123",
		"spotify:playlist:playlist123",
	} {
		if got := DetectSource(link); got != "spotify" {
			t.Fatalf("DetectSource(%q) = %q, want spotify", link, got)
		}
	}

	tests := []struct {
		id       string
		typeName string
		want     string
	}{
		{id: "track123", typeName: "song", want: "https://open.spotify.com/track/track123"},
		{id: "album123", typeName: "album", want: "https://open.spotify.com/album/album123"},
		{id: "playlist123", typeName: "playlist", want: "https://open.spotify.com/playlist/playlist123"},
		{id: "global-top-50", typeName: "playlist", want: "https://open.spotify.com/playlist/37i9dQZEVXbMDoHDwVN2tF"},
		{id: "jp-top-50", typeName: "playlist", want: "https://open.spotify.com/playlist/37i9dQZEVXbKXQ4mDTEBXq"},
	}
	for _, tt := range tests {
		if got := GetOriginalLink("spotify", tt.id, tt.typeName); got != tt.want {
			t.Fatalf("GetOriginalLink(spotify, %q, %q) = %q, want %q", tt.id, tt.typeName, got, tt.want)
		}
	}

	if got := GetSourceDescription("spotify"); got != "Spotify" {
		t.Fatalf("GetSourceDescription(spotify) = %q", got)
	}
}

func TestSpotifyConfigIsNotForwardedAsCookie(t *testing.T) {
	old := CM.Get("spotify")
	CM.SetAll(map[string]string{"spotify": `{"client_id":"id","client_secret":"secret"}`})
	t.Cleanup(func() {
		CM.SetAll(map[string]string{"spotify": old})
	})

	req, err := BuildSourceRequest("GET", "https://cdn.example/preview.mp3", "spotify", "")
	if err != nil {
		t.Fatalf("BuildSourceRequest() error = %v", err)
	}
	if got := req.Header.Get("Cookie"); got != "" {
		t.Fatalf("Spotify API credentials leaked as Cookie header: %q", got)
	}
	if got := req.Header.Get("User-Agent"); !strings.Contains(got, "Mozilla") {
		t.Fatalf("unexpected user agent: %q", got)
	}
}

func TestSpotifySourceListsHaveNoDuplicates(t *testing.T) {
	lists := [][]string{
		GetAllSourceNames(),
		GetPlaylistSourceNames(),
		GetAlbumSourceNames(),
		GetPlaylistCategorySourceNames(),
		GetTopListSourceNames(),
		GetRecommendSourceNames(),
		GetUserPlaylistSourceNames(),
	}
	for _, list := range lists {
		seen := make(map[string]struct{}, len(list))
		for _, source := range list {
			if _, exists := seen[source]; exists {
				t.Fatalf("duplicate source %q in %v", source, list)
			}
			seen[source] = struct{}{}
		}
	}
}

func TestSpotifySourceOrderIsStable(t *testing.T) {
	want := []string{"spotify"}
	got := filterSource(GetTopListSourceNames(), "spotify")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("spotify top list order = %v", got)
	}
}

func TestSpotifyTopListLinksMatchLibraryDefinitions(t *testing.T) {
	lists, err := spotify.GetTopLists()
	if err != nil {
		t.Fatalf("spotify.GetTopLists() error = %v", err)
	}
	if len(lists) == 0 {
		t.Fatal("spotify.GetTopLists() returned no lists")
	}
	for _, list := range lists {
		want := list.Link
		if strings.TrimSpace(want) == "" {
			want = "https://open.spotify.com/playlist/" + list.Extra["playlist_id"]
		}
		if got := GetOriginalLink("spotify", list.ID, "playlist"); got != want {
			t.Fatalf("GetOriginalLink(spotify, %q) = %q, want %q", list.ID, got, want)
		}
	}
}

func containsSource(sources []string, want string) bool {
	for _, source := range sources {
		if source == want {
			return true
		}
	}
	return false
}

func filterSource(sources []string, want string) []string {
	filtered := make([]string, 0, 1)
	for _, source := range sources {
		if source == want {
			filtered = append(filtered, source)
		}
	}
	return filtered
}
