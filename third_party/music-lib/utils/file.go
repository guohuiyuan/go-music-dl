package utils

import "strings"

// SanitizeFilename 移除文件名中的非法字符
func SanitizeFilename(name string) string {
	// Windows 文件名非法字符: \ / : * ? " < > |
	illegalChars := []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|"}
	result := name
	for _, char := range illegalChars {
		result = strings.ReplaceAll(result, char, "_")
	}
	// 移除首尾空格
	result = strings.TrimSpace(result)
	// 如果为空，返回默认值
	if result == "" {
		return "unknown"
	}
	return result
}
