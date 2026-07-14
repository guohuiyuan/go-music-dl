package web

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSetDownloadHeaderUsesUTF8FilenameStar(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	filename := "没地址的信 - 阮俊霖.mp3"
	setDownloadHeader(c, filename)

	header := rec.Header().Get("Content-Disposition")
	if !strings.Contains(header, `filename="download.mp3"`) {
		t.Fatalf("Content-Disposition = %q, want ASCII fallback filename", header)
	}
	if !strings.Contains(header, "filename*=UTF-8''"+url.PathEscape(filename)) {
		t.Fatalf("Content-Disposition = %q, want UTF-8 filename*", header)
	}
	if strings.Contains(header, "%25E6") {
		t.Fatalf("Content-Disposition double-encoded UTF-8 filename: %q", header)
	}
}

func TestSetDownloadHeaderUsesBaseNameForRelativePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	filename := "阮俊霖/专辑/没地址的信 - 阮俊霖.flac"
	want := "没地址的信 - 阮俊霖.flac"
	setDownloadHeader(c, filename)

	header := rec.Header().Get("Content-Disposition")
	if !strings.Contains(header, "filename*=UTF-8''"+url.PathEscape(want)) {
		t.Fatalf("Content-Disposition = %q, want UTF-8 base filename*", header)
	}
}

func TestASCIIDownloadFilenameFallback(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "没地址的信 - 阮俊霖.mp3", want: "download.mp3"},
		{name: "Song - Artist.flac", want: "Song - Artist.flac"},
		{name: " 测试 demo - 歌手.m4a ", want: "demo.m4a"},
		{name: "", want: "download"},
	}

	for _, tt := range tests {
		if got := asciiDownloadFilenameFallback(tt.name); got != tt.want {
			t.Fatalf("asciiDownloadFilenameFallback(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}
