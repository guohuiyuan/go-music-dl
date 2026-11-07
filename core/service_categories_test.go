package core

import (
	"reflect"
	"testing"
)

func TestPlaylistCategoryFactoriesAndSourceList(t *testing.T) {
	supported := []string{"netease", "qq", "kugou", "kuwo", "migu", "qianqian", "joox", "soda", "apple", "spotify"}
	if got := GetPlaylistCategorySourceNames(); !reflect.DeepEqual(got, supported) {
		t.Fatalf("GetPlaylistCategorySourceNames() = %v, want %v", got, supported)
	}

	for _, source := range supported {
		if fn := GetPlaylistCategoriesFunc(source); fn == nil {
			t.Fatalf("GetPlaylistCategoriesFunc(%q) returned nil", source)
		}
		if fn := GetCategoryPlaylistsFunc(source); fn == nil {
			t.Fatalf("GetCategoryPlaylistsFunc(%q) returned nil", source)
		}
	}

	for _, source := range GetPlaylistSourceNames() {
		skip := false
		for _, item := range supported {
			if item == source {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		if fn := GetPlaylistCategoriesFunc(source); fn != nil {
			t.Fatalf("GetPlaylistCategoriesFunc(%q) = non-nil, want nil", source)
		}
		if fn := GetCategoryPlaylistsFunc(source); fn != nil {
			t.Fatalf("GetCategoryPlaylistsFunc(%q) = non-nil, want nil", source)
		}
	}
}

// TestSodaPlaylistCategoriesViaCore 验证汽水分类能力经 core 工厂可用。
func TestSodaPlaylistCategoriesViaCore(t *testing.T) {
	categories, err := GetPlaylistCategoriesFunc("soda")()
	if err != nil {
		t.Fatalf("soda GetPlaylistCategories failed: %v", err)
	}
	if len(categories) < 13 {
		t.Fatalf("soda categories = %d, want at least 13", len(categories))
	}
	for _, category := range categories {
		if category.Source != "soda" || category.ID == "" || category.Name == "" {
			t.Fatalf("unexpected soda category: %#v", category)
		}
	}

	playlists, err := GetCategoryPlaylistsFunc("soda")("", 1, 5)
	if err != nil {
		t.Fatalf("soda GetCategoryPlaylists failed: %v", err)
	}
	if len(playlists) == 0 {
		t.Fatal("soda GetCategoryPlaylists returned no playlists")
	}
	for _, playlist := range playlists {
		if playlist.Source != "soda" || playlist.ID == "" || playlist.Name == "" {
			t.Fatalf("unexpected soda playlist: %#v", playlist)
		}
	}
}
