package core

import (
	"reflect"
	"testing"
)

func TestTopListFactoriesAndSourceList(t *testing.T) {
	supported := []string{"netease", "qq", "kugou", "kuwo", "soda", "migu", "apple", "spotify"}
	if got := GetTopListSourceNames(); !reflect.DeepEqual(got, supported) {
		t.Fatalf("GetTopListSourceNames() = %v, want %v", got, supported)
	}

	for _, source := range supported {
		if fn := GetTopListsFunc(source); fn == nil {
			t.Fatalf("GetTopListsFunc(%q) returned nil", source)
		}
		if fn := GetTopListSongsFunc(source); fn == nil {
			t.Fatalf("GetTopListSongsFunc(%q) returned nil", source)
		}
	}

	for _, source := range GetPlaylistSourceNames() {
		if source == "netease" || source == "qq" || source == "kugou" || source == "kuwo" || source == "soda" || source == "migu" || source == "apple" || source == "spotify" {
			continue
		}
		if fn := GetTopListsFunc(source); fn != nil {
			t.Fatalf("GetTopListsFunc(%q) = non-nil, want nil", source)
		}
		if fn := GetTopListSongsFunc(source); fn != nil {
			t.Fatalf("GetTopListSongsFunc(%q) = non-nil, want nil", source)
		}
	}
}
