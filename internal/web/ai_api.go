package web

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

var aiPlaybackExtraKeys = map[string]struct{}{
	"album_audio_id":    {},
	"album_id":          {},
	"audio_id":          {},
	"bvid":              {},
	"cid":               {},
	"content_id":        {},
	"copyright_id":      {},
	"file_hash":         {},
	"format_type":       {},
	"hash":              {},
	"hq_hash":           {},
	"is_vip":            {},
	"level":             {},
	"netease_level":     {},
	"ogg_128_hash":      {},
	"ogg_320_hash":      {},
	"only_vip_download": {},
	"privilege":         {},
	"quality":           {},
	"res_hash":          {},
	"resource_type":     {},
	"rid":               {},
	"song_id":           {},
	"songid":            {},
	"songmid":           {},
	"songtype":          {},
	"sq_hash":           {},
	"track_id":          {},
	"tsid":              {},
}

type aiSong struct {
	ID       string            `json:"id"`
	Source   string            `json:"source"`
	Name     string            `json:"name"`
	Artist   string            `json:"artist"`
	Album    string            `json:"album,omitempty"`
	Duration int               `json:"duration,omitempty"`
	IsVIP    bool              `json:"is_vip,omitempty"`
	Rank     string            `json:"rank,omitempty"`
	Extra    map[string]string `json:"extra,omitempty"`
}

type aiSourceError struct {
	Source string `json:"source"`
	Error  string `json:"error"`
}

type aiSongSearchResult struct {
	Total     int
	Truncated bool
	Songs     []aiSong
	Errors    []aiSourceError
}

type aiEndpoint struct {
	Group       string            `json:"group"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Description string            `json:"description"`
	Params      map[string]string `json:"params,omitempty"`
	Body        map[string]string `json:"body,omitempty"`
	Returns     string            `json:"returns"`
	Write       bool              `json:"write,omitempty"`
	Auth        string            `json:"auth,omitempty"`
}

// aiSearchProviders is only used to resolve the song requested by the player
// route. Resource discovery uses the existing /music routes.
type aiSearchProviders struct {
	SearchSongs      func(source string) func(string) ([]model.Song, error)
	SearchLocalSongs func(string, int) []model.Song
}

func defaultAISearchProviders() aiSearchProviders {
	return aiSearchProviders{
		SearchSongs: func(source string) func(string) ([]model.Song, error) {
			return core.GetSearchFunc(source)
		},
		SearchLocalSongs: localMusicSearchSongs,
	}
}

// RegisterAIRoutes exposes an AI discovery index and player controls. Every
// non-player operation referenced by the index is an existing go-music-dl
// route, so search, source switching, downloads, and library handling have one
// implementation and one set of behavior.
func RegisterAIRoutes(api *gin.RouterGroup) {
	basePath := strings.TrimRight(api.BasePath(), "/")
	indexPath := basePath + "/api/ai/index"
	providers := defaultAISearchProviders()

	api.GET("/api/ai/index", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=300")
		c.JSON(http.StatusOK, gin.H{
			"name":           "go-music-dl",
			"schema_version": 2,
			"description":    "AI capability index and player controls. All non-player operations reuse the existing go-music-dl routes listed below.",
			"base_path":      basePath,
			"index":          indexPath,
			"search":         basePath + "/search",
			"switch_source":  basePath + "/switch_source",
			"player":         basePath + "/api/ai/player/state",
			"llms_txt":       basePath + "/llms.txt",
			"auth":           "player controls require an open web or desktop player and the same-origin write header; personalized existing sources may use configured platform cookies",
			"token_guidance": "Use GET /api/ai/player/state before controls. Use the existing search/playlist/album/toplist/source-switch/download routes for resources, then POST /api/ai/player/play when the user wants the open player to play a song.",
			"sources": gin.H{
				"song":              core.GetAllSourceNames(),
				"playlist":          core.GetPlaylistSourceNames(),
				"album":             core.GetAlbumSourceNames(),
				"toplist":           core.GetTopListSourceNames(),
				"playlist_category": core.GetPlaylistCategorySourceNames(),
				"recommend":         core.GetRecommendSourceNames(),
				"user_playlist":     core.GetUserPlaylistSourceNames(),
			},
			"endpoints": aiEndpointCatalog(basePath),
		})
	})

	api.GET("/llms.txt", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=300")
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(aiLLMSText(basePath)))
	})

	registerAIPlayerRoutes(api, providers)
}

func indexPath(basePath string) string {
	return basePath + "/api/ai/index"
}

func aiEndpointCatalog(basePath string) []aiEndpoint {
	writeAuth := "X-Requested-With: XMLHttpRequest"
	return []aiEndpoint{
		{
			Group:       "discover",
			Method:      http.MethodGet,
			Path:        basePath + "/search",
			Description: "Existing aggregated search for songs, playlists, or albums. Returns the regular rendered page.",
			Params: map[string]string{
				"q":            "required search text or a supported platform URL",
				"type":         "song (default), playlist, or album",
				"sources":      "optional repeated query parameter, for example sources=qq&sources=netease",
				"exact_artist": "optional exact artist filter for songs",
			},
			Returns: "html",
		},
		{
			Group:       "discover",
			Method:      http.MethodGet,
			Path:        basePath + "/recommend",
			Description: "Existing recommended playlist page.",
			Params:      map[string]string{"sources": "optional repeated source names"},
			Returns:     "html",
		},
		{
			Group:       "discover",
			Method:      http.MethodGet,
			Path:        basePath + "/user_playlists",
			Description: "Existing playlists created or collected by the configured platform accounts.",
			Params:      map[string]string{"sources": "optional repeated source names"},
			Returns:     "html",
		},
		{
			Group:       "discover",
			Method:      http.MethodGet,
			Path:        basePath + "/playlist_categories",
			Description: "Existing playlist category discovery page.",
			Params:      map[string]string{"sources": "optional repeated source names"},
			Returns:     "html",
		},
		{
			Group:       "discover",
			Method:      http.MethodGet,
			Path:        basePath + "/category_playlists",
			Description: "Existing playlists inside one source category.",
			Params: map[string]string{
				"source":        "required",
				"category_id":   "required; an empty value requests all categories where supported",
				"category_name": "optional display name",
			},
			Returns: "html",
		},
		{
			Group:       "discover",
			Method:      http.MethodGet,
			Path:        basePath + "/toplists",
			Description: "Existing rankings and chart-like playlist page.",
			Params:      map[string]string{"sources": "optional repeated source names"},
			Returns:     "html",
		},
		{
			Group:       "discover",
			Method:      http.MethodGet,
			Path:        basePath + "/toplist_songs",
			Description: "Existing ranked songs page for one chart.",
			Params: map[string]string{
				"source":           "required",
				"toplist_id":       "required",
				"toplist_name":     "optional display name",
				"update_frequency": "optional display metadata",
				"link":             "optional original link",
			},
			Returns: "html",
		},
		{
			Group:       "details",
			Method:      http.MethodGet,
			Path:        basePath + "/playlist",
			Description: "Existing platform playlist detail page.",
			Params: map[string]string{
				"id":     "required playlist id",
				"source": "required",
				"link":   "optional original link",
			},
			Returns: "html",
		},
		{
			Group:       "details",
			Method:      http.MethodGet,
			Path:        basePath + "/album",
			Description: "Existing album detail page.",
			Params: map[string]string{
				"id":     "required album id",
				"source": "required",
			},
			Returns: "html",
		},
		{
			Group:       "playback",
			Method:      http.MethodGet,
			Path:        basePath + "/inspect",
			Description: "Existing source-resolution check. A valid result may set preview_only=true; preview samples are playable.",
			Params: map[string]string{
				"id":       "required",
				"source":   "required",
				"duration": "optional seconds",
				"extra":    "optional URL-encoded JSON object",
			},
			Returns: "json",
		},
		{
			Group:       "playback",
			Method:      http.MethodGet,
			Path:        basePath + "/switch_source",
			Description: "Existing automatic source switch. Use this route instead of a duplicate AI endpoint.",
			Params: map[string]string{
				"name":     "required",
				"artist":   "optional",
				"source":   "optional current source",
				"target":   "optional target source",
				"duration": "optional seconds",
			},
			Returns: "json",
		},
		{
			Group:       "playback",
			Method:      http.MethodGet,
			Path:        basePath + "/download",
			Description: "Existing audio stream or download route. Use stream=1 for playback-only requests.",
			Params: map[string]string{
				"id":     "required",
				"source": "required",
				"name":   "recommended",
				"artist": "recommended",
				"album":  "optional",
				"cover":  "optional",
				"extra":  "optional URL-encoded JSON object",
				"stream": "use 1 for browser/audio playback",
				"embed":  "optional 1 when saving a file",
			},
			Returns: "audio stream or file",
		},
		{
			Group:       "playback",
			Method:      http.MethodGet,
			Path:        basePath + "/lyric",
			Description: "Existing lyrics route.",
			Params: map[string]string{
				"id":     "required",
				"source": "required",
				"name":   "recommended",
				"artist": "recommended",
				"extra":  "optional URL-encoded JSON object",
				"format": "auto, lrc, or plain",
			},
			Returns: "text",
		},
		{
			Group:       "playback",
			Method:      http.MethodGet,
			Path:        basePath + "/download_lrc",
			Description: "Existing standard LRC download route.",
			Params: map[string]string{
				"id":     "required",
				"source": "required",
				"name":   "recommended",
				"artist": "recommended",
				"extra":  "optional URL-encoded JSON object",
			},
			Returns: "text file",
		},
		{
			Group:       "playback",
			Method:      http.MethodGet,
			Path:        basePath + "/cover_proxy",
			Description: "Existing source-aware remote cover proxy.",
			Params: map[string]string{
				"url":    "required",
				"source": "optional source name",
			},
			Returns: "image",
		},
		{
			Group:       "downloads",
			Method:      http.MethodGet,
			Path:        basePath + "/api/downloads/records",
			Description: "Existing download history.",
			Params: map[string]string{
				"page":      "optional, default 1",
				"page_size": "1-100, default 20",
			},
			Returns: "json",
		},
		{
			Group:       "downloads",
			Method:      http.MethodPost,
			Path:        basePath + "/api/downloads/precheck",
			Description: "Existing read-only check for songs that are already downloaded.",
			Body:        map[string]string{"songs": "array of {name,artist}; maximum 20000"},
			Returns:     "json",
		},
		{
			Group:       "downloads",
			Method:      http.MethodGet,
			Path:        basePath + "/api/downloads/queue",
			Description: "Existing persistent download queue.",
			Params: map[string]string{
				"page":   "optional, default 1",
				"status": "waiting, downloading, success, failed, skipped, canceled, or active",
			},
			Returns: "json",
		},
		{
			Group:       "downloads",
			Method:      http.MethodPost,
			Path:        basePath + "/api/downloads/queue",
			Description: "Existing route to enqueue up to 2000 explicit songs.",
			Body: map[string]string{
				"request_id": "required unique request id",
				"songs":      "array of {id,source,name,artist,album,cover,extra,embed}; extra is a JSON-encoded string",
			},
			Returns: "json",
			Write:   true,
			Auth:    writeAuth,
		},
		{
			Group:       "downloads",
			Method:      http.MethodPost,
			Path:        basePath + "/api/downloads/queue/container",
			Description: "Existing route to enqueue a complete playlist, album, toplist, or collection.",
			Body: map[string]string{
				"request_id": "required unique request id",
				"kind":       "playlist, album, or collection",
				"id":         "required container id",
				"source":     "required for platform containers",
				"link":       "optional original link",
				"embed":      "optional boolean",
			},
			Returns: "json",
			Write:   true,
			Auth:    writeAuth,
		},
		{
			Group:       "downloads",
			Method:      http.MethodPost,
			Path:        basePath + "/api/downloads/queue/{id}/{action}",
			Description: "Existing route to retry or cancel one queue task.",
			Params:      map[string]string{"id": "required task id", "action": "retry or cancel"},
			Returns:     "json",
			Write:       true,
			Auth:        writeAuth,
		},
		{
			Group:       "downloads",
			Method:      http.MethodDelete,
			Path:        basePath + "/api/downloads/queue/{id}",
			Description: "Existing route to remove one inactive queue task.",
			Params:      map[string]string{"id": "required task id"},
			Returns:     "json",
			Write:       true,
			Auth:        writeAuth,
		},
		{
			Group:       "downloads",
			Method:      http.MethodDelete,
			Path:        basePath + "/api/downloads/queue",
			Description: "Existing route to clear inactive queue tasks. Use only on explicit user request.",
			Returns:     "json",
			Write:       true,
			Auth:        writeAuth,
		},
		{
			Group:       "library",
			Method:      http.MethodGet,
			Path:        basePath + "/local_music",
			Description: "Existing paginated local music index.",
			Params: map[string]string{
				"offset":        "optional, default 0",
				"limit":         "optional page size",
				"refresh":       "use 1 to rescan",
				"force":         "use 1 to rescan",
				"collection_id": "optional imported collection id",
			},
			Returns: "json",
		},
		{
			Group:       "library",
			Method:      http.MethodGet,
			Path:        basePath + "/collections",
			Description: "Existing local collection list.",
			Params:      map[string]string{"include_imported": "optional 0 or 1"},
			Returns:     "json",
		},
		{
			Group:       "library",
			Method:      http.MethodGet,
			Path:        basePath + "/collections/{id}/songs",
			Description: "Existing local collection song list.",
			Params:      map[string]string{"id": "required collection id"},
			Returns:     "json",
		},
		{
			Group:       "player_control",
			Method:      http.MethodGet,
			Path:        basePath + "/api/ai/player/state",
			Description: "Read one or all connected players' state. Without client_id, connected, playing, and visible players are preferred, then connection order is used for a stable default. players[] and clients[] list every retained client.",
			Params: map[string]string{
				"client_id": "optional; read one specific player",
			},
			Returns: "json",
		},
		{
			Group:       "player_control",
			Method:      http.MethodGet,
			Path:        basePath + "/api/ai/player/events",
			Description: "Server-sent event stream consumed by the open player. AI clients normally do not call this route.",
			Returns:     "text/event-stream",
		},
		{
			Group:       "player_control",
			Method:      http.MethodPost,
			Path:        basePath + "/api/ai/player/play",
			Description: "Search for or explicitly identify a song, then make the connected player play it. A full player queue returns 503 player_busy.",
			Body: map[string]string{
				"client_id": "optional target player id; required when more than one player is connected",
				"query":     "song search text, or omit when id and source are provided",
				"artist":    "optional exact artist filter",
				"sources":   "optional comma-separated string or string array",
				"id":        "explicit song id",
				"source":    "explicit song source, required with id",
				"name":      "optional explicit song name",
				"album":     "optional explicit album",
				"duration":  "optional explicit duration in seconds",
				"extra":     "optional source-specific playback fields",
			},
			Returns: "command response",
			Write:   true,
			Auth:    writeAuth,
		},
		{
			Group:       "player_control",
			Method:      http.MethodPost,
			Path:        basePath + "/api/ai/player/command",
			Description: "Control the connected player: play, pause, toggle, stop, next, previous, seek, volume, or rate. A full player queue returns 503 player_busy.",
			Body: map[string]string{
				"client_id": "optional target player id; required when more than one player is connected",
				"action":    "required",
				"position":  "seconds, required for seek",
				"volume":    "0-1, required for volume",
				"rate":      "0.5-3 in 0.25 steps, required for rate",
			},
			Returns: "command response",
			Write:   true,
			Auth:    writeAuth,
		},
		{
			Group:       "player_control",
			Method:      http.MethodPost,
			Path:        basePath + "/api/ai/player/state",
			Description: "State reporting route used by the connected player page. AI clients normally only need GET.",
			Returns:     "json",
			Write:       true,
			Auth:        writeAuth,
		},
		{
			Group:       "system",
			Method:      http.MethodGet,
			Path:        basePath + "/healthz",
			Description: "Existing service health check.",
			Returns:     "json",
		},
		{
			Group:       "system",
			Method:      http.MethodGet,
			Path:        indexPath(basePath),
			Description: "This machine-readable capability index.",
			Returns:     "json",
		},
		{
			Group:       "system",
			Method:      http.MethodGet,
			Path:        basePath + "/llms.txt",
			Description: "Short operational guide for language models.",
			Returns:     "text",
		},
	}
}

func aiPlayableSources() []string {
	return append([]string(nil), core.GetAllSourceNames()...)
}

func parseAIPlayerSources(values []string) ([]string, error) {
	return parseAISources(values, aiPlayableSources(), defaultSourcesForSearchType("song"))
}

func parseAISources(values, allowed, defaults []string) ([]string, error) {
	if len(values) == 0 {
		return append([]string(nil), defaults...), nil
	}

	allowedSet := make(map[string]struct{}, len(allowed))
	for _, source := range allowed {
		allowedSet[source] = struct{}{}
	}

	seen := make(map[string]struct{}, len(values))
	sources := make([]string, 0, len(values))
	add := func(source string) {
		source = strings.ToLower(strings.TrimSpace(source))
		if source == "" {
			return
		}
		if _, exists := seen[source]; exists {
			return
		}
		seen[source] = struct{}{}
		sources = append(sources, source)
	}

	var invalid []string
	for _, value := range values {
		for _, source := range strings.Split(value, ",") {
			source = strings.ToLower(strings.TrimSpace(source))
			if source == "all" {
				for _, item := range allowed {
					add(item)
				}
				continue
			}
			if _, ok := allowedSet[source]; !ok {
				invalid = append(invalid, source)
				continue
			}
			add(source)
		}
	}
	if len(invalid) > 0 {
		return nil, fmt.Errorf("unsupported sources: %s", strings.Join(invalid, ", "))
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("at least one source is required")
	}
	return sources, nil
}

func parseAISingleSource(raw string, allowed []string) (string, error) {
	sources, err := parseAISources([]string{raw}, allowed, nil)
	if err != nil {
		return "", err
	}
	if len(sources) != 1 {
		return "", fmt.Errorf("exactly one source is required")
	}
	return sources[0], nil
}

func executeAISongSearch(keyword string, sources []string, exactArtist string, limit int, providers aiSearchProviders) aiSongSearchResult {
	if limit < 1 {
		limit = 1
	}

	groups := make([][]model.Song, len(sources))
	sourceErrors := make([]error, len(sources))
	var wg sync.WaitGroup
	for i, source := range sources {
		if isLocalMusicSource(source) {
			if providers.SearchLocalSongs != nil {
				groups[i] = providers.SearchLocalSongs(keyword, 200)
			}
			continue
		}

		search := providers.SearchSongs
		if search == nil {
			sourceErrors[i] = fmt.Errorf("search provider is unavailable")
			continue
		}
		searchFn := search(source)
		if searchFn == nil {
			sourceErrors[i] = fmt.Errorf("source does not support song search")
			continue
		}
		wg.Add(1)
		go func(index int, source string, fn func(string) ([]model.Song, error)) {
			defer wg.Done()
			groups[index], sourceErrors[index] = fn(keyword)
		}(i, source, searchFn)
	}
	wg.Wait()

	total := 0
	for i := range groups {
		for j := range groups[i] {
			groups[i][j].Source = sources[i]
		}
		if exactArtist != "" {
			groups[i] = filterSongsByExactArtist(groups[i], exactArtist)
		}
		total += len(groups[i])
	}

	limited := interleaveSongs(groups, limit)
	result := aiSongSearchResult{
		Total:     total,
		Truncated: total > len(limited),
		Songs:     compactAISongs(limited),
	}
	for i, err := range sourceErrors {
		if err != nil {
			result.Errors = append(result.Errors, aiSourceError{
				Source: sources[i],
				Error:  compactAIError(err),
			})
		}
	}
	return result
}

func interleaveSongs(groups [][]model.Song, limit int) []model.Song {
	result := make([]model.Song, 0, limit)
	for row := 0; len(result) < limit; row++ {
		added := false
		for _, group := range groups {
			if row >= len(group) {
				continue
			}
			result = append(result, group[row])
			added = true
			if len(result) == limit {
				break
			}
		}
		if !added {
			break
		}
	}
	return result
}

func compactAISongs(songs []model.Song) []aiSong {
	result := make([]aiSong, 0, len(songs))
	for _, song := range songs {
		result = append(result, aiSong{
			ID:       song.ID,
			Source:   song.Source,
			Name:     song.Name,
			Artist:   song.Artist,
			Album:    song.Album,
			Duration: song.Duration,
			IsVIP:    song.IsVIP,
			Rank:     strings.TrimSpace(song.Extra["rank"]),
			Extra:    aiPlaybackExtra(song.Extra),
		})
	}
	return result
}

func aiPlaybackExtra(extra map[string]string) map[string]string {
	if len(extra) == 0 {
		return nil
	}
	compact := make(map[string]string)
	for key := range aiPlaybackExtraKeys {
		if value := strings.TrimSpace(extra[key]); value != "" {
			compact[key] = value
		}
	}
	if len(compact) == 0 {
		return nil
	}
	return compact
}

func compactAIError(err error) string {
	const maxRunes = 300
	message := strings.TrimSpace(err.Error())
	runes := []rune(message)
	if len(runes) <= maxRunes {
		return message
	}
	return string(runes[:maxRunes]) + "..."
}

func aiLLMSText(basePath string) string {
	return fmt.Sprintf(`# go-music-dl AI API

Only player control is AI-specific. Every other operation uses the existing go-music-dl route; there are no parallel AI search, playlist, source-switch, download, or library routes.

Start:
- GET %[1]s/api/ai/index
- GET %[1]s/search?q=周杰伦&type=song&sources=qq&sources=netease

Discover:
- Rankings: GET %[1]s/toplists
- Ranked songs: GET %[1]s/toplist_songs?source=...&toplist_id=...
- Playlist categories: GET %[1]s/playlist_categories
- Category playlists: GET %[1]s/category_playlists?source=...&category_id=...
- Recommendations: GET %[1]s/recommend
- Account playlists: GET %[1]s/user_playlists

Details:
- Playlist: GET %[1]s/playlist?id=...&source=...
- Album: GET %[1]s/album?id=...&source=...

Playback and downloads:
- Player state: GET %[1]s/api/ai/player/state
- Play a searched or explicit song: POST %[1]s/api/ai/player/play
- Control current playback: POST %[1]s/api/ai/player/command
- Source switch: GET %[1]s/switch_source?name=...&artist=...&source=...&target=...
- Playback resolution check: GET %[1]s/inspect (preview_only=true is playable)
- Audio stream: GET %[1]s/download?...&stream=1
- Lyrics: GET %[1]s/lyric
- Queue status: GET %[1]s/api/downloads/queue
- Enqueue songs: POST %[1]s/api/downloads/queue
- Enqueue a complete container: POST %[1]s/api/downloads/queue/container

Library and health:
- Local music: GET %[1]s/local_music?offset=0&limit=50
- Local collections: GET %[1]s/collections
- Collection songs: GET %[1]s/collections/{id}/songs
- Health: GET %[1]s/healthz

Rules:
- {base} defaults to /music. The runtime index is the source of truth.
- Existing search, playlist, album, ranking, recommendation, and category routes return the normal rendered page.
- Preserve a song's extra object because some sources require it for playback or lyrics.
- Treat preview_only=true as playable; do not switch sources just because it is a preview.
- Player controls require an open web or desktop player; otherwise the route returns 409 player_not_connected.
- Every player keeps its own state and last_command. Read it with GET %[1]s/api/ai/player/state?client_id=...; without client_id the default prefers connected, playing, and visible players, then uses stable connection order rather than heartbeat recency.
- With more than one player connected, send client_id with play/command; an untargeted call returns 409 ambiguous_client and the candidate list. One connected player is targeted automatically.
- A full player command queue returns 503 player_busy. Disconnected player records expire after 10 minutes, and at most 64 players may have active event streams.
- Player writes must send X-Requested-With: XMLHttpRequest.
- Never expose cookies, login, or configuration routes to an AI client.
`, basePath)
}
