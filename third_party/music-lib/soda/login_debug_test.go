package soda

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/guohuiyuan/music-lib/model"
)

func TestSodaQRLoginDebug(t *testing.T) {
	if os.Getenv("SODA_QR_DEBUG") != "1" {
		t.Skip("set SODA_QR_DEBUG=1 to run Soda QR login debug test")
	}

	fmt.Println("=== 汽水扫码测试 ===")
	session, err := CreateQRLogin()
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	fmt.Printf("URL: %s\nToken: %s\nURL长度: %d\n", sodaRedactURLSecrets(session.URL), sodaRedactValue(session.Key), len(session.URL))

	outputDir := strings.TrimSpace(os.Getenv("SODA_QR_OUTPUT_DIR"))
	if outputDir == "" {
		outputDir = t.TempDir()
	} else if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("创建输出目录失败: %v", err)
	}

	qrURLPath := filepath.Join(outputDir, "qr_url.txt")
	if err := os.WriteFile(qrURLPath, []byte(session.URL), 0644); err != nil {
		t.Fatalf("写入 qr_url.txt 失败: %v", err)
	}
	fmt.Printf("URL已写入 %s\n", qrURLPath)

	qrPagePath := filepath.Join(outputDir, "soda_qr.html")
	if err := writeLocalQRCodeFiles(outputDir, session.URL, session.ImageURL); err != nil {
		fmt.Printf("二维码文件生成失败: %v\n", err)
	} else {
		if strings.TrimSpace(session.ImageURL) != "" {
			fmt.Printf("官方二维码页面已写入 %s\n", qrPagePath)
		} else {
			fmt.Printf("二维码已写入 %s 和 %s\n", filepath.Join(outputDir, "soda_qr.svg"), qrPagePath)
		}
		if os.Getenv("SODA_QR_OPEN") == "1" {
			openLocalQRCodePage(qrPagePath)
		}
	}
	if os.Getenv("SODA_QR_CREATE_ONLY") == "1" {
		return
	}
	fmt.Println("请扫码并在手机上确认登录")
	if os.Getenv("SODA_QR_AUTO_POLL") == "1" {
		fmt.Println("立即开始轮询...")
	} else {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("扫码后按 Enter 开始轮询...")
		if _, err := reader.ReadString('\n'); err != nil {
			fmt.Printf("未检测到交互输入，已退出轮询: %v\n", err)
			return
		}
	}

	token := session.Key
	scannedSeen := false
	pollInterval := sodaQRDebugPollInterval()
	fmt.Printf("轮询间隔: %s\n", pollInterval)

	for i := 0; ; i++ {
		result, err := debugCheckSodaQRLogin(token)
		if err != nil {
			fmt.Printf("[%d %s] err: %v\n", i, time.Now().Format("15:04:05"), err)
			time.Sleep(pollInterval)
			continue
		}

		j, _ := json.Marshal(redactedQRLoginResult(result))
		fmt.Printf("[%d %s] %s\n", i, time.Now().Format("15:04:05"), string(j))

		if result.Status == model.QRLoginStatusSuccess {
			fmt.Printf("\n✅ 登录成功! Cookie长度=%d\n", len(result.Cookie))
			return
		}

		if result.Extra != nil && result.Extra["need_sms"] == "true" {
			handleMFADebug(t, token, result)
			return
		}

		if result.Status == model.QRLoginStatusScanned {
			if !scannedSeen {
				fmt.Println("  -> 已扫码! 请在手机上确认登录，继续轮询...")
				scannedSeen = true
			} else {
				fmt.Println("  -> 等待手机确认中...")
			}
			time.Sleep(pollInterval)
			continue
		}

		if result.Status == model.QRLoginStatusFailed {
			if strings.Contains(result.Message, "频繁") {
				t.Fatalf("限流了，等汽水接口冷却后重新运行: %s", result.Message)
			}
			t.Fatalf("失败: %s", result.Message)
		}

		if result.Status == model.QRLoginStatusExpired {
			t.Fatal("二维码过期")
		}

		time.Sleep(pollInterval)
	}
}

func debugCheckSodaQRLogin(token string) (*model.QRLoginResult, error) {
	return CheckQRLogin(token)
}

func sodaQRDebugPollInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv("SODA_QR_POLL_INTERVAL"))
	if raw == "" {
		return 2 * time.Second
	}
	interval, err := time.ParseDuration(raw)
	if err != nil || interval <= 0 {
		return 2 * time.Second
	}
	return interval
}

func handleMFADebug(t *testing.T, token string, result *model.QRLoginResult) {
	t.Helper()
	fmt.Println("\n=== 拿到MFA参数 ===")
	if result.Extra != nil {
		fmt.Printf("手机号: %s\n", result.Extra["mobile"])
	}
	forceSendCode := os.Getenv("SODA_QR_TRY_SEND_CODE") == "1"
	if result.Extra != nil && !forceSendCode && (result.Extra["need_user_sms"] == "true" || result.Extra["sms_mode"] == "up") {
		target := strings.TrimSpace(result.Extra["up_sms_mobile"])
		content := strings.TrimSpace(result.Extra["up_sms_content"])
		if target == "" {
			target = "指定号码"
		}
		if content == "" {
			content = "指定内容"
		}
		fmt.Printf("请使用绑定手机号发送短信 %s 到 %s\n", content, target)
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("发送完成后按 Enter 确认...")
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("等待确认失败: %v", err)
		}

		confirmRes, err := CheckQRLogin(token + "|up_sms||")
		if err != nil {
			t.Fatalf("up_sms 失败: %v", err)
		}
		j, _ := json.Marshal(redactedQRLoginResult(confirmRes))
		fmt.Printf("up_sms: %s\n", string(j))
		if confirmRes.Status == model.QRLoginStatusSuccess {
			fmt.Printf("\n✅ 登录成功! Cookie长度=%d\n", len(confirmRes.Cookie))
			return
		}
		t.Fatalf("未成功: %s", confirmRes.Message)
	}

	fmt.Println("调用 send_code 发送验证码...")
	sendRes, err := CheckQRLogin(token + "|send_code||")
	if err != nil {
		t.Fatalf("send_code 失败: %v", err)
	}
	j, _ := json.Marshal(redactedQRLoginResult(sendRes))
	fmt.Printf("send_code: %s\n", string(j))

	if sendRes.Status == model.QRLoginStatusSuccess {
		fmt.Printf("\n✅ 登录成功! Cookie长度=%d\n", len(sendRes.Cookie))
		return
	}
	if sendRes.Status == model.QRLoginStatusFailed {
		t.Fatalf("发送失败: %s", sendRes.Message)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\n请输入短信验证码: ")
	code, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("读取验证码失败: %v", err)
	}
	code = strings.TrimSpace(code)
	if code == "" {
		t.Fatal("验证码为空，已停止验证")
	}

	fmt.Println("验证中...")
	vRes, err := CheckQRLogin(token + "|validate|||" + code)
	if err != nil {
		t.Fatalf("validate 失败: %v", err)
	}
	j, _ = json.Marshal(redactedQRLoginResult(vRes))
	fmt.Printf("validate: %s\n", string(j))

	if vRes.Status == model.QRLoginStatusSuccess {
		fmt.Printf("\n✅ 登录成功! Cookie长度=%d\n", len(vRes.Cookie))
	} else {
		t.Fatalf("未成功: %s", vRes.Message)
	}
}

func redactedQRLoginResult(result *model.QRLoginResult) *model.QRLoginResult {
	if result == nil {
		return nil
	}
	redacted := *result
	redacted.Key = sodaRedactValue(redacted.Key)
	if redacted.Cookie != "" {
		redacted.Cookie = fmt.Sprintf("<cookie len=%d>", len(redacted.Cookie))
	}
	if len(redacted.Cookies) > 0 {
		cookies := make(map[string]string, len(redacted.Cookies))
		for key, value := range redacted.Cookies {
			if value == "" {
				cookies[key] = ""
			} else {
				cookies[key] = "***"
			}
		}
		redacted.Cookies = cookies
	}
	if len(redacted.Extra) > 0 {
		extra := make(map[string]string, len(redacted.Extra))
		for key, value := range redacted.Extra {
			switch key {
			case "encrypt_uid", "verify_params", "token", "mfa_token", "passport_mfa_token":
				extra[key] = sodaRedactValue(value)
			default:
				extra[key] = value
			}
		}
		redacted.Extra = extra
	}
	return &redacted
}

type qrSpec struct {
	data   int
	ecc    int
	blocks int
	align  []int
}

type qrBlock struct {
	data []int
	ecc  []int
}

type qrMatrix struct {
	size    int
	modules [][]bool
	fixed   [][]bool
}

var qrVersionSpecs = []qrSpec{
	{},
	{data: 19, ecc: 7, blocks: 1, align: nil},
	{data: 34, ecc: 10, blocks: 1, align: []int{6, 18}},
	{data: 55, ecc: 15, blocks: 1, align: []int{6, 22}},
	{data: 80, ecc: 20, blocks: 1, align: []int{6, 26}},
	{data: 108, ecc: 26, blocks: 1, align: []int{6, 30}},
	{data: 136, ecc: 18, blocks: 2, align: []int{6, 34}},
}

func writeLocalQRCodeFiles(dir, text, imageURL string) error {
	if strings.TrimSpace(imageURL) != "" {
		page := localQRCodeHTML(imageURL, text)
		return os.WriteFile(filepath.Join(dir, "soda_qr.html"), []byte(page), 0644)
	}
	qr, err := buildQRMatrix(text)
	if err != nil {
		return err
	}
	svg := qrSVG(qr)
	if err := os.WriteFile(filepath.Join(dir, "soda_qr.svg"), []byte(svg), 0644); err != nil {
		return err
	}
	page := localQRCodeHTML("soda_qr.svg", text)
	return os.WriteFile(filepath.Join(dir, "soda_qr.html"), []byte(page), 0644)
}

func localQRCodeHTML(imageSrc, text string) string {
	return `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<title>汽水扫码登录</title>
<style>
body{margin:0;min-height:100vh;display:grid;place-items:center;background:#f8fafc;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:#0f172a}
.card{width:min(92vw,440px);padding:28px;border-radius:22px;background:white;box-shadow:0 22px 70px rgba(15,23,42,.14);text-align:center}
img{width:320px;max-width:82vw;height:auto;image-rendering:pixelated}
p{margin:12px 0 0;color:#475569}
code{display:block;margin-top:14px;padding:10px;border-radius:10px;background:#f1f5f9;word-break:break-all;font-size:12px;text-align:left}
</style>
</head>
<body>
<div class="card">
<h2>汽水音乐扫码登录</h2>
<img src="` + html.EscapeString(imageSrc) + `" alt="汽水音乐扫码登录二维码">
<p>请使用汽水音乐 App 扫码并在手机上确认登录。</p>
<code>` + html.EscapeString(text) + `</code>
</div>
</body>
</html>`
}

func openLocalQRCodePage(name string) {
	path, err := filepath.Abs(name)
	if err != nil {
		return
	}
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
}

func pickQRVersion(byteLength int) (int, error) {
	for version := 1; version < len(qrVersionSpecs); version++ {
		if byteLength <= (qrVersionSpecs[version].data*8-12)/8 {
			return version, nil
		}
	}
	return 0, fmt.Errorf("二维码内容过长，请使用更短的登录 URL")
}

func appendQRBits(bits *[]bool, value, length int) {
	for i := length - 1; i >= 0; i-- {
		*bits = append(*bits, ((value>>i)&1) != 0)
	}
}

func qrGFMul(x, y int) int {
	z := 0
	for i := 7; i >= 0; i-- {
		z = ((z << 1) ^ (((z >> 7) & 1) * 0x11D)) & 0xFF
		if ((y >> i) & 1) != 0 {
			z ^= x
		}
	}
	return z
}

func qrRSGenerator(degree int) []int {
	result := make([]int, degree)
	result[degree-1] = 1
	root := 1
	for i := 0; i < degree; i++ {
		for j := 0; j < degree; j++ {
			result[j] = qrGFMul(result[j], root)
			if j+1 < degree {
				result[j] ^= result[j+1]
			}
		}
		root = qrGFMul(root, 2)
	}
	return result
}

func qrRSRemainder(data, generator []int) []int {
	result := make([]int, len(generator))
	for _, value := range data {
		factor := value ^ result[0]
		copy(result, result[1:])
		result[len(result)-1] = 0
		for i := range result {
			result[i] ^= qrGFMul(generator[i], factor)
		}
	}
	return result
}

func qrCodewords(text string) (int, []int, error) {
	bytes := []byte(text)
	version, err := pickQRVersion(len(bytes))
	if err != nil {
		return 0, nil, err
	}
	spec := qrVersionSpecs[version]
	bits := make([]bool, 0, spec.data*8)
	appendQRBits(&bits, 0x4, 4)
	appendQRBits(&bits, len(bytes), 8)
	for _, value := range bytes {
		appendQRBits(&bits, int(value), 8)
	}
	maxBits := spec.data * 8
	if len(bits) > maxBits {
		return 0, nil, fmt.Errorf("二维码内容过长")
	}
	for i, n := 0, minInt(4, maxBits-len(bits)); i < n; i++ {
		bits = append(bits, false)
	}
	for len(bits)%8 != 0 {
		bits = append(bits, false)
	}
	data := make([]int, 0, spec.data)
	for i := 0; i < len(bits); i += 8 {
		value := 0
		for j := 0; j < 8; j++ {
			value <<= 1
			if bits[i+j] {
				value |= 1
			}
		}
		data = append(data, value)
	}
	for pad := 0xEC; len(data) < spec.data; pad ^= 0xFD {
		data = append(data, pad)
	}
	generator := qrRSGenerator(spec.ecc)
	blockLen := spec.data / spec.blocks
	blocks := make([]qrBlock, 0, spec.blocks)
	for i := 0; i < spec.blocks; i++ {
		blockData := append([]int(nil), data[i*blockLen:(i+1)*blockLen]...)
		blocks = append(blocks, qrBlock{data: blockData, ecc: qrRSRemainder(blockData, generator)})
	}
	result := make([]int, 0, spec.data+spec.ecc*spec.blocks)
	for i := 0; i < blockLen; i++ {
		for _, block := range blocks {
			result = append(result, block.data[i])
		}
	}
	for i := 0; i < spec.ecc; i++ {
		for _, block := range blocks {
			result = append(result, block.ecc[i])
		}
	}
	return version, result, nil
}

func newQRMatrix(size int) qrMatrix {
	modules := make([][]bool, size)
	fixed := make([][]bool, size)
	for i := 0; i < size; i++ {
		modules[i] = make([]bool, size)
		fixed[i] = make([]bool, size)
	}
	return qrMatrix{size: size, modules: modules, fixed: fixed}
}

func setQRModule(qr *qrMatrix, x, y int, dark, fixed bool) {
	if x < 0 || y < 0 || x >= qr.size || y >= qr.size {
		return
	}
	qr.modules[y][x] = dark
	if fixed {
		qr.fixed[y][x] = true
	}
}

func addQRFinder(qr *qrMatrix, cx, cy int) {
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			dist := maxInt(absInt(dx), absInt(dy))
			setQRModule(qr, cx+dx, cy+dy, dist != 2 && dist != 4, true)
		}
	}
}

func addQRAlignment(qr *qrMatrix, cx, cy int) {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			dist := maxInt(absInt(dx), absInt(dy))
			setQRModule(qr, cx+dx, cy+dy, dist == 0 || dist == 2, true)
		}
	}
}

func qrFormatBits(mask int) int {
	data := (1 << 3) | mask
	bits := data << 10
	for i := 14; i >= 10; i-- {
		if ((bits >> i) & 1) != 0 {
			bits ^= 0x537 << (i - 10)
		}
	}
	return ((data << 10) | (bits & 0x3FF)) ^ 0x5412
}

func placeQRFormat(qr *qrMatrix, mask int) {
	bits := qrFormatBits(mask)
	n := qr.size
	for i := 0; i <= 5; i++ {
		setQRModule(qr, 8, i, ((bits>>i)&1) != 0, true)
	}
	setQRModule(qr, 8, 7, ((bits>>6)&1) != 0, true)
	setQRModule(qr, 8, 8, ((bits>>7)&1) != 0, true)
	setQRModule(qr, 7, 8, ((bits>>8)&1) != 0, true)
	for i := 9; i < 15; i++ {
		setQRModule(qr, 14-i, 8, ((bits>>i)&1) != 0, true)
	}
	for i := 0; i < 8; i++ {
		setQRModule(qr, n-1-i, 8, ((bits>>i)&1) != 0, true)
	}
	for i := 8; i < 15; i++ {
		setQRModule(qr, 8, n-15+i, ((bits>>i)&1) != 0, true)
	}
	setQRModule(qr, 8, n-8, true, true)
}

func buildQRMatrix(text string) (qrMatrix, error) {
	version, codewords, err := qrCodewords(text)
	if err != nil {
		return qrMatrix{}, err
	}
	size := 17 + version*4
	qr := newQRMatrix(size)
	addQRFinder(&qr, 3, 3)
	addQRFinder(&qr, size-4, 3)
	addQRFinder(&qr, 3, size-4)
	for i := 8; i < size-8; i++ {
		setQRModule(&qr, i, 6, i%2 == 0, true)
		setQRModule(&qr, 6, i, i%2 == 0, true)
	}
	for _, cy := range qrVersionSpecs[version].align {
		for _, cx := range qrVersionSpecs[version].align {
			if !qr.fixed[cy][cx] {
				addQRAlignment(&qr, cx, cy)
			}
		}
	}
	for i := 0; i < 9; i++ {
		if i != 6 {
			setQRModule(&qr, 8, i, false, true)
			setQRModule(&qr, i, 8, false, true)
		}
	}
	for i := 0; i < 8; i++ {
		setQRModule(&qr, size-1-i, 8, false, true)
	}
	for i := 0; i < 7; i++ {
		setQRModule(&qr, 8, size-1-i, false, true)
	}
	setQRModule(&qr, 8, size-8, true, true)
	bits := make([]bool, 0, len(codewords)*8)
	for _, value := range codewords {
		appendQRBits(&bits, value, 8)
	}
	bitIndex := 0
	upward := true
	for right := size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right--
		}
		for vert := 0; vert < size; vert++ {
			y := vert
			if upward {
				y = size - 1 - vert
			}
			for j := 0; j < 2; j++ {
				x := right - j
				if qr.fixed[y][x] {
					continue
				}
				dark := false
				if bitIndex < len(bits) {
					dark = bits[bitIndex]
				}
				bitIndex++
				if (x+y)%2 == 0 {
					dark = !dark
				}
				setQRModule(&qr, x, y, dark, false)
			}
		}
		upward = !upward
	}
	placeQRFormat(&qr, 0)
	return qr, nil
}

func qrSVG(qr qrMatrix) string {
	const border = 4
	total := qr.size + border*2
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="360" height="360" shape-rendering="crispEdges">`, total, total)
	b.WriteString(`<rect width="100%" height="100%" fill="#fff"/><path fill="#000" d="`)
	for y := 0; y < qr.size; y++ {
		for x := 0; x < qr.size; x++ {
			if qr.modules[y][x] {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x+border, y+border)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
