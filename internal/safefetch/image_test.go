package safefetch

import (
	"errors"
	"net"
	"net/url"
	"testing"
)

func TestValidateURLRejectsUnsafeTargets(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1/x.png",
		"http://[::1]/x.png",
		"http://localhost/x.png",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1/x.png",
		"file:///etc/passwd",
		"ftp://example.com/x.png",
		"http://user:pass@example.com/x.png",
	} {
		parsed, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if err := ValidateURL(parsed); err == nil {
			t.Fatalf("ValidateURL(%q) = nil, want error", raw)
		}
	}
	parsed, _ := url.Parse("https://cdn.example/cover.png")
	if err := ValidateURL(parsed); err != nil {
		t.Fatalf("ValidateURL(public) = %v, want nil", err)
	}
}

func TestFetchImageRejectsPrivateURLWithoutRequest(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1:9/cover.png",
		"http://localhost:9/cover.png",
		"http://169.254.169.254/latest/meta-data/",
	} {
		if _, _, err := FetchImage(raw, "", "", 0); !errors.Is(err, ErrUnsafeURL) {
			t.Fatalf("FetchImage(%q) error = %v, want ErrUnsafeURL", raw, err)
		}
	}
}

func TestIsPublicIP(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"8.8.8.8", true},
		{"127.0.0.1", false},
		{"::ffff:127.0.0.1", false},
		{"::ffff:10.0.0.1", false},
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
		{"::1", false},
		{"fc00::1", false},
		{"fe80::1", false},
		{"2001:db8::1", false},
		{"2001:4860:4860::8888", true},
	}
	for _, test := range tests {
		if got := IsPublicIP(net.ParseIP(test.ip)); got != test.want {
			t.Fatalf("IsPublicIP(%s) = %t, want %t", test.ip, got, test.want)
		}
	}
}

func TestDetectImageTypeRejectsMarkup(t *testing.T) {
	if got := DetectImageType([]byte("<html><script>alert(1)</script></html>")); got != "" {
		t.Fatalf("DetectImageType(html) = %q, want empty", got)
	}
	if got := DetectImageType([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x01}); got != "image/png" {
		t.Fatalf("DetectImageType(png) = %q, want image/png", got)
	}
}
