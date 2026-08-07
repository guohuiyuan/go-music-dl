package netease

import (
	"encoding/json"
	"fmt"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"strings"
)

// IsVipAccount reports whether the current account is VIP.
func (n *Netease) IsVipAccount() (bool, error) {
	if n.isVipCache != nil {
		return *n.isVipCache, nil
	}

	if n.cookie == "" {
		isVip := false
		n.isVipCache = &isVip
		return false, nil
	}

	if cached, ok := n.getCachedVIPStatus(); ok {
		n.isVipCache = &cached
		return cached, nil
	}

	reqData := map[string]interface{}{
		"csrf_token": "",
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post(UserAccountAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return false, fmt.Errorf("failed to fetch user account info: %w", err)
	}

	var resp struct {
		Code    int `json:"code"`
		Profile struct {
			VipType int `json:"vipType"`
		} `json:"profile"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return false, fmt.Errorf("netease user account json parse error: %w", err)
	}

	isVip := resp.Code == 200 && resp.Profile.VipType != 0
	n.isVipCache = &isVip
	n.setCachedVIPStatus(isVip)
	return isVip, nil
}
