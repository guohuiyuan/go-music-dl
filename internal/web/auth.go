package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"golang.org/x/crypto/bcrypt"
)

const (
	authCookieName      = "music_dl_session"
	sessionMaxAge       = 7 * 24 * time.Hour
	minAuthPasswordSize = 6
	setupTokenBytes     = 24
	loginLockBaseDelay  = time.Second
	loginLockMaxDelay   = time.Minute
)

type authSettingsProvider func() (core.WebAuthSettings, error)

var authRuntime = newAuthRuntimeState()

type sessionPayload struct {
	Username string `json:"u"`
	IssuedAt int64  `json:"iat"`
	Nonce    string `json:"n"`
}

type loginAttemptState struct {
	Failures    int
	LockedUntil time.Time
}

type authRuntimeState struct {
	mu            sync.Mutex
	setupToken    string
	loginAttempts map[string]loginAttemptState
}

func newAuthRuntimeState() *authRuntimeState {
	return &authRuntimeState{
		loginAttempts: make(map[string]loginAttemptState),
	}
}

func resetAuthRuntimeForTest() {
	authRuntime = newAuthRuntimeState()
}

func prepareSetupToken(settings core.WebAuthSettings) (string, error) {
	authRuntime.mu.Lock()
	defer authRuntime.mu.Unlock()

	if authConfigured(settings) {
		authRuntime.setupToken = ""
		return "", nil
	}
	if authRuntime.setupToken != "" {
		return authRuntime.setupToken, nil
	}
	token, err := randomToken(setupTokenBytes)
	if err != nil {
		return "", err
	}
	authRuntime.setupToken = token
	return token, nil
}

func currentSetupToken() string {
	authRuntime.mu.Lock()
	defer authRuntime.mu.Unlock()
	return authRuntime.setupToken
}

func consumeSetupToken() {
	authRuntime.mu.Lock()
	defer authRuntime.mu.Unlock()
	authRuntime.setupToken = ""
}

func loginAttemptKey(c *gin.Context, username string) string {
	ip := "unknown"
	if c != nil {
		ip = c.ClientIP()
	}
	return strings.ToLower(strings.TrimSpace(username)) + "|" + ip
}

func loginLockDelay(failures int) time.Duration {
	if failures <= 0 {
		return 0
	}
	delay := loginLockBaseDelay << min(failures-1, 6)
	if delay > loginLockMaxDelay {
		return loginLockMaxDelay
	}
	return delay
}

func loginLockedUntil(key string, now time.Time) (time.Time, bool) {
	authRuntime.mu.Lock()
	defer authRuntime.mu.Unlock()
	attempt := authRuntime.loginAttempts[key]
	if attempt.LockedUntil.After(now) {
		return attempt.LockedUntil, true
	}
	return time.Time{}, false
}

func recordLoginFailure(key string, now time.Time) time.Time {
	authRuntime.mu.Lock()
	defer authRuntime.mu.Unlock()

	attempt := authRuntime.loginAttempts[key]
	attempt.Failures++
	attempt.LockedUntil = now.Add(loginLockDelay(attempt.Failures))
	authRuntime.loginAttempts[key] = attempt
	return attempt.LockedUntil
}

func clearLoginFailures(key string) {
	authRuntime.mu.Lock()
	defer authRuntime.mu.Unlock()
	delete(authRuntime.loginAttempts, key)
}

func authConfigured(settings core.WebAuthSettings) bool {
	return strings.TrimSpace(settings.Username) != "" &&
		strings.TrimSpace(settings.PasswordHash) != "" &&
		strings.TrimSpace(settings.SessionSecret) != ""
}

func randomToken(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func createSessionValue(settings core.WebAuthSettings, now time.Time) (string, error) {
	nonce, err := randomToken(18)
	if err != nil {
		return "", err
	}
	payload := sessionPayload{
		Username: settings.Username,
		IssuedAt: now.Unix(),
		Nonce:    nonce,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(raw)
	signature := signSessionPayload(settings.SessionSecret, encodedPayload)
	return encodedPayload + "." + signature, nil
}

func signSessionPayload(secret string, encodedPayload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(encodedPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func validateSessionValue(settings core.WebAuthSettings, value string, now time.Time) bool {
	if !authConfigured(settings) {
		return false
	}

	parts := strings.Split(value, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}

	expectedSig := signSessionPayload(settings.SessionSecret, parts[0])
	if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(expectedSig)) != 1 {
		return false
	}

	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	var payload sessionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	if payload.Username != settings.Username || payload.IssuedAt <= 0 || strings.TrimSpace(payload.Nonce) == "" {
		return false
	}

	issuedAt := time.Unix(payload.IssuedAt, 0)
	return !issuedAt.After(now.Add(2*time.Minute)) && now.Sub(issuedAt) <= sessionMaxAge
}

func setAuthCookie(c *gin.Context, value string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, value, int(sessionMaxAge.Seconds()), RoutePrefix, "", c.Request.TLS != nil, true)
}

func clearAuthCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, "", -1, RoutePrefix, "", c.Request.TLS != nil, true)
}

func safeAuthRedirectTarget(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return RoutePrefix
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || strings.HasPrefix(raw, "//") {
		return RoutePrefix
	}
	if parsed.Path == "" {
		return RoutePrefix
	}
	if parsed.Path != RoutePrefix && !strings.HasPrefix(parsed.Path, RoutePrefix+"/") {
		return RoutePrefix
	}
	if parsed.Path == RoutePrefix+"/login" || parsed.Path == RoutePrefix+"/setup" {
		return RoutePrefix
	}
	return parsed.String()
}

func loginRedirectTarget(c *gin.Context) string {
	target := c.Request.URL.RequestURI()
	return RoutePrefix + "/login?next=" + url.QueryEscape(safeAuthRedirectTarget(target))
}

func wantsHTML(c *gin.Context) bool {
	if c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
		return false
	}
	accept := c.GetHeader("Accept")
	return accept == "" || strings.Contains(accept, "text/html")
}

func authRequired(provider authSettingsProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := provider()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "读取登录配置失败"})
			return
		}
		if !authConfigured(settings) {
			if wantsHTML(c) {
				c.Redirect(http.StatusFound, RoutePrefix+"/setup")
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "请先初始化管理员账号", "setupRequired": true})
			}
			c.Abort()
			return
		}

		value, err := c.Cookie(authCookieName)
		if err == nil && validateSessionValue(settings, value, time.Now()) {
			c.Set("AuthUsername", settings.Username)
			c.Next()
			return
		}

		clearAuthCookie(c)
		if wantsHTML(c) {
			c.Redirect(http.StatusFound, loginRedirectTarget(c))
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		}
		c.Abort()
	}
}

func renderAuthPage(c *gin.Context, mode string, errMsg string, username string) {
	title := "登录 music-dl"
	action := RoutePrefix + "/login"
	button := "登录"
	if mode == "setup" {
		title = "初始化管理员账号"
		action = RoutePrefix + "/setup"
		button = "创建账号"
	}

	c.HTML(http.StatusOK, "auth.html", gin.H{
		"Root":     RoutePrefix,
		"Title":    title,
		"Mode":     mode,
		"Action":   action,
		"Button":   button,
		"Error":    errMsg,
		"Username": username,
		"Next":     safeAuthRedirectTarget(c.Query("next")),
	})
}

func bindAuthRoutes(api *gin.RouterGroup) {
	api.GET("/setup", func(c *gin.Context) {
		settings, err := core.GetWebAuthSettings()
		if err != nil {
			renderAuthPage(c, "setup", "读取登录配置失败", core.DefaultWebAuthUsername)
			return
		}
		if authConfigured(settings) {
			c.Redirect(http.StatusFound, RoutePrefix+"/login")
			return
		}
		renderAuthPage(c, "setup", "", settings.Username)
	})

	api.POST("/setup", func(c *gin.Context) {
		settings, err := core.GetWebAuthSettings()
		if err != nil {
			renderAuthPage(c, "setup", "读取登录配置失败", core.DefaultWebAuthUsername)
			return
		}
		if authConfigured(settings) {
			c.Redirect(http.StatusFound, RoutePrefix+"/login")
			return
		}

		username := strings.TrimSpace(c.PostForm("username"))
		password := c.PostForm("password")
		confirm := c.PostForm("password_confirm")
		if username == "" {
			renderAuthPage(c, "setup", "请输入用户名", username)
			return
		}
		setupToken := currentSetupToken()
		if setupToken == "" || subtle.ConstantTimeCompare([]byte(c.PostForm("setup_token")), []byte(setupToken)) != 1 {
			renderAuthPage(c, "setup", "初始化令牌不正确，请查看启动终端输出", username)
			return
		}
		if len(password) < minAuthPasswordSize {
			renderAuthPage(c, "setup", fmt.Sprintf("密码至少需要 %d 位", minAuthPasswordSize), username)
			return
		}
		if password != confirm {
			renderAuthPage(c, "setup", "两次输入的密码不一致", username)
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			renderAuthPage(c, "setup", "创建密码哈希失败", username)
			return
		}
		secret, err := randomToken(32)
		if err != nil {
			renderAuthPage(c, "setup", "生成会话密钥失败", username)
			return
		}

		settings = core.WebAuthSettings{
			Username:      username,
			PasswordHash:  string(hash),
			SessionSecret: secret,
		}
		if err := core.SaveWebAuthSettings(settings); err != nil {
			renderAuthPage(c, "setup", "保存管理员账号失败", username)
			return
		}
		consumeSetupToken()
		sessionValue, err := createSessionValue(settings, time.Now())
		if err != nil {
			renderAuthPage(c, "setup", "创建登录会话失败", username)
			return
		}
		setAuthCookie(c, sessionValue)
		c.Redirect(http.StatusFound, safeAuthRedirectTarget(c.PostForm("next")))
	})

	api.GET("/login", func(c *gin.Context) {
		settings, err := core.GetWebAuthSettings()
		if err != nil {
			renderAuthPage(c, "login", "读取登录配置失败", "")
			return
		}
		if !authConfigured(settings) {
			c.Redirect(http.StatusFound, RoutePrefix+"/setup")
			return
		}
		if value, err := c.Cookie(authCookieName); err == nil && validateSessionValue(settings, value, time.Now()) {
			c.Redirect(http.StatusFound, safeAuthRedirectTarget(c.Query("next")))
			return
		}
		renderAuthPage(c, "login", "", "")
	})

	api.POST("/login", func(c *gin.Context) {
		settings, err := core.GetWebAuthSettings()
		if err != nil {
			renderAuthPage(c, "login", "读取登录配置失败", "")
			return
		}
		if !authConfigured(settings) {
			c.Redirect(http.StatusFound, RoutePrefix+"/setup")
			return
		}

		username := strings.TrimSpace(c.PostForm("username"))
		password := c.PostForm("password")
		attemptKey := loginAttemptKey(c, username)
		now := time.Now()
		if lockedUntil, locked := loginLockedUntil(attemptKey, now); locked {
			wait := int(time.Until(lockedUntil).Seconds()) + 1
			renderAuthPage(c, "login", fmt.Sprintf("登录失败次数过多，请 %d 秒后重试", wait), username)
			return
		}
		if !strings.EqualFold(username, settings.Username) ||
			bcrypt.CompareHashAndPassword([]byte(settings.PasswordHash), []byte(password)) != nil {
			lockedUntil := recordLoginFailure(attemptKey, now)
			wait := int(time.Until(lockedUntil).Seconds()) + 1
			if wait > 1 {
				renderAuthPage(c, "login", fmt.Sprintf("用户名或密码不正确，请 %d 秒后重试", wait), username)
				return
			}
			renderAuthPage(c, "login", "用户名或密码不正确", username)
			return
		}
		clearLoginFailures(attemptKey)

		sessionValue, err := createSessionValue(settings, time.Now())
		if err != nil {
			renderAuthPage(c, "login", "创建登录会话失败", username)
			return
		}
		setAuthCookie(c, sessionValue)
		c.Redirect(http.StatusFound, safeAuthRedirectTarget(c.PostForm("next")))
	})

	api.POST("/logout", func(c *gin.Context) {
		clearAuthCookie(c)
		c.Redirect(http.StatusFound, RoutePrefix)
	})
}
