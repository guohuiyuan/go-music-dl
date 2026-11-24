package core

import (
	"net/http"
	"testing"
)

func TestSourceCookieHostAllowed(t *testing.T) {
	tests := []struct {
		source string
		host   string
		want   bool
	}{
		{"netease", "music.163.com", true},
		{"netease", "m10.music.126.net", true},
		{"netease", "MUSIC.163.COM.", true},
		{"netease", "evil163.com", false},
		{"netease", "music.163.com.evil.example", false},
		{"netease", "127.0.0.1", false},
		{"netease", "attacker.example", false},
		{"qq", "y.qq.com", true},
		{"qq", "qq.com.evil.example", false},
		{"", "music.163.com", false},
		{"unknown", "music.163.com", false},
	}
	for _, test := range tests {
		if got := sourceCookieHostAllowed(test.source, test.host); got != test.want {
			t.Fatalf("sourceCookieHostAllowed(%q, %q) = %t, want %t", test.source, test.host, got, test.want)
		}
	}
}

func TestBuildSourceRequestDoesNotLeakCookieToArbitraryHost(t *testing.T) {
	oldCookies := CM.GetAll()
	CM.SetAll(map[string]string{"netease": "MUSIC_U=secret", "qq": "uin=1; qm_keyst=secret"})
	t.Cleanup(func() {
		CM.SetAll(map[string]string{"netease": oldCookies["netease"], "qq": oldCookies["qq"]})
	})

	tests := []struct {
		name       string
		url        string
		source     string
		wantCookie bool
	}{
		{name: "netease api host", url: "https://music.163.com/api/song/enhance/player/url", source: "netease", wantCookie: true},
		{name: "netease cdn host", url: "https://m10.music.126.net/song.flac", source: "netease", wantCookie: true},
		{name: "attacker host", url: "https://attacker.example/x.png", source: "netease", wantCookie: false},
		{name: "lookalike host", url: "https://evil163.com/x.png", source: "netease", wantCookie: false},
		{name: "suffix attack host", url: "https://music.163.com.attacker.example/x.png", source: "netease", wantCookie: false},
		{name: "ip host", url: "http://127.0.0.1:8080/x.png", source: "netease", wantCookie: false},
		{name: "qq trusted host", url: "https://u.y.qq.com/cgi-bin/musicu.fcg", source: "qq", wantCookie: true},
		{name: "qq attacker host", url: "https://attacker.example/x.png", source: "qq", wantCookie: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req, err := BuildSourceRequest(http.MethodGet, test.url, test.source, "")
			if err != nil {
				t.Fatalf("BuildSourceRequest: %v", err)
			}
			if got := req.Header.Get("Cookie") != ""; got != test.wantCookie {
				t.Fatalf("Cookie present = %t, want %t (url=%s)", got, test.wantCookie, test.url)
			}
		})
	}
}
