package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
)

func TestPrepareSetupTokenLifecycle(t *testing.T) {
	resetAuthRuntimeForTest()
	t.Cleanup(resetAuthRuntimeForTest)

	token, err := prepareSetupToken(core.WebAuthSettings{Username: core.DefaultWebAuthUsername})
	if err != nil {
		t.Fatalf("prepare setup token: %v", err)
	}
	if token == "" {
		t.Fatal("setup token should not be empty")
	}
	if got := currentSetupToken(); got != token {
		t.Fatalf("current setup token = %q, want %q", got, token)
	}

	again, err := prepareSetupToken(core.WebAuthSettings{Username: core.DefaultWebAuthUsername})
	if err != nil {
		t.Fatalf("prepare setup token again: %v", err)
	}
	if again != token {
		t.Fatalf("setup token changed before consumption: got %q want %q", again, token)
	}

	consumeSetupToken()
	if got := currentSetupToken(); got != "" {
		t.Fatalf("setup token should be consumed, got %q", got)
	}

	configuredToken, err := prepareSetupToken(core.WebAuthSettings{
		Username:      "owner",
		PasswordHash:  "hash",
		SessionSecret: "secret",
	})
	if err != nil {
		t.Fatalf("prepare configured setup token: %v", err)
	}
	if configuredToken != "" {
		t.Fatalf("configured auth should not have setup token, got %q", configuredToken)
	}
}

func TestSessionValueValidation(t *testing.T) {
	settings := core.WebAuthSettings{
		Username:      "owner",
		PasswordHash:  "hash",
		SessionSecret: "secret",
	}
	now := time.Unix(1000, 0)

	value, err := createSessionValue(settings, now)
	if err != nil {
		t.Fatalf("create session value: %v", err)
	}
	if !validateSessionValue(settings, value, now.Add(time.Minute)) {
		t.Fatal("fresh session should be valid")
	}
	if validateSessionValue(settings, value+"x", now.Add(time.Minute)) {
		t.Fatal("tampered session should be invalid")
	}
	if validateSessionValue(settings, value, now.Add(sessionMaxAge+time.Second)) {
		t.Fatal("expired session should be invalid")
	}

	otherSettings := settings
	otherSettings.SessionSecret = "other-secret"
	if validateSessionValue(otherSettings, value, now.Add(time.Minute)) {
		t.Fatal("session signed with another secret should be invalid")
	}
}

func TestLoginFailureLocksAndClears(t *testing.T) {
	resetAuthRuntimeForTest()
	t.Cleanup(resetAuthRuntimeForTest)

	now := time.Unix(1000, 0)
	key := "owner|127.0.0.1"
	firstLockedUntil := recordLoginFailure(key, now)
	if firstLockedUntil.Sub(now) != loginLockBaseDelay {
		t.Fatalf("first lock delay = %s, want %s", firstLockedUntil.Sub(now), loginLockBaseDelay)
	}
	if got, locked := loginLockedUntil(key, now.Add(500*time.Millisecond)); !locked || !got.Equal(firstLockedUntil) {
		t.Fatalf("login should be locked until %s, got %s locked=%v", firstLockedUntil, got, locked)
	}
	if _, locked := loginLockedUntil(key, firstLockedUntil.Add(time.Millisecond)); locked {
		t.Fatal("expired lock should not remain locked")
	}

	secondLockedUntil := recordLoginFailure(key, firstLockedUntil.Add(time.Millisecond))
	if secondLockedUntil.Sub(firstLockedUntil.Add(time.Millisecond)) != 2*loginLockBaseDelay {
		t.Fatalf("second lock delay = %s, want %s", secondLockedUntil.Sub(firstLockedUntil.Add(time.Millisecond)), 2*loginLockBaseDelay)
	}
	clearLoginFailures(key)
	if _, locked := loginLockedUntil(key, secondLockedUntil.Add(-time.Millisecond)); locked {
		t.Fatal("cleared failures should unlock login")
	}
}

func TestSafeAuthRedirectTarget(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "", want: RoutePrefix},
		{raw: "/music/search?q=test", want: "/music/search?q=test"},
		{raw: "/music/login", want: RoutePrefix},
		{raw: "/music/setup", want: RoutePrefix},
		{raw: "/other", want: RoutePrefix},
		{raw: "https://example.com/music", want: RoutePrefix},
		{raw: "//example.com/music", want: RoutePrefix},
	}

	for _, tt := range tests {
		if got := safeAuthRedirectTarget(tt.raw); got != tt.want {
			t.Fatalf("safeAuthRedirectTarget(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestAllowSaveLocalRequestRequiresPostAndSameOriginXHR(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		method     string
		origin     string
		xrw        string
		wantStatus int
		wantAllow  bool
	}{
		{name: "get rejected", method: http.MethodGet, xrw: "XMLHttpRequest", wantStatus: http.StatusMethodNotAllowed},
		{name: "missing xhr rejected", method: http.MethodPost, wantStatus: http.StatusForbidden},
		{name: "cross origin rejected", method: http.MethodPost, xrw: "XMLHttpRequest", origin: "https://evil.example", wantStatus: http.StatusForbidden},
		{name: "same origin allowed", method: http.MethodPost, xrw: "XMLHttpRequest", origin: "http://example.test", wantAllow: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			req := httptest.NewRequest(tt.method, "http://example.test"+RoutePrefix+"/download?save_local=1", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.xrw != "" {
				req.Header.Set("X-Requested-With", tt.xrw)
			}
			c.Request = req

			gotAllow := allowSaveLocalRequest(c)
			if gotAllow != tt.wantAllow {
				t.Fatalf("allowSaveLocalRequest = %v, want %v", gotAllow, tt.wantAllow)
			}
			if !tt.wantAllow && rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestAuthRequiredRedirectsWhenSetupMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(authRequired(func() (core.WebAuthSettings, error) {
		return core.WebAuthSettings{Username: core.DefaultWebAuthUsername}, nil
	}))
	router.GET(RoutePrefix, func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, RoutePrefix, nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if got := rec.Header().Get("Location"); got != RoutePrefix+"/setup" {
		t.Fatalf("Location = %q, want setup", got)
	}
}

func TestAuthRequiredAllowsSignedSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := core.WebAuthSettings{
		Username:      "owner",
		PasswordHash:  "hash",
		SessionSecret: "secret",
	}
	value, err := createSessionValue(settings, time.Now())
	if err != nil {
		t.Fatalf("create session value: %v", err)
	}

	router := gin.New()
	router.Use(authRequired(func() (core.WebAuthSettings, error) {
		return settings, nil
	}))
	router.GET(RoutePrefix, func(c *gin.Context) {
		username, _ := c.Get("AuthUsername")
		c.String(http.StatusOK, username.(string))
	})

	req := httptest.NewRequest(http.MethodGet, RoutePrefix, nil)
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: value})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "owner" {
		t.Fatalf("body = %q, want owner", rec.Body.String())
	}
}
