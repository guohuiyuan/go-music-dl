package qianqian

import "testing"

func TestSearchAlbumSpecialChars(t *testing.T) {
	albums, err := SearchAlbum("SSR Beats Vol.10: 新春特辑")
	if err != nil {
		t.Fatalf("SearchAlbum failed: %v", err)
	}
	if len(albums) == 0 {
		t.Fatal("SearchAlbum returned no albums")
	}

	for _, album := range albums {
		if album.Name == "SSR Beats Vol.10: 新春特辑" && album.ID != "" {
			return
		}
	}

	t.Fatalf("target album not found in %d results", len(albums))
}

func TestSearchCarriesAlbumID(t *testing.T) {
	songs, err := Search("邓紫棋")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(songs) == 0 {
		t.Fatal("Search returned no songs")
	}

	for _, song := range songs {
		if song.Album == "SSR Beats Vol.10: 新春特辑" {
			if song.AlbumID == "" {
				t.Fatal("song album id is empty")
			}
			if song.Extra == nil || song.Extra["album_id"] == "" {
				t.Fatal("song extra album_id is empty")
			}
			return
		}
	}

	t.Fatalf("target album not found in %d songs", len(songs))
}
