package web

import (
	"os"
	"strings"
	"testing"
)

func TestSodaQRLoginUIIncludesSMSFlow(t *testing.T) {
	jsBytes, err := os.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	js := string(jsBytes)
	for _, want := range []string{
		"soda: '汽水音乐'",
		"function sendSodaSMSCode()",
		"function validateSodaSMSCode()",
		"need_sms",
		"token, action, encryptUID, verifyParams",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js missing %q", want)
		}
	}

	modalBytes, err := os.ReadFile("templates/partials/modals.html")
	if err != nil {
		t.Fatalf("read modals.html: %v", err)
	}
	modal := string(modalBytes)
	for _, want := range []string{"qrLoginSMSPanel", "qrLoginSMSCode", "sendSodaSMSCode()", "validateSodaSMSCode()"} {
		if !strings.Contains(modal, want) {
			t.Fatalf("modals.html missing %q", want)
		}
	}

	cssBytes, err := os.ReadFile("templates/static/css/style.css")
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	if !strings.Contains(string(cssBytes), ".qr-login-sms") {
		t.Fatal("style.css missing qr-login-sms styles")
	}
}
