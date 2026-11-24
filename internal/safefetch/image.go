// Package safefetch fetches remote images after rejecting non-public addresses.
package safefetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrUnsafeURL 表示目标不是允许抓取的公网 http(s) 地址。
var ErrUnsafeURL = errors.New("URL is not a public http(s) URL")

// DefaultMaxImageBytes 是单张远程图片的默认大小上限。
const DefaultMaxImageBytes int64 = 8 << 20

var imageHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		Proxy:                 nil,
		DialContext:           dialPublicAddress,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		DisableKeepAlives:     true,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("too many redirects")
		}
		return ValidateURL(req.URL)
	},
}

// FetchImage 只抓取公网 http(s) 图片，并返回按内容识别出的类型。
func FetchImage(rawURL, userAgent, referer string, maxBytes int64) ([]byte, string, error) {
	rawURL = strings.TrimSpace(rawURL)
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", ErrUnsafeURL
	}
	if err := ValidateURL(parsed); err != nil {
		return nil, "", err
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxImageBytes
	}

	req, err := http.NewRequest(http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", ErrUnsafeURL
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/png,image/jpeg,image/gif,image/*;q=0.8")

	resp, err := imageHTTPClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("upstream status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 {
		return nil, "", errors.New("upstream returned empty body")
	}
	if int64(len(data)) > maxBytes {
		return nil, "", errors.New("image exceeds size limit")
	}
	contentType := DetectImageType(data)
	if contentType == "" {
		return nil, "", errors.New("upstream is not a supported image")
	}
	return data, contentType, nil
}

// ValidateURL 检查 URL 是否为可抓取的公网 http(s) 地址。
func ValidateURL(parsed *url.URL) error {
	if parsed == nil {
		return ErrUnsafeURL
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrUnsafeURL
	}
	if parsed.User != nil {
		return ErrUnsafeURL
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "" {
		return ErrUnsafeURL
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return ErrUnsafeURL
	}
	if ip := net.ParseIP(host); ip != nil && !IsPublicIP(ip) {
		return ErrUnsafeURL
	}
	return nil
}

func dialPublicAddress(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	var lastErr error
	for _, candidate := range ips {
		if !IsPublicIP(candidate.IP) {
			lastErr = fmt.Errorf("blocked non-public address for %s", host)
			continue
		}
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no public address for %s", host)
	}
	return nil, lastErr
}

// IsPublicIP 判断 IP 是否属于可访问的公网单播地址。
func IsPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		switch {
		case ip4[0] == 0:
			return false
		case ip4[0] == 10:
			return false
		case ip4[0] == 127:
			return false
		case ip4[0] == 169 && ip4[1] == 254:
			return false
		case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
			return false
		case ip4[0] == 192 && ip4[1] == 168:
			return false
		case ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127: // 100.64.0.0/10 CGNAT
			return false
		case ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 0: // 192.0.0.0/24
			return false
		case ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 2: // 192.0.2.0/24 TEST-NET-1
			return false
		case ip4[0] == 198 && (ip4[1] == 18 || ip4[1] == 19): // 198.18.0.0/15
			return false
		case ip4[0] == 198 && ip4[1] == 51 && ip4[2] == 100: // 198.51.100.0/24 TEST-NET-2
			return false
		case ip4[0] == 203 && ip4[1] == 0 && ip4[2] == 113: // 203.0.113.0/24 TEST-NET-3
			return false
		case ip4[0] >= 240:
			return false
		}
		return true
	}
	// fec0::/10 旧站点本地地址
	if len(ip) == net.IPv6len && ip[0] == 0xfe && (ip[1]&0xc0) == 0xc0 {
		return false
	}
	// 2001:db8::/32 文档地址
	if len(ip) == net.IPv6len && ip[0] == 0x20 && ip[1] == 0x01 && ip[2] == 0x0d && ip[3] == 0xb8 {
		return false
	}
	return true
}

// DetectImageType 只接受常见的位图格式，拒绝 HTML/SVG 等可执行内容。
func DetectImageType(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	switch http.DetectContentType(data) {
	case "image/jpeg":
		return "image/jpeg"
	case "image/png":
		return "image/png"
	case "image/gif":
		return "image/gif"
	case "image/webp":
		return "image/webp"
	}
	if len(data) >= 12 && string(data[4:8]) == "ftyp" {
		brand := string(data[8:12])
		if brand == "avif" || brand == "avis" {
			return "image/avif"
		}
	}
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	return ""
}

// FileExtension 返回图片类型对应的常见文件后缀。
func FileExtension(contentType string) string {
	switch contentType {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/avif":
		return ".avif"
	default:
		return ".jpg"
	}
}
