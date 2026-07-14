package web

import (
	"reflect"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestSplitArtistTokens(t *testing.T) {
	tests := []struct {
		name   string
		artist string
		want   []string
	}{
		{
			name:   "single artist kept",
			artist: "\u5468\u6770\u4f26",
			want:   []string{"\u5468\u6770\u4f26"},
		},
		{
			name:   "east asian slash split",
			artist: "\u5468\u6770\u4f26/\u6768\u745e\u4ee3",
			want:   []string{"\u5468\u6770\u4f26", "\u6768\u745e\u4ee3"},
		},
		{
			name:   "english feat split",
			artist: "Taylor Swift feat. Ed Sheeran",
			want:   []string{"Taylor Swift", "Ed Sheeran"},
		},
		{
			name:   "duplicate artist removed",
			artist: "\u5468\u6770\u4f26\u3001\u5468\u6770\u4f26\u3001\u6768\u745e\u4ee3",
			want:   []string{"\u5468\u6770\u4f26", "\u6768\u745e\u4ee3"},
		},
		{
			name:   "trim dangling separators",
			artist: "\u5468\u6770\u4f26-\u3001Asasblue",
			want:   []string{"\u5468\u6770\u4f26", "Asasblue"},
		},
		{
			name:   "band name with slash preserved",
			artist: "AC/DC",
			want:   []string{"AC/DC"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitArtistTokens(tt.artist); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("splitArtistTokens(%q) = %v, want %v", tt.artist, got, tt.want)
			}
		})
	}
}

func TestFilterSongsByExactArtist(t *testing.T) {
	songs := []model.Song{
		{Name: "Song A", Artist: "\u5468\u6770\u4f26/\u6768\u745e\u4ee3"},
		{Name: "Song B", Artist: "\u5468\u6770\u4f26"},
		{Name: "Song C", Artist: "\u5f20\u5b66\u53cb"},
		{Name: "Song D", Artist: "AC/DC"},
	}

	got := filterSongsByExactArtist(songs, " \u5468\u6770\u4f26 ")
	want := []model.Song{
		{Name: "Song A", Artist: "\u5468\u6770\u4f26/\u6768\u745e\u4ee3"},
		{Name: "Song B", Artist: "\u5468\u6770\u4f26"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filterSongsByExactArtist returned %v, want %v", got, want)
	}

	got = filterSongsByExactArtist(songs, "ac/dc")
	want = []model.Song{
		{Name: "Song D", Artist: "AC/DC"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filterSongsByExactArtist(ac/dc) returned %v, want %v", got, want)
	}
}

func TestSongAlbumID(t *testing.T) {
	tests := []struct {
		name string
		song model.Song
		want string
	}{
		{
			name: "prefer explicit album id",
			song: model.Song{AlbumID: "123", Extra: map[string]string{"album_id": "456"}},
			want: "123",
		},
		{
			name: "fallback to extra album id",
			song: model.Song{Extra: map[string]string{"album_id": "456"}},
			want: "456",
		},
		{
			name: "empty when missing",
			song: model.Song{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := songAlbumID(tt.song); got != tt.want {
				t.Fatalf("songAlbumID(%+v) = %q, want %q", tt.song, got, tt.want)
			}
		})
	}
}

func TestPickBestAlbumMatch(t *testing.T) {
	albums := []model.Playlist{
		{ID: "1", Name: "\u7a3b\u9999", Creator: "\u5176\u4ed6\u6b4c\u624b"},
		{ID: "2", Name: "\u7a3b\u9999", Creator: "\u5468\u6770\u4f26"},
		{ID: "3", Name: "\u6211\u5f88\u5fd9", Creator: "\u5468\u6770\u4f26"},
	}

	got := pickBestAlbumMatch("\u7a3b\u9999", "\u5468\u6770\u4f26-\u3001Asasblue", albums)
	if got == nil {
		t.Fatal("pickBestAlbumMatch returned nil")
	}
	if got.ID != "2" {
		t.Fatalf("pickBestAlbumMatch selected %q, want %q", got.ID, "2")
	}
}
