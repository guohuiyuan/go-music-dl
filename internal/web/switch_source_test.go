package web

import (
	"strings"
	"testing"
	"time"

	"github.com/guohuiyuan/music-lib/model"
)

func withSwitchSourceTestHooks(t *testing.T) {
	t.Helper()

	origSearchProvider := switchSearchFuncProvider
	origValidatePlayable := switchValidatePlayable
	origAllSources := switchAllSourceNames
	origDefaultSources := switchDefaultSourceNames
	t.Cleanup(func() {
		switchSearchFuncProvider = origSearchProvider
		switchValidatePlayable = origValidatePlayable
		switchAllSourceNames = origAllSources
		switchDefaultSourceNames = origDefaultSources
	})
}

func TestAppJSBatchSwitchSourceUsesConcurrentWorkers(t *testing.T) {
	content, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}

	js := string(content)
	for _, want := range []string{
		"return fetch(url)",
		"const concurrency = Math.min(3, cards.length)",
		"Promise.all(Array.from({ length: concurrency }, runWorker))",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js missing %q", want)
		}
	}
	if strings.Contains(js, "index * 1000") {
		t.Fatal("batchSwitchSource still staggers source switching by one second per song")
	}
}

func TestAppJSAutoSwitchInvalidSources(t *testing.T) {
	content, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}

	js := string(content)
	for _, want := range []string{
		"autoSwitchInvalidSources: true",
		"function scheduleAutoSwitchInvalidSources",
		"async function autoSwitchInvalidSources()",
		"card.dataset.autoSwitchInvalidAttempted = '1'",
		"selectInvalidSongCards({ silent: true, cards: invalidCards })",
		"await batchSwitchSource({ skipConfirm: true, silent: true, auto: true, cards: invalidCards })",
		"if (document.querySelector('.tag-fail'))",
		"card.dataset.inspectPending = '1'",
		"async function batchSwitchSource(options = {})",
		"if (!options.skipConfirm && !confirm",
		"clearSongCardSelection(card, { deferToolbar: !!options.deferToolbar })",
		"options.cards.filter(card => card && card.isConnected)",
		"if (!shouldCheck && options.clearExisting !== false)",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js missing %q", want)
		}
	}
}

func TestFindBestSwitchSongReturnsBeforeSlowSourcesOnHighConfidenceMatch(t *testing.T) {
	withSwitchSourceTestHooks(t)

	switchAllSourceNames = func() []string { return []string{"slow", "fast"} }
	switchDefaultSourceNames = func() []string { return []string{"slow", "fast"} }
	switchSearchFuncProvider = func(source string) func(string) ([]model.Song, error) {
		switch source {
		case "slow":
			return func(string) ([]model.Song, error) {
				time.Sleep(2 * time.Second)
				return []model.Song{{ID: "slow-song", Name: "Track", Artist: "Artist", Duration: 180}}, nil
			}
		case "fast":
			return func(string) ([]model.Song, error) {
				return []model.Song{{ID: "fast-song", Name: "Track", Artist: "Artist", Duration: 180}}, nil
			}
		default:
			return nil
		}
	}
	switchValidatePlayable = func(song *model.Song) bool {
		return song != nil && song.ID == "fast-song"
	}

	start := time.Now()
	got, score, err := findBestSwitchSong("Track", "Artist", "netease", "", 180)
	if err != nil {
		t.Fatalf("findBestSwitchSong returned error: %v", err)
	}
	if got == nil || got.ID != "fast-song" || got.Source != "fast" {
		t.Fatalf("findBestSwitchSong selected %#v, want fast-song from fast", got)
	}
	if score < switchHighConfidenceScore {
		t.Fatalf("selected score = %f, want high confidence", score)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("findBestSwitchSong waited for slow source, elapsed=%s", elapsed)
	}
}

func TestValidateSwitchCandidatesKeepsRankedOrderWithParallelChecks(t *testing.T) {
	withSwitchSourceTestHooks(t)

	switchValidatePlayable = func(song *model.Song) bool {
		if song != nil && song.ID == "best" {
			time.Sleep(100 * time.Millisecond)
			return true
		}
		return song != nil && song.ID == "second"
	}

	candidates := []switchCandidate{
		{song: model.Song{ID: "best", Source: "fast"}, score: 1},
		{song: model.Song{ID: "second", Source: "fast"}, score: 0.99},
	}
	got, score, ok := validateSwitchCandidates(candidates)
	if !ok {
		t.Fatal("validateSwitchCandidates returned no playable candidate")
	}
	if got == nil || got.ID != "best" || score != 1 {
		t.Fatalf("validateSwitchCandidates selected %#v score=%f, want best score=1", got, score)
	}
}
