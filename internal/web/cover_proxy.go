package web

import (
	"net"
	"net/url"

	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/go-music-dl/internal/safefetch"
)

const maxCoverImageBytes = safefetch.DefaultMaxImageBytes

var errUnsafeCoverURL = safefetch.ErrUnsafeURL

// fetchCoverImage 是封面抓取的测试注入点；生产环境使用公共网络地址校验后的抓取器。
var fetchCoverImage = fetchPublicCoverImage

func fetchPublicCoverImage(rawURL, source string) ([]byte, string, error) {
	userAgent := core.UA_Common
	if source == "migu" {
		userAgent = core.UA_Mobile
	}
	return safefetch.FetchImage(rawURL, userAgent, core.SourceReferer(source), maxCoverImageBytes)
}

func validateCoverURL(parsed *url.URL) error {
	return safefetch.ValidateURL(parsed)
}

func isPublicIP(ip net.IP) bool {
	return safefetch.IsPublicIP(ip)
}

func detectCoverImageType(data []byte) string {
	return safefetch.DetectImageType(data)
}

func coverFileExtension(contentType string) string {
	return safefetch.FileExtension(contentType)
}
