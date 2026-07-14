package web

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

func qrLoginCookieString(result *model.QRLoginResult) string {
	if result == nil {
		return ""
	}
	if cookie := strings.TrimSpace(result.Cookie); cookie != "" {
		return cookie
	}
	if len(result.Cookies) == 0 {
		return ""
	}
	keys := make([]string, 0, len(result.Cookies))
	for k := range result.Cookies {
		if strings.TrimSpace(k) == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := strings.TrimSpace(result.Cookies[k])
		if v == "" {
			continue
		}
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "; ")
}

func qrLoginCookieSource(source string) string {
	if source == "qq_wx" {
		return "qq"
	}
	return source
}

func RegisterQRLoginRoutes(api *gin.RouterGroup) {
	api.POST("/qr_login/:source", func(c *gin.Context) {
		source := strings.TrimSpace(c.Param("source"))
		fn := core.GetQRLoginCreateFunc(source)
		if fn == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "unsupported qr login source"})
			return
		}
		session, err := fn()
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, session)
	})

	api.GET("/qr_login/:source", func(c *gin.Context) {
		source := strings.TrimSpace(c.Param("source"))
		key := strings.TrimSpace(c.Query("key"))
		if key == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing qr login key"})
			return
		}
		fn := core.GetQRLoginCheckFunc(source)
		if fn == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "unsupported qr login source"})
			return
		}
		result, err := fn(key)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		if result != nil && result.Status == model.QRLoginStatusSuccess {
			cookie := qrLoginCookieString(result)
			if cookie != "" {
				cookieSource := qrLoginCookieSource(source)
				result.Cookie = cookie
				core.CM.SetAll(map[string]string{cookieSource: cookie})
				core.CM.Save()
				if result.Extra == nil {
					result.Extra = make(map[string]string)
				}
				result.Extra["cookie_saved"] = "true"
				result.Extra["cookie_source"] = cookieSource
				result.Extra["cookie_length"] = strconv.Itoa(len(cookie))
			}
		}
		c.JSON(http.StatusOK, result)
	})
}
