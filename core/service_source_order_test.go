package core

import (
	"reflect"
	"testing"
)

func TestAllSourceNamesKeepSharedUIVisibilityOrder(t *testing.T) {
	want := []string{
		"netease",
		"qq",
		"kugou",
		"kuwo",
		"migu",
		"soda",
		"apple",
		"spotify",
		"qianqian",
		"joox",
		"bilibili",
		"fivesing",
		"jamendo",
		"local",
	}
	if got := GetAllSourceNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("GetAllSourceNames() = %v, want %v", got, want)
	}
}
