package core

import (
	"errors"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestNeteaseFallbackOrderSkipsAV3AQualities(t *testing.T) {
	tests := []struct {
		name      string
		requested string
		want      []model.AudioQuality
	}{
		{
			name:      "master falls back through playable ladder first",
			requested: "master",
			want: []model.AudioQuality{
				model.AudioQualityMaster,
				model.AudioQualitySpatial,
				model.AudioQualityHiRes,
				model.AudioQualityLossless,
				model.AudioQualityHigh,
				model.AudioQualityHigher,
				model.AudioQualityStandard,
				model.AudioQualityVivid,
				model.AudioQualityAtmos,
			},
		},
		{
			name:      "explicit vivid keeps other immersive tiers last",
			requested: "vivid",
			want: []model.AudioQuality{
				model.AudioQualityVivid,
				model.AudioQualityHiRes,
				model.AudioQualityLossless,
				model.AudioQualityHigh,
				model.AudioQualityHigher,
				model.AudioQualityStandard,
				model.AudioQualityAtmos,
			},
		},
		{
			name:      "explicit atmos falls back to playable tiers",
			requested: "atmos",
			want: []model.AudioQuality{
				model.AudioQualityAtmos,
				model.AudioQualityHiRes,
				model.AudioQualityLossless,
				model.AudioQualityHigh,
				model.AudioQualityHigher,
				model.AudioQualityStandard,
			},
		},
		{
			name:      "lossless request keeps the standard ladder",
			requested: "lossless",
			want: []model.AudioQuality{
				model.AudioQualityLossless,
				model.AudioQualityHigh,
				model.AudioQualityHigher,
				model.AudioQualityStandard,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			song := &model.Song{Source: "netease"}
			got, err := audioQualityFallbackOrder(song, test.requested)
			if err != nil {
				t.Fatalf("audioQualityFallbackOrder() error = %v", err)
			}
			if len(got) != len(test.want) {
				t.Fatalf("fallback order = %v, want %v", got, test.want)
			}
			for i := range test.want {
				if got[i] != test.want[i] {
					t.Fatalf("fallback order = %v, want %v", got, test.want)
				}
			}
		})
	}
}

func TestNonNeteaseFallbackOrderIsUnchanged(t *testing.T) {
	choices, ok := audioQualityChoicesForSource("qq")
	if !ok || len(choices) == 0 {
		t.Fatalf("qq choices are unavailable")
	}
	start := -1
	for i, option := range choices {
		if option.Value == model.AudioQualityLossless {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("qq choices = %#v, want lossless", choices)
	}
	want := make([]model.AudioQuality, 0, len(choices)-start)
	for _, option := range choices[start:] {
		want = append(want, option.Value)
	}
	got, err := audioQualityFallbackOrder(&model.Song{Source: "qq"}, "lossless")
	if err != nil {
		t.Fatalf("audioQualityFallbackOrder() error = %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("fallback order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("fallback order = %v, want %v", got, want)
		}
	}
}

// TestNeteaseFallbackDoesNotDownloadAV3AByAccident pins the reported bug: a
// master request on a track that only offers NetEase's 3D tiers used to resolve
// to an Audio Vivid (av3a) M4A that desktop players report as corrupted.
func TestNeteaseFallbackDoesNotDownloadAV3AByAccident(t *testing.T) {
	song := &model.Song{Source: "netease"}
	var tried []model.AudioQuality
	got, err := resolveWithAudioQualityFallback(
		song,
		"master",
		func(quality string) (string, error) {
			q, _ := model.NormalizeAudioQuality(quality)
			tried = append(tried, q)
			switch q {
			case model.AudioQualityVivid, model.AudioQualityAtmos:
				return "audio-vivid", nil
			case model.AudioQualityStandard:
				return "mp3", nil
			default:
				return "", errors.New("unavailable")
			}
		},
		func(value string) bool { return value != "" },
	)
	if err != nil {
		t.Fatalf("resolveWithAudioQualityFallback() error = %v", err)
	}
	if got != "mp3" {
		t.Fatalf("resolved value = %q, want mp3", got)
	}
	for _, quality := range tried {
		if quality == model.AudioQualityVivid || quality == model.AudioQualityAtmos {
			t.Fatalf("fallback tried AV3A tier %s before a playable tier: %v", quality, tried)
		}
	}
}
