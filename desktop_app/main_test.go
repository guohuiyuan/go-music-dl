package main

import (
	"net/url"
	"strings"
	"testing"
)

func TestBridgeRoutesOnlyBrowserDownloadsExternally(t *testing.T) {
	if strings.Contains(bridgeScript, "setting-download-to-local") {
		t.Fatal("bridgeScript must not depend on removed download-to-local toggle")
	}
	if !strings.Contains(bridgeScript, `closest(".btn-download, .btn-browser-download")`) {
		t.Fatal("bridgeScript should inspect both local-save and browser-download buttons")
	}
	if !strings.Contains(bridgeScript, `if (link.classList.contains("btn-download"))`) {
		t.Fatal("bridgeScript should leave local-save downloads to app.js POST handling")
	}
	if !strings.Contains(bridgeScript, `url.searchParams.delete("save_local")`) {
		t.Fatal("bridgeScript should strip save_local before opening external browser downloads")
	}
}

func TestBridgeReportsPlaybackState(t *testing.T) {
	for _, want := range []string{
		"musicDlPlaybackState",
		`notifyPlaybackState("playing")`,
		`notifyPlaybackState("paused")`,
		`notifyPlaybackState("ended")`,
		`notifyPlaybackState("released")`,
	} {
		if !strings.Contains(bridgeScript, want) {
			t.Fatalf("bridgeScript missing playback token %q", want)
		}
	}
}

func TestHandleWebViewMessagePlaybackDoesNotOpenURL(t *testing.T) {
	app := newDesktopApp(nil, nil)
	app.handleWebViewMessage("playback:paused")
	if app.pendingExternalOpenTo != nil {
		t.Fatalf("playback message should not be treated as URL: %s", app.pendingExternalOpenTo)
	}
}

func TestHandleWebViewMessageDownloadURL(t *testing.T) {
	app := newDesktopApp(nil, nil)
	raw := "http://127.0.0.1:37777/music/download?id=1&source=qq"
	app.handleWebViewMessage(raw)
	if app.pendingExternalOpenTo == nil {
		t.Fatal("download URL was not queued for external open")
	}
	if got := app.pendingExternalOpenTo.String(); got != raw {
		t.Fatalf("pendingExternalOpenTo = %q, want %q", got, raw)
	}
	if _, err := url.Parse(gotURL(app)); err != nil {
		t.Fatalf("pending URL is not parseable: %v", err)
	}
}

func TestInitialNavigationWaitsForServerReady(t *testing.T) {
	ch := make(chan initialNavigationResult, 1)
	app := newDesktopApp(nil, ch)
	app.callbackRegistered = true
	app.pendingInitialNav = true

	app.consumeInitialNavigationResult()
	if app.initialNavReady {
		t.Fatal("initial navigation should not be ready before the probe result")
	}

	ch <- initialNavigationResult{URL: "http://127.0.0.1:37777/music/"}
	app.consumeInitialNavigationResult()
	if !app.initialNavReady {
		t.Fatal("initial navigation should be ready after the probe result")
	}
	if app.initialNavURL != "http://127.0.0.1:37777/music/" {
		t.Fatalf("initialNavURL = %q", app.initialNavURL)
	}
	if !app.pendingInitialNav {
		t.Fatal("pendingInitialNav should be restored after the server becomes ready")
	}
}

func gotURL(app *desktopApp) string {
	if app.pendingExternalOpenTo == nil {
		return ""
	}
	return app.pendingExternalOpenTo.String()
}
