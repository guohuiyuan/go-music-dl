package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

const (
	aiSearchDefaultLimit   = 10
	aiSearchMaxLimit       = 50
	aiSearchMaxQueryLen    = 200
	aiResourceDefaultLimit = 50
	aiResourceMaxLimit     = 200
	aiCategoryMaxLimit     = 500
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

type aiPlaylist struct {
	ID         string `json:"id"`
	Source     string `json:"source"`
	Name       string `json:"name"`
	Creator    string `json:"creator,omitempty"`
	TrackCount int    `json:"track_count,omitempty"`
}

type aiTopList struct {
	ID              string `json:"id"`
	Source          string `json:"source"`
	Name            string `json:"name"`
	TrackCount      int    `json:"track_count,omitempty"`
	PlayCount       int64  `json:"play_count,omitempty"`
	UpdateFrequency string `json:"update_frequency,omitempty"`
	Group           string `json:"group,omitempty"`
}

type aiPlaylistCategory struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Name   string `json:"name"`
	Group  string `json:"group,omitempty"`
	Count  int    `json:"count,omitempty"`
	Hot    bool   `json:"hot,omitempty"`
}

type aiSourceError struct {
	Source string `json:"source"`
	Error  string `json:"error"`
}

type aiSearchResponse struct {
	Query      string          `json:"query"`
	Type       string          `json:"type"`
	Sources    []string        `json:"sources"`
	Total      int             `json:"total"`
	Count      int             `json:"count"`
	Truncated  bool            `json:"truncated,omitempty"`
	Songs      []aiSong        `json:"songs,omitempty"`
	Playlists  []aiPlaylist    `json:"playlists,omitempty"`
	Errors     []aiSourceError `json:"errors,omitempty"`
	NextAction string          `json:"next_action,omitempty"`
}

type aiCollectionSongsResponse struct {
	Kind       string          `json:"kind"`
	ID         string          `json:"id"`
	Source     string          `json:"source"`
	Total      int             `json:"total"`
	Offset     int             `json:"offset"`
	Limit      int             `json:"limit"`
	Count      int             `json:"count"`
	Truncated  bool            `json:"truncated,omitempty"`
	HasMore    bool            `json:"has_more,omitempty"`
	Songs      []aiSong        `json:"songs"`
	Errors     []aiSourceError `json:"errors,omitempty"`
	NextAction string          `json:"next_action,omitempty"`
}

type aiTopListsResponse struct {
	Sources    []string        `json:"sources"`
	Total      int             `json:"total"`
	Offset     int             `json:"offset"`
	Limit      int             `json:"limit"`
	Count      int             `json:"count"`
	Truncated  bool            `json:"truncated,omitempty"`
	HasMore    bool            `json:"has_more,omitempty"`
	TopLists   []aiTopList     `json:"toplists"`
	Errors     []aiSourceError `json:"errors,omitempty"`
	NextAction string          `json:"next_action,omitempty"`
}

type aiPlaylistCategoriesResponse struct {
	Sources    []string             `json:"sources"`
	Total      int                  `json:"total"`
	Offset     int                  `json:"offset"`
	Limit      int                  `json:"limit"`
	Count      int                  `json:"count"`
	Truncated  bool                 `json:"truncated,omitempty"`
	HasMore    bool                 `json:"has_more,omitempty"`
	Categories []aiPlaylistCategory `json:"categories"`
	Errors     []aiSourceError      `json:"errors,omitempty"`
	NextAction string               `json:"next_action,omitempty"`
}

type aiPlaylistListResponse struct {
	Source     string          `json:"source,omitempty"`
	Sources    []string        `json:"sources,omitempty"`
	Page       int             `json:"page,omitempty"`
	Total      int             `json:"total"`
	Count      int             `json:"count"`
	Truncated  bool            `json:"truncated,omitempty"`
	HasMore    bool            `json:"has_more,omitempty"`
	Playlists  []aiPlaylist    `json:"playlists"`
	Errors     []aiSourceError `json:"errors,omitempty"`
	NextAction string          `json:"next_action,omitempty"`
}

type aiLocalMusicResponse struct {
	Total      int             `json:"total"`
	Offset     int             `json:"offset"`
	Limit      int             `json:"limit"`
	Count      int             `json:"count"`
	HasMore    bool            `json:"has_more,omitempty"`
	Songs      []aiSong        `json:"songs"`
	Errors     []aiSourceError `json:"errors,omitempty"`
	NextAction string          `json:"next_action,omitempty"`
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

type aiSearchProviders struct {
	SearchSongs             func(source string) func(string) ([]model.Song, error)
	SearchPlaylists         func(source string) func(string) ([]model.Playlist, error)
	SearchAlbums            func(source string) func(string) ([]model.Playlist, error)
	GetPlaylistSongs        func(source string) func(string) ([]model.Song, error)
	GetAlbumSongs           func(source string) func(string) ([]model.Song, error)
	GetLocalCollectionSongs func(string) ([]model.Song, error)
	GetTopLists             func(source string) func() ([]model.TopList, error)
	GetTopListSongs         func(source string) func(string) ([]model.Song, error)
	GetPlaylistCategories   func(source string) func() ([]model.PlaylistCategory, error)
	GetCategoryPlaylists    func(source string) func(string, int, int) ([]model.Playlist, error)
	GetRecommended          func(source string) func() ([]model.Playlist, error)
	GetUserPlaylists        func(source string) func(int, int) ([]model.Playlist, error)
	SearchLocalSongs        func(string, int) []model.Song
	SearchLocalPlaylists    func(string) []model.Playlist
	ListLocalSongs          func(int, int) ([]model.Song, int, error)
}

func defaultAISearchProviders() aiSearchProviders {
	return aiSearchProviders{
		SearchSongs: func(source string) func(string) ([]model.Song, error) {
			return core.GetSearchFunc(source)
		},
		SearchPlaylists: func(source string) func(string) ([]model.Playlist, error) {
			return core.GetPlaylistSearchFunc(source)
		},
		SearchAlbums: func(source string) func(string) ([]model.Playlist, error) {
			return core.GetAlbumSearchFunc(source)
		},
		GetPlaylistSongs: func(source string) func(string) ([]model.Song, error) {
			return core.GetPlaylistDetailFunc(source)
		},
		GetAlbumSongs: func(source string) func(string) ([]model.Song, error) {
			return core.GetAlbumDetailFunc(source)
		},
		GetLocalCollectionSongs: func(id string) ([]model.Song, error) {
			collection, err := loadCollection(id)
			if err != nil {
				return nil, err
			}
			return loadCollectionSongs(collection)
		},
		GetTopLists: func(source string) func() ([]model.TopList, error) {
			return core.GetTopListsFunc(source)
		},
		GetTopListSongs: func(source string) func(string) ([]model.Song, error) {
			return core.GetTopListSongsFunc(source)
		},
		GetPlaylistCategories: func(source string) func() ([]model.PlaylistCategory, error) {
			return core.GetPlaylistCategoriesFunc(source)
		},
		GetCategoryPlaylists: func(source string) func(string, int, int) ([]model.Playlist, error) {
			return core.GetCategoryPlaylistsFunc(source)
		},
		GetRecommended: func(source string) func() ([]model.Playlist, error) {
			return core.GetRecommendFunc(source)
		},
		GetUserPlaylists: func(source string) func(int, int) ([]model.Playlist, error) {
			return core.GetUserPlaylistsFunc(source)
		},
		SearchLocalSongs:     localMusicSearchSongs,
		SearchLocalPlaylists: localCollectionSearchPlaylists,
		ListLocalSongs:       loadAILocalSongs,
	}
}

// RegisterAIRoutes exposes compact discovery, detail, playback, and download
// routes for AI clients. It avoids sending rendered pages to models just to
// retrieve a few structured fields.
func RegisterAIRoutes(api *gin.RouterGroup) {
	basePath := strings.TrimRight(api.BasePath(), "/")
	indexPath := basePath + "/api/ai/index"
	searchPath := basePath + "/api/ai/search"
	providers := defaultAISearchProviders()

	api.GET("/api/ai/index", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=300")
		c.JSON(http.StatusOK, gin.H{
			"name":           "go-music-dl",
			"schema_version": 1,
			"description":    "Compact JSON discovery, details, playback and download API for AI agents. Prefer this index over rendered HTML pages.",
			"base_path":      basePath,
			"index":          indexPath,
			"search":         searchPath,
			"llms_txt":       basePath + "/llms.txt",
			"auth":           "most listed endpoints need no login; writes require X-Requested-With: XMLHttpRequest; some personalized sources use configured platform cookies",
			"token_guidance": "Use /api/ai/search first, request only needed sources, keep limit small, then call the returned detail or playback next_action. Do not fetch rendered HTML for structured data.",
			"sources": gin.H{
				"song":              core.GetAllSourceNames(),
				"playlist":          append(core.GetPlaylistSourceNames(), "local"),
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

	api.GET("/api/ai/search", aiSearchHandler(basePath, providers))
	registerAIResourceRoutes(api, basePath, providers)
}

func aiEndpointCatalog(basePath string) []aiEndpoint {
	writeAuth := "X-Requested-With: XMLHttpRequest"
	return []aiEndpoint{
		{
			Group:       "discover",
			Method:      http.MethodGet,
			Path:        basePath + "/api/ai/search",
			Description: "Search compact songs, playlists, or albums across selected sources.",
			Params: map[string]string{
				"q":            "required",
				"type":         "song (default), playlist, or album",
				"sources":      "optional comma-separated names or all",
				"limit":        "1-50, default 10",
				"exact_artist": "optional exact artist filter for songs",
			},
			Returns: "json",
		},
		{
			Group:       "details",
			Method:      http.MethodGet,
			Path:        basePath + "/api/ai/playlist",
			Description: "Get compact songs from a platform playlist or a local collection.",
			Params: map[string]string{
				"id":     "required playlist or local collection id",
				"source": "required",
				"offset": "optional, default 0",
				"limit":  "1-200, default 50",
			},
			Returns: "json",
		},
		{
			Group:       "details",
			Method:      http.MethodGet,
			Path:        basePath + "/api/ai/album",
			Description: "Get compact songs from an album.",
			Params: map[string]string{
				"id":     "required album id",
				"source": "required",
				"offset": "optional, default 0",
				"limit":  "1-200, default 50",
			},
			Returns: "json",
		},
		{
			Group:       "discover",
			Method:      http.MethodGet,
			Path:        basePath + "/api/ai/toplists",
			Description: "List music rankings/charts and other chart-like playlists from selected sources.",
			Params: map[string]string{
				"sources": "optional comma-separated names or all",
				"offset":  "optional, default 0",
				"limit":   "1-200, default 50",
			},
			Returns: "json",
		},
		{
			Group:       "details",
			Method:      http.MethodGet,
			Path:        basePath + "/api/ai/toplist",
			Description: "Get compact ranked songs from one ranking/chart.",
			Params: map[string]string{
				"id":     "required toplist id",
				"source": "required",
				"offset": "optional, default 0",
				"limit":  "1-200, default 50",
			},
			Returns: "json",
		},
		{
			Group:       "discover",
			Method:      http.MethodGet,
			Path:        basePath + "/api/ai/playlist-categories",
			Description: "List playlist categories for discovery.",
			Params: map[string]string{
				"sources": "optional comma-separated names or all",
				"offset":  "optional, default 0",
				"limit":   "1-500, default 50",
			},
			Returns: "json",
		},
		{
			Group:       "discover",
			Method:      http.MethodGet,
			Path:        basePath + "/api/ai/category-playlists",
			Description: "List playlists inside one category.",
			Params: map[string]string{
				"source":      "required",
				"category_id": "required query parameter; use the returned id, including an empty id when it means all",
				"page":        "optional, default 1",
				"limit":       "1-200, default 50",
			},
			Returns: "json",
		},
		{
			Group:       "discover",
			Method:      http.MethodGet,
			Path:        basePath + "/api/ai/recommend",
			Description: "List recommended playlists.",
			Params: map[string]string{
				"sources": "optional comma-separated names or all",
				"offset":  "optional, default 0",
				"limit":   "1-200, default 50",
			},
			Returns: "json",
		},
		{
			Group:       "discover",
			Method:      http.MethodGet,
			Path:        basePath + "/api/ai/user-playlists",
			Description: "List playlists collected by the configured platform account. Missing cookies or login are returned per source in errors.",
			Params: map[string]string{
				"sources": "optional comma-separated names or all",
				"page":    "optional, default 1",
				"limit":   "1-200, default 50",
			},
			Returns: "json",
		},
		{
			Group:       "playback",
			Method:      http.MethodGet,
			Path:        basePath + "/inspect",
			Description: "Check whether a song can currently be resolved to playable media.",
			Params: map[string]string{
				"id":       "required",
				"source":   "required",
				"duration": "optional seconds",
				"extra":    "optional JSON object with source-specific playback fields",
			},
			Returns: "json",
		},
		{
			Group:       "playback",
			Method:      http.MethodGet,
			Path:        basePath + "/switch_source",
			Description: "Find the best matching song on another source.",
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
			Description: "Stream or download audio. Always pass stream=1 for playback-only requests. URL-encode extra as JSON from the compact song result.",
			Params: map[string]string{
				"id":     "required",
				"source": "required",
				"name":   "recommended",
				"artist": "recommended",
				"album":  "optional",
				"cover":  "optional",
				"extra":  "optional URL-encoded JSON object",
				"stream": "use 1 for browser/audio playback",
				"embed":  "optional 1 to write metadata when downloading",
			},
			Returns: "audio stream or file",
		},
		{
			Group:       "playback",
			Method:      http.MethodGet,
			Path:        basePath + "/lyric",
			Description: "Fetch lyrics as text without forcing a file download.",
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
			Description: "Download standard LRC lyrics.",
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
			Description: "Fetch a remote cover image through the service.",
			Params: map[string]string{
				"url":    "required",
				"source": "optional for source-specific request headers",
			},
			Returns: "image bytes",
		},
		{
			Group:       "downloads",
			Method:      http.MethodGet,
			Path:        basePath + "/api/downloads/records",
			Description: "Read download history.",
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
			Description: "Precheck how many songs are already downloaded.",
			Body:        map[string]string{"songs": "array of {name,artist}; maximum 20000"},
			Returns:     "json",
		},
		{
			Group:       "downloads",
			Method:      http.MethodGet,
			Path:        basePath + "/api/downloads/queue",
			Description: "Read the persistent download queue and status counts.",
			Params: map[string]string{
				"page":   "optional, default 1",
				"status": "optional waiting, downloading, success, failed, skipped, canceled, or active",
			},
			Returns: "json",
		},
		{
			Group:       "downloads",
			Method:      http.MethodPost,
			Path:        basePath + "/api/downloads/queue",
			Description: "Enqueue up to 2000 explicit songs.",
			Body: map[string]string{
				"request_id": "required unique string, maximum 128 bytes",
				"songs":      "array of {id,source,name,artist,album,cover,extra,embed}; extra must be a JSON-encoded string",
			},
			Returns: "json",
			Write:   true,
			Auth:    writeAuth,
		},
		{
			Group:       "downloads",
			Method:      http.MethodPost,
			Path:        basePath + "/api/downloads/queue/container",
			Description: "Resolve an entire platform playlist, album, toplist, chart, or local collection and enqueue valid songs. Use kind=playlist for both playlists and toplists.",
			Body: map[string]string{
				"request_id": "required unique string",
				"kind":       "playlist, album, or collection",
				"id":         "required container id",
				"source":     "required for platform containers",
				"link":       "optional original container link",
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
			Description: "Retry or cancel one queue task. action must be retry or cancel.",
			Params:      map[string]string{"id": "required queue task id", "action": "retry or cancel"},
			Returns:     "json",
			Write:       true,
			Auth:        writeAuth,
		},
		{
			Group:       "downloads",
			Method:      http.MethodDelete,
			Path:        basePath + "/api/downloads/queue/{id}",
			Description: "Remove one inactive queue task.",
			Params:      map[string]string{"id": "required queue task id"},
			Returns:     "json",
			Write:       true,
			Auth:        writeAuth,
		},
		{
			Group:       "downloads",
			Method:      http.MethodDelete,
			Path:        basePath + "/api/downloads/queue",
			Description: "Clear inactive queue tasks. This is destructive and should only be used on explicit user request.",
			Returns:     "json",
			Write:       true,
			Auth:        writeAuth,
		},
		{
			Group:       "library",
			Method:      http.MethodGet,
			Path:        basePath + "/api/ai/local-music",
			Description: "List the local music library as compact paginated songs.",
			Params: map[string]string{
				"offset": "optional, default 0",
				"limit":  "1-200, default 50",
			},
			Returns: "json",
		},
		{
			Group:       "library",
			Method:      http.MethodGet,
			Path:        basePath + "/local_music",
			Description: "List the full local music index, including file metadata. Prefer /api/ai/local-music for AI clients.",
			Params: map[string]string{
				"offset": "optional, default 0",
				"limit":  "optional page size",
				"force":  "use 1 to rescan the filesystem",
			},
			Returns: "json",
		},
		{
			Group:       "library",
			Method:      http.MethodGet,
			Path:        basePath + "/collections",
			Description: "List local collections. Use include_imported=1 to include imported external metadata.",
			Params:      map[string]string{"include_imported": "optional 0 or 1"},
			Returns:     "json",
		},
		{
			Group:       "library",
			Method:      http.MethodGet,
			Path:        basePath + "/collections/{id}/songs",
			Description: "List songs in a local collection.",
			Params:      map[string]string{"id": "required local collection id"},
			Returns:     "json",
		},
		{
			Group:       "system",
			Method:      http.MethodGet,
			Path:        basePath + "/healthz",
			Description: "Service health check.",
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

func indexPath(basePath string) string {
	return basePath + "/api/ai/index"
}

func aiSearchHandler(basePath string, providers aiSearchProviders) gin.HandlerFunc {
	return func(c *gin.Context) {
		keyword := strings.TrimSpace(c.Query("q"))
		if keyword == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing_q", "message": "q is required"})
			return
		}
		if len(keyword) > aiSearchMaxQueryLen {
			c.JSON(http.StatusBadRequest, gin.H{"error": "query_too_long", "message": fmt.Sprintf("q must be at most %d bytes", aiSearchMaxQueryLen)})
			return
		}

		searchType := strings.ToLower(strings.TrimSpace(c.DefaultQuery("type", "song")))
		if searchType != "song" && searchType != "playlist" && searchType != "album" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_type", "message": "type must be song, playlist, or album"})
			return
		}

		limit, err := parseAISearchLimit(c.Query("limit"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_limit", "message": err.Error()})
			return
		}

		sources, err := parseAISearchSources(c.QueryArray("sources"), searchType)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":         "invalid_sources",
				"message":       err.Error(),
				"valid_sources": aiSourceNamesForType(searchType),
			})
			return
		}

		result := executeAISearch(
			keyword,
			searchType,
			sources,
			strings.TrimSpace(c.Query("exact_artist")),
			limit,
			providers,
		)
		if result.Count > 0 {
			switch searchType {
			case "playlist":
				result.NextAction = basePath + "/api/ai/playlist?id={id}&source={source}&offset=0&limit=50"
			case "album":
				result.NextAction = basePath + "/api/ai/album?id={id}&source={source}&offset=0&limit=50"
			default:
				result.NextAction = aiDownloadAction(basePath)
			}
		}
		c.JSON(http.StatusOK, result)
	}
}

func registerAIResourceRoutes(api *gin.RouterGroup, basePath string, providers aiSearchProviders) {
	api.GET("/api/ai/playlist", aiCollectionSongsHandler(
		basePath,
		"playlist",
		append(core.GetPlaylistSourceNames(), "local"),
		providers.GetPlaylistSongs,
		providers.GetLocalCollectionSongs,
	))
	api.GET("/api/ai/album", aiCollectionSongsHandler(
		basePath,
		"album",
		core.GetAlbumSourceNames(),
		providers.GetAlbumSongs,
		nil,
	))
	api.GET("/api/ai/toplists", aiTopListsHandler(basePath, providers.GetTopLists))
	api.GET("/api/ai/toplist", aiCollectionSongsHandler(
		basePath,
		"toplist",
		core.GetTopListSourceNames(),
		providers.GetTopListSongs,
		nil,
	))
	api.GET("/api/ai/playlist-categories", aiPlaylistCategoriesHandler(basePath, providers.GetPlaylistCategories))
	api.GET("/api/ai/category-playlists", aiCategoryPlaylistsHandler(basePath, providers.GetCategoryPlaylists))
	api.GET("/api/ai/recommend", aiRecommendHandler(basePath, providers.GetRecommended))
	api.GET("/api/ai/user-playlists", aiUserPlaylistsHandler(basePath, providers.GetUserPlaylists))
	api.GET("/api/ai/local-music", aiLocalMusicHandler(basePath, providers.ListLocalSongs))
}

func aiCollectionSongsHandler(
	basePath string,
	kind string,
	allowedSources []string,
	provider func(source string) func(string) ([]model.Song, error),
	localProvider func(string) ([]model.Song, error),
) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Query("id"))
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing_id", "message": "id is required"})
			return
		}
		if len(id) > 1024 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id", "message": "id is too long"})
			return
		}
		source, err := parseAISingleSource(c.Query("source"), allowedSources)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":         "invalid_source",
				"message":       err.Error(),
				"valid_sources": allowedSources,
			})
			return
		}
		offset, err := parseAIOffset(c.Query("offset"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_offset", "message": err.Error()})
			return
		}
		limit, err := parseAIBoundedLimit(c.Query("limit"), aiResourceDefaultLimit, aiResourceMaxLimit)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_limit", "message": err.Error()})
			return
		}

		var lookup func(string) ([]model.Song, error)
		if kind == "playlist" && isLocalMusicSource(source) {
			lookup = localProvider
		} else if provider != nil {
			lookup = provider(source)
		}
		if lookup == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "unsupported_source",
				"message": fmt.Sprintf("%s detail is not supported for source %s", kind, source),
			})
			return
		}

		songs, sourceErr := lookup(id)
		for i := range songs {
			songs[i].Source = source
		}
		total := len(songs)
		pageSongs := paginateAISongs(songs, offset, limit)
		response := aiCollectionSongsResponse{
			Kind:      kind,
			ID:        id,
			Source:    source,
			Total:     total,
			Offset:    offset,
			Limit:     limit,
			Count:     len(pageSongs),
			Truncated: total > offset+len(pageSongs),
			HasMore:   total > offset+len(pageSongs),
			Songs:     compactAISongs(pageSongs),
		}
		if sourceErr != nil {
			response.Errors = []aiSourceError{{Source: source, Error: compactAIError(sourceErr)}}
		}
		if response.Count > 0 {
			response.NextAction = aiDownloadAction(basePath)
		}
		c.JSON(http.StatusOK, response)
	}
}

func paginateAISongs(songs []model.Song, offset, limit int) []model.Song {
	if offset >= len(songs) {
		return nil
	}
	end := offset + limit
	if end > len(songs) {
		end = len(songs)
	}
	return songs[offset:end]
}

func aiTopListsHandler(basePath string, provider func(source string) func() ([]model.TopList, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		sources, err := parseAISources(c.QueryArray("sources"), core.GetTopListSourceNames(), core.GetTopListSourceNames())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":         "invalid_sources",
				"message":       err.Error(),
				"valid_sources": core.GetTopListSourceNames(),
			})
			return
		}
		offset, err := parseAIOffset(c.Query("offset"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_offset", "message": err.Error()})
			return
		}
		limit, err := parseAIBoundedLimit(c.Query("limit"), aiResourceDefaultLimit, aiResourceMaxLimit)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_limit", "message": err.Error()})
			return
		}

		groups := make([][]model.TopList, len(sources))
		sourceErrors := make([]error, len(sources))
		var wg sync.WaitGroup
		for i, source := range sources {
			fn := sourceFunc(provider, source)
			if fn == nil {
				sourceErrors[i] = fmt.Errorf("top lists are not supported")
				continue
			}
			wg.Add(1)
			go func(index int, source string, lookup func() ([]model.TopList, error)) {
				defer wg.Done()
				groups[index], sourceErrors[index] = lookup()
				for j := range groups[index] {
					groups[index][j].Source = source
				}
			}(i, source, fn)
		}
		wg.Wait()

		total := 0
		for _, group := range groups {
			total += len(group)
		}
		page := interleaveAIItems(groups, offset, limit)
		response := aiTopListsResponse{
			Sources:   sources,
			Total:     total,
			Offset:    offset,
			Limit:     limit,
			Count:     len(page),
			Truncated: total > offset+len(page),
			HasMore:   total > offset+len(page),
			TopLists:  compactAITopLists(page),
			Errors:    compactAISourceErrors(sources, sourceErrors),
		}
		if response.Count > 0 {
			response.NextAction = basePath + "/api/ai/toplist?id={id}&source={source}&offset=0&limit=50"
		}
		c.JSON(http.StatusOK, response)
	}
}

func aiPlaylistCategoriesHandler(basePath string, provider func(source string) func() ([]model.PlaylistCategory, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		sources, err := parseAISources(
			c.QueryArray("sources"),
			core.GetPlaylistCategorySourceNames(),
			core.GetPlaylistCategorySourceNames(),
		)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":         "invalid_sources",
				"message":       err.Error(),
				"valid_sources": core.GetPlaylistCategorySourceNames(),
			})
			return
		}
		offset, err := parseAIOffset(c.Query("offset"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_offset", "message": err.Error()})
			return
		}
		limit, err := parseAIBoundedLimit(c.Query("limit"), aiResourceDefaultLimit, aiCategoryMaxLimit)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_limit", "message": err.Error()})
			return
		}

		groups := make([][]model.PlaylistCategory, len(sources))
		sourceErrors := make([]error, len(sources))
		var wg sync.WaitGroup
		for i, source := range sources {
			fn := sourceFunc(provider, source)
			if fn == nil {
				sourceErrors[i] = fmt.Errorf("playlist categories are not supported")
				continue
			}
			wg.Add(1)
			go func(index int, source string, lookup func() ([]model.PlaylistCategory, error)) {
				defer wg.Done()
				groups[index], sourceErrors[index] = lookup()
				for j := range groups[index] {
					groups[index][j].Source = source
				}
			}(i, source, fn)
		}
		wg.Wait()

		total := 0
		for _, group := range groups {
			total += len(group)
		}
		page := interleaveAIItems(groups, offset, limit)
		response := aiPlaylistCategoriesResponse{
			Sources:    sources,
			Total:      total,
			Offset:     offset,
			Limit:      limit,
			Count:      len(page),
			Truncated:  total > offset+len(page),
			HasMore:    total > offset+len(page),
			Categories: compactAIPlaylistCategories(page),
			Errors:     compactAISourceErrors(sources, sourceErrors),
		}
		if response.Count > 0 {
			response.NextAction = basePath + "/api/ai/category-playlists?source={source}&category_id={id}&page=1&limit=50"
		}
		c.JSON(http.StatusOK, response)
	}
}

func aiCategoryPlaylistsHandler(basePath string, provider func(source string) func(string, int, int) ([]model.Playlist, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		source, err := parseAISingleSource(c.Query("source"), core.GetPlaylistCategorySourceNames())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":         "invalid_source",
				"message":       err.Error(),
				"valid_sources": core.GetPlaylistCategorySourceNames(),
			})
			return
		}
		categoryIDValue, hasCategoryID := c.GetQuery("category_id")
		if !hasCategoryID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing_category_id", "message": "category_id is required"})
			return
		}
		categoryID := strings.TrimSpace(categoryIDValue)
		pageNumber, err := parseAIPage(c.Query("page"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_page", "message": err.Error()})
			return
		}
		limit, err := parseAIBoundedLimit(c.Query("limit"), aiResourceDefaultLimit, aiResourceMaxLimit)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_limit", "message": err.Error()})
			return
		}
		fn := sourceFunc(provider, source)
		if fn == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "unsupported_source",
				"message": "category playlists are not supported for source " + source,
			})
			return
		}

		playlists, sourceErr := fn(categoryID, pageNumber, limit)
		for i := range playlists {
			playlists[i].Source = source
		}
		returned := len(playlists)
		if len(playlists) > limit {
			playlists = playlists[:limit]
		}
		response := aiPlaylistListResponse{
			Source:     source,
			Page:       pageNumber,
			Total:      returned,
			Count:      len(playlists),
			Truncated:  returned > len(playlists),
			HasMore:    returned >= limit,
			Playlists:  compactAIPlaylists(playlists),
			NextAction: basePath + "/api/ai/playlist?id={id}&source={source}&offset=0&limit=50",
		}
		if sourceErr != nil {
			response.Errors = []aiSourceError{{Source: source, Error: compactAIError(sourceErr)}}
		}
		c.JSON(http.StatusOK, response)
	}
}

func aiRecommendHandler(basePath string, provider func(source string) func() ([]model.Playlist, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		sources, err := parseAISources(
			c.QueryArray("sources"),
			core.GetRecommendSourceNames(),
			core.GetRecommendSourceNames(),
		)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":         "invalid_sources",
				"message":       err.Error(),
				"valid_sources": core.GetRecommendSourceNames(),
			})
			return
		}
		offset, err := parseAIOffset(c.Query("offset"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_offset", "message": err.Error()})
			return
		}
		limit, err := parseAIBoundedLimit(c.Query("limit"), aiResourceDefaultLimit, aiResourceMaxLimit)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_limit", "message": err.Error()})
			return
		}

		groups := make([][]model.Playlist, len(sources))
		sourceErrors := make([]error, len(sources))
		var wg sync.WaitGroup
		for i, source := range sources {
			fn := sourceFunc(provider, source)
			if fn == nil {
				sourceErrors[i] = fmt.Errorf("recommendations are not supported")
				continue
			}
			wg.Add(1)
			go func(index int, source string, lookup func() ([]model.Playlist, error)) {
				defer wg.Done()
				groups[index], sourceErrors[index] = lookup()
				for j := range groups[index] {
					groups[index][j].Source = source
				}
			}(i, source, fn)
		}
		wg.Wait()

		total := 0
		for _, group := range groups {
			total += len(group)
		}
		page := interleaveAIItems(groups, offset, limit)
		response := aiPlaylistListResponse{
			Sources:    sources,
			Total:      total,
			Count:      len(page),
			Truncated:  total > offset+len(page),
			HasMore:    total > offset+len(page),
			Playlists:  compactAIPlaylists(page),
			Errors:     compactAISourceErrors(sources, sourceErrors),
			NextAction: basePath + "/api/ai/playlist?id={id}&source={source}&offset=0&limit=50",
		}
		c.JSON(http.StatusOK, response)
	}
}

func aiUserPlaylistsHandler(basePath string, provider func(source string) func(int, int) ([]model.Playlist, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		sources, err := parseAISources(
			c.QueryArray("sources"),
			core.GetUserPlaylistSourceNames(),
			core.GetUserPlaylistSourceNames(),
		)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":         "invalid_sources",
				"message":       err.Error(),
				"valid_sources": core.GetUserPlaylistSourceNames(),
			})
			return
		}
		pageNumber, err := parseAIPage(c.Query("page"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_page", "message": err.Error()})
			return
		}
		limit, err := parseAIBoundedLimit(c.Query("limit"), aiResourceDefaultLimit, aiResourceMaxLimit)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_limit", "message": err.Error()})
			return
		}

		groups := make([][]model.Playlist, len(sources))
		sourceErrors := make([]error, len(sources))
		var wg sync.WaitGroup
		for i, source := range sources {
			fn := sourceFunc(provider, source)
			if fn == nil {
				sourceErrors[i] = fmt.Errorf("user playlists are not supported")
				continue
			}
			wg.Add(1)
			go func(index int, source string, lookup func(int, int) ([]model.Playlist, error)) {
				defer wg.Done()
				groups[index], sourceErrors[index] = lookup(pageNumber, limit)
				for j := range groups[index] {
					groups[index][j].Source = source
				}
			}(i, source, fn)
		}
		wg.Wait()

		total := 0
		hasMore := false
		for _, group := range groups {
			total += len(group)
			if len(group) >= limit {
				hasMore = true
			}
		}
		selected := interleaveAIItems(groups, 0, limit)
		response := aiPlaylistListResponse{
			Sources:    sources,
			Page:       pageNumber,
			Total:      total,
			Count:      len(selected),
			Truncated:  total > len(selected),
			HasMore:    hasMore,
			Playlists:  compactAIPlaylists(selected),
			Errors:     compactAISourceErrors(sources, sourceErrors),
			NextAction: basePath + "/api/ai/playlist?id={id}&source={source}&offset=0&limit=50",
		}
		c.JSON(http.StatusOK, response)
	}
}

func aiLocalMusicHandler(basePath string, provider func(int, int) ([]model.Song, int, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		offset, err := parseAIOffset(c.Query("offset"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_offset", "message": err.Error()})
			return
		}
		limit, err := parseAIBoundedLimit(c.Query("limit"), aiResourceDefaultLimit, aiResourceMaxLimit)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_limit", "message": err.Error()})
			return
		}
		if provider == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "local_music_unavailable"})
			return
		}

		songs, total, sourceErr := provider(offset, limit)
		if sourceErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "local_music_unavailable", "message": compactAIError(sourceErr)})
			return
		}
		for i := range songs {
			songs[i].Source = localMusicSource
		}
		response := aiLocalMusicResponse{
			Total:   total,
			Offset:  offset,
			Limit:   limit,
			Count:   len(songs),
			HasMore: total > offset+len(songs),
			Songs:   compactAISongs(songs),
		}
		if response.Count > 0 {
			response.NextAction = aiDownloadAction(basePath)
		}
		c.JSON(http.StatusOK, response)
	}
}

func loadAILocalSongs(offset, limit int) ([]model.Song, int, error) {
	if tracks, total, ok := loadTracksFromIndex(offset, limit); ok {
		return localMusicTracksToSongs(tracks), total, nil
	}
	tracks, _, _, err, _, _ := scanLocalMusicTracksCached(false)
	if err != nil {
		return nil, 0, err
	}
	page := paginateLocalMusicTracks(tracks, offset, limit)
	return localMusicTracksToSongs(page), len(tracks), nil
}

func sourceFunc[T any](provider func(string) T, source string) T {
	if provider == nil {
		var zero T
		return zero
	}
	return provider(source)
}

func interleaveAIItems[T any](groups [][]T, offset, limit int) []T {
	result := make([]T, 0, limit)
	seen := 0
	for row := 0; len(result) < limit; row++ {
		added := false
		for _, group := range groups {
			if row >= len(group) {
				continue
			}
			added = true
			if seen >= offset {
				result = append(result, group[row])
				if len(result) == limit {
					break
				}
			}
			seen++
		}
		if !added {
			break
		}
	}
	return result
}

func compactAISourceErrors(sources []string, sourceErrors []error) []aiSourceError {
	result := make([]aiSourceError, 0)
	for i, err := range sourceErrors {
		if err != nil {
			result = append(result, aiSourceError{Source: sources[i], Error: compactAIError(err)})
		}
	}
	return result
}

func compactAITopLists(toplists []model.TopList) []aiTopList {
	result := make([]aiTopList, 0, len(toplists))
	for _, toplist := range toplists {
		result = append(result, aiTopList{
			ID:              toplist.ID,
			Source:          toplist.Source,
			Name:            toplist.Name,
			TrackCount:      toplist.TrackCount,
			PlayCount:       toplist.PlayCount,
			UpdateFrequency: toplist.UpdateFrequency,
			Group:           strings.TrimSpace(toplist.Extra["group"]),
		})
	}
	return result
}

func compactAIPlaylistCategories(categories []model.PlaylistCategory) []aiPlaylistCategory {
	result := make([]aiPlaylistCategory, 0, len(categories))
	for _, category := range categories {
		result = append(result, aiPlaylistCategory{
			ID:     category.ID,
			Source: category.Source,
			Name:   category.Name,
			Group:  category.Group,
			Count:  category.Count,
			Hot:    category.Hot,
		})
	}
	return result
}

func parseAISearchLimit(raw string) (int, error) {
	return parseAIBoundedLimit(raw, aiSearchDefaultLimit, aiSearchMaxLimit)
}

func parseAIBoundedLimit(raw string, defaultLimit, maxLimit int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > maxLimit {
		return 0, fmt.Errorf("limit must be between 1 and %d", maxLimit)
	}
	return limit, nil
}

func parseAISearchSources(values []string, searchType string) ([]string, error) {
	allowed := aiSourceNamesForType(searchType)
	return parseAISources(values, allowed, defaultSourcesForSearchType(searchType))
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

func parseAIOffset(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(raw)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("offset must be zero or greater")
	}
	return offset, nil
}

func parseAIPage(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 1, nil
	}
	page, err := strconv.Atoi(raw)
	if err != nil || page < 1 {
		return 0, fmt.Errorf("page must be at least 1")
	}
	return page, nil
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

func aiSourceNamesForType(searchType string) []string {
	switch searchType {
	case "playlist":
		return append(core.GetPlaylistSourceNames(), "local")
	case "album":
		return core.GetAlbumSourceNames()
	default:
		return core.GetAllSourceNames()
	}
}

func executeAISearch(keyword, searchType string, sources []string, exactArtist string, limit int, providers aiSearchProviders) aiSearchResponse {
	response := aiSearchResponse{
		Query:   keyword,
		Type:    searchType,
		Sources: sources,
	}

	if searchType == "song" {
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
			fn := search(source)
			if fn == nil {
				sourceErrors[i] = fmt.Errorf("source does not support song search")
				continue
			}
			wg.Add(1)
			go func(index int, source string, searchFn func(string) ([]model.Song, error)) {
				defer wg.Done()
				groups[index], sourceErrors[index] = searchFn(keyword)
			}(i, source, fn)
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
		response.Total = total
		response.Count = len(limited)
		response.Truncated = total > len(limited)
		response.Songs = compactAISongs(limited)
		for i, err := range sourceErrors {
			if err != nil {
				response.Errors = append(response.Errors, aiSourceError{Source: sources[i], Error: compactAIError(err)})
			}
		}
		return response
	}

	groups := make([][]model.Playlist, len(sources))
	sourceErrors := make([]error, len(sources))
	var wg sync.WaitGroup
	for i, source := range sources {
		if isLocalMusicSource(source) {
			if providers.SearchLocalPlaylists != nil {
				groups[i] = providers.SearchLocalPlaylists(keyword)
			}
			continue
		}
		var search func(string) func(string) ([]model.Playlist, error)
		if searchType == "album" {
			search = providers.SearchAlbums
		} else {
			search = providers.SearchPlaylists
		}
		if search == nil {
			sourceErrors[i] = fmt.Errorf("search provider is unavailable")
			continue
		}
		fn := search(source)
		if fn == nil {
			sourceErrors[i] = fmt.Errorf("source does not support %s search", searchType)
			continue
		}
		wg.Add(1)
		go func(index int, source string, searchFn func(string) ([]model.Playlist, error)) {
			defer wg.Done()
			groups[index], sourceErrors[index] = searchFn(keyword)
		}(i, source, fn)
	}
	wg.Wait()

	total := 0
	for i, group := range groups {
		for j := range group {
			groups[i][j].Source = sources[i]
		}
		total += len(group)
	}
	limited := interleavePlaylists(groups, limit)
	response.Total = total
	response.Count = len(limited)
	response.Truncated = total > len(limited)
	response.Playlists = compactAIPlaylists(limited)
	for i, err := range sourceErrors {
		if err != nil {
			response.Errors = append(response.Errors, aiSourceError{Source: sources[i], Error: compactAIError(err)})
		}
	}
	return response
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

func interleavePlaylists(groups [][]model.Playlist, limit int) []model.Playlist {
	result := make([]model.Playlist, 0, limit)
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

func compactAIPlaylists(playlists []model.Playlist) []aiPlaylist {
	result := make([]aiPlaylist, 0, len(playlists))
	for _, playlist := range playlists {
		result = append(result, aiPlaylist{
			ID:         playlist.ID,
			Source:     playlist.Source,
			Name:       playlist.Name,
			Creator:    playlist.Creator,
			TrackCount: playlist.TrackCount,
		})
	}
	return result
}

func compactAIError(err error) string {
	const maxRunes = 300
	message := strings.TrimSpace(err.Error())
	runes := []rune(message)
	if len(runes) <= maxRunes {
		return message
	}
	return string(runes[:maxRunes]) + "…"
}

func aiDownloadAction(basePath string) string {
	return basePath + "/download?id={id}&source={source}&name={name}&artist={artist}&stream=1&extra={extra_json}"
}

func aiLLMSText(basePath string) string {
	return fmt.Sprintf(`# go-music-dl AI API

Use this compact JSON API instead of scraping rendered HTML.

Start:
- GET %[1]s/api/ai/index
- GET %[1]s/api/ai/search?q=周杰伦&type=song&sources=qq,netease&limit=5

Discover:
- GET %[1]s/api/ai/toplists?sources=netease,qq&limit=20
- GET %[1]s/api/ai/playlist-categories?sources=netease,qq&limit=50
- GET %[1]s/api/ai/category-playlists?source=netease&category_id=...&limit=50
- GET %[1]s/api/ai/recommend?sources=netease,qq&limit=20
- GET %[1]s/api/ai/user-playlists?sources=netease,qq&page=1&limit=20

Details:
- GET %[1]s/api/ai/playlist?id=...&source=...&offset=0&limit=50
- GET %[1]s/api/ai/album?id=...&source=...&offset=0&limit=50
- GET %[1]s/api/ai/toplist?id=...&source=...&offset=0&limit=50
- Local music: GET %[1]s/api/ai/local-music?offset=0&limit=50

Play and download:
- Playback: GET %[1]s/download?id=...&source=...&name=...&artist=...&stream=1&extra=<urlencoded JSON>
- Playback check: GET %[1]s/inspect?id=...&source=...&extra=<urlencoded JSON>
- Lyrics: GET %[1]s/lyric?id=...&source=...&extra=<urlencoded JSON>
- Queue: POST %[1]s/api/downloads/queue
- Entire container: POST %[1]s/api/downloads/queue/container
- Queue status: GET %[1]s/api/downloads/queue

Rules:
- Search limit is 1-50; discovery/detail limit is 1-200.
- Prefer small limits and only requested sources.
- Preserve the extra object returned with a song; some sources require it for playback.
- Successful sources are returned even if others fail in errors.
- Write calls require X-Requested-With: XMLHttpRequest.
- Never expose cookies or login/config endpoints to an AI client.
`, basePath)
}
