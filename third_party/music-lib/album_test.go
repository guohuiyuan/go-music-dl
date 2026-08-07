package main

import (
	"testing"

	"github.com/guohuiyuan/music-lib/apple"
	"github.com/guohuiyuan/music-lib/jamendo"
	"github.com/guohuiyuan/music-lib/joox"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/kuwo"
	"github.com/guohuiyuan/music-lib/migu"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qianqian"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/soda"
)

type AlbumTestSuite struct {
	Name          string
	Keyword       string
	SearchAlbum   func(string) ([]model.Playlist, error)
	GetAlbumSongs func(string) ([]model.Song, error)
	ParseAlbum    func(string) (*model.Playlist, []model.Song, error)
}

func TestAlbumPlatforms(t *testing.T) {
	suites := []AlbumTestSuite{
		{
			Name:          "netease",
			Keyword:       "Taylor",
			SearchAlbum:   netease.SearchAlbum,
			GetAlbumSongs: netease.GetAlbumSongs,
			ParseAlbum:    netease.ParseAlbum,
		},
		{
			Name:          "qq",
			Keyword:       "Taylor",
			SearchAlbum:   qq.SearchAlbum,
			GetAlbumSongs: qq.GetAlbumSongs,
			ParseAlbum:    qq.ParseAlbum,
		},
		{
			Name:          "kugou",
			Keyword:       "Taylor",
			SearchAlbum:   kugou.SearchAlbum,
			GetAlbumSongs: kugou.GetAlbumSongs,
			ParseAlbum:    kugou.ParseAlbum,
		},
		{
			Name:          "kuwo",
			Keyword:       "Taylor 1989",
			SearchAlbum:   kuwo.SearchAlbum,
			GetAlbumSongs: kuwo.GetAlbumSongs,
			ParseAlbum:    kuwo.ParseAlbum,
		},
		{
			Name:          "joox",
			Keyword:       "Elizabeth Taylor",
			SearchAlbum:   joox.SearchAlbum,
			GetAlbumSongs: joox.GetAlbumSongs,
			ParseAlbum:    joox.ParseAlbum,
		},
		{
			Name:          "jamendo",
			Keyword:       "Taylor",
			SearchAlbum:   jamendo.SearchAlbum,
			GetAlbumSongs: jamendo.GetAlbumSongs,
			ParseAlbum:    jamendo.ParseAlbum,
		},
		{
			Name:          "migu",
			Keyword:       "JJ林俊杰",
			SearchAlbum:   migu.SearchAlbum,
			GetAlbumSongs: migu.GetAlbumSongs,
			ParseAlbum:    migu.ParseAlbum,
		},
		{
			Name:          "qianqian",
			Keyword:       "林俊杰",
			SearchAlbum:   qianqian.SearchAlbum,
			GetAlbumSongs: qianqian.GetAlbumSongs,
			ParseAlbum:    qianqian.ParseAlbum,
		},
		{
			Name:          "soda",
			Keyword:       "Elizabeth Taylor",
			SearchAlbum:   soda.SearchAlbum,
			GetAlbumSongs: soda.GetAlbumSongs,
			ParseAlbum:    soda.ParseAlbum,
		},
		{
			Name:          "apple",
			Keyword:       "Taylor Swift",
			SearchAlbum:   apple.SearchAlbum,
			GetAlbumSongs: apple.GetAlbumSongs,
			ParseAlbum:    apple.ParseAlbum,
		},
	}

	for _, suite := range suites {
		suite := suite
		t.Run(suite.Name, func(t *testing.T) {
			t.Parallel()

			t.Logf("[%s] SearchAlbum start, keyword=%q", suite.Name, suite.Keyword)
			albums, err := suite.SearchAlbum(suite.Keyword)
			if err != nil {
				t.Logf("[%s] REMINDER: SearchAlbum failed: %v", suite.Name, err)
				t.Fatalf("[%s] SearchAlbum failed: %v", suite.Name, err)
			}
			if len(albums) == 0 {
				t.Logf("[%s] REMINDER: SearchAlbum succeeded but album count is 0", suite.Name)
				t.Fatalf("[%s] SearchAlbum returned no albums", suite.Name)
			}
			t.Logf("[%s] SearchAlbum success, got %d albums", suite.Name, len(albums))

			first := albums[0]
			t.Logf("[%s] First album: name=%q id=%s tracks=%d link=%s", suite.Name, first.Name, first.ID, first.TrackCount, first.Link)
			if first.ID == "" {
				t.Logf("[%s] REMINDER: SearchAlbum returned an empty album id for the first result", suite.Name)
				t.Fatalf("[%s] SearchAlbum returned empty album id", suite.Name)
			}

			t.Logf("[%s] GetAlbumSongs start, album_id=%s", suite.Name, first.ID)
			songs, err := suite.GetAlbumSongs(first.ID)
			if err != nil {
				t.Logf("[%s] REMINDER: GetAlbumSongs failed: %v", suite.Name, err)
				t.Fatalf("[%s] GetAlbumSongs failed: %v", suite.Name, err)
			}
			if len(songs) == 0 {
				t.Logf("[%s] REMINDER: GetAlbumSongs succeeded but song count is 0", suite.Name)
				t.Fatalf("[%s] GetAlbumSongs returned no songs", suite.Name)
			}
			t.Logf("[%s] GetAlbumSongs success, got %d songs for album_id=%s", suite.Name, len(songs), first.ID)
			if first.TrackCount > 0 && len(songs) != first.TrackCount {
				t.Logf("[%s] REMINDER: fetched song count %d differs from album metadata track count %d", suite.Name, len(songs), first.TrackCount)
			}
			t.Logf("[%s] Sample album song: %s - %s (id=%s)", suite.Name, songs[0].Name, songs[0].Artist, songs[0].ID)

			if suite.ParseAlbum == nil || first.Link == "" {
				t.Logf("[%s] Skip ParseAlbum, link empty or func nil", suite.Name)
				return
			}

			t.Logf("[%s] ParseAlbum start, link=%s", suite.Name, first.Link)
			parsedAlbum, parsedSongs, err := suite.ParseAlbum(first.Link)
			if err != nil {
				t.Logf("[%s] REMINDER: ParseAlbum failed: %v", suite.Name, err)
				t.Fatalf("[%s] ParseAlbum failed: %v", suite.Name, err)
			}
			if parsedAlbum == nil {
				t.Logf("[%s] REMINDER: ParseAlbum returned a nil album", suite.Name)
				t.Fatalf("[%s] ParseAlbum returned nil album", suite.Name)
			}
			if parsedAlbum.ID == "" {
				t.Logf("[%s] REMINDER: ParseAlbum returned an empty album id", suite.Name)
				t.Fatalf("[%s] ParseAlbum returned empty album id", suite.Name)
			}
			if len(parsedSongs) == 0 {
				t.Logf("[%s] REMINDER: ParseAlbum succeeded but parsed song count is 0", suite.Name)
				t.Fatalf("[%s] ParseAlbum returned no songs", suite.Name)
			}
			t.Logf("[%s] ParseAlbum success, parsed album=%q id=%s songs=%d", suite.Name, parsedAlbum.Name, parsedAlbum.ID, len(parsedSongs))
			if parsedAlbum.TrackCount > 0 && len(parsedSongs) != parsedAlbum.TrackCount {
				t.Logf("[%s] REMINDER: ParseAlbum parsed %d songs but album track count is %d", suite.Name, len(parsedSongs), parsedAlbum.TrackCount)
			}
		})
	}
}
