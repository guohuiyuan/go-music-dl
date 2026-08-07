package main

import (
	"errors"
	"testing"

	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/fivesing"
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

func TestPlaylistCategoryPlatforms(t *testing.T) {
	suites := []struct {
		name                 string
		source               string
		getCategories        func() ([]model.PlaylistCategory, error)
		getCategoryPlaylists func(string, int, int) ([]model.Playlist, error)
	}{
		{name: "netease", source: "netease", getCategories: netease.GetPlaylistCategories, getCategoryPlaylists: netease.GetCategoryPlaylists},
		{name: "qianqian", source: "qianqian", getCategories: qianqian.GetPlaylistCategories, getCategoryPlaylists: qianqian.GetCategoryPlaylists},
		{name: "qq", source: "qq", getCategories: qq.GetPlaylistCategories, getCategoryPlaylists: qq.GetCategoryPlaylists},
		{name: "kugou", source: "kugou", getCategories: kugou.GetPlaylistCategories, getCategoryPlaylists: kugou.GetCategoryPlaylists},
		{name: "kuwo", source: "kuwo", getCategories: kuwo.GetPlaylistCategories, getCategoryPlaylists: kuwo.GetCategoryPlaylists},
		{name: "joox", source: "joox", getCategories: joox.GetPlaylistCategories, getCategoryPlaylists: joox.GetCategoryPlaylists},
		{name: "migu", source: "migu", getCategories: migu.GetPlaylistCategories, getCategoryPlaylists: migu.GetCategoryPlaylists},
	}

	for _, suite := range suites {
		suite := suite
		t.Run(suite.name, func(t *testing.T) {
			categories, err := suite.getCategories()
			if err != nil {
				t.Fatalf("GetPlaylistCategories failed: %v", err)
			}
			if len(categories) == 0 {
				t.Fatal("GetPlaylistCategories returned no categories")
			}

			var categoryID string
			for _, category := range categories {
				if category.Source != suite.source {
					t.Fatalf("category source mismatch: got %q want %q", category.Source, suite.source)
				}
				if category.Name == "" {
					t.Fatal("category name is empty")
				}
				if category.ID != "" && categoryID == "" {
					categoryID = category.ID
				}
			}
			if categoryID == "" {
				t.Fatal("no non-empty category id found")
			}

			allPlaylists, err := suite.getCategoryPlaylists("", 1, 5)
			if err != nil {
				t.Fatalf("GetCategoryPlaylists all failed: %v", err)
			}
			if len(allPlaylists) == 0 {
				t.Fatal("GetCategoryPlaylists all returned no playlists")
			}

			categoryPlaylists, err := suite.getCategoryPlaylists(categoryID, 1, 5)
			if err != nil {
				t.Fatalf("GetCategoryPlaylists category %q failed: %v", categoryID, err)
			}
			if len(categoryPlaylists) == 0 {
				t.Fatalf("GetCategoryPlaylists category %q returned no playlists", categoryID)
			}
			for _, playlist := range categoryPlaylists {
				if playlist.Source != suite.source {
					t.Fatalf("playlist source mismatch: got %q want %q", playlist.Source, suite.source)
				}
				if playlist.ID == "" || playlist.Name == "" {
					t.Fatalf("playlist id/name is empty: %#v", playlist)
				}
			}
		})
	}
}

func TestPlaylistCategoryUnsupportedPlatforms(t *testing.T) {
	suites := []struct {
		name          string
		getCategories func() ([]model.PlaylistCategory, error)
		getPlaylists  func(string, int, int) ([]model.Playlist, error)
	}{
		{name: "bilibili", getCategories: bilibili.GetPlaylistCategories, getPlaylists: bilibili.GetCategoryPlaylists},
		{name: "fivesing", getCategories: fivesing.GetPlaylistCategories, getPlaylists: fivesing.GetCategoryPlaylists},
		{name: "jamendo", getCategories: jamendo.GetPlaylistCategories, getPlaylists: jamendo.GetCategoryPlaylists},
		{name: "soda", getCategories: soda.GetPlaylistCategories, getPlaylists: soda.GetCategoryPlaylists},
	}

	for _, suite := range suites {
		suite := suite
		t.Run(suite.name, func(t *testing.T) {
			if categories, err := suite.getCategories(); !errors.Is(err, model.ErrPlaylistCategoriesUnsupported) || categories != nil {
				t.Fatalf("GetPlaylistCategories mismatch: categories=%v err=%v", categories, err)
			}
			if playlists, err := suite.getPlaylists("", 1, 5); !errors.Is(err, model.ErrPlaylistCategoriesUnsupported) || playlists != nil {
				t.Fatalf("GetCategoryPlaylists mismatch: playlists=%v err=%v", playlists, err)
			}
		})
	}
}
