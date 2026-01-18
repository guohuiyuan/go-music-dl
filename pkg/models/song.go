package models

import (
	"fmt"
	"strings"
)

// Song 扩展自 music-lib 的 model.Song，增加格式化方法
type Song struct {
	Source   string
	ID       string
	Name     string
	Artist   string
	Album    string
	Duration int   // 秒
	Size     int64 // 字节
	URL      string
	Ext      string // mp3/flac/m4a
}

// FormatDuration 格式化时长 (e.g. 03:45)
func (s *Song) FormatDuration() string {
	return FormatDurationSeconds(s.Duration)
}

// FormatDurationSeconds 通用的时长格式化函数
func FormatDurationSeconds(seconds int) string {
	if seconds == 0 {
		return "-"
	}
	min := seconds / 60
	sec := seconds % 60
	return fmt.Sprintf("%02d:%02d", min, sec)
}

// FormatSize 格式化大小 (e.g. 4.5 MB)
func (s *Song) FormatSize() string {
	if s.Size == 0 {
		return "-"
	}
	mb := float64(s.Size) / 1024 / 1024
	return fmt.Sprintf("%.2f MB", mb)
}

// Filename 生成清晰的文件名 (歌手 - 歌名.ext)
func (s *Song) Filename() string {
	ext := s.Ext
	if ext == "" {
		ext = "mp3" // 默认
	}
	// 简单的文件名清洗，防止非法字符
	safeName := cleanFilename(s.Name)
	safeArtist := cleanFilename(s.Artist)
	return fmt.Sprintf("%s - %s.%s", safeArtist, safeName, ext)
}

// cleanFilename 移除文件名中的非法字符
func cleanFilename(name string) string {
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
