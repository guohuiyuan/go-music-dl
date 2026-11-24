package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAppJSMediaSessionArtworkUsesCoverProxy(t *testing.T) {
	content, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}

	js := string(content)
	if !strings.Contains(js, "function buildMediaSessionCoverURL(audio = getCurrentAPlayerAudio())") {
		t.Fatal("app.js missing buildMediaSessionCoverURL helper")
	}
	if !strings.Contains(js, "cover_proxy") {
		t.Fatal("app.js missing cover_proxy media session artwork path")
	}
	if !strings.Contains(js, "function scheduleMediaSessionSync(") {
		t.Fatal("app.js missing delayed media session resync helper")
	}
	if !strings.Contains(js, "const mediaSessionCoverCache = new Map();") {
		t.Fatal("app.js missing media session cover cache")
	}
	if !strings.Contains(js, "function buildMediaSessionTrackKey(audio = getCurrentAPlayerAudio())") {
		t.Fatal("app.js missing media session track key helper")
	}
	if !strings.Contains(js, "function isTransientMediaSessionURL(value)") {
		t.Fatal("app.js missing transient media session URL helper")
	}
	if !strings.Contains(js, "mediaSessionCoverCache.set(trackKey, resolved);") {
		t.Fatal("app.js missing stable media session cover caching")
	}
	if !strings.Contains(js, "const cached = mediaSessionCoverCache.get(trackKey);") {
		t.Fatal("app.js missing cached media session cover lookup")
	}
	if !strings.Contains(js, "function shouldPreserveMediaSessionMetadata()") {
		t.Fatal("app.js missing transient media session metadata guard")
	}
	if !strings.Contains(js, "if (shouldPreserveMediaSessionMetadata()) {") {
		t.Fatal("app.js missing transient metadata preservation logic")
	}
}

func TestAppJSPlaybackShortcutAndMediaKeys(t *testing.T) {
	content, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}

	js := string(content)
	required := []string{
		"function togglePlayback()",
		"function handlePlaybackShortcut(event)",
		"function normalizeShortcutBinding(binding)",
		"function shortcutBindingFromKeyboardEvent(event)",
		"function readPlaybackShortcutSettingsFromForm()",
		"function applyLyricAppearance()",
		"webSettings.shortcutPrevious",
		"webSettings.shortcutSeekForward",
		`document.addEventListener("keydown", handlePlaybackShortcut);`,
		"function bindMediaKeyFallback()",
		"bindMediaKeyFallback();",
		`"MediaTrackNext"`,
		`"MediaTrackPrevious"`,
		`"MediaPlayPause"`,
		`"MediaStop"`,
	}
	for _, token := range required {
		if !strings.Contains(js, token) {
			t.Fatalf("app.js missing %q", token)
		}
	}
}

func TestPlaybackShortcutAndLyricSettingsUi(t *testing.T) {
	modalContent, err := templateFS.ReadFile("templates/partials/modals.html")
	if err != nil {
		t.Fatalf("ReadFile(modals.html): %v", err)
	}
	html := string(modalContent)
	for _, want := range []string{
		`id="setting-shortcut-play-pause"`,
		`id="setting-shortcut-previous"`,
		`id="setting-shortcut-next"`,
		`id="setting-shortcut-seek-back"`,
		`id="setting-shortcut-seek-forward"`,
		`id="setting-shortcut-status"`,
		`id="setting-lyric-font-family"`,
		`id="setting-lyric-font-size"`,
		`id="setting-lyric-font-preview"`,
		`应用内全局快捷键`,
		`PageUp / PageDown 保留给列表翻页`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("modals.html missing playback setting token %q", want)
		}
	}

	cssContent, err := templateFS.ReadFile("templates/static/css/style.css")
	if err != nil {
		t.Fatalf("ReadFile(style.css): %v", err)
	}
	css := string(cssContent)
	for _, want := range []string{
		"--lyric-font-family",
		"--lyric-font-size",
		".shortcut-setting-row",
		".lyric-font-preview",
		"font-size: var(--lyric-font-size)",
		"font-family: var(--lyric-font-family) !important",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("style.css missing playback setting token %q", want)
		}
	}
}

func TestCoverProxyReturnsInlineImage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	imageBytes := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x01, 0x02, 0x03, 0x04}
	oldFetch := fetchCoverImage
	fetchCoverImage = func(rawURL, source string) ([]byte, string, error) {
		return imageBytes, "image/png", nil
	}
	t.Cleanup(func() { fetchCoverImage = oldFetch })

	router := gin.New()
	RegisterMusicRoutes(router.Group(RoutePrefix), router.Group(RoutePrefix))

	req := httptest.NewRequest(http.MethodGet, RoutePrefix+"/cover_proxy?url="+url.QueryEscape("https://cdn.example/cover.png"), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "image/png") {
		t.Fatalf("Content-Type = %q, want image/png", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=21600" {
		t.Fatalf("Cache-Control = %q, want public, max-age=21600", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), imageBytes) {
		t.Fatal("cover_proxy returned unexpected body")
	}
}
