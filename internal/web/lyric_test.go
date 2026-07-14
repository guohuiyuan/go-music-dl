package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLyricEndpointReturnsReadableFallbackWhenLyricMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterMusicRoutes(router.Group(RoutePrefix))

	req := httptest.NewRequest(http.MethodGet, RoutePrefix+"/lyric?id=test-id&source=missing", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	const want = "[00:00.00] 纯音乐 / 无歌词"
	if got := rec.Body.String(); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}
