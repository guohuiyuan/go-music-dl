package core

import "strings"

// NetEase's player endpoint decides which quality tier a request may receive
// from the client identity stored in the Cookie. A bare MUSIC_U token (the
// usual result of copying cookies out of a browser) is treated as an unknown
// web client: every premium request is silently downgraded to the lossless
// label and lossless stream, even for an account that owns Hi-Res, spatial
// audio or 超清母带. Supplying the PC client fields makes the same account
// receive the requested tier (jymaster/hires/sky/vivid/dolby).
const (
	neteaseClientOS     = "pc"
	neteaseClientAppVer = "8.9.70"
)

// withNeteaseClientIdentity appends the client identity fields required by
// NetEase unless the user already supplied them. Values that are present but
// blank count as missing, and an empty cookie stays empty so anonymous
// requests do not pretend to be a logged-in desktop client.
func withNeteaseClientIdentity(cookie string) string {
	cookie = strings.TrimSpace(cookie)
	if cookie == "" {
		return ""
	}

	missing := make([]string, 0, 2)
	if !neteaseCookieHasValue(cookie, "os") {
		missing = append(missing, "os="+neteaseClientOS)
	}
	if !neteaseCookieHasValue(cookie, "appver") {
		missing = append(missing, "appver="+neteaseClientAppVer)
	}
	if len(missing) == 0 {
		return cookie
	}
	return cookie + "; " + strings.Join(missing, "; ")
}

func neteaseCookieHasValue(cookie, name string) bool {
	for _, part := range strings.Split(cookie, ";") {
		key, value, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(key), name) && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}
