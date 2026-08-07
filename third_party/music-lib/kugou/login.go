package kugou

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const kugouLitePublicKey = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDECi0Np2UR87scwrvTr72L6oO01rBbbBPriSDFPxr3Z5syug0O24QyQO8bg27+0+4kBzTBTBOZ/WWU0WryL1JSXRTXLgFVxtzIY41Pe7lPOgsfTCn5kZcvKhYKJesKnnJDNr5/abvTGf+rHG3YRwsCHcQ08/q6ifSioBszvb3QiwIDAQAB
-----END PUBLIC KEY-----`

func CreateQRLogin() (*model.QRLoginSession, error) { return defaultKugou.CreateQRLogin() }

func CheckQRLogin(key string) (*model.QRLoginResult, error) { return defaultKugou.CheckQRLogin(key) }

func (k *Kugou) CreateQRLogin() (*model.QRLoginSession, error) {
	cookies := initKugouLoginDevice(nil)
	params := map[string]string{
		"appid":      "1001",
		"type":       "1",
		"plat":       "4",
		"qrcode_txt": "https://h5.kugou.com/apps/loginQRCode/html/index.html?appid=" + KugouLiteAppID + "&",
		"srcappid":   "2919",
	}
	body, err := kugouLoginWebGet("https://login-user.kugou.com/v2/qrcode", params, cookies)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Status int `json:"status"`
		Data   struct {
			QRCode string `json:"qrcode"`
		} `json:"data"`
		ErrorCode int    `json:"error_code"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kugou qr key json parse error: %w", err)
	}
	key := strings.TrimSpace(resp.Data.QRCode)
	if key == "" {
		return nil, fmt.Errorf("kugou qr key api error: status=%d error_code=%d error=%s", resp.Status, resp.ErrorCode, resp.Error)
	}

	loginURL := "https://h5.kugou.com/apps/loginQRCode/html/index.html?qrcode=" + url.QueryEscape(key)
	return &model.QRLoginSession{
		Source:    "kugou",
		Key:       key,
		URL:       loginURL,
		ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
	}, nil
}

func (k *Kugou) CheckQRLogin(key string) (*model.QRLoginResult, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("kugou qr login key is empty")
	}

	cookies := initKugouLoginDevice(nil)
	params := map[string]string{
		"plat":     "4",
		"appid":    KugouLiteAppID,
		"srcappid": "2919",
		"qrcode":   key,
	}
	body, err := kugouLoginWebGet("https://login-user.kugou.com/v2/get_userinfo_qrcode", params, cookies)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Status int `json:"status"`
		Data   struct {
			Status int         `json:"status"`
			Token  string      `json:"token"`
			UserID interface{} `json:"userid"`
		} `json:"data"`
		ErrorCode int    `json:"error_code"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kugou qr check json parse error: %w", err)
	}

	status := mapKugouQRStatus(resp.Data.Status)
	result := &model.QRLoginResult{
		Source:  "kugou",
		Key:     key,
		Status:  status,
		Message: firstNonEmpty(resp.Error, fmt.Sprintf("status=%d", resp.Data.Status)),
		Extra: map[string]string{
			"status":     strconv.Itoa(resp.Data.Status),
			"error_code": strconv.Itoa(resp.ErrorCode),
		},
	}
	if status != model.QRLoginStatusSuccess {
		return result, nil
	}

	token := strings.TrimSpace(resp.Data.Token)
	userID := formatKugouNumericString(resp.Data.UserID)
	if token == "" || userID == "" || userID == "0" {
		result.Status = model.QRLoginStatusFailed
		result.Message = "kugou qr login succeeded but token or userid is empty"
		return result, nil
	}

	cookies["token"] = token
	cookies["userid"] = userID
	if err := registerKugouLoginDevice(cookies); err != nil {
		result.Extra["register_error"] = err.Error()
	}
	result.Cookies = cookies
	result.Cookie = joinKugouCookieMap(cookies)
	k.cookie = result.Cookie
	k.isVipCache = nil
	return result, nil
}

func initKugouLoginDevice(cookies map[string]string) map[string]string {
	if cookies == nil {
		cookies = map[string]string{}
	}
	guid := randomKugouGUID()
	cookies["KUGOU_API_GUID"] = guid
	cookies["KUGOU_API_MID"] = calculateKugouMID(guid)
	cookies["KUGOU_API_MAC"] = randomKugouString(12)
	cookies["KUGOU_API_DEV"] = randomKugouString(16)
	return cookies
}

func kugouLoginWebGet(apiURL string, params map[string]string, cookies map[string]string) ([]byte, error) {
	clienttime := strconv.FormatInt(time.Now().Unix(), 10)
	finalParams := map[string]string{
		"dfid":       firstNonEmpty(cookies["dfid"], "-"),
		"mid":        firstNonEmpty(cookies["KUGOU_API_MID"], "-"),
		"uuid":       "-",
		"appid":      KugouLiteAppID,
		"clientver":  KugouLiteVer,
		"clienttime": clienttime,
	}
	for key, value := range params {
		finalParams[key] = value
	}
	query := url.Values{}
	for key, value := range finalParams {
		query.Set(key, value)
	}
	query.Set("signature", signKugouSonginfoParams(finalParams))
	return utils.Get(apiURL+"?"+query.Encode(),
		utils.WithHeader("User-Agent", "Android15-1070-11083-46-0-DiscoveryDRADProtocol-wifi"),
		utils.WithHeader("dfid", finalParams["dfid"]),
		utils.WithHeader("clienttime", clienttime),
		utils.WithHeader("mid", finalParams["mid"]),
		utils.WithHeader("kg-rc", "1"),
		utils.WithHeader("kg-thash", "5d816a0"),
		utils.WithHeader("kg-rec", "1"),
		utils.WithHeader("kg-rf", "B9EDA08A64250DEFFBCADDEE00F8F25F"),
		utils.WithHeader("Cookie", joinKugouCookieMap(cookies)),
		utils.WithRandomIPHeader(),
	)
}

func registerKugouLoginDevice(cookies map[string]string) error {
	aesSeed := strings.ToLower(randomKugouString(6))
	digest := utils.MD5(aesSeed)
	aesKey := []byte(digest[:16])
	aesIV := []byte(digest[16:32])
	deviceData := map[string]interface{}{
		"availableRamSize":   int64(4983533568),
		"availableRomSize":   48114719,
		"availableSDSize":    48114717,
		"basebandVer":        "",
		"batteryLevel":       100,
		"batteryStatus":      3,
		"brand":              "Redmi",
		"buildSerial":        "unknown",
		"device":             "marble",
		"imei":               cookies["KUGOU_API_GUID"],
		"imsi":               "",
		"manufacturer":       "Xiaomi",
		"uuid":               cookies["KUGOU_API_GUID"],
		"accelerometer":      false,
		"accelerometerValue": "",
		"gravity":            false,
		"gravityValue":       "",
		"gyroscope":          false,
		"gyroscopeValue":     "",
		"light":              false,
		"lightValue":         "",
		"magnetic":           false,
		"magneticValue":      "",
		"orientation":        false,
		"orientationValue":   "",
		"pressure":           false,
		"pressureValue":      "",
		"step_counter":       false,
		"step_counterValue":  "",
		"temperature":        false,
		"temperatureValue":   "",
	}
	deviceJSON, err := json.Marshal(deviceData)
	if err != nil {
		return err
	}
	encBody, err := aesCBCEncrypt(deviceJSON, aesKey, aesIV)
	if err != nil {
		return err
	}
	p, err := rsaPKCS1Hex(map[string]interface{}{
		"aes":   aesSeed,
		"uid":   cookies["userid"],
		"token": cookies["token"],
	})
	if err != nil {
		return err
	}

	data := base64.StdEncoding.EncodeToString(encBody)
	clienttime := strconv.FormatInt(time.Now().Unix(), 10)
	params := map[string]string{
		"dfid":       firstNonEmpty(cookies["dfid"], "-"),
		"mid":        firstNonEmpty(cookies["KUGOU_API_MID"], "-"),
		"uuid":       "-",
		"appid":      KugouLiteAppID,
		"clientver":  KugouLiteVer,
		"clienttime": clienttime,
		"token":      cookies["token"],
		"userid":     cookies["userid"],
		"part":       "1",
		"platid":     "1",
		"p":          p,
	}
	apiURL := buildKugouAndroidURL("https://userservice.kugou.com/risk/v2/r_register_dev", params, data)
	body, err := utils.Post(apiURL, strings.NewReader(data),
		utils.WithHeader("User-Agent", "Android15-1070-11083-46-0-DiscoveryDRADProtocol-wifi"),
		utils.WithHeader("dfid", params["dfid"]),
		utils.WithHeader("clienttime", clienttime),
		utils.WithHeader("mid", params["mid"]),
		utils.WithHeader("kg-rc", "1"),
		utils.WithHeader("kg-thash", "5d816a0"),
		utils.WithHeader("kg-rec", "1"),
		utils.WithHeader("kg-rf", "B9EDA08A64250DEFFBCADDEE00F8F25F"),
		utils.WithHeader("Cookie", joinKugouCookieMap(cookies)),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return err
	}
	if !bytes.HasPrefix(bytes.TrimSpace(body), []byte("{")) {
		body, err = aesCBCDecrypt(body, aesKey, aesIV)
		if err != nil {
			return err
		}
	}

	var resp struct {
		Status int `json:"status"`
		Data   struct {
			DFID string `json:"dfid"`
		} `json:"data"`
		ErrorCode int    `json:"error_code"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("kugou register device json parse error: %w", err)
	}
	if resp.Status != 1 {
		return fmt.Errorf("kugou register device api error: status=%d error_code=%d error=%s", resp.Status, resp.ErrorCode, resp.Error)
	}
	if strings.TrimSpace(resp.Data.DFID) != "" {
		cookies["dfid"] = resp.Data.DFID
	}
	return nil
}

func mapKugouQRStatus(status int) model.QRLoginStatus {
	switch status {
	case 4:
		return model.QRLoginStatusSuccess
	case 2, 3:
		return model.QRLoginStatusScanned
	case -1, 5, 6:
		return model.QRLoginStatusExpired
	case 0, 1:
		return model.QRLoginStatusWaiting
	default:
		return model.QRLoginStatusFailed
	}
}

func randomKugouString(length int) string {
	const chars = "1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	buf := make([]byte, length)
	if _, err := cryptorand.Read(buf); err != nil {
		for i := range buf {
			buf[i] = chars[int(time.Now().UnixNano()+int64(i))%len(chars)]
		}
		return string(buf)
	}
	for i := range buf {
		buf[i] = chars[int(buf[i])%len(chars)]
	}
	return string(buf)
}

func randomKugouGUID() string {
	buf := make([]byte, 16)
	if _, err := cryptorand.Read(buf); err != nil {
		hexValue := strings.ToLower(utils.MD5(strconv.FormatInt(time.Now().UnixNano(), 10)))
		return hexValue[:8] + "-" + hexValue[8:12] + "-" + hexValue[12:16] + "-" + hexValue[16:20] + "-" + hexValue[20:32]
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	hexValue := hex.EncodeToString(buf)
	return hexValue[:8] + "-" + hexValue[8:12] + "-" + hexValue[12:16] + "-" + hexValue[16:20] + "-" + hexValue[20:]
}

func calculateKugouMID(seed string) string {
	sum := md5.Sum([]byte(seed))
	return new(big.Int).SetBytes(sum[:]).String()
}

func aesCBCEncrypt(data, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	data = pkcs7Pad(data, block.BlockSize())
	out := make([]byte, len(data))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, data)
	return out, nil
}

func aesCBCDecrypt(data, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(data)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("invalid aes cbc data length")
	}
	out := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, data)
	return pkcs7Unpad(out)
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padLen := blockSize - len(data)%blockSize
	return append(data, bytes.Repeat([]byte{byte(padLen)}, padLen)...)
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty pkcs7 data")
	}
	padLen := int(data[len(data)-1])
	if padLen <= 0 || padLen > len(data) {
		return nil, fmt.Errorf("invalid pkcs7 padding")
	}
	return data[:len(data)-padLen], nil
}

func rsaPKCS1Hex(data interface{}) (string, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	block, _ := pem.Decode([]byte(kugouLitePublicKey))
	if block == nil {
		return "", fmt.Errorf("invalid kugou rsa public key")
	}
	pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", err
	}
	pub, ok := pubAny.(*rsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("invalid kugou rsa public key type")
	}
	enc, err := rsa.EncryptPKCS1v15(cryptorand.Reader, pub, payload)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(enc)), nil
}

func joinKugouCookieMap(cookies map[string]string) string {
	keys := make([]string, 0, len(cookies))
	for key := range cookies {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+cookies[key])
	}
	return strings.Join(parts, "; ")
}
