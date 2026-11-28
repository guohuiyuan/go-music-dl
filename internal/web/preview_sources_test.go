package web

import (
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestMarkPreviewSongsDispatchesBySource(t *testing.T) {
	const source = "preview-test-source"
	old, existed := previewEnrichers[source]
	previewEnrichers[source] = func(songs []model.Song) []model.Song {
		if len(songs) != 1 || songs[0].Source != source {
			t.Fatalf("enricher received unexpected songs: %#v", songs)
		}
		if songs[0].Extra == nil {
			songs[0].Extra = make(map[string]string)
		}
		songs[0].Extra["preview_only"] = "true"
		return songs
	}
	t.Cleanup(func() {
		if existed {
			previewEnrichers[source] = old
			return
		}
		delete(previewEnrichers, source)
	})

	songs := []model.Song{
		{ID: "1", Source: source},
		{ID: "2", Source: "unknown-source"},
	}
	songs = markPreviewSongs(songs)
	if songs[0].Extra["preview_only"] != "true" {
		t.Fatalf("registered source was not enriched: %#v", songs[0])
	}
	if len(songs[1].Extra) != 0 {
		t.Fatalf("unknown source was modified: %#v", songs[1])
	}
}
