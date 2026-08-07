package soda

import "testing"

func TestSodaExtractTrackIDFromText(t *testing.T) {
	const id = "7304719759323564095"

	cases := []string{
		id,
		"https://www.qishui.com/track/" + id,
		"https://music.douyin.com/qishui/share/track?track_id=" + id + "&auto_play_bgm=1",
		"https://www.douyin.com/qishui/song/" + id,
		`_ROUTER_DATA = {"loaderData":{"track_page":{"track_id":"` + id + `"}}}`,
		"https%3A%2F%2Fmusic.douyin.com%2Fqishui%2Fshare%2Ftrack%3Ftrack_id%3D" + id,
	}

	for _, tc := range cases {
		if got := sodaExtractTrackIDFromText(tc); got != id {
			t.Fatalf("sodaExtractTrackIDFromText(%q) = %q, want %q", tc, got, id)
		}
	}
}

func TestSodaExtractPlaylistIDFromText(t *testing.T) {
	const id = "7291667294287183907"

	cases := []string{
		id,
		"https://www.qishui.com/playlist/" + id,
		"https://music.douyin.com/qishui/share/playlist?playlist_id=" + id + "&auto_play_bgm=1",
		`_ROUTER_DATA = {"loaderData":{"playlist_page":{"playlist_id":"` + id + `"}}}`,
		"https%3A%2F%2Fmusic.douyin.com%2Fqishui%2Fshare%2Fplaylist%3Fplaylist_id%3D" + id,
	}

	for _, tc := range cases {
		if got := sodaExtractPlaylistIDFromText(tc); got != id {
			t.Fatalf("sodaExtractPlaylistIDFromText(%q) = %q, want %q", tc, got, id)
		}
	}
}

func TestSodaBuildPlaylistFromUserItem(t *testing.T) {
	item := sodaUserPlaylistItem{
		ID:          "7444529378593275956",
		Title:       "My Favorite Music",
		PublicTitle: "Tester Favorite Music",
		Type:        1,
		URLCover: sodaImage{
			Urls: []string{"https://p3-luna.douyinpic.com/img/"},
			Uri:  "tos-cn-v-2774c002/cover",
		},
	}
	item.ResourceCnt.TrackCnt = 78
	item.Owner.ID = "109953989288"
	item.Owner.Nickname = "Tester"

	playlist := sodaBuildPlaylistFromUserItem(item, "109953989288", "Tester")
	if playlist.ID != item.ID || playlist.Source != "soda" {
		t.Fatalf("playlist identity mismatch: %#v", playlist)
	}
	if playlist.TrackCount != 78 {
		t.Fatalf("playlist TrackCount = %d, want 78", playlist.TrackCount)
	}
	if playlist.Creator != "Tester" {
		t.Fatalf("playlist Creator = %q, want Tester", playlist.Creator)
	}
	if playlist.Extra["user_id"] != "109953989288" || playlist.Extra["type"] != "1" {
		t.Fatalf("playlist extra mismatch: %#v", playlist.Extra)
	}
	if playlist.Cover == "" || playlist.Link == "" {
		t.Fatalf("playlist should include cover and link: %#v", playlist)
	}
}

func TestSodaLabelInfoIsVIP(t *testing.T) {
	if (sodaLabelInfo{}).IsVIP() {
		t.Fatal("empty label info should not be VIP")
	}

	label := sodaLabelInfo{
		QualityMap: map[string]sodaQualityPolicy{
			"lossless": {
				PlayDetail: &sodaQualityBenefit{NeedVIP: true},
			},
		},
	}
	if !label.IsVIP() {
		t.Fatal("quality map with need_vip should be VIP")
	}

	label = sodaLabelInfo{OnlyVIPDownload: true}
	if !label.IsVIP() {
		t.Fatal("only_vip_download should be VIP")
	}
}

func TestSodaBuildSongFromTrackMarksVIP(t *testing.T) {
	song := sodaBuildSongFromTrack(sodaTrack{
		ID:       "7304719759323564095",
		Name:     "落了白",
		Duration: 180822,
		Artists:  []sodaArtist{{Name: "蒋雪儿Snow.J"}},
		Album: sodaAlbum{
			ID:   "1",
			Name: "落了白",
		},
		BitRates:  []sodaBitRate{{Size: 5882690, Quality: "highest"}},
		LabelInfo: sodaLabelInfo{OnlyVIPDownload: true},
	})

	if !song.IsVIP {
		t.Fatal("song should be marked VIP")
	}
	if song.Extra["is_vip"] != "true" || song.Extra["only_vip_download"] != "true" {
		t.Fatalf("missing VIP extra flags: %#v", song.Extra)
	}
	if song.Duration != 180 {
		t.Fatalf("duration = %d, want 180", song.Duration)
	}
}

func TestSodaDownloadInfoIsPreview(t *testing.T) {
	if !sodaDownloadInfoIsPreview(&DownloadInfo{Duration: 60}, 180) {
		t.Fatal("60-second stream should be treated as preview for a 180-second track")
	}
	if sodaDownloadInfoIsPreview(&DownloadInfo{Duration: 178}, 180) {
		t.Fatal("near full-duration stream should not be treated as preview")
	}
}

func TestSodaBestPlayerInfoPrefersFullDurationThenHighestQuality(t *testing.T) {
	list := []sodaPlayerInfo{
		{MainPlayURL: "preview-lossless", Duration: 60, Quality: "lossless", Format: "flac", Bitrate: 1000000, Size: 12},
		{MainPlayURL: "full-higher", Duration: 180, Quality: "higher", Format: "m4a", Bitrate: 320000, Size: 20},
		{MainPlayURL: "full-lossless", Duration: 180, Quality: "lossless", Format: "flac", Bitrate: 960000, Size: 30},
	}

	best, ok := sodaBestPlayerInfo(list)
	if !ok {
		t.Fatal("expected best player info")
	}
	if best.MainPlayURL != "full-lossless" {
		t.Fatalf("best stream = %q, want full-lossless", best.MainPlayURL)
	}
}

func TestSodaBestPlayerInfoPrefersLosslessOverSpatial(t *testing.T) {
	list := []sodaPlayerInfo{
		{MainPlayURL: "spatial", Duration: 180, Quality: "spatial", Format: "m4a", Bitrate: 324000, Size: 8_000_000},
		{MainPlayURL: "lossless", Duration: 180, Quality: "lossless", Format: "flac", Bitrate: 1650000, Size: 40_000_000},
	}

	best, ok := sodaBestPlayerInfo(list)
	if !ok {
		t.Fatal("expected best player info")
	}
	if best.MainPlayURL != "lossless" {
		t.Fatalf("best stream = %q, want lossless", best.MainPlayURL)
	}
}

func TestSodaBestPlayerInfoPrefersRealHiResOverLossless(t *testing.T) {
	list := []sodaPlayerInfo{
		{MainPlayURL: "lossless", Duration: 180, Quality: "lossless", Format: "flac", Bitrate: 960000, Size: 21_000_000},
		{MainPlayURL: "hires", Duration: 180, Quality: "hi_res", Format: "flac", Bitrate: 2400000, Size: 58_000_000},
	}

	best, ok := sodaBestPlayerInfo(list)
	if !ok {
		t.Fatal("expected best player info")
	}
	if best.MainPlayURL != "hires" {
		t.Fatalf("best stream = %q, want hires", best.MainPlayURL)
	}
}

func TestSodaBuildSongFromTrackUsesHighestAudioInfoQuality(t *testing.T) {
	song := sodaBuildSongFromTrack(sodaTrack{
		ID:       "1",
		Name:     "test",
		Duration: 180000,
		AudioInfo: sodaTrackAudioInfo{PlayInfoList: []sodaTrackPlayInfo{
			{MainPlayURL: "higher", Quality: "higher", Format: "m4a", Bitrate: 320000, Size: 9},
			{MainPlayURL: "lossless", Quality: "lossless", Format: "flac", Bitrate: 960000, Size: 8},
		}},
	})

	if song.URL != "lossless" {
		t.Fatalf("song URL = %q, want lossless", song.URL)
	}
	if song.Extra["quality"] != "lossless" {
		t.Fatalf("song quality extra = %q, want lossless", song.Extra["quality"])
	}
}

func TestSodaBestFromVideoModelUsesLosslessAndSpadeAuth(t *testing.T) {
	info, ok := sodaBestFromVideoModel([]byte(`{
		"encrypt_info": {"spade_a": "auth-token"},
		"track": {
			"play_info_list": [
				{"main_play_url": "standard-url", "quality": "standard", "format": "m4a", "bitrate": 128000, "size": 4000000, "duration": 210},
				{"main_play_url": "lossless-url", "quality": "lossless", "format": "m4a", "bitrate": 1200000, "size": 32000000, "duration": 210}
			]
		}
	}`))
	if !ok {
		t.Fatal("expected video_model stream")
	}
	if info.URL != "lossless-url" {
		t.Fatalf("URL = %q, want lossless-url", info.URL)
	}
	if info.PlayAuth != "auth-token" {
		t.Fatalf("PlayAuth = %q, want auth-token", info.PlayAuth)
	}
	if info.Quality != "lossless" {
		t.Fatalf("Quality = %q, want lossless", info.Quality)
	}
}
