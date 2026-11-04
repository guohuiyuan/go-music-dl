package core

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	lrcTimingTokenRe     = regexp.MustCompile(`\[\d{1,3}:\d{1,2}(?:[.:]\d{1,3})?\]`)
	lrcTimingLineStartRe = regexp.MustCompile(`^(?:\[\d{1,3}:\d{1,2}(?:[.:]\d{1,3})?\])+`)
	lrcMetadataLineRe    = regexp.MustCompile(`^\[[A-Za-z][A-Za-z0-9_-]*:[^\]]*\]$`)
)

// StandardLRCForExport converts provider-specific karaoke timing into a
// conservative line-based LRC suitable for local music players. It preserves
// metadata, translations, valid repeated leading timestamps, and line order.
func StandardLRCForExport(raw string) string {
	raw = strings.TrimPrefix(raw, "\uFEFF")
	if strings.TrimSpace(raw) == "" {
		return ""
	}

	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	lines := make([]string, 0, strings.Count(raw, "\n")+1)
	for _, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if lrcMetadataLineRe.MatchString(line) {
			lines = append(lines, line)
			continue
		}

		start := lrcTimingLineStartRe.FindString(line)
		if start == "" {
			// Keep non-timestamped text rather than silently discarding content.
			lines = append(lines, line)
			continue
		}
		text := lrcTimingTokenRe.ReplaceAllString(line[len(start):], "")
		text = strings.TrimSpace(text)
		lines = append(lines, start+text)
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\r\n") + "\r\n"
}

func saveDownloadedSongLyricSidecar(result *DownloadedSong, exportLyrics bool) {
	if !exportLyrics || result == nil || strings.TrimSpace(result.SavedPath) == "" || strings.TrimSpace(result.Lyric) == "" {
		return
	}

	lrc := StandardLRCForExport(result.Lyric)
	if lrc == "" {
		return
	}
	lrcPath := strings.TrimSuffix(result.SavedPath, filepath.Ext(result.SavedPath)) + ".lrc"
	result.LyricPath = lrcPath

	existing, err := os.ReadFile(lrcPath)
	if err == nil {
		if !bytes.Equal(existing, []byte(lrc)) {
			result.Warning = appendDownloadWarning(result.Warning, "lyrics file already exists; kept existing file")
		}
		return
	}
	if !os.IsNotExist(err) {
		result.LyricPath = ""
		result.Warning = appendDownloadWarning(result.Warning, fmt.Sprintf("lyrics file check failed: %v", err))
		return
	}
	if err := writeNewFile(lrcPath, []byte(lrc), 0644); err != nil {
		result.LyricPath = ""
		result.Warning = appendDownloadWarning(result.Warning, fmt.Sprintf("lyrics file write failed: %v", err))
	}
}

func writeNewFile(path string, data []byte, perm os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(path)
		}
	}()

	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	keep = true
	return nil
}

func appendDownloadWarning(current, next string) string {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	if current == "" {
		return next
	}
	if next == "" {
		return current
	}
	return current + "; " + next
}
