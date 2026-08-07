package soda

import (
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestExtractSodaMFAFields(t *testing.T) {
	body := []byte(`{
		"data": {
			"encrypt_uid": "encrypted-user",
			"mobile": "182******92",
			"verify": {
				"std_verify_flow_id": "flow.login",
				"std_verify_token": "token_lf",
				"std_verify_scene": "account_login",
				"std_verify_template": "ato",
				"std_verify_type": "MFA"
			}
		}
	}`)

	if got := extractSodaMFAField(body, "encrypt_uid"); got != "encrypted-user" {
		t.Fatalf("encrypt_uid = %q, want encrypted-user", got)
	}
	if got := extractSodaMFAField(body, "mobile"); got != "182******92" {
		t.Fatalf("mobile = %q, want masked mobile", got)
	}

	params := extractSodaMFAVerifyParams(body, map[string]string{"passport_mfa_token": "mfa"})
	for _, key := range []string{"std_verify_flow_id", "std_verify_token", "std_verify_scene"} {
		if params == "" || !containsQueryParam(params, key) {
			t.Fatalf("verify params %q missing %s", params, key)
		}
	}
	if containsQueryParam(params, "passport_mfa_token") {
		t.Fatalf("verify params should not include passport_mfa_token: %q", params)
	}
}

func TestSodaQRLoginPendingStateMergesCookies(t *testing.T) {
	token := "unit-token"
	clearSodaQRLoginPending(token)
	rememberSodaQRLoginPending(token, sodaQRLoginPendingState{
		Cookies:      map[string]string{"sessionid": "old"},
		EncryptUID:   "uid",
		VerifyParams: "std_verify_token=t",
	})

	state, ok := getSodaQRLoginPending(token)
	if !ok {
		t.Fatal("pending login state not found")
	}
	merged := mergeSodaCookies(state.Cookies, map[string]string{"d_ticket": "ticket"})
	if merged["sessionid"] != "old" || merged["d_ticket"] != "ticket" {
		t.Fatalf("merged cookies mismatch: %#v", merged)
	}
	clearSodaQRLoginPending(token)
}

func TestSodaQRCodeImageURL(t *testing.T) {
	dataURI := "data:image/png;base64,iVBORw0KGgo="
	if got := sodaQRCodeImageURL(dataURI); got != dataURI {
		t.Fatalf("data URI should pass through: %q", got)
	}

	rawBase64 := "iVBORw0KGgo="
	if got := sodaQRCodeImageURL(rawBase64); got != "data:image/png;base64,"+rawBase64 {
		t.Fatalf("raw base64 should be wrapped: %q", got)
	}
}

func TestSodaQRConnectResultConfirmedWithSessionCookieSucceeds(t *testing.T) {
	token := "unit-token"
	var resp sodaQRConnectResponse
	resp.Data.Status = "confirmed"
	resp.Message = "success"

	result := (&Soda{}).sodaQRConnectResult(token, []byte(`{"data":{"status":"confirmed"},"message":"success"}`), map[string]string{
		"sessionid": "sid",
		"sid_tt":    "sid",
	}, resp)

	if result.Status != model.QRLoginStatusSuccess {
		t.Fatalf("status = %s, want success", result.Status)
	}
	if !strings.Contains(result.Cookie, "sessionid=sid") {
		t.Fatalf("cookie missing sessionid: %q", result.Cookie)
	}
}

func TestSodaQRConnectResultScannedStateDoesNotRegressToWaiting(t *testing.T) {
	token := "unit-token-scanned-cache"
	clearSodaQRLoginPending(token)

	var scannedResp sodaQRConnectResponse
	scannedResp.Data.Status = "scanned"
	scannedResp.Message = "success"
	result := (&Soda{}).sodaQRConnectResult(token, []byte(`{"data":{"status":"scanned"},"message":"success"}`), nil, scannedResp)
	if result.Status != model.QRLoginStatusScanned {
		t.Fatalf("status = %s, want scanned", result.Status)
	}

	state, ok := getSodaQRLoginPending(token)
	if !ok {
		t.Fatal("scanned state was not cached")
	}
	throttled := sodaThrottledResult(token, state)
	if throttled.Status != model.QRLoginStatusScanned {
		t.Fatalf("throttled status = %s, want scanned", throttled.Status)
	}
	if throttled.Message == "等待扫码确认" {
		t.Fatalf("throttled message regressed to waiting: %q", throttled.Message)
	}

	var waitingResp sodaQRConnectResponse
	waitingResp.Data.Status = "new"
	waitingResp.Message = "success"
	result = (&Soda{}).sodaQRConnectResult(token, []byte(`{"data":{"status":"new"},"message":"success"}`), nil, waitingResp)
	if result.Status != model.QRLoginStatusScanned {
		t.Fatalf("cached scanned status regressed to %s", result.Status)
	}

	clearSodaQRLoginPending(token)
}

func TestSodaQRConnectResultErrorIncludesAPIStatus(t *testing.T) {
	var resp sodaQRConnectResponse
	resp.Data.Status = "error"
	resp.Data.ErrorCode = 16
	resp.Message = "error"

	result := (&Soda{}).sodaQRConnectResult("unit-token", nil, nil, resp)
	if result.Status != model.QRLoginStatusFailed {
		t.Fatalf("status = %s, want failed", result.Status)
	}
	if !strings.Contains(result.Message, "code=16") {
		t.Fatalf("message should include error code, got %q", result.Message)
	}
	if result.Extra["api_status"] != "error" || result.Extra["error_code"] != "16" {
		t.Fatalf("extra mismatch: %#v", result.Extra)
	}
}

func TestSodaQRConnectResultErrorWithMFAShowsSMSFlow(t *testing.T) {
	token := "unit-token-mfa"
	clearSodaQRLoginPending(token)
	var resp sodaQRConnectResponse
	resp.Data.Status = "error"
	resp.Data.ErrorCode = 1105
	resp.Message = "verify required"

	body := []byte(`{
		"data": {
			"encrypt_uid": "encrypted-user",
			"mobile": "182******92",
			"verify": {
				"std_verify_flow_id": "flow.login",
				"std_verify_token": "token_lf",
				"std_verify_scene": "account_login",
				"std_verify_template": "ato",
				"std_verify_type": "MFA"
			}
		}
	}`)
	result := (&Soda{}).sodaQRConnectResult(token, body, map[string]string{
		"passport_mfa_token": "mfa-token",
	}, resp)

	if result.Status != model.QRLoginStatusScanned {
		t.Fatalf("status = %s, want scanned", result.Status)
	}
	if result.Extra["need_sms"] != "true" {
		t.Fatalf("need_sms not set: %#v", result.Extra)
	}
	if result.Extra["encrypt_uid"] != "encrypted-user" {
		t.Fatalf("encrypt_uid mismatch: %#v", result.Extra)
	}
	if _, ok := getSodaQRLoginPending(token); !ok {
		t.Fatal("MFA pending state was not recorded")
	}
	clearSodaQRLoginPending(token)
}

func TestSodaQRConnectResultVerifyAccountFlowTriggersMFA(t *testing.T) {
	token := "unit-token-mfa-2046"
	clearSodaQRLoginPending(token)
	var resp sodaQRConnectResponse
	resp.Data.Status = ""
	resp.Data.AccountFlow = "verify"
	resp.Data.ErrorCode = 2046
	resp.Message = "error"

	body := []byte(`{
		"data": {
			"account_flow": "verify",
			"encrypt_uid": "fXk7m7ghB+QOAxph2pEnkmXFNwvisB2Jelr0Ky0gCTXpAorULtJc",
			"error_code": 2046,
			"biz_params": {
				"passport_mfa_retry_tag": "1",
				"std_verify_flow_id": "d3b23102-3ed3-4934-97df-a5f347970630.login",
				"std_verify_scene": "account_login",
				"std_verify_template": "ato",
				"std_verify_token": "f64e4da4-4ef3-11f1-9b86-00620b590b2a_lq",
				"std_verify_type": "MFA",
				"std_verify_way": ""
			},
			"verify_ways": [
				{"act_type":"22","mobile":"159******49","verify_way":"mobile_sms_verify"},
				{"channel_mobile":"9515211003","mobile":"159******49","sms_content":"YZ","verify_way":"mobile_up_sms_verify"}
			]
		},
		"message": "error"
	}`)

	result := (&Soda{}).sodaQRConnectResult(token, body, map[string]string{
		"passport_mfa_token": "mfa-token",
	}, resp)

	if result.Status != model.QRLoginStatusScanned {
		t.Fatalf("status = %s, want scanned", result.Status)
	}
	if result.Extra["need_sms"] != "true" {
		t.Fatalf("need_sms not set: %#v", result.Extra)
	}
	if !strings.HasPrefix(result.Extra["encrypt_uid"], "fXk7m7gh") {
		t.Fatalf("encrypt_uid mismatch: %#v", result.Extra["encrypt_uid"])
	}
	if !containsQueryParam(result.Extra["verify_params"], "std_verify_token") {
		t.Fatalf("verify_params missing std_verify_token: %q", result.Extra["verify_params"])
	}
	if !containsQueryParam(result.Extra["verify_params"], "passport_mfa_retry_tag") {
		t.Fatalf("verify_params missing passport_mfa_retry_tag: %q", result.Extra["verify_params"])
	}
	if result.Extra["sms_mode"] != "sms" || result.Extra["can_up_sms"] != "true" {
		t.Fatalf("sms mode should prefer normal sms with up-sms fallback: %#v", result.Extra)
	}
	if result.Extra["up_sms_mobile"] != "9515211003" || result.Extra["up_sms_content"] != "YZ" {
		t.Fatalf("up sms instruction mismatch: %#v", result.Extra)
	}
	state, ok := getSodaQRLoginPending(token)
	if !ok {
		t.Fatal("MFA pending state was not recorded")
	}
	if state.EncryptUID == "" {
		t.Fatal("pending state missing encrypt_uid")
	}
	if state.UpSMSMobile != "9515211003" || state.UpSMSContent != "YZ" {
		t.Fatalf("pending up sms mismatch: %#v", state)
	}
	clearSodaQRLoginPending(token)
}

func TestSodaQRPollAllowedThrottlesRepeatedCalls(t *testing.T) {
	token := "unit-token-throttle"
	sodaQRPollForget(token)
	if !sodaQRPollAllowed(token) {
		t.Fatal("first call should be allowed")
	}
	if sodaQRPollAllowed(token) {
		t.Fatal("immediate second call must be throttled")
	}
	sodaQRPollForget(token)
	if !sodaQRPollAllowed(token) {
		t.Fatal("after forget, next call should be allowed again")
	}
	sodaQRPollForget(token)
}

func TestSodaQRConnectResultRateLimitedBacksOff(t *testing.T) {
	token := "unit-token-rate-limited"
	sodaQRPollForget(token)
	clearSodaQRLoginPending(token)

	var resp sodaQRConnectResponse
	resp.Data.ErrorCode = 7
	resp.Data.Description = "访问太频繁，请稍后再试"
	resp.Message = "error"

	result := (&Soda{}).sodaQRConnectResult(token, []byte(`{"data":{"error_code":7,"description":"访问太频繁"},"message":"error"}`), nil, resp)

	if result.Status != model.QRLoginStatusWaiting {
		t.Fatalf("status = %s, want waiting (back off, not failure)", result.Status)
	}
	if result.Extra["rate_limited"] != "true" {
		t.Fatalf("rate_limited flag missing: %#v", result.Extra)
	}
	// Subsequent allow check must be denied during the backoff window.
	if sodaQRPollAllowed(token) {
		t.Fatal("rate-limited token should remain throttled after backoff")
	}
	sodaQRPollForget(token)
}

func TestSodaThrottledResultUsesPendingEncryptUID(t *testing.T) {
	r := sodaThrottledResult("tok", sodaQRLoginPendingState{
		EncryptUID:   "uid",
		VerifyParams: "std_verify_token=t",
	})
	if r.Status != model.QRLoginStatusScanned {
		t.Fatalf("status = %s, want scanned", r.Status)
	}
	if r.Extra["need_sms"] != "true" || r.Extra["encrypt_uid"] != "uid" {
		t.Fatalf("extra mismatch: %#v", r.Extra)
	}
}

func TestSodaEncodeSMSCodeMatchesOfficialPCShape(t *testing.T) {
	if got := sodaEncodeSMSCode("661701"); got != "363631373031" {
		t.Fatalf("encoded sms code = %q, want 363631373031", got)
	}
}

func TestSodaQRConnectFormMatchesOfficialPCShape(t *testing.T) {
	createQuery := buildSodaQRCreateQuery()
	for _, key := range []string{
		"passport_jssdk_version=2.4.13",
		"passport_jssdk_type=normal",
		"is_from_ttaccountsdk=1",
		"aid=386088",
		"language=zh",
		"account_sdk_source=web",
		"p_js_v=2.4.13",
		"p_js_t=pro",
		"p_zt=3.3.5",
		"p_ver=1.0.29",
		"request_host=app%253A%252F%252Fresources",
		"p_bd=1.0.0.41",
		"is_new_login=1",
		"is_from_iesaccountsaas=1",
		"device_platform=PC",
		"version_code=3.3.0",
		"next=https%3A%2F%2Fapi.qishui.com",
		"need_logo=false",
		"need_short_url=false",
		"is_frontier=true",
	} {
		if !strings.Contains(createQuery, key) {
			t.Fatalf("create query %q missing %s", createQuery, key)
		}
	}

	checkQuery := buildSodaQRCheckQuery()
	for _, key := range []string{
		"passport_jssdk_version=2.4.13",
		"passport_jssdk_type=normal",
		"is_from_ttaccountsdk=1",
		"aid=386088",
		"language=zh",
		"account_sdk_source=web",
		"p_js_v=2.4.13",
		"p_js_t=pro",
		"p_zt=3.3.5",
		"p_ver=1.0.29",
		"request_host=app%253A%252F%252Fresources",
		"p_bd=1.0.0.41",
		"is_new_login=1",
		"is_from_iesaccountsaas=1",
		"device_platform=PC",
		"version_code=3.3.0",
	} {
		if !strings.Contains(checkQuery, key) {
			t.Fatalf("check query %q missing %s", checkQuery, key)
		}
	}

	form := sodaQRConnectForm("token")
	if form.Get("token") != "token" || form.Get("is_frontier") != "true" || form.Get("next") != "https://api.qishui.com" {
		t.Fatalf("unexpected check form: %s", form.Encode())
	}
	if form.Get("aid") != "" || form.Get("passport_jssdk_version") != "" {
		t.Fatalf("check form should not duplicate query params: %s", form.Encode())
	}

	mfaForm := sodaQRConnectForm("token")
	applySodaVerifyParams(mfaForm, "passport_mfa_retry_tag=1&std_verify_flow_id=5eb1ab29-7c00-48a3-8b57-84f1ae7c8a8e.login&std_verify_scene=account_login&std_verify_template=ato&std_verify_token=e3e9d17b-50dc-11f1-8bb9-043f72b9dba6_hl&std_verify_type=MFA&std_verify_way=")
	for _, key := range []string{
		"passport_mfa_retry_tag=1",
		"std_verify_flow_id=5eb1ab29-7c00-48a3-8b57-84f1ae7c8a8e.login",
		"std_verify_scene=account_login",
		"std_verify_template=ato",
		"std_verify_token=e3e9d17b-50dc-11f1-8bb9-043f72b9dba6_hl",
		"std_verify_type=MFA",
		"std_verify_way=",
	} {
		if !strings.Contains(mfaForm.Encode(), key) {
			t.Fatalf("mfa check form %q missing %s", mfaForm.Encode(), key)
		}
	}

	liteQuery := buildSodaPassportLiteQuery()
	for _, key := range []string{
		"passport_jssdk_version=5.1.2",
		"passport_jssdk_type=lite",
		"account_app_language=en-US",
		"new_authn_sdk_version=1.0.0.404-web",
		"did=",
		"iid=",
		"biz_trace_id=",
	} {
		if !strings.Contains(liteQuery, key) {
			t.Fatalf("lite query %q missing %s", liteQuery, key)
		}
	}
}

func containsQueryParam(raw, key string) bool {
	return strings.HasPrefix(raw, key+"=") || strings.Contains(raw, "&"+key+"=")
}
