package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

//go:embed templates/*
var templateFS embed.FS

// DefaultRoutePrefix is the web UI base path used when no base path is configured.
const DefaultRoutePrefix = "/music"

var RoutePrefix = DefaultRoutePrefix

type importCollectionMeta struct {
	Enabled     bool
	Name        string
	Description string
	Cover       string
	Creator     string
	TrackCount  int
	Source      string
	ExternalID  string
	Link        string
	ContentType string
	HoverText   string
}

func defaultSourcesForSearchType(searchType string) []string {
	switch searchType {
	case "playlist":
		return core.GetPlaylistSourceNames()
	case "album":
		return core.GetAlbumSourceNames()
	default:
		return core.GetDefaultSourceNames()
	}
}

func collectionLabelForSearchType(searchType string) string {
	if searchType == "album" {
		return "专辑"
	}
	return "歌单"
}

func collectionCreatorLabelForSearchType(searchType string) string {
	if searchType == "album" {
		return "歌手"
	}
	return "创建者"
}

func searchPlaceholderForType(searchType string) string {
	switch searchType {
	case "playlist":
		return "搜索歌单、创建者，或直接粘贴歌单链接"
	case "album":
		return "搜索专辑、歌手，或直接粘贴专辑链接"
	default:
		return "搜索歌曲、歌手，或直接粘贴分享链接"
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
		}
		c.Next()
	}
}

func setDownloadHeader(c *gin.Context, filename string) {
	filename = strings.ReplaceAll(strings.TrimSpace(filename), "\\", "/")
	if slash := strings.LastIndex(filename, "/"); slash >= 0 {
		filename = strings.TrimSpace(filename[slash+1:])
	}
	if filename == "" {
		filename = "download"
	}
	encoded := url.PathEscape(filename)
	fallback := asciiDownloadFilenameFallback(filename)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", fallback, encoded))
}

func publicWebSettings() core.WebSettings {
	settings := core.GetWebSettings()
	settings.WebDAVPassword = ""
	// 签名服务 token 等同取流能力，不回显到页面（留空保存时保留旧值）。
	settings.SodaSignerToken = ""
	return settings
}

// webSettingsPayload 在公开设置上追加运行时状态（不落库）：本机签名器是否可用。
type webSettingsPayload struct {
	core.WebSettings
	SodaLocalSignerAvailable bool `json:"sodaLocalSignerAvailable"`
}

func publicWebSettingsPayload() webSettingsPayload {
	return webSettingsPayload{
		WebSettings:              publicWebSettings(),
		SodaLocalSignerAvailable: core.LocalSodaSignerAvailable(),
	}
}

func asciiDownloadFilenameFallback(filename string) string {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return "download"
	}

	base := filename
	ext := ""
	if dot := strings.LastIndex(filename, "."); dot > 0 && dot < len(filename)-1 {
		candidateExt := filename[dot:]
		if candidateExt == asciiDownloadFilenamePart(candidateExt) {
			base = filename[:dot]
			ext = candidateExt
		}
	}

	fallback := strings.Trim(asciiDownloadFilenamePart(base), " .-_")
	if fallback == "" {
		fallback = "download"
	}
	return fallback + ext
}

func asciiDownloadFilenamePart(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.', r == '-', r == '_', r == ' ':
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func playlistExtraValue(playlist model.Playlist, key string) string {
	if playlist.Extra == nil {
		return ""
	}
	return strings.TrimSpace(playlist.Extra[key])
}

func importCollectionHoverText(contentType string) string {
	if contentType == collectionContentAlbum {
		return "导入到本地歌单列表，保存为外部导入专辑；仅保存元数据，不保存具体歌曲明细。"
	}
	return "导入到本地歌单列表，保存为外部导入歌单；仅保存元数据，不保存具体歌曲明细。"
}

func playlistDetailURL(root string, searchType string, playlist model.Playlist) string {
	if strings.TrimSpace(playlist.Source) == "local" {
		return fmt.Sprintf("%s/collection?id=%s", root, url.QueryEscape(playlist.ID))
	}
	if playlistExtraValue(playlist, "external_only") == "true" && strings.TrimSpace(playlist.Link) != "" {
		return playlist.Link
	}

	route := "playlist"
	contentType := collectionContentPlaylist
	if searchType == collectionContentAlbum {
		route = "album"
		contentType = collectionContentAlbum
	}

	values := url.Values{}
	values.Set("id", playlist.ID)
	values.Set("source", playlist.Source)
	if name := strings.TrimSpace(playlist.Name); name != "" {
		values.Set("name", name)
	}
	if description := strings.TrimSpace(playlist.Description); description != "" {
		values.Set("description", description)
	}
	if cover := strings.TrimSpace(playlist.Cover); cover != "" {
		values.Set("cover", cover)
	}
	if creator := strings.TrimSpace(playlist.Creator); creator != "" {
		values.Set("creator", creator)
	}
	if playlist.TrackCount > 0 {
		values.Set("track_count", strconv.Itoa(playlist.TrackCount))
	}
	link := strings.TrimSpace(playlist.Link)
	if link == "" {
		link = core.GetOriginalLink(playlist.Source, playlist.ID, contentType)
	}
	if link != "" {
		values.Set("link", link)
	}

	return fmt.Sprintf("%s/%s?%s", root, route, values.Encode())
}

func isDetailPagePath(requestPath string) bool {
	requestPath = strings.TrimRight(strings.TrimSpace(requestPath), "/")
	return strings.HasSuffix(requestPath, "/playlist") ||
		strings.HasSuffix(requestPath, "/album") ||
		strings.HasSuffix(requestPath, "/collection")
}

type cookieCaptureHint struct {
	LoginURL string
	Domain   string
	Hint     string
}

func cookieCaptureHints() map[string]cookieCaptureHint {
	return map[string]cookieCaptureHint{
		"netease": {
			LoginURL: "https://music.163.com",
			Domain:   "music.163.com",
			Hint:     "登录后打开任意歌曲或歌单页面，筛选同域请求。",
		},
		"qq": {
			LoginURL: "https://y.qq.com",
			Domain:   "u.y.qq.com",
			Hint:     "登录并播放一首歌后筛选该域请求；也可使用右侧 QQ / 微信扫码登录。",
		},
		"kugou": {
			LoginURL: "https://www.kugou.com",
			Domain:   "www.kugou.com",
			Hint:     "登录后打开任意歌单或播放页，筛选 /api/ 请求。",
		},
		"kuwo": {
			LoginURL: "https://www.kuwo.cn",
			Domain:   "www.kuwo.cn",
			Hint:     "登录并播放一首歌，筛选同域请求。",
		},
		"migu": {
			LoginURL: "https://music.migu.cn",
			Domain:   "music.migu.cn",
			Hint:     "登录并播放一首歌，筛选同域的音乐接口请求。",
		},
		"fivesing": {
			LoginURL: "https://5sing.kugou.com",
			Domain:   "5sing.kugou.com",
			Hint:     "登录后打开歌曲或歌单页面，筛选同域请求。",
		},
		"jamendo": {
			LoginURL: "https://www.jamendo.com",
			Domain:   "www.jamendo.com",
			Hint:     "普通试听通常不需要 Cookie；需要账号态时登录后筛选同域请求。",
		},
		"joox": {
			LoginURL: "https://www.joox.com",
			Domain:   "www.joox.com",
			Hint:     "登录并播放一首歌，筛选同域请求。",
		},
		"qianqian": {
			LoginURL: "https://music.taihe.com",
			Domain:   "music.taihe.com",
			Hint:     "登录并播放一首歌，筛选同域请求。",
		},
		"soda": {
			LoginURL: "https://www.qishui.com",
			Domain:   "api.qishui.com",
			Hint:     "登录并播放一首歌，筛选 api.qishui.com/luna/ 请求；扫码登录成功后也会自动保存 Cookie。",
		},
		"bilibili": {
			LoginURL: "https://www.bilibili.com",
			Domain:   "api.bilibili.com",
			Hint:     "登录并播放任意视频或音频，筛选该域请求；Cookie 中通常包含 SESSDATA。",
		},
		"apple": {
			LoginURL: "https://music.apple.com",
			Domain:   "amp-api.music.apple.com",
			Hint:     "登录后播放一首歌并刷新页面，筛选该域请求；Cookie 中至少应包含 media-user-token。",
		},
	}
}

const shortPreviewFilterSeconds = 60

func splitVersionFilterKeywords(raw string) []string {
	seen := make(map[string]struct{})
	keywords := make([]string, 0)
	for _, value := range strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '，', ';', '；', '\n', '\r':
			return true
		default:
			return false
		}
	}) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		keywords = append(keywords, value)
	}
	return keywords
}

func filterSongsForDisplay(songs []model.Song, settings core.WebSettings) []model.Song {
	if len(songs) == 0 {
		return songs
	}
	keywords := splitVersionFilterKeywords(settings.VersionFilterKeywords)
	if !settings.FilterVersionKeywords && !settings.FilterShortTracks {
		return songs
	}

	filtered := make([]model.Song, 0, len(songs))
	for _, song := range songs {
		if settings.FilterShortTracks && song.Duration > 0 && song.Duration < shortPreviewFilterSeconds {
			continue
		}
		if settings.FilterVersionKeywords && len(keywords) > 0 {
			name := strings.ToLower(song.Name)
			blocked := false
			for _, keyword := range keywords {
				if strings.Contains(name, keyword) {
					blocked = true
					break
				}
			}
			if blocked {
				continue
			}
		}
		filtered = append(filtered, song)
	}
	return filtered
}

func renderIndex(c *gin.Context, songs []model.Song, playlists []model.Playlist, q string, selected []string, errMsg string, searchType string, playlistLink string, colID string, colName string, isLocalColPage bool, collectionKind string, importCollection *importCollectionMeta) {
	allSrc := core.GetAllSourceNames()
	desc := make(map[string]string)
	for _, s := range allSrc {
		desc[s] = core.GetSourceDescription(s)
	}

	playlistSupported := make(map[string]bool)
	for _, s := range core.GetPlaylistSourceNames() {
		playlistSupported[s] = true
	}
	// 本地音乐支持歌单搜索（搜本地自建/导入歌单），但不进 GetPlaylistSourceNames
	// 以免成为默认勾选项——这里仅开启复选框在歌单模式下可用。
	playlistSupported["local"] = true
	albumSupported := make(map[string]bool)
	for _, s := range core.GetAlbumSourceNames() {
		albumSupported[s] = true
	}
	playlistCategorySupported := make(map[string]bool)
	for _, s := range core.GetPlaylistCategorySourceNames() {
		playlistCategorySupported[s] = true
	}
	topListSupported := make(map[string]bool)
	for _, s := range core.GetTopListSourceNames() {
		topListSupported[s] = true
	}
	qrLoginSupported := make(map[string]bool)
	for _, s := range core.GetQRLoginSourceNames() {
		qrLoginSupported[s] = true
	}
	userPlaylistSupported := make(map[string]bool)
	for _, s := range core.GetUserPlaylistSourceNames() {
		userPlaylistSupported[s] = true
	}

	playlistCategorySources, _ := c.Get("PlaylistCategorySources")
	playlistCategoryCurrent, _ := c.Get("PlaylistCategoryCurrent")
	playlistSourceTabs, _ := c.Get("PlaylistSourceTabs")
	topListSources, _ := c.Get("TopListSources")
	topListCurrent, _ := c.Get("TopListCurrent")

	settings := core.GetWebSettings()
	songs = filterSongsForDisplay(songs, settings)
	defaultPageSize := settings.WebPageSize
	if defaultPageSize <= 0 {
		defaultPageSize = core.DefaultWebPageSize
	}
	pageSize := defaultPageSize
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			pageSize = n
		}
	}
	if pageSize > 500 {
		pageSize = 500
	}

	page := 1
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			page = n
		}
	}

	totalCount := 0
	if len(songs) > 0 {
		totalCount = len(songs)
	} else if len(playlists) > 0 {
		totalCount = len(playlists)
	}

	totalPages := 1
	pageStart := 0
	pageEnd := totalCount
	if totalCount > 0 {
		totalPages = (totalCount + pageSize - 1) / pageSize
		if page > totalPages {
			page = totalPages
		}
		pageStart = (page - 1) * pageSize
		if pageStart < 0 {
			pageStart = 0
		}
		pageEnd = pageStart + pageSize
		if pageEnd > totalCount {
			pageEnd = totalCount
		}

		if len(songs) > 0 {
			songs = songs[pageStart:pageEnd]
		}
		if len(playlists) > 0 {
			playlists = playlists[pageStart:pageEnd]
		}
	}

	pageStartDisplay := 0
	if totalCount > 0 {
		pageStartDisplay = pageStart + 1
	}
	c.HTML(200, "index.html", gin.H{
		"Result":                  songs,
		"Playlists":               playlists,
		"Page":                    page,
		"PageSize":                pageSize,
		"TotalCount":              totalCount,
		"TotalPages":              totalPages,
		"PageStart":               pageStartDisplay,
		"PageEnd":                 pageEnd,
		"Keyword":                 q,
		"AllSources":              allSrc,
		"DefaultSources":          defaultSourcesForSearchType(searchType),
		"SourceDescriptions":      desc,
		"CookieCaptureHints":      cookieCaptureHints(),
		"Selected":                selected,
		"Error":                   errMsg,
		"SearchType":              searchType,
		"ShowDetailBack":          isDetailPagePath(c.Request.URL.Path),
		"PlaylistSupported":       playlistSupported,
		"AlbumSupported":          albumSupported,
		"CategorySupported":       playlistCategorySupported,
		"TopListSupported":        topListSupported,
		"QRLoginSupported":        qrLoginSupported,
		"SearchPlaceholder":       searchPlaceholderForType(searchType),
		"CollectionLabel":         collectionLabelForSearchType(searchType),
		"CollectionCreator":       collectionCreatorLabelForSearchType(searchType),
		"Root":                    RoutePrefix,
		"PlaylistLink":            playlistLink,
		"ColID":                   colID,
		"ColName":                 colName,
		"CollectionKind":          collectionKind,
		"CollectionContentType":   c.GetString("CollectionContentType"),
		"ImportCollection":        importCollection,
		"CanRemoveSongs":          colID != "" && collectionKind == collectionKindManual,
		"IsLocalColPage":          isLocalColPage,
		"PlaylistCategorySources": playlistCategorySources,
		"PlaylistCategoryCurrent": playlistCategoryCurrent,
		"PlaylistSourceTabs":      playlistSourceTabs,
		"TopListSources":          topListSources,
		"TopListCurrent":          topListCurrent,
		"UserPlaylistSupported":   userPlaylistSupported,
	})
}

func NormalizeBasePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimRight(p, "/")
	if p == "" {
		return DefaultRoutePrefix
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

type StartOptions struct {
	ShouldOpenBrowser bool
	DisableAuth       bool
	ListenHost        string
	BasePath          string
}

func Start(port string, shouldOpenBrowser bool, basePath string) {
	StartWithOptions(port, StartOptions{ShouldOpenBrowser: shouldOpenBrowser, BasePath: basePath})
}

func StartDesktop(port string) {
	StartWithOptions(port, StartOptions{
		DisableAuth: true,
		ListenHost:  "127.0.0.1",
	})
}

func StartWithOptions(port string, opts StartOptions) {
	RoutePrefix = NormalizeBasePath(opts.BasePath)
	core.CM.Load()
	if !opts.DisableAuth {
		settings, err := core.GetWebAuthSettings()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read web auth settings: %v\n", err)
		} else if token, tokenErr := prepareSetupToken(settings); tokenErr == nil && token != "" {
			fmt.Printf("Web setup token: %s\nOpen %s/setup and keep this startup terminal private until setup is complete.\n", token, RoutePrefix)
		}
	}
	InitDB()
	defer CloseDB()
	// Local music is indexed on demand. Scanning the configured download
	// directory at startup is expensive on Android, especially when the user
	// points it at a large shared-storage directory.

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(corsMiddleware(), gzipMiddleware())

	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"artistTokens":       splitArtistTokens,
		"albumID":            songAlbumID,
		"playlistDetailURL":  playlistDetailURL,
		"playlistExtraValue": playlistExtraValue,
		"tojson": func(v interface{}) string {
			if v == nil {
				return ""
			}
			b, err := json.Marshal(v)
			if err != nil {
				return ""
			}
			return string(b)
		},
	}).ParseFS(templateFS,
		"templates/pages/*.html",
		"templates/partials/*.html",
	))
	r.SetHTMLTemplate(tmpl)

	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, RoutePrefix)
	})

	videoDir := "data/video_output"
	os.MkdirAll(videoDir, 0755)

	api := r.Group(RoutePrefix)

	api.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"app":    "go-music-dl",
			"status": "ok",
		})
	})

	// Static assets embedded at build time.
	api.GET("/icon.png", func(c *gin.Context) { c.FileFromFS("templates/static/images/icon.png", http.FS(templateFS)) })
	api.GET("/style.css", func(c *gin.Context) { c.FileFromFS("templates/static/css/style.css", http.FS(templateFS)) })
	api.GET("/videogen.css", func(c *gin.Context) { c.FileFromFS("templates/static/css/videogen.css", http.FS(templateFS)) })
	api.GET("/videogen.js", func(c *gin.Context) { c.FileFromFS("templates/static/js/videogen.js", http.FS(templateFS)) })
	api.GET("/app.js", func(c *gin.Context) { c.FileFromFS("templates/static/js/app.js", http.FS(templateFS)) })
	vendorAssets, err := embeddedVendorFS()
	if err != nil {
		panic(fmt.Sprintf("load embedded vendor assets: %v", err))
	}
	api.StaticFS("/vendor", http.FS(vendorAssets))
	configAPI := bindAuthMiddleware(api, opts)
	api.Static("/videos", videoDir)

	api.GET("/render", func(c *gin.Context) {
		c.HTML(200, "render.html", gin.H{
			"Root": RoutePrefix,
		})
	})

	configAPI.HEAD("/cookies", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	configAPI.GET("/cookies", func(c *gin.Context) { c.JSON(200, core.CM.GetAll()) })
	configAPI.POST("/cookies", func(c *gin.Context) {
		var req map[string]string
		if err := c.ShouldBindJSON(&req); err == nil {
			core.CM.SetAll(req)
			core.CM.Save()
			c.JSON(200, gin.H{"status": "ok"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cookies payload"})
	})

	api.GET("/settings", func(c *gin.Context) {
		c.JSON(200, publicWebSettingsPayload())
	})
	configAPI.POST("/settings", func(c *gin.Context) {
		var req core.WebSettings
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid settings payload"})
			return
		}
		if err := core.SaveWebSettings(req); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, core.ErrInvalidWebSettings) {
				status = http.StatusBadRequest
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, publicWebSettingsPayload())
	})

	RegisterMusicRoutes(api, configAPI)
	RegisterQRLoginRoutes(configAPI)
	RegisterCollectionRoutes(api)
	RegisterLocalMusicRoutes(api)
	RegisterVideogenRoutes(api, videoDir)
	RegisterUpdateRoutes(api)

	queue, err := core.OpenDownloadQueue()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Initialize download queue: %v\n", err)
		return
	}
	RegisterDownloadQueueRoutes(api, queue)

	listenAddr := opts.ListenHost + ":" + port
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "address already in use") {
			fmt.Fprintf(os.Stderr, "Failed to start web server: port %s is already in use. Please use --port to specify another port, e.g. music-dl web --port 8081\n", port)
			return
		}
		fmt.Fprintf(os.Stderr, "Failed to start web server on %s: %v\n", listenAddr, err)
		return
	}

	queueContext, stopQueue := context.WithCancel(context.Background())
	defer stopQueue()
	if err := queue.Start(queueContext, processQueuedDownload); err != nil {
		_ = listener.Close()
		fmt.Fprintf(os.Stderr, "Start download queue: %v\n", err)
		return
	}

	urlHost := opts.ListenHost
	if urlHost == "" || urlHost == "0.0.0.0" || urlHost == "::" {
		urlHost = "localhost"
	}
	urlStr := "http://" + urlHost + ":" + port + RoutePrefix
	fmt.Printf("Web started at %s\n", urlStr)
	if opts.ShouldOpenBrowser {
		go func() { time.Sleep(500 * time.Millisecond); core.OpenBrowser(urlStr) }()
	}
	if err := http.Serve(listener, r); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(os.Stderr, "Web server stopped with error: %v\n", err)
	}
}

func embeddedVendorFS() (fs.FS, error) {
	return fs.Sub(templateFS, "templates/static/vendor")
}

func bindAuthMiddleware(api *gin.RouterGroup, opts StartOptions) *gin.RouterGroup {
	bindAuthRoutes(api)
	if opts.DisableAuth {
		return api
	}
	configAPI := api.Group("")
	configAPI.Use(authRequired(core.GetWebAuthSettings))
	return configAPI
}
