package web

import (
	"regexp"
	"sort"
	"strings"
)

const (
	lyricFormatKaraoke = "karaoke"
	lyricFormatLine    = "line"
)

var (
	lrcTimestampRe = regexp.MustCompile(`\[(\d+):(\d+)\.(\d{1,3})\]`)
	lrcTagLineRe   = regexp.MustCompile(`^\[[A-Za-z]+:[^\]]*\]$`)
)

func classifyLyricFormat(raw string) string {
	startCounts := map[string]int{}
	for _, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || lrcTagLineRe.MatchString(line) {
			continue
		}
		matches := lrcTimestampRe.FindAllStringIndex(line, -1)
		if len(matches) == 0 {
			continue
		}
		if len(matches) > 1 {
			return lyricFormatKaraoke
		}
		start := line[matches[0][0]:matches[0][1]]
		startCounts[start]++
		if startCounts[start] > 1 {
			return lyricFormatKaraoke
		}
	}
	return lyricFormatLine
}

func formatLyricForMode(raw string, mode string) string {
	if strings.EqualFold(mode, lyricFormatLine) {
		return lyricOriginalLineOnly(raw)
	}
	return raw
}

func lyricOriginalLineOnly(raw string) string {
	seenStarts := map[string]struct{}{}
	type lyricLine struct {
		start string
		text  string
	}
	var tags []string
	var lines []lyricLine

	for _, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if lrcTagLineRe.MatchString(line) {
			tags = append(tags, line)
			continue
		}
		matches := lrcTimestampRe.FindAllStringIndex(line, -1)
		if len(matches) == 0 {
			continue
		}
		start := line[matches[0][0]:matches[0][1]]
		if _, ok := seenStarts[start]; ok {
			continue
		}
		seenStarts[start] = struct{}{}

		text := strings.TrimSpace(lrcTimestampRe.ReplaceAllString(line, ""))
		if text == "" {
			continue
		}
		lines = append(lines, lyricLine{start: start, text: text})
	}

	sort.SliceStable(lines, func(i, j int) bool {
		return lines[i].start < lines[j].start
	})

	var b strings.Builder
	for _, tag := range tags {
		b.WriteString(tag)
		b.WriteByte('\n')
	}
	if len(tags) > 0 && len(lines) > 0 {
		b.WriteByte('\n')
	}
	for _, line := range lines {
		b.WriteString(line.start)
		b.WriteString(line.text)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}
