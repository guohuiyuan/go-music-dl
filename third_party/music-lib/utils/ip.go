package utils

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// RandomChinaIP 生成一个随机的中国大陆 IP 地址
func RandomChinaIP() string {
	// 常见的中国大陆 IP 段前缀 (电信/联通/移动)
	prefixes := [][]int{
		{116, 255, 0, 0},
		{116, 228, 0, 0},
		{218, 192, 0, 0},
		{124, 0, 0, 0},
		{14, 132, 0, 0},
		{183, 14, 0, 0},
		{58, 14, 0, 0},
		{113, 116, 0, 0},
		{120, 230, 0, 0},
	}

	prefix := prefixes[rand.Intn(len(prefixes))]
	return fmt.Sprintf("%d.%d.%d.%d",
		prefix[0],
		prefix[1],
		rand.Intn(254)+1,
		rand.Intn(254)+1,
	)
}

// WithRandomIPHeader 为请求添加随机的国内 IP Header
func WithRandomIPHeader() RequestOption {
	return func(req *http.Request) {
		ip := RandomChinaIP()
		req.Header.Set("X-Forwarded-For", ip)
		req.Header.Set("X-Real-IP", ip)
	}
}
