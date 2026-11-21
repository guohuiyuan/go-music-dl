package core

import (
	"strings"
	"testing"
)

func TestWithNeteaseClientIdentity(t *testing.T) {
	tests := []struct {
		name   string
		cookie string
		want   []string
		absent []string
	}{
		{
			name:   "empty cookie stays empty",
			cookie: "  ",
			absent: []string{"os=", "appver="},
		},
		{
			name:   "adds missing client fields",
			cookie: "MUSIC_U=abc; __csrf=def",
			want:   []string{"MUSIC_U=abc", "__csrf=def", "os=pc", "appver=8.9.70"},
		},
		{
			name:   "keeps user supplied values",
			cookie: "MUSIC_U=abc; os=android; appver=9.1.20",
			want:   []string{"os=android", "appver=9.1.20"},
			absent: []string{"os=pc", "appver=8.9.70"},
		},
		{
			name:   "fills blank values",
			cookie: "MUSIC_U=abc; os=pc; appver=",
			want:   []string{"os=pc", "appver=8.9.70"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := withNeteaseClientIdentity(test.cookie)
			for _, want := range test.want {
				if !strings.Contains(got, want) {
					t.Fatalf("withNeteaseClientIdentity(%q) = %q, missing %q", test.cookie, got, want)
				}
			}
			for _, absent := range test.absent {
				if strings.Contains(got, absent) {
					t.Fatalf("withNeteaseClientIdentity(%q) = %q, unexpectedly contains %q", test.cookie, got, absent)
				}
			}
			if test.cookie == "  " && got != "" {
				t.Fatalf("empty cookie should stay empty, got %q", got)
			}
		})
	}
}

func TestCookieManagerGetNormalizesNeteaseCookie(t *testing.T) {
	oldCookies := CM.GetAll()
	t.Cleanup(func() {
		CM.mu.Lock()
		CM.cookies = oldCookies
		CM.mu.Unlock()
	})

	CM.mu.Lock()
	CM.cookies = map[string]string{
		"netease": "MUSIC_U=abc",
		"qq":      "uin=1; qm_keyst=xyz",
	}
	CM.mu.Unlock()

	if got := CM.Get("netease"); got != "MUSIC_U=abc; os=pc; appver=8.9.70" {
		t.Fatalf("CM.Get(netease) = %q, want client identity appended", got)
	}
	if got := CM.Get("qq"); got != "uin=1; qm_keyst=xyz" {
		t.Fatalf("CM.Get(qq) = %q, want untouched cookie", got)
	}
}
