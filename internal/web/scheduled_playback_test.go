package web

import (
	"strings"
	"testing"
)

func TestScheduledPlaybackStopUiAndBehavior(t *testing.T) {
	indexContent, err := templateFS.ReadFile("templates/pages/index.html")
	if err != nil {
		t.Fatalf("ReadFile(index.html): %v", err)
	}
	indexHTML := string(indexContent)
	for _, want := range []string{
		`id="scheduled-playback-toolbar-button"`,
		`onclick="openScheduledPlaybackModal()"`,
		`id="scheduled-playback-toolbar-badge"`,
		`title="定时停止"`,
		`class="fa-solid fa-stopwatch"`,
	} {
		if !strings.Contains(indexHTML, want) {
			t.Fatalf("index.html missing scheduled stop token %q", want)
		}
	}

	modalContent, err := templateFS.ReadFile("templates/partials/modals.html")
	if err != nil {
		t.Fatalf("ReadFile(modals.html): %v", err)
	}
	modalHTML := string(modalContent)
	for _, want := range []string{
		`id="scheduledPlaybackModal"`,
		`id="scheduled-playback-form"`,
		`id="scheduled-playback-time"`,
		`id="scheduled-playback-active"`,
		`data-scheduled-playback-minutes="15"`,
		`onclick="cancelScheduledPlayback()"`,
		`onsubmit="return saveScheduledPlayback(event)"`,
		"定时停止",
		"设置停止",
	} {
		if !strings.Contains(modalHTML, want) {
			t.Fatalf("modals.html missing scheduled stop token %q", want)
		}
	}
	if strings.Contains(modalHTML, `id="scheduled-playback-repeat"`) {
		t.Fatal("modals.html still contains the removed repeat option")
	}

	appContent, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}
	appJS := string(appContent)
	for _, want := range []string{
		`const SCHEDULED_PLAYBACK_STORAGE_KEY = "musicdl:scheduledStop";`,
		`const SCHEDULED_PLAYBACK_LEGACY_STORAGE_KEY = "musicdl:scheduledPlayback";`,
		"localStorage.removeItem(SCHEDULED_PLAYBACK_LEGACY_STORAGE_KEY);",
		"function normalizeScheduledPlayback(",
		"function scheduleScheduledPlaybackWake(",
		"function triggerScheduledPlayback(",
		"ap.pause();",
		`"定时停止已执行"`,
		"function saveScheduledPlayback(",
		"function cancelScheduledPlayback(",
		"window.addEventListener(\"focus\", checkScheduledPlayback);",
		"window.addEventListener(\"pageshow\", checkScheduledPlayback);",
		`document.addEventListener("visibilitychange", () => {`,
		"window.musicDlScheduledStop",
	} {
		if !strings.Contains(appJS, want) {
			t.Fatalf("app.js missing scheduled stop token %q", want)
		}
	}
	for _, removed := range []string{
		"SCHEDULED_PLAYBACK_REPEAT_DAILY",
		"const playResult = ap.play();",
		"浏览器拦截了自动播放",
	} {
		if strings.Contains(appJS, removed) {
			t.Fatalf("app.js still contains removed playback behavior %q", removed)
		}
	}

	cssContent, err := templateFS.ReadFile("templates/static/css/style.css")
	if err != nil {
		t.Fatalf("ReadFile(style.css): %v", err)
	}
	css := string(cssContent)
	for _, want := range []string{
		".rt-btn-scheduled-playback",
		".rt-scheduled-badge",
		".scheduled-playback-modal",
		".scheduled-playback-fields",
		".scheduled-playback-active[hidden]",
		"body.tv-ui .scheduled-playback-modal",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("style.css missing scheduled stop token %q", want)
		}
	}
}
