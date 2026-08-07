package bilibili

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	bilibiliQRGenerateAPI = "https://passport.bilibili.com/x/passport-login/web/qrcode/generate"
	bilibiliQRPollAPI     = "https://passport.bilibili.com/x/passport-login/web/qrcode/poll"
)

func CreateQRLogin() (*model.QRLoginSession, error) { return defaultBilibili.CreateQRLogin() }

func CheckQRLogin(key string) (*model.QRLoginResult, error) { return defaultBilibili.CheckQRLogin(key) }

func (b *Bilibili) CreateQRLogin() (*model.QRLoginSession, error) {
	body, err := utils.Get(bilibiliQRGenerateAPI,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			URL       string `json:"url"`
			QRCodeKey string `json:"qrcode_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bilibili qr generate json parse error: %w", err)
	}
	if resp.Code != 0 || strings.TrimSpace(resp.Data.QRCodeKey) == "" {
		return nil, fmt.Errorf("bilibili qr generate api error: code=%d message=%s", resp.Code, resp.Message)
	}
	return &model.QRLoginSession{
		Source:    "bilibili",
		Key:       resp.Data.QRCodeKey,
		URL:       resp.Data.URL,
		ExpiresAt: time.Now().Add(3 * time.Minute).Unix(),
	}, nil
}

func (b *Bilibili) CheckQRLogin(key string) (*model.QRLoginResult, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("bilibili qr login key is empty")
	}
	params := url.Values{}
	params.Set("qrcode_key", key)
	req, err := http.NewRequest("GET", bilibiliQRPollAPI+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Referer", Referer)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bilibili qr poll http status %d", resp.StatusCode)
	}
	body, err := ioReadAll(resp)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			URL          string `json:"url"`
			RefreshToken string `json:"refresh_token"`
			Timestamp    int64  `json:"timestamp"`
			Code         int    `json:"code"`
			Message      string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("bilibili qr poll json parse error: %w", err)
	}
	result := &model.QRLoginResult{
		Source:  "bilibili",
		Key:     key,
		Status:  mapBilibiliQRStatus(payload.Data.Code),
		Message: firstNonEmptyBilibili(payload.Data.Message, payload.Message),
		Extra: map[string]string{
			"code": fmt.Sprintf("%d", payload.Data.Code),
		},
	}
	if result.Status == model.QRLoginStatusSuccess {
		cookies := responseCookiesBilibili(resp)
		result.Cookies = cookies
		result.Cookie = joinCookieMapBilibili(cookies)
		b.cookie = result.Cookie
		b.isVipCache = nil
		if payload.Data.RefreshToken != "" {
			result.Extra["refresh_token"] = payload.Data.RefreshToken
		}
	}
	return result, nil
}

func mapBilibiliQRStatus(code int) model.QRLoginStatus {
	switch code {
	case 0:
		return model.QRLoginStatusSuccess
	case 86038:
		return model.QRLoginStatusExpired
	case 86090:
		return model.QRLoginStatusScanned
	case 86101:
		return model.QRLoginStatusWaiting
	default:
		return model.QRLoginStatusFailed
	}
}

func firstNonEmptyBilibili(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func responseCookiesBilibili(resp *http.Response) map[string]string {
	cookies := map[string]string{}
	for _, cookie := range resp.Cookies() {
		if strings.TrimSpace(cookie.Name) != "" {
			cookies[cookie.Name] = cookie.Value
		}
	}
	return cookies
}

func joinCookieMapBilibili(cookies map[string]string) string {
	parts := make([]string, 0, len(cookies))
	for key, value := range cookies {
		if strings.TrimSpace(key) != "" {
			parts = append(parts, key+"="+value)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func ioReadAll(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body)
}
