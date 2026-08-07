package qq

import "testing"

func TestExtractQQPlaylistIDSupportsKnownURLShapes(t *testing.T) {
	tests := []struct {
		name string
		link string
		want string
	}{
		{
			name: "modern ryqq playlist route",
			link: "https://y.qq.com/n/ryqq/playlist/8825279434",
			want: "8825279434",
		},
		{
			name: "legacy details playlist page with id query",
			link: "https://i2.y.qq.com/n3/other/pages/details/playlist.html?id=734191243",
			want: "734191243",
		},
		{
			name: "legacy playlist page with disstid query",
			link: "https://i.y.qq.com/n3/other/pages/details/playlist.html?disstid=734191243&foo=bar",
			want: "734191243",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := extractQQPlaylistID(tt.link)
			if !ok {
				t.Fatalf("extractQQPlaylistID(%q) returned ok=false", tt.link)
			}
			if got != tt.want {
				t.Fatalf("extractQQPlaylistID(%q) = %q, want %q", tt.link, got, tt.want)
			}
		})
	}
}

func TestExtractQQPlaylistIDRejectsInvalidValues(t *testing.T) {
	tests := []string{
		"",
		"https://y.qq.com/n/ryqq/songDetail/0039MnYb0qxYhV",
		"https://i2.y.qq.com/n3/other/pages/details/playlist.html?id=abc",
		"https://i2.y.qq.com/n3/other/pages/details/album.html?id=734191243",
	}

	for _, link := range tests {
		if got, ok := extractQQPlaylistID(link); ok {
			t.Fatalf("extractQQPlaylistID(%q) = %q, true; want false", link, got)
		}
	}
}
