package web

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/guohuiyuan/music-lib/model"
)

var artistKeywordSeparatorPattern = regexp.MustCompile(`(?i)\s+(?:feat(?:uring)?\.?|ft\.?|with|x)\s+`)

var commonArtistSeparatorReplacer = strings.NewReplacer(
	"\u3001", "|",
	",", "|",
	"\uFF0C", "|",
	";", "|",
	"\uFF1B", "|",
	"|", "|",
)

var eastAsianArtistSeparatorReplacer = strings.NewReplacer(
	"/", "|",
	"\uFF0F", "|",
	"&", "|",
	"\uFF06", "|",
)

var spacedArtistSeparatorPattern = regexp.MustCompile("\\s+(?:/|\uFF0F|&|\uFF06)\\s+")

func normalizeArtistToken(artist string) string {
	artist = strings.TrimSpace(strings.ToLower(artist))
	if artist == "" {
		return ""
	}
	return strings.Join(strings.Fields(artist), " ")
}

func containsEastAsianRune(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			return true
		}
	}
	return false
}

func trimArtistToken(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "-_/\u00B7\u2022|\\,\uFF0C\u3001;\uFF1B&\uFF06")
	return strings.TrimSpace(value)
}

func splitArtistTokens(artist string) []string {
	artist = strings.TrimSpace(artist)
	if artist == "" {
		return []string{}
	}

	normalized := artistKeywordSeparatorPattern.ReplaceAllString(artist, "|")
	normalized = commonArtistSeparatorReplacer.Replace(normalized)
	if containsEastAsianRune(artist) {
		normalized = eastAsianArtistSeparatorReplacer.Replace(normalized)
	} else {
		normalized = spacedArtistSeparatorPattern.ReplaceAllString(normalized, "|")
	}

	parts := strings.Split(normalized, "|")
	tokens := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = trimArtistToken(part)
		if part == "" {
			continue
		}
		key := normalizeArtistToken(part)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		tokens = append(tokens, part)
	}

	if len(tokens) == 0 {
		return []string{artist}
	}
	return tokens
}

func filterSongsByExactArtist(songs []model.Song, exactArtist string) []model.Song {
	exactArtist = normalizeArtistToken(exactArtist)
	if exactArtist == "" {
		return songs
	}

	filtered := make([]model.Song, 0, len(songs))
	for _, song := range songs {
		for _, artist := range splitArtistTokens(song.Artist) {
			if normalizeArtistToken(artist) == exactArtist {
				filtered = append(filtered, song)
				break
			}
		}
	}
	return filtered
}

func songAlbumID(song model.Song) string {
	if id := strings.TrimSpace(song.AlbumID); id != "" {
		return id
	}
	if song.Extra == nil {
		return ""
	}
	return strings.TrimSpace(song.Extra["album_id"])
}
