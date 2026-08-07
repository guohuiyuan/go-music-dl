package kuwo

import (
	"strings"
	"testing"
)

func TestConvertKuwoNewLyricKeepsChineseTranslationAfterOriginal(t *testing.T) {
	raw := strings.Join([]string{
		"[ti:sample]",
		"[00:01.000]<0,400>信<400,300>じ<700,300>る",
		"[00:02.000]<0,0>相<0,0>信",
		"[00:02.000]<0,300>君<300,300>が",
		"[00:03.000]<0,0>你",
	}, "\n")

	got := convertKuwoNewLyric(raw)
	wantOrder := []string{
		"[ti:sample]",
		"[00:01.000]信じる",
		"[00:01.000]相信",
		"[00:02.000]君が",
		"[00:02.000]你",
	}

	last := -1
	for _, want := range wantOrder {
		idx := strings.Index(got, want)
		if idx < 0 {
			t.Fatalf("converted lyric missing %q:\n%s", want, got)
		}
		if idx < last {
			t.Fatalf("converted lyric order is wrong for %q:\n%s", want, got)
		}
		last = idx
	}
}

func TestKuwoTranslationDetectorDoesNotTreatJapaneseAsChineseTranslation(t *testing.T) {
	if isKuwoChineseTranslationPayload("<0,0>君<0,0>が") {
		t.Fatal("japanese kana payload should not be treated as Chinese translation")
	}
	if !isKuwoChineseTranslationPayload("<0,0>你<0,0>好") {
		t.Fatal("Chinese payload should be treated as translation")
	}
}

func TestConvertKuwoNewLyricPlacesRomajiBeforeChineseTranslation(t *testing.T) {
	raw := strings.Join([]string{
		"[00:01.000]<0,400>か<400,300>ぜ",
		"[00:02.000]<0,0>kaze",
		"[00:03.000]<0,0>风",
	}, "\n")

	got := convertKuwoNewLyric(raw)
	orig := strings.Index(got, "[00:01.000]かぜ")
	roma := strings.Index(got, "[00:01.000]kaze")
	trans := strings.Index(got, "[00:01.000]风")
	if orig < 0 || roma < 0 || trans < 0 {
		t.Fatalf("converted lyric missing expected lines:\n%s", got)
	}
	if !(orig < roma && roma < trans) {
		t.Fatalf("romaji should be between original and translation:\n%s", got)
	}
}
