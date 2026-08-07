package netease

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
)

var (
	ncmCoreKey = []byte("hzHRAmso5kInbaxW")
	ncmMetaKey = []byte("#14ljk_!\\]&0U<'(")
)

// --- 常量定义 ---

// Linux API Key (Hex)
const linuxApiKeyHex = "7246674226682325323F5E6544673A51"

// WeApi Constants
const (
	weApiNonce      = "0CoJUm6Qyw8W8jud"
	weApiIv         = "0102030405060708"
	weApiPubModulus = "00e0b509f6259df8642dbc35662901477df22677ec152b5ff68ace615bb7b725152b3ab17a876aea8a5aa76d2e417629ec4ee341f56135fccf695280104e0312ecbda92557c93870114af6c9d05c4f7f0c3685b7a46bee255932575cce10b424d813cfe4875d3e82047b97ddef52741d546b8e289dc6935b3ece0462db0a22b8e7"
	weApiPubKey     = "010001"
)

// --- 辅助函数 ---

// pkcs7Padding 填充
func pkcs7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

// randomString 生成指定长度随机字符串
func randomString(size int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var result string
	b := make([]byte, size)
	rand.Read(b)
	for _, v := range b {
		result += string(letters[int(v)%len(letters)])
	}
	return result
}

// reverseString 反转字符串 (RSA加密需要)
func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// --- 算法实现 ---

// aesEncryptECB 实现 AES-128-ECB 加密 (Go 标准库没有直接提供 ECB，需手动实现)
func aesEncryptECB(origData []byte, key []byte) []byte {
	block, _ := aes.NewCipher(key)
	// 补码
	origData = pkcs7Padding(origData, block.BlockSize())

	crypted := make([]byte, len(origData))
	// 手动循环加密每个块
	for bs, be := 0, block.BlockSize(); bs < len(origData); bs, be = bs+block.BlockSize(), be+block.BlockSize() {
		block.Encrypt(crypted[bs:be], origData[bs:be])
	}
	return crypted
}

// aesEncryptCBC 实现 AES-128-CBC 加密
func aesEncryptCBC(text string, key string, iv string) string {
	keyBytes := []byte(key)
	ivBytes := []byte(iv)
	srcBytes := []byte(text)

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return ""
	}

	srcBytes = pkcs7Padding(srcBytes, block.BlockSize())
	blockMode := cipher.NewCBCEncrypter(block, ivBytes)
	crypted := make([]byte, len(srcBytes))
	blockMode.CryptBlocks(crypted, srcBytes)

	return base64.StdEncoding.EncodeToString(crypted)
}

// rsaEncrypt 实现 RSA 加密 (NoPadding)
// Python: pow(int(hex(text)), int(pub), int(mod))
func rsaEncrypt(text, pubKey, modulus string) string {
	// 1. 反转字符串
	text = reverseString(text)
	// 2. 转为 hex
	hexText := hex.EncodeToString([]byte(text))

	// 3. 大数运算
	biText := new(big.Int)
	biText.SetString(hexText, 16)

	biPub := new(big.Int)
	biPub.SetString(pubKey, 16)

	biMod := new(big.Int)
	biMod.SetString(modulus, 16)

	// exp = text^pub % mod
	biRet := new(big.Int).Exp(biText, biPub, biMod)

	// 4. 补齐 256 位 hex
	return fmt.Sprintf("%0256x", biRet)
}

// --- 对外暴露的加密方法 ---

// EncryptLinux 对应 Python: encode_netease_data
// 用于搜索接口
func EncryptLinux(data string) string {
	key, _ := hex.DecodeString(linuxApiKeyHex)
	encrypted := aesEncryptECB([]byte(data), key)
	return strings.ToUpper(hex.EncodeToString(encrypted))
}

// EncryptWeApi 对应 Python: encrypted_request
// 用于下载接口
func EncryptWeApi(text string) (string, string) {
	// 1. 生成随机 16 位 secKey
	secKey := randomString(16)

	// 2. 第一次 AES 加密 (Text + Nonce)
	encText := aesEncryptCBC(text, weApiNonce, weApiIv)

	// 3. 第二次 AES 加密 (第一次结果 + secKey)
	params := aesEncryptCBC(encText, secKey, weApiIv)

	// 4. RSA 加密 secKey
	encSecKey := rsaEncrypt(secKey, weApiPubKey, weApiPubModulus)

	return params, encSecKey
}

// EncryptEApi 对应 Python: encrypt_params
// 用于 EAPI 接口 (如获取高音质 VIP 下载链接)
func EncryptEApi(urlPath string, payload string) string {
	u, err := url.Parse(urlPath)
	if err == nil && u.Path != "" {
		urlPath = u.Path
	}
	urlPath = strings.ReplaceAll(urlPath, "/eapi/", "/api/")
	text := fmt.Sprintf("nobody%suse%smd5forencrypt", urlPath, payload)

	hasher := md5.New()
	hasher.Write([]byte(text))
	digest := hex.EncodeToString(hasher.Sum(nil))

	data := fmt.Sprintf("%s-36cd479b6b5-%s-36cd479b6b5-%s", urlPath, payload, digest)

	eapiKey := []byte("e82ckenh8dichen8")
	encrypted := aesEncryptECB([]byte(data), eapiKey)

	// Python original uses lower case hex string for eapi
	return hex.EncodeToString(encrypted)
}

func DecryptNCM(encrypted []byte) ([]byte, string, error) {
	if len(encrypted) < 16 || string(encrypted[:8]) != "CTENFDAM" {
		return nil, "", errors.New("invalid ncm file")
	}

	offset := 8
	if offset+2 > len(encrypted) {
		return nil, "", errors.New("invalid ncm header")
	}
	offset += 2

	keyLen, next, ok := readU32LE(encrypted, offset)
	if !ok || next+int(keyLen) > len(encrypted) {
		return nil, "", errors.New("invalid ncm key length")
	}
	keyData := append([]byte(nil), encrypted[next:next+int(keyLen)]...)
	for i := range keyData {
		keyData[i] ^= 0x64
	}
	offset = next + int(keyLen)

	decryptedKey, err := aesECBDecrypt(ncmCoreKey, keyData)
	if err != nil {
		return nil, "", err
	}
	decryptedKey = pkcs7Unpad(decryptedKey)
	if len(decryptedKey) > 17 {
		decryptedKey = decryptedKey[17:]
	}
	if len(decryptedKey) == 0 {
		return nil, "", errors.New("invalid ncm key data")
	}

	keyBox := buildNCMKeyBox(decryptedKey)

	metaLen, next, ok := readU32LE(encrypted, offset)
	if !ok || next+int(metaLen) > len(encrypted) {
		return nil, "", errors.New("invalid ncm meta length")
	}
	metaData := append([]byte(nil), encrypted[next:next+int(metaLen)]...)
	for i := range metaData {
		metaData[i] ^= 0x63
	}
	offset = next + int(metaLen)

	outExt := parseNCMFormat(metaData)

	if offset+9 > len(encrypted) {
		return nil, "", errors.New("invalid ncm payload")
	}
	offset += 9

	imageSize, next, ok := readU32LE(encrypted, offset)
	if !ok {
		return nil, "", errors.New("invalid ncm image length")
	}
	offset = next + int(imageSize)
	if offset > len(encrypted) {
		return nil, "", errors.New("invalid ncm image block")
	}

	audio := append([]byte(nil), encrypted[offset:]...)
	for i := range audio {
		j := byte((i + 1) & 0xff)
		idx := (int(keyBox[j]) + int(keyBox[(int(keyBox[j])+int(j))&0xff])) & 0xff
		audio[i] ^= keyBox[idx]
	}

	if outExt == "" {
		outExt = detectAudioExt(audio)
	}

	return audio, outExt, nil
}

func buildNCMKeyBox(key []byte) [256]byte {
	var box [256]byte
	for i := 0; i < 256; i++ {
		box[i] = byte(i)
	}

	var c, last int
	keyPos := 0
	for i := 0; i < 256; i++ {
		swap := box[i]
		c = (int(swap) + last + int(key[keyPos])) & 0xff
		box[i] = box[c]
		box[c] = swap
		last = c
		keyPos++
		if keyPos >= len(key) {
			keyPos = 0
		}
	}

	return box
}

func parseNCMFormat(metaData []byte) string {
	if len(metaData) <= 22 {
		return ""
	}

	decoded, err := base64.StdEncoding.DecodeString(string(metaData[22:]))
	if err != nil {
		return ""
	}

	decrypted, err := aesECBDecrypt(ncmMetaKey, decoded)
	if err != nil {
		return ""
	}
	decrypted = pkcs7Unpad(decrypted)

	if bytes.HasPrefix(decrypted, []byte("music:")) {
		decrypted = decrypted[len("music:"):]
	}

	var payload struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(decrypted, &payload); err != nil {
		return ""
	}
	return payload.Format
}

func aesECBDecrypt(key, data []byte) ([]byte, error) {
	if len(data)%aes.BlockSize != 0 {
		return nil, errors.New("invalid aes block size")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	out := make([]byte, len(data))
	for i := 0; i < len(data); i += aes.BlockSize {
		block.Decrypt(out[i:i+aes.BlockSize], data[i:i+aes.BlockSize])
	}
	return out, nil
}

func pkcs7Unpad(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	pad := int(data[len(data)-1])
	if pad <= 0 || pad > len(data) {
		return data
	}
	for i := 0; i < pad; i++ {
		if data[len(data)-1-i] != byte(pad) {
			return data
		}
	}
	return data[:len(data)-pad]
}

func readU32LE(data []byte, offset int) (uint32, int, bool) {
	if offset+4 > len(data) {
		return 0, offset, false
	}
	return binary.LittleEndian.Uint32(data[offset : offset+4]), offset + 4, true
}

func detectAudioExt(data []byte) string {
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{'f', 'L', 'a', 'C'}) {
		return "flac"
	}
	if len(data) >= 3 && bytes.Equal(data[:3], []byte{'I', 'D', '3'}) {
		return "mp3"
	}
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{'O', 'g', 'g', 'S'}) {
		return "ogg"
	}
	if len(data) >= 8 && bytes.Equal(data[4:8], []byte{'f', 't', 'y', 'p'}) {
		return "m4a"
	}
	return "mp3"
}
