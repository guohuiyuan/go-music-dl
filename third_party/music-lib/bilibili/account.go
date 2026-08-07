package bilibili

import (
	"encoding/json"
	"github.com/guohuiyuan/music-lib/utils"
)

// IsVipAccount 检测 Bilibili 账号是否为大会员
func (b *Bilibili) IsVipAccount() (bool, error) {
	if b.isVipCache != nil {
		return *b.isVipCache, nil
	}

	if b.cookie == "" {
		isVip := false
		b.isVipCache = &isVip
		return false, nil
	}

	apiURL := "https://api.bilibili.com/x/web-interface/nav"
	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Referer", Referer), utils.WithHeader("Cookie", b.cookie))
	if err != nil {
		return false, err
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			IsLogin   bool `json:"isLogin"`
			VipStatus int  `json:"vipStatus"`
			VipType   int  `json:"vipType"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return false, err
	}

	isVip := false
	if resp.Code == 0 && resp.Data.IsLogin && resp.Data.VipStatus == 1 && resp.Data.VipType > 0 {
		isVip = true
	}

	b.isVipCache = &isVip
	return isVip, nil
}
