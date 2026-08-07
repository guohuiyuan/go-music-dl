package kugou

import "testing"

func TestShouldTryKugouHighQualityDownloadWithAppCookie(t *testing.T) {
	cookie := "token=token; userid=123; KUGOU_API_MID=mid; dfid=dfid"
	if !shouldTryKugouHighQualityDownload(cookie, 0) {
		t.Fatal("expected app cookie to use high quality download path even when privilege is 0")
	}
}

func TestShouldTryKugouHighQualityDownloadWithVIPPrivilege(t *testing.T) {
	if !shouldTryKugouHighQualityDownload("", 10) {
		t.Fatal("expected privilege 10 to use high quality download path")
	}
	if !shouldTryKugouHighQualityDownload("", 8) {
		t.Fatal("expected privilege 8 to use high quality download path")
	}
}

func TestShouldTryKugouHighQualityDownloadWithoutAppCookieOrVIPPrivilege(t *testing.T) {
	if shouldTryKugouHighQualityDownload("userid=123", 0) {
		t.Fatal("did not expect high quality download path without app cookie or VIP privilege")
	}
}
