package web

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCoverProxyRejectsPrivateAndUnsafeTargets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldFetch := fetchCoverImage
	fetchCoverImage = fetchPublicCoverImage
	t.Cleanup(func() { fetchCoverImage = oldFetch })

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("internal secret"))
	}))
	defer upstream.Close()

	router := gin.New()
	RegisterMusicRoutes(router.Group(RoutePrefix), router.Group(RoutePrefix))

	for _, target := range []string{
		upstream.URL + "/cover.png",
		"http://localhost:8080/cover.png",
		"http://169.254.169.254/latest/meta-data/",
		"file:///etc/passwd",
	} {
		req := httptest.NewRequest(http.MethodGet, RoutePrefix+"/cover_proxy?url="+url.QueryEscape(target), nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusBadGateway {
			t.Fatalf("target %q: status = %d, want 400 or 502", target, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "internal secret") {
			t.Fatalf("target %q: internal body leaked: %q", target, rec.Body.String())
		}
	}
}

func TestDownloadCoverRejectsPrivateTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldFetch := fetchCoverImage
	fetchCoverImage = fetchPublicCoverImage
	t.Cleanup(func() { fetchCoverImage = oldFetch })

	router := gin.New()
	RegisterMusicRoutes(router.Group(RoutePrefix), router.Group(RoutePrefix))

	req := httptest.NewRequest(http.MethodGet, RoutePrefix+"/download_cover?url="+url.QueryEscape("http://127.0.0.1/cover.png")+"&name=x", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 400 or 502", rec.Code)
	}
}

func TestDownloadCoverUsesDetectedImageType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	imageBytes := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x01}
	oldFetch := fetchCoverImage
	fetchCoverImage = func(rawURL, source string) ([]byte, string, error) {
		return imageBytes, "image/png", nil
	}
	t.Cleanup(func() { fetchCoverImage = oldFetch })

	router := gin.New()
	RegisterMusicRoutes(router.Group(RoutePrefix), router.Group(RoutePrefix))
	req := httptest.NewRequest(http.MethodGet, RoutePrefix+"/download_cover?url="+url.QueryEscape("https://cdn.example/cover.png")+"&name=song&artist=artist", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("Content-Type = %q, want image/png", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, ".png") {
		t.Fatalf("Content-Disposition = %q, want png filename", got)
	}
}

func TestIsPublicIP(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"127.0.0.1", false},
		{"10.0.0.1", false},
		{"172.16.0.1", false},
		{"192.168.1.1", false},
		{"169.254.169.254", false},
		{"100.64.0.1", false},
		{"0.0.0.0", false},
		{"224.0.0.1", false},
		{"255.255.255.255", false},
		{"198.18.0.1", false},
		{"192.0.2.1", false},
		{"198.51.100.1", false},
		{"203.0.113.1", false},
		{"::1", false},
		{"fc00::1", false},
		{"fe80::1", false},
		{"2001:db8::1", false},
		{"2001:4860:4860::8888", true},
	}
	for _, test := range tests {
		ip := net.ParseIP(test.ip)
		if got := isPublicIP(ip); got != test.want {
			t.Fatalf("isPublicIP(%s) = %t, want %t", test.ip, got, test.want)
		}
	}
}

func TestValidateCoverURL(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1/x.png",
		"http://[::1]/x.png",
		"http://localhost/x.png",
		"http://169.254.169.254/latest/meta-data/",
		"file:///etc/passwd",
		"ftp://example.com/x.png",
		"http://user:pass@example.com/x.png",
	} {
		parsed, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if err := validateCoverURL(parsed); err == nil {
			t.Fatalf("validateCoverURL(%q) = nil, want error", raw)
		}
	}
	parsed, _ := url.Parse("https://cdn.example/cover.png")
	if err := validateCoverURL(parsed); err != nil {
		t.Fatalf("validateCoverURL(public) = %v, want nil", err)
	}
}

func TestDetectCoverImageType(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x01}, "image/png"},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}, "image/jpeg"},
		{"gif", []byte("GIF89a...."), "image/gif"},
		{"webp", []byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' '}, "image/webp"},
		{"avif", []byte{0, 0, 0, 0x20, 'f', 't', 'y', 'p', 'a', 'v', 'i', 'f', 0, 0, 0, 0}, "image/avif"},
		{"html", []byte("<html><script>alert(1)</script></html>"), ""},
		{"svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), ""},
		{"empty", nil, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := detectCoverImageType(test.data); got != test.want {
				t.Fatalf("detectCoverImageType() = %q, want %q", got, test.want)
			}
		})
	}
}
