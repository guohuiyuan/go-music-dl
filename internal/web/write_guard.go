package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

func wantsSaveLocal(c *gin.Context) bool {
	return c != nil && strings.TrimSpace(c.Query("save_local")) == "1"
}

func allowSameOriginWrite(c *gin.Context) bool {
	if c == nil {
		return false
	}
	if c.GetHeader("X-Requested-With") != "XMLHttpRequest" {
		return false
	}

	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return strings.EqualFold(parsed.Host, c.Request.Host)
	}

	secFetchSite := strings.TrimSpace(strings.ToLower(c.GetHeader("Sec-Fetch-Site")))
	return secFetchSite == "" || secFetchSite == "same-origin" || secFetchSite == "same-site" || secFetchSite == "none"
}

func allowSaveLocalRequest(c *gin.Context) bool {
	if !wantsSaveLocal(c) {
		return false
	}
	if c.Request.Method != http.MethodPost {
		c.AbortWithStatusJSON(http.StatusMethodNotAllowed, gin.H{"error": "save_local requires POST"})
		return false
	}
	if !allowSameOriginWrite(c) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return false
	}
	return true
}
