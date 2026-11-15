package web

import (
	"strings"
	"testing"
)

func readEmbeddedTemplate(t *testing.T, name string) string {
	t.Helper()
	content, err := templateFS.ReadFile(name)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", name, err)
	}
	return string(content)
}

func TestIssue240DetailPlayerKeepsVolumeControlsStable(t *testing.T) {
	html := readEmbeddedTemplate(t, "templates/partials/videogen_modal.html")
	for _, want := range []string{
		`id="vg-vol-icon"`,
		`id="vg-favorite-button"`,
		`id="vg-download-button"`,
		`onclick="VideoGen.favoriteCurrent()"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("videogen_modal.html missing detail player control %q", want)
		}
	}
	if strings.Contains(html, `id="vg-vol-icon" onclick="VideoGen.toggleMute()" title="静音/恢复" style="width: auto; height: auto;"`) {
		t.Fatal("detail volume icon still has a changing intrinsic width")
	}

	js := readEmbeddedTemplate(t, "templates/static/js/videogen.js")
	for _, want := range []string{
		`bar.style.setProperty('--vg-volume-progress'`,
		`favoriteCurrent: function()`,
		`syncTrackActions: function(songData)`,
		`window.buildDownloadURL`,
		`this.syncTrackActions(songData);`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("videogen.js missing volume or track action token %q", want)
		}
	}

	css := readEmbeddedTemplate(t, "templates/static/css/videogen.css")
	for _, want := range []string{
		`#vg-volume-bar`,
		`var(--vg-volume-progress, 70%)`,
		`.vg-vol-group #vg-vol-icon`,
		`flex: 0 0 28px;`,
		`.vg-track-actions`,
		`.vg-track-action[aria-disabled="true"]`,
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("videogen.css missing stable volume or action style %q", want)
		}
	}
}

func TestIssue240AppearancePreviewRevertsWithoutSaving(t *testing.T) {
	js := readEmbeddedTemplate(t, "templates/static/js/app.js")
	for _, want := range []string{
		`let appearancePreviewSettings = null;`,
		`function appearanceSnapshot(settings = webSettings)`,
		`function beginAppearancePreview(settings = webSettings)`,
		`function updateAppearancePreview(patch)`,
		`function endAppearancePreview()`,
		`function closeSystemConfig()`,
		`beginAppearancePreview(webSettings);`,
		`endAppearancePreview();`,
		`applyThemeAppearance(appearancePreviewSettings);`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js missing transactional appearance token %q", want)
		}
	}

	html := readEmbeddedTemplate(t, "templates/partials/modals.html")
	if !strings.Contains(html, `onclick="closeSystemConfig()"`) {
		t.Fatal("settings modal does not close through the appearance rollback handler")
	}

	css := readEmbeddedTemplate(t, "templates/static/css/style.css")
	for _, want := range []string{
		`height: 40px;`,
		`min-height: 40px;`,
		`.aplayer .aplayer-time-inner`,
		`.aplayer .aplayer-time .aplayer-icon-lrc`,
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("style.css missing appearance or player alignment token %q", want)
		}
	}
}

func TestIssue240PlaylistEmptyStateOnlyRendersWithoutPlaylists(t *testing.T) {
	html := readEmbeddedTemplate(t, "templates/partials/song_list.html")
	if !strings.Contains(html, `{{ else if and .Keyword (not .Playlists) }}`) {
		t.Fatal("playlist empty state can still render below populated playlist cards")
	}
}

func TestIssue240DownloadHistoryExposesOverflowAndTitles(t *testing.T) {
	js := readEmbeddedTemplate(t, "templates/static/js/app.js")
	for _, want := range []string{
		`class="download-record-name" title="${escapeHtml(r.Name || "")}"`,
		`class="download-record-artist" title="${escapeHtml(r.Artist || "")}"`,
		`class="download-record-time" title="${escapeHtml(time)}"`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js missing download history detail title %q", want)
		}
	}

	css := readEmbeddedTemplate(t, "templates/static/css/style.css")
	for _, want := range []string{
		`min-width: 760px;`,
		`.download-records-modal .download-records-list {`,
		`overflow: auto;`,
		`scrollbar-gutter: stable;`,
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("style.css missing download history overflow token %q", want)
		}
	}
}
