package lyrics

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Time struct {
	MS int
	OK bool
}

type Word struct {
	Start Time
	End   Time
	Text  string
}

type Line struct {
	Start Time
	End   Time
	Words []Word
}

type Data []Line

type MultiData map[string]Data

var (
	lrcLineRe     = regexp.MustCompile(`^\[(\d+):(\d+)\.(\d+)\](.*)$`)
	lrcTagRe      = regexp.MustCompile(`^\[([A-Za-z]+):([^\]]*)\]$`)
	lrcWordTimeRe = regexp.MustCompile(`\[(\d+):(\d+)\.(\d+)\]`)
	yrcLineRe     = regexp.MustCompile(`^\[(\d+),(\d+)\](.*)$`)
	yrcWordFindRe = regexp.MustCompile(`(?:\[\d+,\d+\])?\((\d+),(\d+),\d+\)([^\(\[]*)`)
	qrcContentRe  = regexp.MustCompile(`(?s)<Lyric_1[^>]*LyricContent="(.*?)"\s*/>`)
	qrcLineRe     = regexp.MustCompile(`^\[(\d+),(\d+)\](.*)$`)
	qrcWordRe     = regexp.MustCompile(`(?:\[\d+,\d+\])?([^()]*)\((\d+),(\d+)\)`)
	krcLineRe     = regexp.MustCompile(`^\[(\d+),(\d+)\](.*)$`)
	krcTagRe      = regexp.MustCompile(`^\[([A-Za-z]+):([^\]]*)\]$`)
	krcWordRe     = regexp.MustCompile(`(?:\[\d+,\d+\])?<(\d+),(\d+),\d+>([^<]*)`)
	qrcKey        = []byte("!@#)(*$%123ZXC!@!@#)(NHL")
	krcKey        = []byte{0x40, 0x47, 0x61, 0x77, 0x5e, 0x32, 0x74, 0x47, 0x51, 0x36, 0x31, 0x2d, 0xce, 0xd2, 0x6e, 0x69}
)

func ParseLRC(raw string) (map[string]string, Data) {
	tags := map[string]string{}
	var out Data
	for _, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if m := lrcTagRe.FindStringSubmatch(line); m != nil {
			tags[m[1]] = m[2]
			continue
		}
		m := lrcLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start := parseLrcTime(m[1], m[2], m[3])
		content := m[4]
		words := parseLrcWords(start, content)
		out = append(out, Line{Start: Time{MS: start, OK: true}, Words: words})
	}
	inferLineEnds(out)
	return tags, out
}

func ParseYRC(raw string) Data {
	var out Data
	for _, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		m := yrcLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start := atoi(m[1])
		end := start + atoi(m[2])
		content := m[3]
		words := make([]Word, 0)
		for _, wm := range yrcWordFindRe.FindAllStringSubmatch(content, -1) {
			wordStart := atoi(wm[1])
			wordEnd := wordStart + atoi(wm[2])
			if wm[3] == "" {
				continue
			}
			words = append(words, Word{Start: Time{MS: wordStart, OK: true}, End: Time{MS: wordEnd, OK: true}, Text: wm[3]})
		}
		if len(words) == 0 && content != "" {
			words = []Word{{Start: Time{MS: start, OK: true}, End: Time{MS: end, OK: true}, Text: content}}
		}
		out = append(out, Line{Start: Time{MS: start, OK: true}, End: Time{MS: end, OK: true}, Words: words})
	}
	return out
}

func ParseQRC(raw string) (map[string]string, Data) {
	if m := qrcContentRe.FindStringSubmatch(raw); m != nil {
		raw = html.UnescapeString(m[1])
	}
	tags := map[string]string{}
	var out Data
	for _, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if tm := lrcTagRe.FindStringSubmatch(line); tm != nil {
			tags[tm[1]] = tm[2]
			continue
		}
		m := qrcLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start := atoi(m[1])
		end := start + atoi(m[2])
		content := m[3]
		words := make([]Word, 0)
		for _, wm := range qrcWordRe.FindAllStringSubmatch(content, -1) {
			wordStart := atoi(wm[2])
			wordEnd := wordStart + atoi(wm[3])
			if wm[1] == "" || wm[1] == "\r" {
				continue
			}
			words = append(words, Word{Start: Time{MS: wordStart, OK: true}, End: Time{MS: wordEnd, OK: true}, Text: wm[1]})
		}
		if len(words) == 0 && content != "" {
			words = []Word{{Start: Time{MS: start, OK: true}, End: Time{MS: end, OK: true}, Text: content}}
		}
		out = append(out, Line{Start: Time{MS: start, OK: true}, End: Time{MS: end, OK: true}, Words: words})
	}
	return tags, out
}

func ParseKRC(raw string) (map[string]string, MultiData) {
	tags := map[string]string{}
	orig := Data{}
	result := MultiData{}
	for _, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if tm := krcTagRe.FindStringSubmatch(line); tm != nil {
			tags[tm[1]] = tm[2]
			continue
		}
		m := krcLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start := atoi(m[1])
		end := start + atoi(m[2])
		content := m[3]
		words := make([]Word, 0)
		for _, wm := range krcWordRe.FindAllStringSubmatch(content, -1) {
			wordStart := start + atoi(wm[1])
			wordEnd := wordStart + atoi(wm[2])
			if wm[3] == "" {
				continue
			}
			words = append(words, Word{Start: Time{MS: wordStart, OK: true}, End: Time{MS: wordEnd, OK: true}, Text: wm[3]})
		}
		if len(words) == 0 && content != "" {
			words = []Word{{Start: Time{MS: start, OK: true}, End: Time{MS: end, OK: true}, Text: content}}
		}
		orig = append(orig, Line{Start: Time{MS: start, OK: true}, End: Time{MS: end, OK: true}, Words: words})
	}
	if len(orig) > 0 {
		result["orig"] = orig
	}
	if lang := strings.TrimSpace(tags["language"]); lang != "" {
		addKrcLanguage(result, orig, lang)
	}
	return tags, result
}

func ConvertVerbatimLRC(tags map[string]string, data MultiData, order []string) string {
	var b strings.Builder
	writeTags(&b, tags)
	if len(order) == 0 {
		order = DefaultDisplayOrder()
	}
	orig := data["orig"]
	for i, origLine := range orig {
		lineStart := lineStart(origLine)
		lineEnd := lineEnd(origLine)
		for _, lang := range order {
			lines := data[lang]
			if len(lines) == 0 {
				continue
			}
			idx := mappedIndex(lines, i, origLine)
			if idx < 0 || idx >= len(lines) {
				continue
			}
			if !lineHasText(lines[idx]) {
				continue
			}
			b.WriteString(lineToVerbatimLRC(lines[idx], lineStart, lineEnd))
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func DefaultDisplayOrder() []string {
	return []string{"orig", "roma", "ts"}
}

func DecryptKRC(content []byte) (string, error) {
	if len(content) < 4 {
		return "", errors.New("krc content too short")
	}
	encrypted := content[4:]
	plain := make([]byte, len(encrypted))
	for i, b := range encrypted {
		plain[i] = b ^ krcKey[i%len(krcKey)]
	}
	return zlibString(plain)
}

func DecodeKRCBase64(content string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return "", err
	}
	return DecryptKRC(decoded)
}

func DecryptQRCHex(content string) (string, error) {
	encrypted, err := hex.DecodeString(strings.TrimSpace(content))
	if err != nil {
		return "", err
	}
	if len(encrypted)%8 != 0 {
		return "", fmt.Errorf("qrc encrypted length %d is not multiple of 8", len(encrypted))
	}
	plain := make([]byte, len(encrypted))
	for i := 0; i < len(encrypted); i += 8 {
		copy(plain[i:i+8], qrcTripleDESDecrypt(encrypted[i:i+8]))
	}
	return zlibString(plain)
}

func Merge(primary string, values ...string) MultiData {
	out := MultiData{}
	if strings.TrimSpace(primary) != "" {
		_, out["orig"] = ParseLRC(primary)
	}
	for i, key := range []string{"ts", "roma"} {
		if i < len(values) && strings.TrimSpace(values[i]) != "" {
			_, out[key] = ParseLRC(values[i])
		}
	}
	return out
}

func parseLrcWords(start int, content string) []Word {
	if content == "" {
		return nil
	}
	matches := lrcWordTimeRe.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return []Word{{Start: Time{MS: start, OK: true}, Text: content}}
	}
	words := make([]Word, 0, len(matches)+1)
	cursor := 0
	lastStart := start
	for _, m := range matches {
		text := content[cursor:m[0]]
		endMS := parseLrcTime(content[m[2]:m[3]], content[m[4]:m[5]], content[m[6]:m[7]])
		if text != "" {
			words = append(words, Word{Start: Time{MS: lastStart, OK: true}, End: Time{MS: endMS, OK: true}, Text: text})
		}
		lastStart = endMS
		cursor = m[1]
	}
	if cursor < len(content) {
		text := content[cursor:]
		if text != "" {
			words = append(words, Word{Start: Time{MS: lastStart, OK: true}, Text: text})
		}
	}
	return words
}

func inferLineEnds(data Data) {
	sort.SliceStable(data, func(i, j int) bool {
		return data[i].Start.OK && data[j].Start.OK && data[i].Start.MS < data[j].Start.MS
	})
	for i := 0; i+1 < len(data); i++ {
		if !data[i].End.OK && data[i+1].Start.OK {
			data[i].End = data[i+1].Start
		}
	}
}

func lineToVerbatimLRC(line Line, forcedStart, forcedEnd Time) string {
	start := forcedStart
	if !start.OK {
		start = lineStart(line)
	}
	end := forcedEnd
	if !end.OK {
		end = lineEnd(line)
	}
	var b strings.Builder
	if start.OK {
		b.WriteByte('[')
		b.WriteString(formatTime(start.MS))
		b.WriteByte(']')
	}
	lastEnd := start
	for _, word := range line.Words {
		if word.Text == "" {
			continue
		}
		if word.Start.OK && (!lastEnd.OK || word.Start.MS != lastEnd.MS) {
			b.WriteByte('[')
			b.WriteString(formatTime(word.Start.MS))
			b.WriteByte(']')
		}
		b.WriteString(word.Text)
		if word.End.OK {
			b.WriteByte('[')
			b.WriteString(formatTime(word.End.MS))
			b.WriteByte(']')
			lastEnd = word.End
		}
	}
	if end.OK && !strings.HasSuffix(b.String(), "]") {
		b.WriteByte('[')
		b.WriteString(formatTime(end.MS))
		b.WriteByte(']')
	}
	return b.String()
}

func writeTags(b *strings.Builder, tags map[string]string) {
	for _, k := range []string{"ti", "ar", "al", "by", "offset"} {
		if v := strings.TrimSpace(tags[k]); v != "" {
			b.WriteString("[")
			b.WriteString(k)
			b.WriteString(":")
			b.WriteString(v)
			b.WriteString("]\n")
		}
	}
	if len(tags) > 0 {
		b.WriteByte('\n')
	}
}

func mappedIndex(lines Data, i int, orig Line) int {
	if orig.Start.OK {
		for idx, line := range lines {
			if line.Start.OK && line.Start.MS == orig.Start.MS && lineHasText(line) {
				return idx
			}
		}
		if hasTimedLines(lines) {
			return -1
		}
	}
	if i < len(lines) {
		return i
	}
	return -1
}

func hasTimedLines(lines Data) bool {
	for _, line := range lines {
		if line.Start.OK {
			return true
		}
	}
	return false
}

func lineStart(line Line) Time {
	if len(line.Words) > 0 && line.Words[0].Start.OK {
		return line.Words[0].Start
	}
	return line.Start
}

func lineEnd(line Line) Time {
	if len(line.Words) > 0 && line.Words[len(line.Words)-1].End.OK {
		return line.Words[len(line.Words)-1].End
	}
	return line.End
}

func lineHasText(line Line) bool {
	for _, w := range line.Words {
		if strings.TrimSpace(w.Text) != "" {
			return true
		}
	}
	return false
}

func parseLrcTime(m, s, ms string) int {
	msValue := atoi(ms)
	switch len(ms) {
	case 1:
		msValue *= 100
	case 2:
		msValue *= 10
	default:
		if len(ms) > 3 {
			msValue = atoi(ms[:3])
		}
	}
	return atoi(m)*60000 + atoi(s)*1000 + msValue
}

func formatTime(ms int) string {
	if ms < 0 {
		ms = 0
	}
	minute := ms / 60000
	second := (ms % 60000) / 1000
	centisecond := (ms % 1000) / 10
	return fmt.Sprintf("%02d:%02d.%02d", minute, second, centisecond)
}

func atoi(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

func zlibString(data []byte) (string, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	defer r.Close()
	plain, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func addKrcLanguage(result MultiData, orig Data, encoded string) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return
	}
	var payload struct {
		Content []struct {
			Type         int        `json:"type"`
			LyricContent [][]string `json:"lyricContent"`
		} `json:"content"`
	}
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return
	}
	for _, lang := range payload.Content {
		switch lang.Type {
		case 0:
			roma := make(Data, 0, len(orig))
			offset := 0
			for i, line := range orig {
				if !lineHasText(line) {
					offset++
					continue
				}
				li := i - offset
				if li < 0 || li >= len(lang.LyricContent) {
					continue
				}
				words := make([]Word, 0, len(line.Words))
				for j, word := range line.Words {
					if j >= len(lang.LyricContent[li]) {
						continue
					}
					words = append(words, Word{Start: word.Start, End: word.End, Text: lang.LyricContent[li][j]})
				}
				roma = append(roma, Line{Start: line.Start, End: line.End, Words: words})
			}
			if len(roma) > 0 {
				result["roma"] = roma
			}
		case 1:
			ts := make(Data, 0, len(orig))
			for i, line := range orig {
				if i >= len(lang.LyricContent) || len(lang.LyricContent[i]) == 0 {
					continue
				}
				text := lang.LyricContent[i][0]
				ts = append(ts, Line{Start: line.Start, End: line.End, Words: []Word{{Start: line.Start, End: line.End, Text: text}}})
			}
			if len(ts) > 0 {
				result["ts"] = ts
			}
		}
	}
}
