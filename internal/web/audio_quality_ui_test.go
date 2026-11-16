package web

import (
	"strings"
	"testing"
)

func TestCookieSaveSelectsHighestQualityAndKeepsSettingsOpen(t *testing.T) {
	js := readEmbeddedTemplate(t, "templates/static/js/app.js")
	for _, want := range []string{
		`function syncSavedCookieBaselines(cookies = {})`,
		`input.dataset.savedCookie = value;`,
		`syncSavedCookieBaselines(cookies || {});`,
		`function changedCookieAudioQualitySources(cookies = {})`,
		`function applyHighestAvailableAudioQualityDefaults(preferred, sources)`,
		`audioQualityOptions[source]?.[0]?.value`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js missing changed-cookie quality token %q", want)
		}
	}

	start := strings.Index(js, "async function saveCookies()")
	end := strings.Index(js, `window.addEventListener("scroll"`)
	if start < 0 || end <= start {
		t.Fatal("could not isolate saveCookies implementation")
	}
	saveCookiesJS := js[start:end]
	for _, want := range []string{
		`const changedCookieSources = changedCookieAudioQualitySources(data);`,
		`applyHighestAvailableAudioQualityDefaults(`,
		`syncSavedCookieBaselines(data);`,
		`showToast(`,
		`"设置已保存"`,
	} {
		if !strings.Contains(saveCookiesJS, want) {
			t.Fatalf("saveCookies missing successful-save token %q", want)
		}
	}
	for _, unwanted := range []string{
		`alert("保存成功")`,
		`closeSystemConfig()`,
	} {
		if strings.Contains(saveCookiesJS, unwanted) {
			t.Fatalf("saveCookies must keep settings open after saving, found %q", unwanted)
		}
	}
}
