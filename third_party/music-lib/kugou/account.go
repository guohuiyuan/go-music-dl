package kugou

import (
	"encoding/json"
	"fmt"
	"github.com/guohuiyuan/music-lib/utils"
	"strings"
)

func IsVipAccount() (bool, error) { return defaultKugou.IsVipAccount() }

// IsVipAccount 返回当前 cookie 是否已经探测到可用的 VIP 音质链路。
func (k *Kugou) IsVipAccount() (bool, error) {
	if k.isVipCache != nil {
		return *k.isVipCache, nil
	}

	if strings.TrimSpace(k.cookie) == "" {
		isVip := false
		k.isVipCache = &isVip
		return false, nil
	}

	body, err := utils.Get(VIPInfoAPI,
		utils.WithHeader("User-Agent", PCUserAgent),
		utils.WithHeader("Accept", "*/*"),
		utils.WithHeader("Host", "vip.kugou.com"),
		utils.WithHeader("Connection", "keep-alive"),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return false, fmt.Errorf("failed to fetch kugou vip info: %w", err)
	}

	var resp struct {
		Errno           int    `json:"errno"`
		ErrorCode       int    `json:"error_code"`
		Role            int    `json:"role"`
		UserType        int    `json:"user_type"`
		VIPRemains      int    `json:"vipRemains"`
		VIPEndTime      string `json:"vipEndTime"`
		RawVIPEndTime   string `json:"rawVipEndTime"`
		MusicEndTime    string `json:"musicEndTime"`
		IsExpiredMember int    `json:"isExpiredMember"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return false, fmt.Errorf("kugou vip info json parse error: %w", err)
	}

	isVip := resp.Errno == 0 &&
		resp.ErrorCode == 0 &&
		resp.VIPRemains > 0 &&
		resp.IsExpiredMember == 0 &&
		resp.Role != 0
	k.isVipCache = &isVip
	return isVip, nil
}
