package web

import (
	"strings"
	"testing"
)

func TestIssue259LoginButtonOpensPlatformCookieSettings(t *testing.T) {
	appContent, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}
	modalContent, err := templateFS.ReadFile("templates/partials/modals.html")
	if err != nil {
		t.Fatalf("ReadFile(modals.html): %v", err)
	}

	appJS := string(appContent)
	bindStart := strings.Index(appJS, "function bindAuthFloat()")
	configStart := strings.Index(appJS, "async function openSystemConfig(")
	if bindStart < 0 || configStart <= bindStart {
		t.Fatal("could not isolate login and system-config handlers")
	}
	if !strings.Contains(appJS[bindStart:configStart], `openSystemConfig("cookies")`) {
		t.Fatal("unauthenticated login button should open platform-cookie settings")
	}
	if !strings.Contains(appJS[configStart:], `async function openSystemConfig(initialTab = "general")`) ||
		!strings.Contains(appJS[configStart:], "setSettingsTab(initialTab)") {
		t.Fatal("system settings should select the requested tab while preserving general as the default")
	}
	if !strings.Contains(string(modalContent), `data-settings-tab="cookies"`) {
		t.Fatal("system settings should expose the platform-cookie tab")
	}
}
