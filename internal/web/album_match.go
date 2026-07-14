package web

import (
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

func normalizeLookupText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}

	value = strings.Join(strings.Fields(value), "")
	replacer := strings.NewReplacer(
		"\uFF08", "(",
		"\uFF09", ")",
		"\u3010", "[",
		"\u3011", "]",
		"\u201C", "\"",
		"\u201D", "\"",
		"\u2018", "'",
		"\u2019", "'",
	)
	return replacer.Replace(value)
}

func pickBestAlbumMatch(name string, artist string, albums []model.Playlist) *model.Playlist {
	if len(albums) == 0 {
		return nil
	}

	targetName := normalizeLookupText(name)
	targetArtists := splitArtistTokens(artist)
	bestIndex := 0
	bestScore := -1

	for i, album := range albums {
		score := 0
		albumName := normalizeLookupText(album.Name)
		switch {
		case targetName != "" && albumName == targetName:
			score += 100
		case targetName != "" && (strings.Contains(albumName, targetName) || strings.Contains(targetName, albumName)):
			score += 60
		}

		creatorTokens := splitArtistTokens(album.Creator)
		creatorText := normalizeLookupText(album.Creator)
		for _, targetArtist := range targetArtists {
			normalizedTargetArtist := normalizeArtistToken(targetArtist)
			if normalizedTargetArtist == "" {
				continue
			}

			for _, creator := range creatorTokens {
				if normalizeArtistToken(creator) == normalizedTargetArtist {
					score += 30
					goto albumScored
				}
			}

			targetText := normalizeLookupText(targetArtist)
			if targetText != "" && creatorText != "" && (strings.Contains(creatorText, targetText) || strings.Contains(targetText, creatorText)) {
				score += 10
				goto albumScored
			}
		}

	albumScored:
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}

	return &albums[bestIndex]
}
