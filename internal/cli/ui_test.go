package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/guohuiyuan/go-music-dl/core"
)

func TestNextSearchTypeCyclesAllModes(t *testing.T) {
	if got := nextSearchType(searchTypeSong); got != searchTypePlaylist {
		t.Fatalf("nextSearchType(song) = %q, want %q", got, searchTypePlaylist)
	}
	if got := nextSearchType(searchTypePlaylist); got != searchTypeAlbum {
		t.Fatalf("nextSearchType(playlist) = %q, want %q", got, searchTypeAlbum)
	}
	if got := nextSearchType(searchTypeAlbum); got != searchTypeSong {
		t.Fatalf("nextSearchType(album) = %q, want %q", got, searchTypeSong)
	}
}

func TestPlaceholderForSearchType(t *testing.T) {
	tests := []struct {
		name       string
		searchType string
		want       string
	}{
		{name: "song", searchType: searchTypeSong, want: "歌名"},
		{name: "playlist", searchType: searchTypePlaylist, want: "歌单"},
		{name: "album", searchType: searchTypeAlbum, want: "专辑"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := placeholderForSearchType(tt.searchType); !strings.Contains(got, tt.want) {
				t.Fatalf("placeholderForSearchType(%q) = %q, want contains %q", tt.searchType, got, tt.want)
			}
		})
	}
}

func TestDefaultSourcesForSearchType(t *testing.T) {
	wantAlbumSources := core.GetAlbumSourceNames()
	if got := defaultSourcesForSearchType(searchTypeAlbum); !reflect.DeepEqual(got, wantAlbumSources) {
		t.Fatalf("defaultSourcesForSearchType(album) = %v, want %v", got, wantAlbumSources)
	}

	if got := defaultSourcesForSearchType(searchTypePlaylist); len(got) == 0 {
		t.Fatal("defaultSourcesForSearchType(playlist) returned empty sources")
	}

	if got := defaultSourcesForSearchType(searchTypeSong); len(got) == 0 {
		t.Fatal("defaultSourcesForSearchType(song) returned empty sources")
	}
}

func TestAlbumFunctionsAreWiredForSupportedSources(t *testing.T) {
	supported := core.GetAlbumSourceNames()
	for _, source := range supported {
		if fn := getAlbumSearchFunc(source); fn == nil {
			t.Fatalf("getAlbumSearchFunc(%q) returned nil", source)
		}
		if fn := getAlbumDetailFunc(source); fn == nil {
			t.Fatalf("getAlbumDetailFunc(%q) returned nil", source)
		}
		if fn := getParseAlbumFunc(source); fn == nil {
			t.Fatalf("getParseAlbumFunc(%q) returned nil", source)
		}
	}
}
