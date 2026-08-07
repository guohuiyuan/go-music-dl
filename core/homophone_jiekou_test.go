package core

import (
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

// TestHomophoneJieKou 验证「藉口/借口」在 go-pinyin 下是否同音（藉 为多音字）。
func TestHomophoneJieKou(t *testing.T) {
	SetSkipRules(DefaultSkipRules + ",homophone")
	defer SetSkipRules(DefaultSkipRules)

	set := map[string]struct{}{NormalizeStoredKey("王菲 - 藉口"): {}}
	buildPinyinSet(set)
	song := &model.Song{Artist: "王菲", Name: "借口"}
	if !IsSongDownloaded(song, set) {
		t.Errorf("藉口 vs 借口 应同音命中（若藉取 jiè 音）; pinyin=%q vs %q",
			toPinyinKey("王菲", "藉口"), toPinyinKey("王菲", "借口"))
	}
}
