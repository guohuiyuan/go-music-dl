package soda

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/guohuiyuan/music-lib/model"
)

const (
	sodaQRCreateAPI    = "https://api.qishui.com/passport/web/get_qrcode/"
	sodaQRCheckAPI     = "https://api.qishui.com/passport/web/check_qrconnect/"
	sodaSendCodeAPI    = "https://api.qishui.com/passport/web/send_code/"
	sodaValidateAPI    = "https://api.qishui.com/passport/web/validate_code/"
	sodaUpSMSVerifyAPI = "https://api.qishui.com/passport/upsms/verify/"
	sodaPassportUA     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) SodaMusic/3.1.0 Chrome/136.0.7103.59 Electron/36.4.0-rs.22.release.main.1 TTElectron/36.4.0-rs.22.release.main.1 Safari/537.36"
	sodaPassportJSVer  = "2.4.13"
	sodaPassportAid    = "386088"
	sodaVersionCode    = "3.3.0"
	sodaPZT            = "3.3.5"
	sodaPVer           = "1.0.29"
	sodaPBD            = "1.0.0.41"
)

type sodaQRLoginPendingState struct {
	Cookies      map[string]string
	QRIndexURL   string
	EncryptUID   string
	VerifyParams string
	Mobile       string
	UpSMSMobile  string
	UpSMSContent string
	SMSMode      string
	CanUpSMS     bool
	Status       model.QRLoginStatus
	Message      string
	ExpiresAt    time.Time
}

type sodaQRConnectResponse struct {
	Data struct {
		Status      string `json:"status"`
		ErrorCode   int    `json:"error_code"`
		Redirect    string `json:"redirect_url"`
		Description string `json:"description"`
		AccountFlow string `json:"account_flow"`
		UserData    struct {
			Mobile string `json:"mobile"`
		} `json:"user_data"`
	} `json:"data"`
	Message string `json:"message"`
}

var (
	sodaQRLoginMu      sync.Mutex
	sodaQRLoginPending = map[string]sodaQRLoginPendingState{}
)

const (
	sodaQRPollMinInterval  = 2 * time.Second
	sodaQRRateLimitBackoff = 60 * time.Second
)

var (
	sodaQRPollMu   sync.Mutex
	sodaQRLastPoll = map[string]time.Time{}
)

// sodaQRPollAllowed reports whether enough time has elapsed since the last
// check_qrconnect call for the given token. It also records the call time.
func sodaQRPollAllowed(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return true
	}
	sodaQRPollMu.Lock()
	defer sodaQRPollMu.Unlock()
	now := time.Now()
	for tok, last := range sodaQRLastPoll {
		if now.Sub(last) > 10*time.Minute {
			delete(sodaQRLastPoll, tok)
		}
	}
	if last, ok := sodaQRLastPoll[token]; ok && now.Sub(last) < sodaQRPollMinInterval {
		return false
	}
	sodaQRLastPoll[token] = now
	return true
}

func sodaQRPollForget(token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	sodaQRPollMu.Lock()
	defer sodaQRPollMu.Unlock()
	delete(sodaQRLastPoll, token)
}

// sodaQRPollBackoff suppresses further upstream polling for the given token
// until `duration` has elapsed. Used when Soda returns error_code=7.
func sodaQRPollBackoff(token string, duration time.Duration) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	sodaQRPollMu.Lock()
	defer sodaQRPollMu.Unlock()
	// Record a future "last poll" so the next sodaQRPollAllowed call sees
	// a negative elapsed window and stays throttled for `duration`.
	sodaQRLastPoll[token] = time.Now().Add(duration - sodaQRPollMinInterval)
}

func CreateQRLogin() (*model.QRLoginSession, error) { return defaultSoda.CreateQRLogin() }
func CheckQRLogin(key string) (*model.QRLoginResult, error) {
	return defaultSoda.CheckQRLogin(key)
}

// CreateQRLogin creates a QR login session for Soda.
func (s *Soda) CreateQRLogin() (*model.QRLoginSession, error) {
	cleanupSodaQRLoginPending()

	createURL := sodaQRCreateAPI + "?" + buildSodaQRCreateQuery()
	req, err := http.NewRequest("GET", createURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", sodaPassportUA)
	req.Header.Set("Accept", "application/json, text/javascript")

	client := &http.Client{Timeout: 30 * time.Second}
	httpResp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	// Capture cookies from get_qrcode response (passport_csrf_token etc.)
	createCookies := make(map[string]string)
	for _, c := range httpResp.Cookies() {
		if c.Name != "" && c.Value != "" {
			createCookies[c.Name] = c.Value
		}
	}

	var resp struct {
		Data struct {
			Token    string `json:"token"`
			QRCode   string `json:"qrcode"`
			WebURL   string `json:"web_url"`
			QRCodeB  string `json:"qrcode_index_url"`
			Frontier bool   `json:"is_frontier"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda qr create json error: %w", err)
	}
	if resp.Data.Token == "" {
		return nil, fmt.Errorf("soda qr create failed: %s", resp.Message)
	}

	scanURL := sodaScanLoginURL(resp.Data.Token)
	imageURL := sodaQRCodeImageURL(resp.Data.QRCode)
	displayQRSource := "qrcode_index_url"
	if imageURL != "" {
		displayQRSource = "qrcode_base64"
	}

	sodaQRDebugLogf("[DEBUG get_qrcode] token=%s\n", sodaRedactValue(resp.Data.Token))
	sodaQRDebugLogf("[DEBUG get_qrcode] web_url=%s\n", sodaRedactURLSecrets(resp.Data.WebURL))
	sodaQRDebugLogf("[DEBUG get_qrcode] qrcode_index_url=%s\n", sodaRedactURLSecrets(resp.Data.QRCodeB))
	sodaQRDebugLogf("[DEBUG get_qrcode] custom_scan_url=%s\n", sodaRedactURLSecrets(scanURL))

	// Prefer official qrcode_index_url for Soda PC QR login.
	qrURL := strings.TrimSpace(resp.Data.QRCodeB)
	if qrURL == "" {
		qrURL = strings.TrimSpace(resp.Data.WebURL)
	}
	if qrURL == "" {
		qrURL = strings.TrimSpace(scanURL)
	}

	// Store cookies from get_qrcode so check_qrconnect can use them
	if len(createCookies) > 0 {
		rememberSodaQRLoginPending(resp.Data.Token, sodaQRLoginPendingState{
			Cookies:    createCookies,
			QRIndexURL: resp.Data.QRCodeB,
			ExpiresAt:  time.Now().Add(10 * time.Minute),
		})
	}

	return &model.QRLoginSession{
		Source:    "soda",
		Key:       resp.Data.Token,
		URL:       qrURL,
		ImageURL:  imageURL,
		ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
		Extra: map[string]string{
			"token":             resp.Data.Token,
			"qrcode_index_url":  resp.Data.QRCodeB,
			"scan_login_url":    scanURL,
			"is_frontier":       strconvBool(resp.Data.Frontier),
			"raw_qrcode_image":  strconvBool(resp.Data.QRCode != ""),
			"display_qr_source": displayQRSource,
		},
	}, nil
}

// CheckQRLogin checks the QR scan status and handles the MFA SMS flow.
// The key format is: "token" for initial check, "token|send_code|encrypt_uid|verify_params" for send_code,
// "token|validate|encrypt_uid|verify_params|code" for validate.
func (s *Soda) CheckQRLogin(key string) (*model.QRLoginResult, error) {
	parts := strings.SplitN(key, "|", 5)

	switch {
	case len(parts) >= 5 && parts[1] == "validate":
		return s.sodaValidateCode(parts[0], parts[2], parts[3], parts[4])
	case len(parts) >= 4 && parts[1] == "up_sms":
		return s.sodaVerifyUpSMS(parts[0], parts[2], parts[3])
	case len(parts) >= 4 && parts[1] == "send_code":
		return s.sodaSendCode(parts[0], parts[2], parts[3])
	default:
		return s.sodaCheckQRConnect(parts[0])
	}
}

func (s *Soda) sodaCheckQRConnect(token string) (*model.QRLoginResult, error) {
	pending, _ := getSodaQRLoginPending(token)
	if !sodaQRPollAllowed(token) {
		return sodaThrottledResult(token, pending), nil
	}
	return s.sodaCheckQRConnectWithState(token, pending, false)
}

// sodaThrottledResult returns the most useful cached state when the caller
// polls too often. It mirrors the previous /check_qrconnect outcome instead of
// hitting Soda again and tripping error_code=7.
func sodaThrottledResult(token string, pending sodaQRLoginPendingState) *model.QRLoginResult {
	if pending.EncryptUID != "" {
		result := &model.QRLoginResult{
			Source:  "soda",
			Key:     strings.TrimSpace(token),
			Status:  model.QRLoginStatusScanned,
			Message: "扫码成功，需要短信验证",
			Extra: map[string]string{
				"need_sms":      "true",
				"encrypt_uid":   pending.EncryptUID,
				"verify_params": pending.VerifyParams,
				"throttled":     "true",
			},
		}
		if pending.Mobile != "" {
			result.Extra["mobile"] = pending.Mobile
		}
		if pending.UpSMSMobile != "" || pending.UpSMSContent != "" {
			result.Extra["up_sms_mobile"] = pending.UpSMSMobile
			result.Extra["up_sms_content"] = pending.UpSMSContent
			switch pending.SMSMode {
			case "sms":
				result.Extra["sms_mode"] = "sms"
				if pending.CanUpSMS {
					result.Extra["can_up_sms"] = "true"
				}
			default:
				result.Extra["sms_mode"] = "up"
				result.Extra["need_user_sms"] = "true"
			}
		}
		return result
	}
	if pending.Status == model.QRLoginStatusScanned {
		message := strings.TrimSpace(pending.Message)
		if message == "" {
			message = "已扫码，请在手机上确认"
		}
		return &model.QRLoginResult{
			Source:  "soda",
			Key:     strings.TrimSpace(token),
			Status:  model.QRLoginStatusScanned,
			Message: message,
			Extra: map[string]string{
				"throttled":     "true",
				"cached_status": "true",
			},
		}
	}
	return &model.QRLoginResult{
		Source:  "soda",
		Key:     strings.TrimSpace(token),
		Status:  model.QRLoginStatusWaiting,
		Message: "等待扫码确认",
		Extra: map[string]string{
			"throttled": "true",
		},
	}
}

func (s *Soda) sodaCheckQRConnectWithState(token string, state sodaQRLoginPendingState, includeVerifyParams bool) (*model.QRLoginResult, error) {
	token = strings.TrimSpace(token)
	params := sodaQRConnectForm(token)
	if includeVerifyParams {
		applySodaVerifyParams(params, state.VerifyParams)
		if _, ok := params["std_verify_way"]; !ok {
			params.Set("std_verify_way", "")
		}
	}

	apiURL := sodaQRCheckAPI + "?" + buildSodaQRCheckQuery()

	body, cookies, err := s.postSodaPassportWithCookie(apiURL, params, sodaCookieHeader(s.cookie, state.Cookies))
	if err != nil {
		return nil, err
	}

	if len(body) < 2000 {
		sodaQRDebugLogf("  [DEBUG check_qrconnect] raw=%s\n", string(body))
	} else {
		sodaQRDebugLogf("  [DEBUG check_qrconnect] raw(first 500)=%s\n", string(body[:500]))
	}

	var resp sodaQRConnectResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda qr check json error: %w", err)
	}

	return s.sodaQRConnectResult(token, body, mergeSodaCookies(state.Cookies, cookies), resp), nil
}

func (s *Soda) sodaQRConnectResult(token string, body []byte, cookies map[string]string, resp sodaQRConnectResponse) *model.QRLoginResult {
	result := &model.QRLoginResult{
		Source:  "soda",
		Key:     token,
		Message: resp.Message,
	}

	// Handle rate limiting: instead of failing the whole session, back off
	// upstream calls so the frontend can keep polling without hitting Soda.
	if resp.Data.ErrorCode == 7 {
		pending, _ := getSodaQRLoginPending(token)
		// 用户扫码/确认后，汽水可能要求先访问授权 URL（sdk-next）才会发放 session cookie。
		// 汽水可能直接返回 error_code=7（不经过 scanned），因此只要持有授权地址就尝试访问。
		if strings.TrimSpace(pending.QRIndexURL) != "" {
			if fetched := s.sodaFetchQRIndexSession(token, pending); fetched != nil {
				return fetched
			}
		}
		sodaQRPollBackoff(token, sodaQRRateLimitBackoff)
		throttled := sodaThrottledResult(token, pending)
		if throttled.Extra == nil {
			throttled.Extra = map[string]string{}
		}
		throttled.Extra["rate_limited"] = "true"
		if throttled.Status == model.QRLoginStatusWaiting {
			throttled.Message = "正在等待汽水接口冷却..."
		}
		return throttled
	}

	// Final success: cookies carry a real session
	if sodaCookiesHaveSession(cookies) {
		cookie := sodaJoinCookies(cookies)
		result.Status = model.QRLoginStatusSuccess
		result.Cookie = cookie
		result.Cookies = cookies
		result.Message = "登录成功"
		s.cookie = cookie
		s.isVipCache = nil
		clearSodaQRLoginPending(token)
		sodaQRPollForget(token)
		return result
	}

	// MFA branch: official check_qrconnect returns no `status` but
	// account_flow=verify and error_code=2046 with encrypt_uid + biz_params.
	status := strings.ToLower(strings.TrimSpace(resp.Data.Status))
	accountFlow := strings.ToLower(strings.TrimSpace(resp.Data.AccountFlow))
	if accountFlow == "verify" || resp.Data.ErrorCode == 2046 {
		if mfaResult, ok := s.sodaMFARequiredResult(token, body, cookies, resp); ok {
			return mfaResult
		}
		result.Status = model.QRLoginStatusFailed
		result.Message = sodaQRConnectErrorMessage(resp)
		result.Extra = sodaQRConnectExtra(resp)
		return result
	}

	switch status {
	case "confirmed":
		// QR confirmed on phone but session cookies not yet issued.
		// Caller should keep polling; the next check_qrconnect typically
		// returns Set-Cookie with sessionid.
		result.Status = model.QRLoginStatusScanned
		result.Message = "已扫码确认，等待登录结果"
		oldPending, _ := getSodaQRLoginPending(token)
		rememberSodaQRLoginPending(token, sodaQRLoginPendingState{
			Cookies:    mergeSodaCookies(oldPending.Cookies, cookies),
			QRIndexURL: oldPending.QRIndexURL,
			Status:     model.QRLoginStatusScanned,
			Message:    result.Message,
			ExpiresAt:  time.Now().Add(10 * time.Minute),
		})
	case "new", "":
		if resp.Data.ErrorCode != 0 {
			result.Status = model.QRLoginStatusFailed
			result.Message = sodaQRConnectErrorMessage(resp)
			result.Extra = sodaQRConnectExtra(resp)
		} else if pending, ok := getSodaQRLoginPending(token); ok && pending.Status == model.QRLoginStatusScanned {
			result.Status = model.QRLoginStatusScanned
			result.Message = strings.TrimSpace(pending.Message)
			if result.Message == "" {
				result.Message = "已扫码，请在手机上确认"
			}
			result.Extra = map[string]string{"cached_status": "true"}
		} else {
			result.Status = model.QRLoginStatusWaiting
		}
	case "scanned":
		result.Status = model.QRLoginStatusScanned
		result.Message = "已扫码，请在手机上确认"
		oldPending, _ := getSodaQRLoginPending(token)
		rememberSodaQRLoginPending(token, sodaQRLoginPendingState{
			Cookies:    mergeSodaCookies(oldPending.Cookies, cookies),
			QRIndexURL: oldPending.QRIndexURL,
			Status:     model.QRLoginStatusScanned,
			Message:    result.Message,
			ExpiresAt:  time.Now().Add(10 * time.Minute),
		})
	case "expired":
		result.Status = model.QRLoginStatusExpired
		clearSodaQRLoginPending(token)
		sodaQRPollForget(token)
	case "error", "failed":
		if mfaResult, ok := s.sodaMFARequiredResult(token, body, cookies, resp); ok {
			return mfaResult
		}
		result.Status = model.QRLoginStatusFailed
		result.Message = sodaQRConnectErrorMessage(resp)
		result.Extra = sodaQRConnectExtra(resp)
	default:
		if resp.Data.ErrorCode != 0 {
			if mfaResult, ok := s.sodaMFARequiredResult(token, body, cookies, resp); ok {
				return mfaResult
			}
			result.Status = model.QRLoginStatusFailed
			result.Message = sodaQRConnectErrorMessage(resp)
			result.Extra = sodaQRConnectExtra(resp)
		} else {
			result.Status = model.QRLoginStatusWaiting
		}
	}

	return result
}

// sodaFetchQRIndexSession 在扫码确认后尝试访问授权地址换取 session cookie。
// 优先 light 客户端接口（light/invoke/scan_login，JSON 且会种 cookie），
// 失败再试 Web 授权页（qrcode_index_url / sdk-next）。
func (s *Soda) sodaFetchQRIndexSession(token string, pending sodaQRLoginPendingState) *model.QRLoginResult {
	candidates := []string{sodaScanLoginURL(token), pending.QRIndexURL}
	for _, target := range candidates {
		if strings.TrimSpace(target) == "" {
			continue
		}
		if result := s.sodaFetchURLForSession(token, pending, target); result != nil {
			return result
		}
	}
	return nil
}

// captureCookieJar 捕获重定向链上所有 Set-Cookie。
type captureCookieJar struct {
	mu      sync.Mutex
	cookies map[string]*http.Cookie
}

func newCaptureCookieJar() *captureCookieJar {
	return &captureCookieJar{cookies: map[string]*http.Cookie{}}
}

func (j *captureCookieJar) SetCookies(u *url.URL, cs []*http.Cookie) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, c := range cs {
		if c != nil && c.Name != "" {
			j.cookies[c.Name] = c
		}
	}
}

func (j *captureCookieJar) Cookies(u *url.URL) []*http.Cookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]*http.Cookie, 0, len(j.cookies))
	for _, c := range j.cookies {
		out = append(out, c)
	}
	return out
}

// sodaFetchURLForSession 访问单个授权地址（跟随重定向），用 Cookie Jar 捕获链上所有 cookie；
// 含 session 时返回成功结果，否则返回 nil。
func (s *Soda) sodaFetchURLForSession(token string, pending sodaQRLoginPendingState, target string) *model.QRLoginResult {
	sodaQRDebugLogf("  [DEBUG fetch_qr_index] GET %s\n", sodaRedactURLSecrets(target))

	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil
	}
	for k, v := range pending.Cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36")
	req.Header.Set("Referer", "https://bff-pc.qishui.com/")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	jar := newCaptureCookieJar()
	client := &http.Client{
		Timeout:   20 * time.Second,
		Jar:       jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 8 {
				return http.ErrUseLastResponse
			}
			return nil // 跟随重定向，收集 Set-Cookie
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		sodaQRDebugLogf("  [DEBUG fetch_qr_index] error: %v\n", err)
		return nil
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if len(bodyBytes) > 0 {
		bodySnippet := string(bodyBytes)
		if len(bodySnippet) > 500 {
			bodySnippet = bodySnippet[:500]
		}
		sodaQRDebugLogf("  [DEBUG fetch_qr_index] status=%d body=%s\n", resp.StatusCode, bodySnippet)
	}

	cookies := make(map[string]string)
	for _, c := range jar.Cookies(req.URL) {
		if c.Name != "" && c.Value != "" {
			cookies[c.Name] = c.Value
		}
	}

	// 部分接口把会话信息放在 JSON body 而非 Set-Cookie：提取常见字段作为 cookie 候选
	for k, v := range extractSodaSessionFromBody(bodyBytes) {
		if _, exists := cookies[k]; !exists && v != "" {
			cookies[k] = v
		}
	}
	if len(cookies) == 0 {
		sodaQRDebugLogf("  [DEBUG fetch_qr_index] status=%d, no new cookies\n", resp.StatusCode)
		return nil
	}

	merged := mergeSodaCookies(pending.Cookies, cookies)
	if sodaCookiesHaveSession(merged) {
		cookie := sodaJoinCookies(merged)
		sodaQRDebugLogf("  [DEBUG fetch_qr_index] got session cookie from %s\n", sodaRedactURLSecrets(target))
		result := &model.QRLoginResult{
			Source:  "soda",
			Key:     token,
			Status:  model.QRLoginStatusSuccess,
			Cookie:  cookie,
			Cookies: merged,
			Message: "登录成功",
		}
		s.cookie = cookie
		s.isVipCache = nil
		clearSodaQRLoginPending(token)
		sodaQRPollForget(token)
		return result
	}
	sodaQRDebugLogf("  [DEBUG fetch_qr_index] status=%d, no session in cookies, keep waiting\n", resp.StatusCode)
	return nil
}

// extractSodaSessionFromBody 从响应 JSON body 中提取常见的会话字段（部分接口经 body 返回登录态）。
func extractSodaSessionFromBody(body []byte) map[string]string {
	out := map[string]string{}
	if len(body) == 0 {
		return out
	}
	var raw interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return out
	}
	for _, field := range []string{"session_key", "sessionid", "sid_tt", "session", "passport_csrf_token", "token", "sid_guard", "uid_tt", "store-idc"} {
		if v := findSodaJSONString(raw, field); v != "" {
			out[field] = v
		}
	}
	return out
}

func (s *Soda) sodaMFARequiredResult(token string, body []byte, cookies map[string]string, resp sodaQRConnectResponse) (*model.QRLoginResult, bool) {	mfaToken := extractSodaCookieValue(cookies, "passport_mfa_token")
	encryptUID := extractSodaMFAField(body, "encrypt_uid")
	verifyParams := extractSodaMFAVerifyParams(body, cookies)
	if mfaToken == "" && encryptUID == "" && verifyParams == "" {
		return nil, false
	}

	mobile := extractSodaMFAField(body, "mobile")
	if mobile == "" {
		mobile = strings.TrimSpace(resp.Data.UserData.Mobile)
	}
	upSMSMobile := extractSodaMFAField(body, "channel_mobile")
	upSMSContent := extractSodaMFAField(body, "sms_content")
	hasMobileSMSVerify := sodaBodyContainsVerifyWay(body, "mobile_sms_verify")

	rememberSodaQRLoginPending(token, sodaQRLoginPendingState{
		Cookies:      cookies,
		EncryptUID:   encryptUID,
		VerifyParams: verifyParams,
		Mobile:       mobile,
		UpSMSMobile:  upSMSMobile,
		UpSMSContent: upSMSContent,
		SMSMode:      sodaMFASMSMode(hasMobileSMSVerify, upSMSMobile, upSMSContent),
		CanUpSMS:     hasMobileSMSVerify && (upSMSMobile != "" || upSMSContent != ""),
		Status:       model.QRLoginStatusScanned,
		Message:      "QR confirmed, SMS verification required",
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	})

	extra := sodaQRConnectExtra(resp)
	extra["need_sms"] = "true"
	extra["encrypt_uid"] = encryptUID
	extra["verify_params"] = verifyParams
	extra["mobile"] = mobile
	if upSMSMobile != "" || upSMSContent != "" {
		extra["up_sms_mobile"] = upSMSMobile
		extra["up_sms_content"] = upSMSContent
		if hasMobileSMSVerify {
			extra["sms_mode"] = "sms"
			extra["can_up_sms"] = "true"
		} else {
			extra["sms_mode"] = "up"
			extra["need_user_sms"] = "true"
		}
	}

	return &model.QRLoginResult{
		Source:  "soda",
		Key:     token,
		Status:  model.QRLoginStatusScanned,
		Message: "QR confirmed, SMS verification required",
		Extra:   extra,
	}, true
}

func sodaMFASMSMode(hasMobileSMSVerify bool, upSMSMobile, upSMSContent string) string {
	if hasMobileSMSVerify {
		return "sms"
	}
	if strings.TrimSpace(upSMSMobile) != "" || strings.TrimSpace(upSMSContent) != "" {
		return "up"
	}
	return ""
}

func (s *Soda) sodaSendCode(token, encryptUID, verifyParams string) (*model.QRLoginResult, error) {
	pending, hasPending := getSodaQRLoginPending(token)
	if strings.TrimSpace(encryptUID) == "" {
		encryptUID = pending.EncryptUID
	}
	if strings.TrimSpace(verifyParams) == "" {
		verifyParams = pending.VerifyParams
	}
	if strings.TrimSpace(encryptUID) == "" {
		return &model.QRLoginResult{
			Source:  "soda",
			Key:     token,
			Status:  model.QRLoginStatusFailed,
			Message: "缺少短信验证参数，请刷新二维码重试",
		}, nil
	}

	params := url.Values{}
	params.Set("mix_mode", "1")
	params.Set("type", "3737")
	params.Set("encrypt_uid", encryptUID)
	params.Set("verify_ticket", "")
	params.Set("copywriting_key", "qr_connect")
	params.Set("ies_safety_diversion_tag", "mfa")
	params.Set("new_verify_flow", "")
	params.Set("std_verify_way", "mobile_sms_verify")
	params.Set("is6Digits", "1")
	params.Set("aid", sodaPassportAid)
	params.Set("new_authn_sdk_version", "1.0.0.404-web")

	applySodaVerifyParams(params, verifyParams)

	apiURL := sodaSendCodeAPI + "?" + buildSodaPassportLiteQueryFor("/passport/web/send_code/")

	body, cookies, err := s.postSodaPassportWithCookie(apiURL, params, sodaCookieHeader(s.cookie, pending.Cookies))
	if err != nil {
		return nil, err
	}

	sodaQRDebugLogf("  [DEBUG send_code] raw=%s\n", string(body))

	var resp struct {
		Data struct {
			Mobile    string `json:"mobile"`
			RetryTime int    `json:"retry_time"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda send_code json error: %w", err)
	}
	if !sodaMessageOK(resp.Message) {
		return &model.QRLoginResult{
			Source:  "soda",
			Key:     token,
			Status:  model.QRLoginStatusFailed,
			Message: "SMS code send failed: " + resp.Message,
		}, nil
	}

	result := &model.QRLoginResult{
		Source:  "soda",
		Key:     token,
		Status:  model.QRLoginStatusScanned,
		Message: fmt.Sprintf("验证码已发送至 %s", resp.Data.Mobile),
		Extra: map[string]string{
			"need_sms":      "true",
			"need_sms_code": "true",
			"mobile":        resp.Data.Mobile,
			"encrypt_uid":   encryptUID,
			"verify_params": verifyParams,
			"retry_time":    fmt.Sprintf("%d", resp.Data.RetryTime),
		},
	}
	if hasPending {
		pending.EncryptUID = encryptUID
		pending.VerifyParams = verifyParams
		pending.Cookies = mergeSodaCookies(pending.Cookies, cookies)
		rememberSodaQRLoginPending(token, pending)
	}
	return result, nil
}

func (s *Soda) sodaVerifyUpSMS(token, encryptUID, verifyParams string) (*model.QRLoginResult, error) {
	pending, _ := getSodaQRLoginPending(token)
	if strings.TrimSpace(encryptUID) == "" {
		encryptUID = pending.EncryptUID
	}
	if strings.TrimSpace(verifyParams) == "" {
		verifyParams = pending.VerifyParams
	}
	if strings.TrimSpace(encryptUID) == "" {
		return &model.QRLoginResult{
			Source:  "soda",
			Key:     token,
			Status:  model.QRLoginStatusFailed,
			Message: "缺少短信验证参数，请刷新二维码重试",
		}, nil
	}

	params := url.Values{}
	params.Set("encrypt_uid", encryptUID)
	params.Set("verify_ticket", "")
	params.Set("copywriting_key", "qr_connect")
	params.Set("ies_safety_diversion_tag", "mfa")
	params.Set("new_verify_flow", "")
	params.Set("aid", sodaPassportAid)
	params.Set("new_authn_sdk_version", "1.0.0.404-web")
	applySodaVerifyParams(params, verifyParams)
	params.Set("std_verify_way", "mobile_up_sms_verify")

	apiURL := sodaUpSMSVerifyAPI + "?" + buildSodaPassportLiteQueryFor("/passport/upsms/verify/")
	body, cookies, err := s.postSodaPassportWithCookie(apiURL, params, sodaCookieHeader(s.cookie, pending.Cookies))
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Registered bool   `json:"registered"`
			Ticket     string `json:"ticket"`
			ErrorCode  int    `json:"error_code"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda upsms verify json error: %w", err)
	}
	if (!resp.Data.Registered && resp.Data.Ticket == "") || !sodaMessageOK(resp.Message) {
		return &model.QRLoginResult{
			Source:  "soda",
			Key:     token,
			Status:  model.QRLoginStatusFailed,
			Message: "上行短信确认失败: " + resp.Message,
		}, nil
	}

	mergedCookies := mergeSodaCookies(pending.Cookies, cookies)
	pending.Cookies = mergedCookies
	pending.EncryptUID = encryptUID
	pending.VerifyParams = verifyParams
	rememberSodaQRLoginPending(token, pending)

	finalResult, err := s.sodaCheckQRConnectWithState(token, pending, true)
	if err != nil {
		return nil, err
	}
	if finalResult.Status == model.QRLoginStatusSuccess {
		return finalResult, nil
	}
	finalResult.Status = model.QRLoginStatusFailed
	if strings.TrimSpace(finalResult.Message) == "" || finalResult.Message == "success" {
		finalResult.Message = "上行短信已确认，但汽水未下发登录 Cookie"
	}
	return finalResult, nil
}

func (s *Soda) sodaValidateCode(token, encryptUID, verifyParams, code string) (*model.QRLoginResult, error) {
	pending, _ := getSodaQRLoginPending(token)
	if strings.TrimSpace(encryptUID) == "" {
		encryptUID = pending.EncryptUID
	}
	if strings.TrimSpace(verifyParams) == "" {
		verifyParams = pending.VerifyParams
	}
	if strings.TrimSpace(encryptUID) == "" {
		return &model.QRLoginResult{
			Source:  "soda",
			Key:     token,
			Status:  model.QRLoginStatusFailed,
			Message: "缺少短信验证参数，请刷新二维码重试",
		}, nil
	}

	params := url.Values{}
	params.Set("mix_mode", "1")
	params.Set("type", "3737")
	params.Set("encrypt_uid", encryptUID)
	params.Set("verify_ticket", "")
	params.Set("copywriting_key", "qr_connect")
	params.Set("ies_safety_diversion_tag", "mfa")
	params.Set("new_verify_flow", "")
	params.Set("std_verify_way", "mobile_sms_verify")
	params.Set("code", sodaEncodeSMSCode(code))
	params.Set("aid", sodaPassportAid)
	params.Set("new_authn_sdk_version", "1.0.0.404-web")

	applySodaVerifyParams(params, verifyParams)

	apiURL := sodaValidateAPI + "?" + buildSodaPassportLiteQueryFor("/passport/web/validate_code/")

	body, cookies, err := s.postSodaPassportWithCookie(apiURL, params, sodaCookieHeader(s.cookie, pending.Cookies))
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Ticket string `json:"ticket"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda validate_code json error: %w", err)
	}

	if resp.Data.Ticket == "" && !sodaMessageOK(resp.Message) {
		return &model.QRLoginResult{
			Source:  "soda",
			Key:     token,
			Status:  model.QRLoginStatusFailed,
			Message: "验证码错误: " + resp.Message,
		}, nil
	}

	// Success - collect cookies
	mergedCookies := mergeSodaCookies(pending.Cookies, cookies)
	pending.Cookies = mergedCookies
	pending.EncryptUID = encryptUID
	pending.VerifyParams = verifyParams
	rememberSodaQRLoginPending(token, pending)

	finalResult, err := s.sodaCheckQRConnectWithState(token, pending, true)
	if err != nil {
		return nil, err
	}
	if finalResult.Status == model.QRLoginStatusSuccess {
		return finalResult, nil
	}
	finalResult.Status = model.QRLoginStatusFailed
	if strings.TrimSpace(finalResult.Message) == "" || finalResult.Message == "success" {
		finalResult.Message = "SMS verified, but Soda did not issue session cookie"
	}
	return finalResult, nil
}

func (s *Soda) postSodaPassport(apiURL string, form url.Values) ([]byte, map[string]string, error) {
	return s.postSodaPassportWithCookie(apiURL, form, s.cookie)
}

func (s *Soda) postSodaPassportWithCookie(apiURL string, form url.Values, cookie string) ([]byte, map[string]string, error) {
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(sodaEncodePassportForm(apiURL, form)))
	if err != nil {
		return nil, nil, err
	}
	apiPath := sodaAPIPath(apiURL)
	req.Header.Set("User-Agent", sodaPassportUA)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("sec-ch-ua", `"Not.A/Brand";v="99", "Chromium";v="136"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	if strings.Contains(apiURL, "/send_code/") || strings.Contains(apiURL, "/validate_code/") || strings.Contains(apiURL, "/upsms/verify/") {
		req.Header.Set("Accept", "application/json, text/plain, */*")
	} else {
		req.Header.Set("Accept", "application/json, text/javascript")
	}
	// bd-ticket-guard headers required by Bytedance Passport
	req.Header.Set("bd-ticket-guard-version", "2")
	req.Header.Set("bd-ticket-guard-iteration-version", "2")
	req.Header.Set("bd-ticket-guard-ree-public-key", "BAnIxKL96Jby5x+Um9i7HZ2c8O6lfZJRxm6yk73Mqcr06l2qIw2iqu2Mtm3U/6OI98usukA9dqxUlsctVWK9rKA=")
	req.Header.Set("bd-ticket-guard-server-cert-sn", "0")
	if traceID := sodaPassportTraceID(apiURL); traceID != "" {
		req.Header.Set("X-Tt-Passport-Trace-Id", traceID)
	}
	if flowID := strings.TrimSpace(form.Get("std_verify_flow_id")); flowID != "" {
		req.Header.Set("X-Tt-Passport-Verify-Portrait", flowID)
	}
	applySodaCapturedHeaders(req, apiPath)
	if strings.TrimSpace(cookie) != "" {
		req.Header.Set("Cookie", strings.TrimSpace(cookie))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	cookies := make(map[string]string)
	for _, c := range resp.Cookies() {
		if c.Name != "" && c.Value != "" {
			cookies[c.Name] = c.Value
		}
	}
	return bodyBytes, cookies, nil
}

func sodaQRConnectErrorMessage(resp sodaQRConnectResponse) string {
	for _, value := range []string{resp.Data.Description, resp.Message} {
		value = strings.TrimSpace(value)
		if value != "" {
			if resp.Data.ErrorCode != 0 {
				return fmt.Sprintf("%s (code=%d)", value, resp.Data.ErrorCode)
			}
			return value
		}
	}
	if resp.Data.ErrorCode != 0 {
		return fmt.Sprintf("Soda QR 登录失败 (code=%d)", resp.Data.ErrorCode)
	}
	return "Soda QR 登录失败"
}

func sodaQRConnectExtra(resp sodaQRConnectResponse) map[string]string {
	extra := map[string]string{}
	if status := strings.TrimSpace(resp.Data.Status); status != "" {
		extra["api_status"] = status
	}
	if resp.Data.ErrorCode != 0 {
		extra["error_code"] = fmt.Sprintf("%d", resp.Data.ErrorCode)
	}
	if redirect := strings.TrimSpace(resp.Data.Redirect); redirect != "" {
		extra["redirect_url"] = redirect
	}
	return extra
}

func sodaMessageOK(message string) bool {
	message = strings.TrimSpace(strings.ToLower(message))
	return message == "" || message == "success"
}

func sodaAPIPath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(u.Path)
}

func sodaEncodePassportForm(apiURL string, form url.Values) string {
	switch {
	case strings.Contains(apiURL, "/check_qrconnect/"):
		return sodaEncodeOrderedForm(form, []string{"need_logo", "need_short_url", "is_frontier", "token", "is_new_login", "next", "passport_mfa_retry_tag", "std_verify_flow_id", "std_verify_scene", "std_verify_template", "std_verify_token", "std_verify_type", "std_verify_way"})
	case strings.Contains(apiURL, "/send_code/"):
		return sodaEncodeOrderedForm(form, []string{"mix_mode", "type", "encrypt_uid", "verify_ticket", "copywriting_key", "ies_safety_diversion_tag", "new_verify_flow", "std_verify_flow_id", "std_verify_scene", "std_verify_template", "std_verify_token", "std_verify_type", "std_verify_way", "is6Digits", "aid", "new_authn_sdk_version"})
	case strings.Contains(apiURL, "/validate_code/"):
		return sodaEncodeOrderedForm(form, []string{"mix_mode", "type", "encrypt_uid", "verify_ticket", "copywriting_key", "ies_safety_diversion_tag", "mfa", "new_verify_flow", "std_verify_flow_id", "std_verify_scene", "std_verify_template", "std_verify_token", "std_verify_type", "std_verify_way", "code", "aid", "new_authn_sdk_version"})
	case strings.Contains(apiURL, "/upsms/verify/"):
		return sodaEncodeOrderedForm(form, []string{"encrypt_uid", "verify_ticket", "copywriting_key", "ies_safety_diversion_tag", "new_verify_flow", "std_verify_flow_id", "std_verify_scene", "std_verify_template", "std_verify_token", "std_verify_type", "std_verify_way", "aid", "new_authn_sdk_version"})
	default:
		return form.Encode()
	}
}

func sodaEncodeOrderedForm(form url.Values, order []string) string {
	seen := map[string]bool{}
	parts := make([]string, 0, len(form))
	for _, key := range order {
		if values, ok := form[key]; ok {
			seen[key] = true
			parts = appendSodaEncodedValues(parts, key, values)
		}
	}
	keys := make([]string, 0, len(form))
	for key := range form {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts = appendSodaEncodedValues(parts, key, form[key])
	}
	return strings.Join(parts, "&")
}

func appendSodaEncodedValues(parts []string, key string, values []string) []string {
	encodedKey := url.QueryEscape(key)
	if len(values) == 0 {
		return append(parts, encodedKey+"=")
	}
	for _, value := range values {
		parts = append(parts, encodedKey+"="+url.QueryEscape(value))
	}
	return parts
}

func buildSodaQRCreateQuery() string {
	params := buildSodaPassportNormalValues()
	params.Set("next", "https://api.qishui.com")
	params.Set("need_logo", "false")
	params.Set("need_short_url", "false")
	params.Set("is_frontier", "true")
	return sodaEncodeOrderedForm(params, append(sodaPassportNormalQueryOrder(), "next", "need_logo", "need_short_url", "is_frontier"))
}

func buildSodaQRCheckQuery() string {
	return sodaEncodeOrderedForm(buildSodaPassportNormalValues(), sodaPassportNormalQueryOrder())
}

func sodaQRConnectForm(token string) url.Values {
	params := url.Values{}
	params.Set("need_logo", "false")
	params.Set("need_short_url", "false")
	params.Set("is_frontier", "true")
	params.Set("token", strings.TrimSpace(token))
	params.Set("is_new_login", "1")
	params.Set("next", "https://api.qishui.com")
	return params
}

func applySodaVerifyParams(params url.Values, verifyParams string) {
	if strings.TrimSpace(verifyParams) == "" {
		return
	}
	vp, _ := url.ParseQuery(verifyParams)
	for k, vs := range vp {
		if len(vs) > 0 {
			params.Set(k, vs[0])
		}
	}
}

func buildSodaPassportQuery() string {
	return sodaEncodeOrderedForm(buildSodaPassportNormalValues(), sodaPassportNormalQueryOrder())
}

func buildSodaPassportBaseValues(jsVersion, jsType string) url.Values {
	params := url.Values{}
	params.Set("passport_jssdk_version", jsVersion)
	params.Set("passport_jssdk_type", jsType)
	params.Set("is_from_ttaccountsdk", "1")
	params.Set("aid", sodaPassportAid)
	params.Set("language", "zh")
	params.Set("is_new_login", "1")
	params.Set("is_from_iesaccountsaas", "1")
	params.Set("device_id", sodaStableDeviceID())
	params.Set("install_id", sodaStableInstallID())
	params.Set("did", sodaStableDeviceID())
	params.Set("iid", sodaStableInstallID())
	params.Set("device_platform", "PC")
	params.Set("version_code", sodaVersionCode)
	params.Set("biz_trace_id", sodaPassportBizTraceID())
	return params
}

var (
	sodaDeviceIDOnce sync.Once
	sodaDeviceIDVal  string
	sodaInstallIDVal string
)

func sodaStableDeviceID() string {
	sodaDeviceIDOnce.Do(func() {
		now := time.Now().UnixMilli()
		sodaDeviceIDVal = fmt.Sprintf("%d", now)
		sodaInstallIDVal = fmt.Sprintf("%d", now+1)
	})
	return sodaDeviceIDVal
}

func sodaStableInstallID() string {
	sodaStableDeviceID() // ensure init
	return sodaInstallIDVal
}

func buildSodaPassportNormalValues() url.Values {
	params := buildSodaPassportBaseValues(sodaPassportJSVer, "normal")
	params.Set("account_sdk_source", "web")
	params.Set("p_js_v", sodaPassportJSVer)
	params.Set("p_js_t", "pro")
	params.Set("p_zt", sodaPZT)
	params.Set("p_ver", sodaPVer)
	params.Set("request_host", "app%3A%2F%2Fresources")
	params.Set("p_bd", sodaPBD)
	applySodaCapturedQueryParams(params, "/passport/web/check_qrconnect/")
	return params
}

func sodaScanLoginURL(token string) string {
	params := url.Values{}
	params.Set("token", strings.TrimSpace(token))
	params.Set("os", "Windows")
	params.Set("computer_name", "go-music-dl")
	return "https://bff-pc.qishui.com/light/invoke/scan_login?" + params.Encode()
}

func buildSodaPassportLiteQuery() string {
	return buildSodaPassportLiteQueryFor("")
}

func buildSodaPassportLiteQueryFor(apiPath string) string {
	params := buildSodaPassportBaseValues("5.1.2", "lite")
	params.Set("new_authn_sdk_version", "1.0.0.404-web")
	params.Set("account_app_language", "en-US")
	applySodaCapturedQueryParams(params, apiPath)
	return sodaEncodeOrderedForm(params, sodaPassportLiteQueryOrder())
}

func sodaPassportNormalQueryOrder() []string {
	return []string{"passport_jssdk_version", "passport_jssdk_type", "is_from_ttaccountsdk", "aid", "language", "account_sdk_source", "account_sdk_source_info", "p_js_v", "p_js_t", "p_zt", "p_ver", "request_host", "p_bd", "biz_trace_id", "is_new_login", "is_from_iesaccountsaas", "device_id", "install_id", "did", "iid", "device_platform", "version_code", "msToken", "a_bogus"}
}

func sodaPassportLiteQueryOrder() []string {
	return []string{"passport_jssdk_version", "passport_jssdk_type", "is_from_ttaccountsdk", "aid", "language", "account_app_language", "new_authn_sdk_version", "is_new_login", "is_from_iesaccountsaas", "device_id", "install_id", "did", "iid", "device_platform", "version_code", "biz_trace_id", "msToken", "a_bogus"}
}

var (
	sodaBizTraceOnce      sync.Once
	sodaBizTraceVal       string
	sodaCaptureParamsOnce sync.Once
	sodaCapturedQueries   map[string]url.Values
	sodaCapturedHeaders   map[string]map[string]string
	sodaCapturedParamsErr error
)

func sodaPassportBizTraceID() string {
	sodaBizTraceOnce.Do(func() {
		sodaBizTraceVal = fmt.Sprintf("%08x", uint32(time.Now().UnixNano()))
	})
	return sodaBizTraceVal
}

func sodaPassportTraceID(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(u.Query().Get("biz_trace_id"))
}

func applySodaCapturedQueryParams(params url.Values, apiPath string) {
	captured := sodaCapturedQuery(apiPath)
	if len(captured) == 0 {
		return
	}
	for _, key := range []string{"account_sdk_source_info", "biz_trace_id", "device_id", "install_id", "did", "iid"} {
		if value := strings.TrimSpace(captured.Get(key)); value != "" {
			params.Set(key, value)
		}
	}
	if os.Getenv("SODA_QR_USE_CAPTURE_SIGNATURE") == "1" {
		for _, key := range []string{"msToken", "a_bogus"} {
			if value := strings.TrimSpace(captured.Get(key)); value != "" {
				params.Set(key, value)
			}
		}
	}
}

func applySodaCapturedHeaders(req *http.Request, apiPath string) {
	headers := sodaCapturedHeader(apiPath)
	for _, key := range []string{"x-tt-passport-trace-id", "x-tt-passport-verify-portrait", "x-tt-trace-id"} {
		if value := strings.TrimSpace(headers[key]); value != "" {
			req.Header.Set(key, value)
		}
	}
}

func sodaCapturedQuery(apiPath string) url.Values {
	sodaLoadCapturedParams()
	if sodaCapturedParamsErr != nil {
		sodaQRDebugLogf("[DEBUG capture_params] %v\n", sodaCapturedParamsErr)
		return nil
	}
	return cloneSodaURLValues(sodaCapturedQueries[strings.TrimSpace(apiPath)])
}

func sodaCapturedHeader(apiPath string) map[string]string {
	sodaLoadCapturedParams()
	if sodaCapturedParamsErr != nil {
		sodaQRDebugLogf("[DEBUG capture_params] %v\n", sodaCapturedParamsErr)
		return nil
	}
	headers := sodaCapturedHeaders[strings.TrimSpace(apiPath)]
	cloned := map[string]string{}
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}

func sodaLoadCapturedParams() {
	if os.Getenv("SODA_QR_USE_CAPTURE_PARAMS") != "1" {
		return
	}
	sodaCaptureParamsOnce.Do(func() {
		sodaCapturedQueries = map[string]url.Values{}
		sodaCapturedHeaders = map[string]map[string]string{}
		body, err := os.ReadFile(sodaCaptureFilePath())
		if err != nil {
			sodaCapturedParamsErr = err
			return
		}
		var currentPath string
		for _, rawLine := range strings.Split(string(body), "\n") {
			line := strings.TrimSpace(rawLine)
			if strings.HasPrefix(line, "POST https://api.qishui.com/") || strings.HasPrefix(line, "GET https://api.qishui.com/") {
				currentPath = ""
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if parsed, err := url.Parse(fields[1]); err == nil {
						currentPath = parsed.Path
						if currentPath != "" {
							if _, exists := sodaCapturedQueries[currentPath]; !exists {
								sodaCapturedQueries[currentPath] = parsed.Query()
							}
							if _, exists := sodaCapturedHeaders[currentPath]; !exists {
								sodaCapturedHeaders[currentPath] = map[string]string{}
							}
						}
					}
				}
				continue
			}
			if currentPath == "" || !strings.Contains(line, ":") {
				continue
			}
			parts := strings.SplitN(line, ":", 2)
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			value := strings.TrimSpace(parts[1])
			if key != "" && value != "" {
				sodaCapturedHeaders[currentPath][key] = value
			}
		}
	})
}

func sodaCaptureFilePath() string {
	if path := strings.TrimSpace(os.Getenv("SODA_QR_CAPTURE_FILE")); path != "" {
		return path
	}
	if cwd, err := os.Getwd(); err == nil {
		for i := 0; i < 5; i++ {
			candidate := filepath.Join(cwd, "汽水扫码20260516.txt")
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
			parent := filepath.Dir(cwd)
			if parent == cwd {
				break
			}
			cwd = parent
		}
	}
	return "汽水扫码20260516.txt"
}

func cloneSodaURLValues(values url.Values) url.Values {
	if len(values) == 0 {
		return nil
	}
	cloned := url.Values{}
	for key, itemValues := range values {
		cloned[key] = append([]string(nil), itemValues...)
	}
	return cloned
}

func extractSodaCookieValue(cookies map[string]string, name string) string {
	return cookies[name]
}

func extractSodaMFAField(body []byte, field string) string {
	var raw interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return ""
	}
	if v := findSodaJSONString(raw, field); v != "" {
		return v
	}
	return ""
}

func extractSodaMFAVerifyParams(body []byte, cookies map[string]string) string {
	var raw interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return ""
	}

	params := url.Values{}
	collectSodaVerifyParams(raw, params)
	return params.Encode()
}

func sodaBodyContainsVerifyWay(body []byte, way string) bool {
	way = strings.TrimSpace(way)
	if way == "" {
		return false
	}
	bodyText := string(body)
	return strings.Contains(bodyText, `"verify_way":"`+way+`"`) ||
		strings.Contains(bodyText, `"verify_way": "`+way+`"`)
}

func normalizeSodaJSONKey(key string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(strings.TrimSpace(key)))
}

func findSodaJSONString(value interface{}, field string) string {
	want := normalizeSodaJSONKey(field)
	switch v := value.(type) {
	case map[string]interface{}:
		for key, child := range v {
			if normalizeSodaJSONKey(key) == want {
				switch s := child.(type) {
				case string:
					return strings.TrimSpace(s)
				case float64:
					return fmt.Sprintf("%.0f", s)
				}
			}
			if found := findSodaJSONString(child, field); found != "" {
				return found
			}
		}
	case []interface{}:
		for _, child := range v {
			if found := findSodaJSONString(child, field); found != "" {
				return found
			}
		}
	case string:
		var nested interface{}
		if strings.HasPrefix(strings.TrimSpace(v), "{") && json.Unmarshal([]byte(v), &nested) == nil {
			return findSodaJSONString(nested, field)
		}
	}
	return ""
}

func collectSodaVerifyParams(value interface{}, params url.Values) {
	allow := map[string]bool{
		"passport_mfa_retry_tag": true,
		"std_verify_flow_id":     true,
		"std_verify_scene":       true,
		"std_verify_template":    true,
		"std_verify_token":       true,
		"std_verify_type":        true,
		"std_verify_way":         true,
	}
	switch v := value.(type) {
	case map[string]interface{}:
		for key, child := range v {
			if allow[key] {
				switch s := child.(type) {
				case string:
					if strings.TrimSpace(s) != "" {
						params.Set(key, strings.TrimSpace(s))
					}
				case float64:
					params.Set(key, fmt.Sprintf("%.0f", s))
				}
			}
			collectSodaVerifyParams(child, params)
		}
	case []interface{}:
		for _, child := range v {
			collectSodaVerifyParams(child, params)
		}
	case string:
		text := strings.TrimSpace(v)
		if strings.Contains(text, "std_verify_") || strings.Contains(text, "passport_mfa_retry_tag") {
			if idx := strings.Index(text, "?"); idx >= 0 {
				text = text[idx+1:]
			}
			if parsed, err := url.ParseQuery(text); err == nil {
				for key, values := range parsed {
					if allow[key] && len(values) > 0 && strings.TrimSpace(values[0]) != "" {
						params.Set(key, strings.TrimSpace(values[0]))
					}
				}
			}
		}
		var nested interface{}
		if strings.HasPrefix(text, "{") && json.Unmarshal([]byte(text), &nested) == nil {
			collectSodaVerifyParams(nested, params)
		}
	}
}

func rememberSodaQRLoginPending(token string, state sodaQRLoginPendingState) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	if state.ExpiresAt.IsZero() {
		state.ExpiresAt = time.Now().Add(10 * time.Minute)
	}
	state.Cookies = mergeSodaCookies(nil, state.Cookies)
	sodaQRLoginMu.Lock()
	defer sodaQRLoginMu.Unlock()
	cleanupSodaQRLoginPendingLocked(time.Now())
	sodaQRLoginPending[token] = state
}

func getSodaQRLoginPending(token string) (sodaQRLoginPendingState, bool) {
	sodaQRLoginMu.Lock()
	defer sodaQRLoginMu.Unlock()
	now := time.Now()
	cleanupSodaQRLoginPendingLocked(now)
	state, ok := sodaQRLoginPending[strings.TrimSpace(token)]
	if !ok || (!state.ExpiresAt.IsZero() && now.After(state.ExpiresAt)) {
		return sodaQRLoginPendingState{}, false
	}
	state.Cookies = mergeSodaCookies(nil, state.Cookies)
	return state, true
}

func clearSodaQRLoginPending(token string) {
	sodaQRLoginMu.Lock()
	defer sodaQRLoginMu.Unlock()
	delete(sodaQRLoginPending, strings.TrimSpace(token))
}

func cleanupSodaQRLoginPending() {
	sodaQRLoginMu.Lock()
	defer sodaQRLoginMu.Unlock()
	cleanupSodaQRLoginPendingLocked(time.Now())
}

func cleanupSodaQRLoginPendingLocked(now time.Time) {
	for token, state := range sodaQRLoginPending {
		if !state.ExpiresAt.IsZero() && now.After(state.ExpiresAt) {
			delete(sodaQRLoginPending, token)
		}
	}
}

func mergeSodaCookies(cookieMaps ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, cookies := range cookieMaps {
		for key, value := range cookies {
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key != "" && value != "" {
				merged[key] = value
			}
		}
	}
	return merged
}

func sodaCookieHeader(base string, cookies map[string]string) string {
	cookie := sodaJoinCookies(cookies)
	base = strings.TrimSpace(base)
	switch {
	case base == "":
		return cookie
	case cookie == "":
		return base
	default:
		return base + "; " + cookie
	}
}

func sodaEncodeSMSCode(code string) string {
	return hex.EncodeToString([]byte(strings.TrimSpace(code)))
}

func sodaCookiesHaveSession(cookies map[string]string) bool {
	for _, key := range []string{"sessionid", "sessionid_ss", "sid_tt", "sid_guard"} {
		if strings.TrimSpace(cookies[key]) != "" {
			return true
		}
	}
	return false
}

func strconvBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func sodaQRDebugLogf(format string, args ...any) {
	if os.Getenv("SODA_QR_DEBUG") == "1" {
		fmt.Printf(format, args...)
	}
}

func sodaRedactValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "..." + value[len(value)-3:]
}

func sodaRedactURLSecrets(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return sodaRedactValue(raw)
	}
	query := parsed.Query()
	for _, key := range []string{"token", "msToken", "a_bogus"} {
		if query.Has(key) {
			query.Set(key, sodaRedactValue(query.Get(key)))
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func sodaQRCodeImageURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "data:") || strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	return "data:image/png;base64," + raw
}

func sodaJoinCookies(cookies map[string]string) string {
	if len(cookies) == 0 {
		return ""
	}
	keys := make([]string, 0, len(cookies))
	for k := range cookies {
		if k != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		if cookies[k] != "" {
			parts = append(parts, k+"="+cookies[k])
		}
	}
	return strings.Join(parts, "; ")
}
