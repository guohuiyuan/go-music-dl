package kugou

import "testing"

func TestParseKugouUserPlaylistsCloudlist(t *testing.T) {
	body := []byte(`{
		"status":1,
		"error_code":0,
		"data":{
			"info":[{
				"listid":"12345",
				"global_collection_id":"gcid_abc123",
				"name":"Created List",
				"pic":"https://example.com/{size}.jpg",
				"count":"7",
				"list_create_username":"Tester",
				"list_create_userid":"42",
				"list_create_listid":"99",
				"list_create_gid":"gid"
			}]
		}
	}`)

	playlists, err := parseKugouUserPlaylists(body, "42")
	if err != nil {
		t.Fatal(err)
	}
	if len(playlists) != 1 {
		t.Fatalf("len(playlists) = %d, want 1", len(playlists))
	}
	playlist := playlists[0]
	if playlist.ID != "cloudlist:12345" {
		t.Fatalf("playlist.ID = %q, want cloudlist:12345", playlist.ID)
	}
	if playlist.TrackCount != 7 {
		t.Fatalf("playlist.TrackCount = %d, want 7", playlist.TrackCount)
	}
	if playlist.Link != "https://www.kugou.com/songlist/gcid_abc123/" {
		t.Fatalf("playlist.Link = %q", playlist.Link)
	}
	if playlist.Extra["cloud_listid"] != "12345" {
		t.Fatalf("cloud_listid = %q, want 12345", playlist.Extra["cloud_listid"])
	}
}
