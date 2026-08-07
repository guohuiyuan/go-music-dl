package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"
)

var defaultClient = &http.Client{
	Timeout: 2 * time.Minute,
}

// RequestOption 定义请求选项函数
type RequestOption func(*http.Request)

// WithHeader 添加自定义 Header
func WithHeader(key, value string) RequestOption {
	return func(req *http.Request) {
		req.Header.Set(key, value)
	}
}

// Get 发送 HTTP GET 请求
func Get(url string, opts ...RequestOption) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// 默认 UA (模拟 Chrome)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// 应用额外选项
	for _, opt := range opts {
		opt(req)
	}

	resp, err := defaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http request failed: status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// Post 发送 HTTP POST 请求
// body 这里的类型是 io.Reader，可以传 strings.NewReader(form.Encode())
func Post(url string, body io.Reader, opts ...RequestOption) ([]byte, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}

	// 默认 UA (模拟 Chrome)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// 默认 content type 如果用户没传，通常默认 json 或 form，这里留给 opts 控制

	for _, opt := range opts {
		opt(req)
	}

	resp, err := defaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http post failed: status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// MD5 计算字符串哈希
func MD5(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}
