package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/soda"
	"github.com/guohuiyuan/music-lib/utils"
)

func importCollectionFromQuery(c *gin.Context, contentType string, source string, externalID string, fallbackLink string, fallbackTrackCount int) *importCollectionMeta {
	source = strings.TrimSpace(source)
	externalID = strings.TrimSpace(externalID)
	if source == "" || externalID == "" {
		return nil
	}

	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		if contentType == collectionContentAlbum {
			name = "导入专辑"
		} else {
			name = "导入歌单"
		}
	}

	trackCount, _ := strconv.Atoi(strings.TrimSpace(c.Query("track_count")))
	if trackCount <= 0 {
		trackCount = fallbackTrackCount
	}

	link := strings.TrimSpace(c.Query("link"))
	if link == "" {
		link = fallbackLink
	}

	return &importCollectionMeta{
		Enabled:     true,
		Name:        name,
		Description: strings.TrimSpace(c.Query("description")),
		Cover:       strings.TrimSpace(c.Query("cover")),
		Creator:     strings.TrimSpace(c.Query("creator")),
		TrackCount:  trackCount,
		Source:      source,
		ExternalID:  externalID,
		Link:        link,
		ContentType: contentType,
		HoverText:   importCollectionHoverText(contentType),
	}
}

func applyImportCollectionFallback(meta *importCollectionMeta, playlist *model.Playlist, fallbackTrackCount int, fallbackLink string) {
	if meta == nil || playlist == nil {
		return
	}

	if strings.TrimSpace(meta.Name) == "" || meta.Name == "导入歌单" || meta.Name == "导入专辑" {
		if name := strings.TrimSpace(playlist.Name); name != "" {
			meta.Name = name
		}
	}
	if strings.TrimSpace(meta.Description) == "" {
		meta.Description = strings.TrimSpace(playlist.Description)
	}
	if strings.TrimSpace(meta.Cover) == "" {
		meta.Cover = strings.TrimSpace(playlist.Cover)
	}
	if strings.TrimSpace(meta.Creator) == "" {
		meta.Creator = strings.TrimSpace(playlist.Creator)
	}
	if meta.TrackCount <= 0 {
		if playlist.TrackCount > 0 {
			meta.TrackCount = playlist.TrackCount
		} else {
			meta.TrackCount = fallbackTrackCount
		}
	}
	if strings.TrimSpace(meta.Link) == "" {
		if link := strings.TrimSpace(playlist.Link); link != "" {
			meta.Link = link
		} else {
			meta.Link = strings.TrimSpace(fallbackLink)
		}
	}
}

type playlistCategoryPageItem struct {
	ID     string
	Name   string
	Hot    bool
	URL    string
	Source string
}

type playlistCategoryPageGroup struct {
	Name       string
	Categories []playlistCategoryPageItem
}

type playlistCategoryPageSource struct {
	Source string
	Name   string
	Count  int
	Groups []playlistCategoryPageGroup
}

type playlistCategoryCurrent struct {
	Source       string
	SourceName   string
	CategoryID   string
	CategoryName string
}

func playlistCategorySourcesFromQuery(c *gin.Context) []string {
	requested := c.QueryArray("sources")
	if len(requested) == 0 {
		return core.GetPlaylistCategorySourceNames()
	}

	supported := make(map[string]bool)
	for _, source := range core.GetPlaylistCategorySourceNames() {
		supported[source] = true
	}

	seen := make(map[string]bool)
	sources := make([]string, 0, len(requested))
	for _, source := range requested {
		source = strings.TrimSpace(source)
		if source == "" || !supported[source] || seen[source] {
			continue
		}
		seen[source] = true
		sources = append(sources, source)
	}
	if len(sources) == 0 {
		return core.GetPlaylistCategorySourceNames()
	}
	return sources
}

func loadPlaylistCategoryPageSources(sources []string) ([]playlistCategoryPageSource, string) {
	type categoryResult struct {
		source     string
		categories []model.PlaylistCategory
		err        error
	}

	results := make(map[string]categoryResult)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, source := range sources {
		fn := core.GetPlaylistCategoriesFunc(source)
		if fn == nil {
			continue
		}
		wg.Add(1)
		go func(src string) {
			defer wg.Done()
			categories, err := fn()
			mu.Lock()
			results[src] = categoryResult{source: src, categories: categories, err: err}
			mu.Unlock()
		}(source)
	}
	wg.Wait()

	views := make([]playlistCategoryPageSource, 0, len(sources))
	failed := make([]string, 0)
	for _, source := range sources {
		result, ok := results[source]
		if !ok {
			continue
		}
		if result.err != nil || len(result.categories) == 0 {
			failed = append(failed, core.GetSourceDescription(source))
			continue
		}
		views = append(views, buildPlaylistCategoryPageSource(source, result.categories))
	}

	errorMsg := ""
	if len(views) == 0 {
		errorMsg = "没有可展示的歌单分类"
	} else if len(failed) > 0 {
		errorMsg = "部分来源分类加载失败：" + strings.Join(failed, "、")
	}
	return views, errorMsg
}

func buildPlaylistCategoryPageSource(source string, categories []model.PlaylistCategory) playlistCategoryPageSource {
	groupIndex := make(map[string]int)
	groups := make([]playlistCategoryPageGroup, 0)

	for _, category := range categories {
		name := strings.TrimSpace(category.Name)
		if name == "" {
			continue
		}
		groupName := strings.TrimSpace(category.Group)
		if groupName == "" {
			groupName = "其他"
		}
		idx, ok := groupIndex[groupName]
		if !ok {
			idx = len(groups)
			groupIndex[groupName] = idx
			groups = append(groups, playlistCategoryPageGroup{Name: groupName})
		}

		item := playlistCategoryPageItem{
			ID:     strings.TrimSpace(category.ID),
			Name:   name,
			Hot:    category.Hot,
			URL:    playlistCategoryPlaylistsURL(source, category),
			Source: source,
		}
		groups[idx].Categories = append(groups[idx].Categories, item)
	}

	count := 0
	for _, group := range groups {
		count += len(group.Categories)
	}
	return playlistCategoryPageSource{
		Source: source,
		Name:   core.GetSourceDescription(source),
		Count:  count,
		Groups: groups,
	}
}

type playlistSourceTab struct {
	Source    string
	Name      string
	Count     int
	Playlists []model.Playlist
	Error     string
}

type playlistSourceTabsData struct {
	ID       string
	Icon     string
	Title    string
	Subtitle string
	Empty    string
	Tabs     []playlistSourceTab
}

func filterAvailableSources(requested, supported []string) []string {
	allowed := make(map[string]bool, len(supported))
	for _, source := range supported {
		allowed[strings.TrimSpace(source)] = true
	}
	seen := make(map[string]bool)
	result := make([]string, 0, len(requested))
	for _, source := range requested {
		source = strings.TrimSpace(source)
		if source == "" || !allowed[source] || seen[source] {
			continue
		}
		seen[source] = true
		result = append(result, source)
	}
	if len(result) == 0 {
		return append([]string(nil), supported...)
	}
	return result
}

func loadPlaylistSourceTabs(sources []string, fetcher func(string) ([]model.Playlist, error)) ([]playlistSourceTab, string) {
	type tabResult struct {
		source    string
		playlists []model.Playlist
		err       error
	}

	results := make(map[string]tabResult, len(sources))
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, src := range sources {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			pls, err := fetcher(s)
			for i := range pls {
				pls[i].Source = s
			}
			mu.Lock()
			results[s] = tabResult{source: s, playlists: pls, err: err}
			mu.Unlock()
		}(src)
	}
	wg.Wait()

	tabs := make([]playlistSourceTab, 0, len(sources))
	failed := make([]string, 0)
	for _, src := range sources {
		res, ok := results[src]
		if !ok {
			continue
		}
		tab := playlistSourceTab{
			Source:    src,
			Name:      core.GetSourceDescription(src),
			Count:     len(res.playlists),
			Playlists: res.playlists,
		}
		if res.err != nil {
			tab.Error = res.err.Error()
			failed = append(failed, tab.Name)
		}
		tabs = append(tabs, tab)
	}

	errMsg := ""
	if len(failed) > 0 && len(failed) == len(tabs) {
		errMsg = "全部来源加载失败：" + strings.Join(failed, "、")
	} else if len(failed) > 0 {
		errMsg = "部分来源加载失败：" + strings.Join(failed, "、")
	}
	return tabs, errMsg
}

func playlistCategoryPlaylistsURL(source string, category model.PlaylistCategory) string {
	values := url.Values{}
	values.Set("source", source)
	if id := strings.TrimSpace(category.ID); id != "" {
		values.Set("category_id", id)
	}
	if name := strings.TrimSpace(category.Name); name != "" {
		values.Set("category_name", name)
	}
	return RoutePrefix + "/category_playlists?" + values.Encode()
}

type topListPageItem struct {
	ID              string
	Name            string
	Cover           string
	Description     string
	UpdateFrequency string
	TrackCount      int
	PlayCount       string
	Source          string
	URL             string
}

type topListPageSource struct {
	Source   string
	Name     string
	Count    int
	TopLists []topListPageItem
}

type topListCurrent struct {
	Source          string
	SourceName      string
	TopListID       string
	TopListName     string
	UpdateFrequency string
}

func topListSourcesFromQuery(c *gin.Context) []string {
	requested := c.QueryArray("sources")
	if len(requested) == 0 {
		return core.GetTopListSourceNames()
	}

	supported := make(map[string]bool)
	for _, source := range core.GetTopListSourceNames() {
		supported[source] = true
	}

	seen := make(map[string]bool)
	sources := make([]string, 0, len(requested))
	for _, source := range requested {
		source = strings.TrimSpace(source)
		if source == "" || !supported[source] || seen[source] {
			continue
		}
		seen[source] = true
		sources = append(sources, source)
	}
	if len(sources) == 0 {
		return core.GetTopListSourceNames()
	}
	return sources
}

func loadTopListPageSources(sources []string) ([]topListPageSource, string) {
	type topListResult struct {
		source   string
		topLists []model.TopList
		err      error
	}

	results := make(map[string]topListResult)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, source := range sources {
		fn := core.GetTopListsFunc(source)
		if fn == nil {
			continue
		}
		wg.Add(1)
		go func(src string) {
			defer wg.Done()
			topLists, err := fn()
			mu.Lock()
			results[src] = topListResult{source: src, topLists: topLists, err: err}
			mu.Unlock()
		}(source)
	}
	wg.Wait()

	views := make([]topListPageSource, 0, len(sources))
	failed := make([]string, 0)
	for _, source := range sources {
		result, ok := results[source]
		if !ok {
			continue
		}
		if result.err != nil || len(result.topLists) == 0 {
			failed = append(failed, core.GetSourceDescription(source))
			continue
		}
		views = append(views, buildTopListPageSource(source, result.topLists))
	}

	errorMsg := ""
	if len(views) == 0 {
		errorMsg = "没有可展示的排行榜"
	} else if len(failed) > 0 {
		errorMsg = "部分来源排行榜加载失败：" + strings.Join(failed, "、")
	}
	return views, errorMsg
}

func buildTopListPageSource(source string, topLists []model.TopList) topListPageSource {
	items := make([]topListPageItem, 0, len(topLists))
	for _, topList := range topLists {
		name := strings.TrimSpace(topList.Name)
		if name == "" {
			continue
		}

		items = append(items, topListPageItem{
			ID:              strings.TrimSpace(topList.ID),
			Name:            name,
			Cover:           strings.TrimSpace(topList.Cover),
			Description:     strings.TrimSpace(topList.Description),
			UpdateFrequency: strings.TrimSpace(topList.UpdateFrequency),
			TrackCount:      topList.TrackCount,
			PlayCount:       formatTopListPlayCount(topList.PlayCount),
			Source:          source,
			URL:             topListSongsURL(source, topList),
		})
	}

	return topListPageSource{
		Source:   source,
		Name:     core.GetSourceDescription(source),
		Count:    len(items),
		TopLists: items,
	}
}

func topListSongsURL(source string, topList model.TopList) string {
	values := url.Values{}
	values.Set("source", source)
	values.Set("toplist_id", strings.TrimSpace(topList.ID))
	if name := strings.TrimSpace(topList.Name); name != "" {
		values.Set("toplist_name", name)
		values.Set("name", name)
	}
	if freq := strings.TrimSpace(topList.UpdateFrequency); freq != "" {
		values.Set("update_frequency", freq)
	}
	if cover := strings.TrimSpace(topList.Cover); cover != "" {
		values.Set("cover", cover)
	}
	if desc := strings.TrimSpace(topList.Description); desc != "" {
		values.Set("description", desc)
	}
	if topList.TrackCount > 0 {
		values.Set("track_count", strconv.Itoa(topList.TrackCount))
	}
	// 榜单的官方页面地址（网易云/QQ 等），由榜单数据提供，避免用歌单链接凑数。
	if link := strings.TrimSpace(topList.Link); link != "" {
		values.Set("link", link)
	}
	return RoutePrefix + "/toplist_songs?" + values.Encode()
}

func formatTopListPlayCount(count int64) string {
	switch {
	case count >= 100000000:
		return fmt.Sprintf("%.1f亿", float64(count)/100000000)
	case count >= 10000:
		return fmt.Sprintf("%.1f万", float64(count)/10000)
	case count > 0:
		return strconv.FormatInt(count, 10)
	default:
		return ""
	}
}

func inspectRetryAfterDelay(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	value := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if retryAt, err := http.ParseTime(value); err == nil {
		delay := time.Until(retryAt)
		if delay > 0 {
			return delay
		}
	}
	return 0
}

func inspectFailurePayload(retryable bool, reason string, retryAfter time.Duration) gin.H {
	payload := gin.H{
		"valid":     false,
		"retryable": retryable,
		"reason":    reason,
		"status":    "invalid",
	}
	if retryable {
		payload["status"] = "retryable"
	}
	if retryAfter > 0 {
		payload["retry_after_ms"] = int64(retryAfter / time.Millisecond)
	}
	return payload
}

func inspectSuccessPayload(source, urlStr, size, bitrate string) gin.H {
	previewOnly := strings.EqualFold(strings.TrimSpace(source), "spotify")
	if previewOnly {
		bitrate = "-"
	}
	return gin.H{
		"valid":        true,
		"status":       "valid",
		"url":          urlStr,
		"size":         size,
		"bitrate":      bitrate,
		"preview_only": previewOnly,
	}
}

func defaultInspectRetryDelay(statusCode int) time.Duration {
	switch statusCode {
	case http.StatusTooManyRequests:
		return 3 * time.Second
	case http.StatusUnauthorized, http.StatusForbidden:
		return 5 * time.Second
	case http.StatusRequestTimeout, http.StatusTooEarly:
		return 2 * time.Second
	}
	if statusCode >= 500 {
		return 2 * time.Second
	}
	return time.Second
}

func inspectHTTPFailure(statusCode int, retryAfter time.Duration) gin.H {
	if statusCode == http.StatusNotFound || statusCode == http.StatusGone {
		return inspectFailurePayload(false, "not_found", 0)
	}
	reason := "temporary_failure"
	switch {
	case statusCode == http.StatusTooManyRequests:
		reason = "rate_limited"
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		reason = "access_denied"
	case statusCode >= 500:
		reason = "upstream_unavailable"
	}
	if retryAfter <= 0 {
		retryAfter = defaultInspectRetryDelay(statusCode)
	}
	payload := inspectFailurePayload(true, reason, retryAfter)
	payload["http_status"] = statusCode
	return payload
}

func isDefinitiveInspectResolutionError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"not found",
		"does not exist",
		"no such song",
		"no playable",
		"no valid download url",
		"track unavailable",
		"song unavailable",
		"song info unavailable",
		"does not provide a preview url",
		"requires cookie",
		"vip required",
		"不存在",
		"无版权",
		"没有版权",
		"无法播放",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func inspectResolutionFailure(err error) gin.H {
	if isDefinitiveInspectResolutionError(err) {
		return inspectFailurePayload(false, "not_found", 0)
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "429") ||
		strings.Contains(message, "too many requests") ||
		strings.Contains(message, "rate limit") {
		return inspectFailurePayload(true, "rate_limited", 3*time.Second)
	}
	if strings.Contains(message, "timeout") ||
		strings.Contains(message, "deadline exceeded") {
		return inspectFailurePayload(true, "upstream_timeout", 2*time.Second)
	}
	return inspectFailurePayload(true, "resolution_failed", 1500*time.Millisecond)
}

func RegisterMusicRoutes(api, configAPI *gin.RouterGroup) {

	api.GET("/", func(c *gin.Context) {
		renderIndex(c, nil, nil, "", nil, "", "song", "", "", "", false, "", nil)
	})

	api.GET("/recommend", func(c *gin.Context) {
		sources := filterAvailableSources(c.QueryArray("sources"), core.GetRecommendSourceNames())
		tabs, errMsg := loadPlaylistSourceTabs(sources, func(src string) ([]model.Playlist, error) {
			fn := core.GetRecommendFunc(src)
			if fn == nil {
				return nil, fmt.Errorf("该源不支持推荐歌单")
			}
			return fn()
		})
		c.Set("PlaylistSourceTabs", playlistSourceTabsData{
			ID:       "recommend",
			Icon:     "fa-fire",
			Title:    "每日推荐歌单",
			Subtitle: "按渠道查看各平台为你推荐的歌单。",
			Empty:    "暂无推荐歌单。",
			Tabs:     tabs,
		})
		renderIndex(c, nil, nil, "每日推荐", sources, errMsg, "playlist", "", "", "", false, "", nil)
	})

	api.GET("/user_playlists", func(c *gin.Context) {
		sources := filterAvailableSources(c.QueryArray("sources"), core.GetUserPlaylistSourceNames())
		tabs, errMsg := loadPlaylistSourceTabs(sources, func(src string) ([]model.Playlist, error) {
			fn := core.GetUserPlaylistsFunc(src)
			if fn == nil {
				return nil, fmt.Errorf("该源不支持个人歌单")
			}
			return fn(1, 50)
		})
		c.Set("PlaylistSourceTabs", playlistSourceTabsData{
			ID:       "user-playlists",
			Icon:     "fa-heart",
			Title:    "我收藏的歌单",
			Subtitle: "查看已登录平台中你创建和收藏的歌单，未登录的渠道请先在设置中扫码登录。",
			Empty:    "该渠道暂无个人歌单，或未登录。",
			Tabs:     tabs,
		})
		renderIndex(c, nil, nil, "我的歌单", sources, errMsg, "playlist", "", "", "", false, "", nil)
	})

	api.GET("/playlist_categories", func(c *gin.Context) {
		sources := playlistCategorySourcesFromQuery(c)
		categorySources, errMsg := loadPlaylistCategoryPageSources(sources)
		c.Set("PlaylistCategorySources", categorySources)
		renderIndex(c, nil, nil, "", sources, errMsg, "playlist", "", "", "", false, "", nil)
	})

	api.GET("/category_playlists", func(c *gin.Context) {
		source := strings.TrimSpace(c.Query("source"))
		categoryID := strings.TrimSpace(c.Query("category_id"))
		categoryName := strings.TrimSpace(c.Query("category_name"))
		if categoryName == "" {
			categoryName = categoryID
		}
		if categoryName == "" {
			categoryName = "全部"
		}

		fn := core.GetCategoryPlaylistsFunc(source)
		if source == "" || fn == nil {
			renderIndex(c, nil, nil, "", nil, "该源不支持歌单分类", "playlist", "", "", "", false, "", nil)
			return
		}

		playlists, err := fn(categoryID, 1, 120)
		for i := range playlists {
			playlists[i].Source = source
		}

		errMsg := ""
		if err != nil {
			errMsg = fmt.Sprintf("获取分类歌单失败: %v", err)
		}

		sourceName := core.GetSourceDescription(source)
		c.Set("PlaylistCategoryCurrent", playlistCategoryCurrent{
			Source:       source,
			SourceName:   sourceName,
			CategoryID:   categoryID,
			CategoryName: categoryName,
		})
		renderIndex(c, nil, playlists, sourceName+" · "+categoryName, []string{source}, errMsg, "playlist", "", "", "", false, "", nil)
	})

	api.GET("/toplists", func(c *gin.Context) {
		sources := topListSourcesFromQuery(c)
		topListSources, errMsg := loadTopListPageSources(sources)
		c.Set("TopListSources", topListSources)
		renderIndex(c, nil, nil, "", sources, errMsg, "playlist", "", "", "", false, "", nil)
	})

	api.GET("/toplist_songs", func(c *gin.Context) {
		source := strings.TrimSpace(c.Query("source"))
		topListID := strings.TrimSpace(c.Query("toplist_id"))
		topListName := strings.TrimSpace(c.Query("toplist_name"))
		if topListName == "" {
			topListName = topListID
		}
		if source == "" || topListID == "" {
			renderIndex(c, nil, nil, "", nil, "缺少参数", "playlist", "", "", "", false, "", nil)
			return
		}

		fn := core.GetTopListSongsFunc(source)
		if fn == nil {
			renderIndex(c, nil, nil, "", nil, "该源不支持排行榜", "playlist", "", "", "", false, "", nil)
			return
		}

		songs, err := fn(topListID)
		errMsg := ""
		if err != nil {
			errMsg = fmt.Sprintf("获取榜单歌曲失败: %v", err)
		}

		topListLink := core.GetOriginalLink(source, topListID, "playlist")
		if link := strings.TrimSpace(c.Query("link")); link != "" {
			topListLink = link
		}
		importCollection := importCollectionFromQuery(c, collectionContentPlaylist, source, topListID, topListLink, len(songs))

		c.Set("TopListCurrent", topListCurrent{
			Source:          source,
			SourceName:      core.GetSourceDescription(source),
			TopListID:       topListID,
			TopListName:     topListName,
			UpdateFrequency: strings.TrimSpace(c.Query("update_frequency")),
		})
		renderIndex(c, songs, nil, "", []string{source}, errMsg, "playlist", topListLink, "", "", false, "", importCollection)
	})

	api.GET("/search", func(c *gin.Context) {
		keyword := strings.TrimSpace(c.Query("q"))
		searchType := c.DefaultQuery("type", "song")
		exactArtist := strings.TrimSpace(c.Query("exact_artist"))
		sources := c.QueryArray("sources")
		var importCollection *importCollectionMeta

		if len(sources) == 0 {
			sources = defaultSourcesForSearchType(searchType)
		}

		var allSongs []model.Song
		var allPlaylists []model.Playlist
		var errorMsg string

		if strings.HasPrefix(keyword, "http") {
			src := core.DetectSource(keyword)
			if src == "" {
				errorMsg = "不支持该链接的解析，或无法识别来源"
			} else {
				parsed := false
				parseFn := core.GetParseFunc(src)
				if parseFn != nil {
					if song, err := parseFn(keyword); err == nil {
						allSongs = append(allSongs, *song)
						searchType = "song"
						parsed = true
					}
				}
				if !parsed {
					parsePlaylistFn := core.GetParsePlaylistFunc(src)
					if parsePlaylistFn != nil {
						if playlist, songs, err := parsePlaylistFn(keyword); err == nil {
							if searchType == "playlist" {
								allPlaylists = append(allPlaylists, *playlist)
							} else {
								allSongs = append(allSongs, songs...)
								searchType = "song"
								if playlist != nil {
									playlistLink := strings.TrimSpace(playlist.Link)
									importCollection = importCollectionFromQuery(c, collectionContentPlaylist, src, playlist.ID, playlistLink, len(songs))
									applyImportCollectionFallback(importCollection, playlist, len(songs), keyword)
								}
							}
							parsed = true
						}
					}
				}
				if !parsed {
					parseAlbumFn := core.GetParseAlbumFunc(src)
					if parseAlbumFn != nil {
						if album, songs, err := parseAlbumFn(keyword); err == nil {
							if searchType == "album" {
								allPlaylists = append(allPlaylists, *album)
							} else {
								allSongs = append(allSongs, songs...)
								searchType = "song"
								if album != nil {
									albumLink := strings.TrimSpace(album.Link)
									importCollection = importCollectionFromQuery(c, collectionContentAlbum, src, album.ID, albumLink, len(songs))
									applyImportCollectionFallback(importCollection, album, len(songs), keyword)
								}
							}
							parsed = true
						}
					}
				}
				if !parsed {
					errorMsg = fmt.Sprintf("解析失败: 暂不支持 %s 平台的此链接类型或解析出错", src)
				}
			}
		} else {
			var wg sync.WaitGroup
			var mu sync.Mutex
			sourceErrors := make(map[string]string)

			for _, src := range sources {
				if isLocalMusicSource(src) {
					continue
				}
				wg.Add(1)
				go func(s string) {
					defer wg.Done()
					if searchType == "playlist" {
						fn := core.GetPlaylistSearchFunc(s)
						if fn != nil {
							res, err := fn(keyword)
							if err == nil {
								for i := range res {
									res[i].Source = s
								}
								mu.Lock()
								allPlaylists = append(allPlaylists, res...)
								mu.Unlock()
							} else {
								mu.Lock()
								sourceErrors[s] = err.Error()
								mu.Unlock()
							}
						} else {
							mu.Lock()
							sourceErrors[s] = "该来源不支持歌单搜索"
							mu.Unlock()
						}
					} else if searchType == "album" {
						fn := core.GetAlbumSearchFunc(s)
						if fn != nil {
							res, err := fn(keyword)
							if err == nil {
								for i := range res {
									res[i].Source = s
								}
								mu.Lock()
								allPlaylists = append(allPlaylists, res...)
								mu.Unlock()
							} else {
								mu.Lock()
								sourceErrors[s] = err.Error()
								mu.Unlock()
							}
						} else {
							mu.Lock()
							sourceErrors[s] = "该来源不支持专辑搜索"
							mu.Unlock()
						}
					} else {
						fn := core.GetSearchFunc(s)
						if fn != nil {
							res, err := fn(keyword)
							if err == nil {
								for i := range res {
									res[i].Source = s
								}
								mu.Lock()
								allSongs = append(allSongs, res...)
								mu.Unlock()
							} else {
								mu.Lock()
								sourceErrors[s] = err.Error()
								mu.Unlock()
							}
						} else {
							mu.Lock()
							sourceErrors[s] = "该来源不支持歌曲搜索"
							mu.Unlock()
						}
					}
				}(src)
			}
			wg.Wait()
			errorMsg = formatSearchSourceErrors(sources, sourceErrors)

			if searchType == "song" && containsLocalSource(sources) {
				localSongs := localMusicSearchSongs(keyword, 200)
				if len(localSongs) > 0 {
					mu.Lock()
					allSongs = append(allSongs, localSongs...)
					mu.Unlock()
				}
			}
			if searchType == "playlist" && containsLocalSource(sources) {
				localPlaylists := localCollectionSearchPlaylists(keyword)
				if len(localPlaylists) > 0 {
					mu.Lock()
					allPlaylists = append(allPlaylists, localPlaylists...)
					mu.Unlock()
				}
			}
		}

		if searchType == "song" && exactArtist != "" && len(allSongs) > 0 {
			allSongs = filterSongsByExactArtist(allSongs, exactArtist)
		}
		renderIndex(c, allSongs, allPlaylists, keyword, sources, errorMsg, searchType, "", "", "", false, "", importCollection)
	})

	api.GET("/playlist", func(c *gin.Context) {
		id := c.Query("id")
		src := c.Query("source")
		if id == "" || src == "" {
			renderIndex(c, nil, nil, "", nil, "缺少参数", "song", "", "", "", false, "", nil)
			return
		}
		fn := core.GetPlaylistDetailFunc(src)
		if fn == nil {
			renderIndex(c, nil, nil, "", nil, "该源不支持查看歌单详情", "song", "", "", "", false, "", nil)
			return
		}
		songs, err := fn(id)
		errMsg := ""
		if err != nil {
			errMsg = fmt.Sprintf("获取歌单失败: %v", err)
		}
		playlistLink := core.GetOriginalLink(src, id, "playlist")
		if link := strings.TrimSpace(c.Query("link")); link != "" {
			playlistLink = link
		}
		importCollection := importCollectionFromQuery(c, collectionContentPlaylist, src, id, playlistLink, len(songs))
		renderIndex(c, songs, nil, "", []string{src}, errMsg, "playlist", playlistLink, "", "", false, "", importCollection)
	})

	api.GET("/album", func(c *gin.Context) {
		id := c.Query("id")
		src := c.Query("source")
		if id == "" || src == "" {
			renderIndex(c, nil, nil, "", nil, "缺少参数", "album", "", "", "", false, "", nil)
			return
		}
		fn := core.GetAlbumDetailFunc(src)
		if fn == nil {
			renderIndex(c, nil, nil, "", nil, "该源不支持查看专辑详情", "album", "", "", "", false, "", nil)
			return
		}
		songs, err := fn(id)
		errMsg := ""
		if err != nil {
			errMsg = fmt.Sprintf("获取专辑失败: %v", err)
		}
		albumLink := core.GetOriginalLink(src, id, "album")
		importCollection := importCollectionFromQuery(c, collectionContentAlbum, src, id, albumLink, len(songs))
		renderIndex(c, songs, nil, "", []string{src}, errMsg, "album", albumLink, "", "", false, "", importCollection)
	})

	api.GET("/album_jump", func(c *gin.Context) {
		name := strings.TrimSpace(c.Query("name"))
		artist := strings.TrimSpace(c.Query("artist"))
		src := strings.TrimSpace(c.Query("source"))
		if name == "" || src == "" {
			renderIndex(c, nil, nil, "", nil, "缺少参数", "album", "", "", "", false, "", nil)
			return
		}

		fn := core.GetAlbumSearchFunc(src)
		if fn == nil {
			renderIndex(c, nil, nil, name, []string{src}, "该源不支持查看专辑详情", "album", "", "", "", false, "", nil)
			return
		}

		albums, err := fn(name)
		if err != nil {
			renderIndex(c, nil, nil, name, []string{src}, fmt.Sprintf("获取专辑失败: %v", err), "album", "", "", "", false, "", nil)
			return
		}
		if len(albums) == 0 {
			renderIndex(c, nil, nil, name, []string{src}, "未找到匹配的专辑", "album", "", "", "", false, "", nil)
			return
		}

		for i := range albums {
			albums[i].Source = src
		}

		selected := pickBestAlbumMatch(name, artist, albums)
		if selected == nil || strings.TrimSpace(selected.ID) == "" {
			renderIndex(c, nil, nil, name, []string{src}, "未找到可跳转的专辑详情", "album", "", "", "", false, "", nil)
			return
		}

		target := fmt.Sprintf("%s/album?id=%s&source=%s", RoutePrefix, url.QueryEscape(selected.ID), url.QueryEscape(src))
		c.Redirect(http.StatusFound, target)
	})

	api.GET("/inspect", func(c *gin.Context) {
		id := c.Query("id")
		src := c.Query("source")
		durStr := c.Query("duration")
		extra := parseSongExtraQuery(c.Query("extra"))

		if isLocalMusicSource(src) {
			payload, _ := inspectLocalMusicFile(id, durStr)
			c.JSON(200, payload)
			return
		}

		var urlStr string
		var err error

		if src == "soda" {
			cookie := core.CM.Get("soda")
			sodaInst := core.NewSodaClient(cookie)
			info, sErr := sodaInst.GetDownloadInfo(&model.Song{ID: id, Source: src})
			if sErr != nil {
				c.JSON(200, inspectResolutionFailure(sErr))
				return
			}
			urlStr = info.URL
		} else {
			fn := core.GetDownloadFunc(src)
			if fn == nil {
				c.JSON(200, inspectFailurePayload(false, "source_unsupported", 0))
				return
			}
			urlStr, err = fn(&model.Song{ID: id, Source: src, Extra: extra})
			if err != nil {
				c.JSON(200, inspectResolutionFailure(err))
				return
			}
			if urlStr == "" {
				c.JSON(200, inspectFailurePayload(false, "empty_download_url", 0))
				return
			}
		}

		req, reqErr := core.BuildSourceRequest("GET", urlStr, src, "bytes=0-1")
		if reqErr != nil {
			c.JSON(200, inspectFailurePayload(true, "media_request_failed", time.Second))
			return
		}

		client := &http.Client{Timeout: 4 * time.Second}
		resp, err := client.Do(req)

		valid := false
		var size int64 = 0

		if err != nil {
			c.JSON(200, inspectFailurePayload(true, "media_unreachable", 2*time.Second))
			return
		}
		if resp == nil {
			c.JSON(200, inspectFailurePayload(true, "media_unreachable", 2*time.Second))
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()
		if resp.StatusCode == 200 || resp.StatusCode == 206 {
			valid = true
			cr := resp.Header.Get("Content-Range")
			if parts := strings.Split(cr, "/"); len(parts) == 2 {
				size, _ = strconv.ParseInt(parts[1], 10, 64)
			} else {
				size = resp.ContentLength
			}
		} else {
			c.JSON(200, inspectHTTPFailure(resp.StatusCode, inspectRetryAfterDelay(resp)))
			return
		}

		bitrate := "-"
		if valid && size > 0 {
			dur, _ := strconv.Atoi(durStr)
			if dur > 0 {
				kbps := int((size * 8) / int64(dur) / 1000)
				bitrate = fmt.Sprintf("%d kbps", kbps)
			}
		}

		c.JSON(200, inspectSuccessPayload(src, urlStr, core.FormatSize(size), bitrate))
	})

	api.GET("/switch_source", func(c *gin.Context) {
		name := strings.TrimSpace(c.Query("name"))
		artist := strings.TrimSpace(c.Query("artist"))
		current := strings.TrimSpace(c.Query("source"))
		target := strings.TrimSpace(c.Query("target"))
		durationStr := strings.TrimSpace(c.Query("duration"))

		origDuration, _ := strconv.Atoi(durationStr)

		if name == "" {
			c.JSON(400, gin.H{"error": "missing name"})
			return
		}

		selected, selectedScore, err := findBestSwitchSong(name, artist, current, target, origDuration)
		if err != nil {
			c.JSON(404, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"id":       selected.ID,
			"name":     selected.Name,
			"artist":   selected.Artist,
			"album":    selected.Album,
			"album_id": selected.AlbumID,
			"duration": selected.Duration,
			"source":   selected.Source,
			"cover":    selected.Cover,
			"extra":    selected.Extra,
			"score":    selectedScore,
			"link":     selected.Link,
		})
	})

	downloadHandler := func(c *gin.Context) {
		id := c.Query("id")
		source := c.Query("source")
		name := c.Query("name")
		artist := c.Query("artist")
		album := strings.TrimSpace(c.Query("album"))
		coverURL := strings.TrimSpace(c.Query("cover"))
		streamPlayback := c.Query("stream") == "1"
		noRangeRequest := strings.TrimSpace(c.GetHeader("Range")) == ""
		embedMeta := !streamPlayback && c.Query("embed") == "1" && noRangeRequest
		saveLocal := !streamPlayback && noRangeRequest && wantsSaveLocal(c)
		if wantsSaveLocal(c) && !allowSaveLocalRequest(c) {
			return
		}
		extra := parseSongExtraQuery(c.Query("extra"))
		if album == "" && extra != nil {
			album = strings.TrimSpace(extra["album"])
		}

		if id == "" || source == "" {
			c.String(400, "Missing params")
			return
		}
		if name == "" {
			name = "Unknown"
		}
		if artist == "" {
			artist = "Unknown"
		}

		if isLocalMusicSource(source) {
			serveLocalMusicDownload(c, id, saveLocal)
			return
		}

		settings := core.GetWebSettings()
		tempSong := &model.Song{ID: id, Source: source, Name: name, Artist: artist, Album: album, Cover: coverURL, Extra: extra}

		if saveLocal {
			// 加载 SQLite 去重集合。
			allSongsSet, _ := core.LoadDownloadDedupSet()
			// 旧记录只存了歌名+歌手、没有文件路径，先用本地曲库索引反查一次：
			// 文件已经被删掉（比如在 NAS 后台直接删）就回收记录，让这次下载正常进行。
			resolveLegacyDedupForSong(allSongsSet, tempSong)

			result, err := core.DownloadWithDedupCheckWithTemplate(tempSong, settings.DownloadDir, embedMeta, embedMeta, settings.DownloadFilenameTemplate, allSongsSet)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
				return
			}
			if !result.Skipped {
				indexSavedLocalMusicFile(result, settings.DownloadDir)
			}

			webdavError := ""
			if !result.Skipped && core.WebDAVConfigured(settings) {
				if err := core.UploadSongToWebDAV(settings, result.Filename, result.Data); err != nil {
					webdavError = err.Error()
				}
			}

			payload := gin.H{
				"status":   "ok",
				"saved":    true,
				"path":     result.SavedPath,
				"filename": result.Filename,
				"skipped":  result.Skipped,
			}
			if result.Warning != "" {
				payload["warning"] = result.Warning
			}
			if result.LyricPath != "" {
				payload["lyric_path"] = result.LyricPath
			}
			if webdavError != "" {
				payload["webdav_error"] = webdavError
			}
			c.JSON(200, payload)
			return
		}

		if embedMeta {
			result, err := core.DownloadSongDataWithTemplate(tempSong, true, true, settings.DownloadFilenameTemplate)
			if err != nil {
				c.String(502, "Upstream stream error")
				return
			}
			warnings := make([]string, 0, 2)
			if result.Warning != "" {
				warnings = append(warnings, result.Warning)
			}
			if core.WebDAVConfigured(settings) {
				if err := core.UploadSongToWebDAV(settings, result.Filename, result.Data); err != nil {
					warnings = append(warnings, err.Error())
				}
			}
			if len(warnings) > 0 {
				c.Header("X-MusicDL-Warning", strings.Join(warnings, "; "))
			}

			setDownloadHeader(c, result.Filename)
			c.Data(200, result.ContentType, result.Data)
			return
		}

		if source == "soda" {
			cookie := core.CM.Get("soda")
			sodaInst := core.NewSodaClient(cookie)
			info, err := sodaInst.GetDownloadInfo(tempSong)
			if err != nil {
				c.String(502, "Soda info error")
				return
			}
			req, reqErr := core.BuildSourceRequest("GET", info.URL, "soda", "")
			if reqErr != nil {
				c.String(502, "Soda request error")
				return
			}
			resp, err := (&http.Client{}).Do(req)
			if err != nil {
				c.String(502, "Soda stream error")
				return
			}
			defer resp.Body.Close()
			encryptedData, _ := io.ReadAll(resp.Body)
			var finalData []byte
			// 汽水 SEO 路径返回明文 m4a（无 play_auth）。
			if strings.TrimSpace(info.PlayAuth) == "" {
				finalData = encryptedData
			} else {
				finalData, err = soda.DecryptAudio(encryptedData, info.PlayAuth)
				if err != nil {
					c.String(500, "Decrypt failed")
					return
				}
			}
			ext := core.DetectAudioExt(finalData)
			filename := core.BuildDownloadFilename(tempSong, ext, settings.DownloadFilenameTemplate)
			if !streamPlayback && core.WebDAVConfigured(settings) {
				if err := core.UploadSongToWebDAV(settings, filename, finalData); err != nil {
					c.Header("X-MusicDL-Warning", err.Error())
				}
			}
			if !streamPlayback {
				setDownloadHeader(c, filename)
			}
			http.ServeContent(c.Writer, c.Request, filename, time.Now(), bytes.NewReader(finalData))
			return
		}

		dlFunc := core.GetDownloadFunc(source)
		if dlFunc == nil {
			c.String(400, "Unknown source")
			return
		}

		downloadUrl, err := dlFunc(tempSong)
		if err != nil {
			c.String(404, "Failed to get URL")
			return
		}

		if rangeFetch, handled, rangeErr := core.NewSourceRangeFetch(downloadUrl, source, c.GetHeader("Range")); rangeErr != nil {
			c.String(502, "Upstream range error")
			return
		} else if handled {
			ext := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(rangeFetch.Ext, ".")))
			if ext == "" {
				ext = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(tempSong.Ext, ".")))
			}
			if ext == "" {
				ext = "mp3"
			}

			filename := core.BuildDownloadFilename(tempSong, ext, settings.DownloadFilenameTemplate)
			if streamPlayback {
				c.Header("Content-Type", core.AudioMimeByExt(ext))
			} else {
				setDownloadHeader(c, filename)
				c.Header("Content-Type", core.AudioMimeByExt(ext))
			}
			c.Header("Accept-Ranges", "bytes")
			c.Header("Content-Length", strconv.FormatInt(rangeFetch.ContentLength, 10))
			if rangeFetch.ContentRange != "" {
				c.Header("Content-Range", rangeFetch.ContentRange)
			}
			c.Status(rangeFetch.StatusCode)
			if err := rangeFetch.WriteRangeTo(c.Writer); err != nil {
				return
			}
			return
		}

		req, reqErr := core.BuildSourceRequest("GET", downloadUrl, source, c.GetHeader("Range"))
		if reqErr != nil {
			c.String(502, "Upstream request error")
			return
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			c.String(502, "Upstream stream error")
			return
		}
		defer resp.Body.Close()

		var bufferedData []byte
		if core.WebDAVConfigured(settings) && !streamPlayback {
			bufferedData, err = io.ReadAll(resp.Body)
			if err != nil {
				c.String(502, "Upstream stream error")
				return
			}
		}

		for k, v := range resp.Header {
			if k != "Transfer-Encoding" && k != "Date" && k != "Access-Control-Allow-Origin" {
				c.Writer.Header()[k] = v
			}
		}

		ext := core.DetectAudioExtByContentType(resp.Header.Get("Content-Type"))
		if ext == "" {
			if parsedURL, parseErr := url.Parse(downloadUrl); parseErr == nil {
				suffix := strings.ToLower(strings.TrimPrefix(path.Ext(parsedURL.Path), "."))
				switch suffix {
				case "mp3", "flac", "ogg", "m4a":
					ext = suffix
				}
			}
		}
		if ext == "" {
			ext = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(tempSong.Ext, ".")))
		}
		if ext == "" {
			ext = "mp3"
		}

		filename := core.BuildDownloadFilename(tempSong, ext, settings.DownloadFilenameTemplate)
		if bufferedData != nil && resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			if err := core.UploadSongToWebDAV(settings, filename, bufferedData); err != nil {
				c.Header("X-MusicDL-Warning", err.Error())
			}
		}
		if streamPlayback {
			contentType := strings.TrimSpace(strings.ToLower(resp.Header.Get("Content-Type")))
			if contentType == "" || strings.HasPrefix(contentType, "application/octet-stream") {
				c.Header("Content-Type", core.AudioMimeByExt(ext))
			}
		} else {
			setDownloadHeader(c, filename)
		}
		c.Status(resp.StatusCode)
		if bufferedData != nil {
			_, _ = c.Writer.Write(bufferedData)
		} else {
			_, _ = io.Copy(c.Writer, resp.Body)
		}
	}
	api.GET("/download", downloadHandler)
	api.POST("/download", downloadHandler)

	downloadLRCHandler := func(c *gin.Context) {
		song := lyricSongFromQuery(c)
		saveLocal := wantsSaveLocal(c)
		if saveLocal && !allowSaveLocalRequest(c) {
			return
		}

		if isLocalMusicSource(song.Source) {
			serveLocalMusicLyric(c, song, true, saveLocal)
			return
		}

		fn := core.GetLyricFunc(song.Source)
		if fn == nil {
			c.String(404, "No support")
			return
		}

		lrc, err := fn(song)
		if err != nil || lrc == "" {
			c.String(404, "Lyric not found")
			return
		}
		if saveLocal {
			standardLRC := core.StandardLRCForExport(lrc)
			if standardLRC == "" {
				c.String(http.StatusNotFound, "Lyric not found")
				return
			}
			filename := lyricDownloadFilename(song)
			c.Header("X-Lyric-Format", classifyLyricFormat(standardLRC))
			saveWebAssetRelativeResponse(c, filename, []byte(standardLRC))
			return
		}
		lrc = formatLyricForMode(lrc, c.DefaultQuery("format", "auto"))
		c.Header("X-Lyric-Format", classifyLyricFormat(lrc))

		filename := lyricDownloadFilename(song)
		setDownloadHeader(c, filename)
		c.String(200, lrc)
	}
	api.GET("/download_lrc", downloadLRCHandler)
	api.POST("/download_lrc", downloadLRCHandler)

	downloadCoverHandler := func(c *gin.Context) {
		u := c.Query("url")
		if u == "" {
			return
		}
		saveLocal := wantsSaveLocal(c)
		if saveLocal && !allowSaveLocalRequest(c) {
			return
		}
		resp, err := utils.Get(u, utils.WithHeader("User-Agent", core.UA_Common))
		if err == nil {
			filename := fmt.Sprintf("%s - %s.jpg", c.Query("name"), c.Query("artist"))
			if saveLocal {
				saveWebAssetResponse(c, filename, resp)
				return
			}
			setDownloadHeader(c, filename)
			c.Data(200, "image/jpeg", resp)
		}
	}
	api.GET("/download_cover", downloadCoverHandler)
	api.POST("/download_cover", downloadCoverHandler)

	api.GET("/cover_proxy", func(c *gin.Context) {
		u := strings.TrimSpace(c.Query("url"))
		if u == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		data, contentType, err := core.FetchBytesWithMime(u, strings.TrimSpace(c.Query("source")))
		if err != nil || len(data) == 0 {
			c.Status(http.StatusBadGateway)
			return
		}
		if contentType == "" {
			contentType = "image/jpeg"
		}

		c.Header("Cache-Control", "public, max-age=21600")
		c.Data(http.StatusOK, contentType, data)
	})

	api.GET("/lyric", func(c *gin.Context) {
		song := lyricSongFromQuery(c)
		if isLocalMusicSource(song.Source) {
			serveLocalMusicLyric(c, song, false)
			return
		}

		fn := core.GetLyricFunc(song.Source)
		if fn != nil {
			lrc, _ := fn(song)
			if lrc != "" {
				lrc = formatLyricForMode(lrc, c.DefaultQuery("format", "auto"))
				c.Header("X-Lyric-Format", classifyLyricFormat(lrc))
				c.String(200, lrc)
				return
			}
		}
		c.String(200, "[00:00.00] 纯音乐 / 无歌词")
	})

	// 下载记录 API
	api.GET("/api/downloads/records", func(c *gin.Context) {
		page, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page", "1")))
		pageSize, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page_size", "20")))
		if page < 1 {
			page = 1
		}
		if pageSize < 1 {
			pageSize = 20
		}
		if pageSize > 100 {
			pageSize = 100
		}

		records, total, err := core.GetDownloadRecordPage(page, pageSize)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		totalPages := 1
		if total > 0 {
			totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
		}
		if page > totalPages {
			page = totalPages
			records, total, err = core.GetDownloadRecordPage(page, pageSize)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		if records == nil {
			records = []core.DownloadRecord{}
		}
		c.JSON(200, gin.H{
			"records":     records,
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": totalPages,
		})
	})

	configAPI.DELETE("/api/downloads/records", func(c *gin.Context) {
		if err := core.ClearDownloadRecords(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 下载预检：统计待下载队列中有多少首将跳过
	api.POST("/api/downloads/precheck", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 20<<20) // 20MB
		var req struct {
			Songs []struct {
				Name   string `json:"name"`
				Artist string `json:"artist"`
			} `json:"songs"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
			return
		}
		if len(req.Songs) > 20000 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "歌曲数量不能超过 20000"})
			return
		}
		for _, s := range req.Songs {
			if len(s.Name) > 500 || len(s.Artist) > 500 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "歌曲名或歌手名长度不能超过 500 字符"})
				return
			}
		}

		dedupSet, err := core.LoadDownloadDedupSet()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		skipCount := 0
		for _, s := range req.Songs {
			song := &model.Song{Name: s.Name, Artist: s.Artist}
			if core.IsSongStillDownloaded(song, dedupSet, core.GetWebSettings().DownloadDir) {
				skipCount++
			}
		}
		c.JSON(200, gin.H{"total": len(req.Songs), "skipped": skipCount})
	})

}

func saveWebAssetResponse(c *gin.Context, filename string, data []byte) {
	savedPath, savedFilename, err := saveWebAssetToLocal(filename, data)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"saved":    true,
		"path":     savedPath,
		"filename": savedFilename,
	})
}

func saveWebAssetRelativeResponse(c *gin.Context, filename string, data []byte) {
	savedPath, savedFilename, err := saveWebAssetRelativeToLocal(filename, data)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, os.ErrExist) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"saved":    true,
		"path":     savedPath,
		"filename": savedFilename,
	})
}

func saveWebAssetToLocal(filename string, data []byte) (string, string, error) {
	if len(data) == 0 {
		return "", "", fmt.Errorf("empty file data")
	}
	settings := core.GetWebSettings()
	targetDir := strings.TrimSpace(settings.DownloadDir)
	if targetDir == "" {
		targetDir = core.DefaultWebDownloadDir
	}
	targetDir = filepath.Clean(targetDir)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", "", err
	}
	savedFilename := utils.SanitizeFilename(strings.TrimSpace(filename))
	if savedFilename == "" {
		savedFilename = "download"
	}
	savedPath := filepath.Join(targetDir, savedFilename)
	if err := os.WriteFile(savedPath, data, 0644); err != nil {
		return "", "", err
	}
	return savedPath, savedFilename, nil
}

func saveWebAssetRelativeToLocal(filename string, data []byte) (string, string, error) {
	if len(data) == 0 {
		return "", "", fmt.Errorf("empty file data")
	}
	settings := core.GetWebSettings()
	targetDir := strings.TrimSpace(settings.DownloadDir)
	if targetDir == "" {
		targetDir = core.DefaultWebDownloadDir
	}
	targetDir = filepath.Clean(targetDir)

	cleanedName := path.Clean(strings.ReplaceAll(strings.TrimSpace(filename), "\\", "/"))
	if cleanedName == "" || cleanedName == "." || strings.HasPrefix(cleanedName, "/") || cleanedName == ".." || strings.HasPrefix(cleanedName, "../") {
		return "", "", fmt.Errorf("invalid relative filename")
	}
	savedFilename := filepath.FromSlash(cleanedName)
	if filepath.VolumeName(savedFilename) != "" {
		return "", "", fmt.Errorf("invalid relative filename")
	}
	savedPath := filepath.Join(targetDir, savedFilename)
	if !isPathInside(targetDir, savedPath) {
		return "", "", fmt.Errorf("invalid relative filename")
	}
	if err := os.MkdirAll(filepath.Dir(savedPath), 0755); err != nil {
		return "", "", err
	}

	existing, err := os.ReadFile(savedPath)
	if err == nil {
		if bytes.Equal(existing, data) {
			return savedPath, savedFilename, nil
		}
		return "", "", fmt.Errorf("%w: %s", os.ErrExist, savedFilename)
	}
	if !os.IsNotExist(err) {
		return "", "", err
	}

	file, err := os.OpenFile(savedPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", "", err
	}
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(savedPath)
		}
	}()
	if _, err := file.Write(data); err != nil {
		return "", "", err
	}
	if err := file.Sync(); err != nil {
		return "", "", err
	}
	if err := file.Close(); err != nil {
		return "", "", err
	}
	keep = true
	return savedPath, savedFilename, nil
}

func lyricSongFromQuery(c *gin.Context) *model.Song {
	duration, _ := strconv.Atoi(strings.TrimSpace(c.Query("duration")))
	return &model.Song{
		ID:       strings.TrimSpace(c.Query("id")),
		Source:   strings.TrimSpace(c.Query("source")),
		Name:     strings.TrimSpace(c.Query("name")),
		Artist:   strings.TrimSpace(c.Query("artist")),
		Album:    strings.TrimSpace(c.Query("album")),
		Duration: duration,
		Extra:    parseSongExtraQuery(c.Query("extra")),
	}
}

func lyricDownloadFilename(song *model.Song) string {
	settings := core.GetWebSettings()
	return core.BuildDownloadFilename(song, "lrc", settings.DownloadFilenameTemplate)
}

type switchCandidate struct {
	song    model.Song
	score   float64
	durDiff int
}

type switchSearchResult struct {
	source     string
	candidates []switchCandidate
}

var (
	switchSearchFuncProvider = func(source string) func(string) ([]model.Song, error) {
		return core.GetSearchFunc(source)
	}
	switchValidatePlayable   = core.ValidatePlayable
	switchAllSourceNames     = core.GetAllSourceNames
	switchDefaultSourceNames = core.GetDefaultSourceNames
)

const (
	switchMaxCandidatesPerSource     = 8
	switchSourceSearchTimeout        = 6 * time.Second
	switchHighConfidenceScore        = 0.98
	switchParallelValidationLimit    = 12
	switchParallelValidationParallel = 6
)

func findBestSwitchSong(name string, artist string, current string, target string, origDuration int) (*model.Song, float64, error) {
	name = strings.TrimSpace(name)
	artist = strings.TrimSpace(artist)
	current = strings.TrimSpace(current)
	target = strings.TrimSpace(target)

	if name == "" {
		return nil, 0, fmt.Errorf("missing name")
	}

	keyword := name
	if artist != "" {
		keyword = name + " " + artist
	}

	sources := switchCandidateSources(current, target)
	if len(sources) == 0 {
		return nil, 0, fmt.Errorf("no match")
	}

	var wg sync.WaitGroup
	results := make(chan switchSearchResult, len(sources))
	var candidates []switchCandidate

	for _, src := range sources {
		wg.Add(1)
		go func(s string, f func(string) ([]model.Song, error)) {
			defer wg.Done()
			sourceCandidates := searchSwitchSourceCandidates(s, f, keyword, name, artist, origDuration)
			if len(sourceCandidates) == 0 {
				return
			}
			results <- switchSearchResult{source: s, candidates: sourceCandidates}
		}(src, switchSearchFuncProvider(src))
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		candidates = append(candidates, result.candidates...)
		sortSwitchCandidates(result.candidates)
		if len(result.candidates) == 0 {
			continue
		}

		best := result.candidates[0]
		if isHighConfidenceSwitchCandidate(best, origDuration) && switchValidatePlayable(&best.song) {
			tmp := best.song
			return &tmp, best.score, nil
		}
	}

	if len(candidates) == 0 {
		return nil, 0, fmt.Errorf("no match")
	}

	sortSwitchCandidates(candidates)
	if selected, score, ok := validateSwitchCandidates(candidates); ok {
		return selected, score, nil
	}

	return nil, 0, fmt.Errorf("no playable match")
}

func switchCandidateSources(current string, target string) []string {
	current = strings.TrimSpace(current)
	target = strings.TrimSpace(target)
	if target != "" {
		if isSwitchSourceAllowed(target, current) && switchSearchFuncProvider(target) != nil {
			return []string{target}
		}
		return nil
	}

	seen := make(map[string]bool)
	sources := make([]string, 0)
	add := func(source string) {
		source = strings.TrimSpace(source)
		if source == "spotify" {
			return
		}
		if seen[source] || !isSwitchSourceAllowed(source, current) || switchSearchFuncProvider(source) == nil {
			return
		}
		seen[source] = true
		sources = append(sources, source)
	}

	for _, source := range switchDefaultSourceNames() {
		add(source)
	}
	for _, source := range switchAllSourceNames() {
		add(source)
	}
	return sources
}

func isSwitchSourceAllowed(source string, current string) bool {
	if source == "" || source == current {
		return false
	}
	if source == "soda" || source == "fivesing" {
		return false
	}
	if isLocalMusicSource(source) {
		return false
	}
	return true
}

func searchSwitchSourceCandidates(source string, fn func(string) ([]model.Song, error), keyword string, name string, artist string, origDuration int) []switchCandidate {
	type searchResponse struct {
		songs []model.Song
		err   error
	}

	callSearch := func(query string) ([]model.Song, error) {
		done := make(chan searchResponse, 1)
		go func() {
			res, err := fn(query)
			done <- searchResponse{songs: res, err: err}
		}()
		select {
		case res := <-done:
			return res.songs, res.err
		case <-time.After(switchSourceSearchTimeout):
			return nil, fmt.Errorf("search timeout")
		}
	}

	res, err := callSearch(keyword)
	if (err != nil || len(res) == 0) && artist != "" {
		res, _ = callSearch(name)
	}
	if len(res) == 0 {
		return nil
	}

	limit := len(res)
	if limit > switchMaxCandidatesPerSource {
		limit = switchMaxCandidatesPerSource
	}

	candidates := make([]switchCandidate, 0, limit)
	for i := 0; i < limit; i++ {
		cand := res[i]
		cand.Source = source
		score := core.CalcSongSimilarity(name, artist, cand.Name, cand.Artist)
		if score <= 0 {
			continue
		}

		durDiff := 0
		if origDuration > 0 && cand.Duration > 0 {
			durDiff = core.IntAbs(origDuration - cand.Duration)
			if !core.IsDurationClose(origDuration, cand.Duration) {
				continue
			}
		}

		candidates = append(candidates, switchCandidate{song: cand, score: score, durDiff: durDiff})
	}

	return candidates
}

func sortSwitchCandidates(candidates []switchCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].durDiff < candidates[j].durDiff
		}
		return candidates[i].score > candidates[j].score
	})
}

func isHighConfidenceSwitchCandidate(candidate switchCandidate, origDuration int) bool {
	if candidate.score < switchHighConfidenceScore {
		return false
	}
	if origDuration > 0 && candidate.song.Duration > 0 && candidate.durDiff > 3 {
		return false
	}
	return true
}

func validateSwitchCandidates(candidates []switchCandidate) (*model.Song, float64, bool) {
	limit := len(candidates)
	if limit > switchParallelValidationLimit {
		limit = switchParallelValidationLimit
	}
	candidates = candidates[:limit]

	type validationResult struct {
		index int
		valid bool
	}

	parallel := switchParallelValidationParallel
	if parallel > len(candidates) {
		parallel = len(candidates)
	}
	if parallel < 1 {
		parallel = 1
	}

	jobs := make(chan int, len(candidates))
	results := make(chan validationResult, len(candidates))
	var wg sync.WaitGroup
	for worker := 0; worker < parallel; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				results <- validationResult{index: index, valid: switchValidatePlayable(&candidates[index].song)}
			}
		}()
	}

	for index := range candidates {
		jobs <- index
	}
	close(jobs)
	wg.Wait()
	close(results)

	valid := make([]bool, len(candidates))
	for result := range results {
		valid[result.index] = result.valid
	}
	for index, ok := range valid {
		if ok {
			tmp := candidates[index].song
			return &tmp, candidates[index].score, true
		}
	}
	return nil, 0, false
}

func parseSongExtraQuery(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil
	}

	extra := make(map[string]string, len(decoded))
	for key, value := range decoded {
		switch v := value.(type) {
		case string:
			extra[key] = v
		case float64:
			extra[key] = strconv.FormatFloat(v, 'f', 0, 64)
		case bool:
			extra[key] = strconv.FormatBool(v)
		default:
			b, err := json.Marshal(v)
			if err == nil {
				extra[key] = string(b)
			}
		}
	}
	if len(extra) == 0 {
		return nil
	}
	return extra
}

func formatSearchSourceErrors(
	sources []string,
	errorsBySource map[string]string,
) string {
	if len(errorsBySource) == 0 {
		return ""
	}
	parts := make([]string, 0, len(errorsBySource))
	seen := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		if isLocalMusicSource(source) {
			continue
		}
		if _, exists := seen[source]; exists {
			continue
		}
		seen[source] = struct{}{}
		message := strings.TrimSpace(errorsBySource[source])
		if message == "" {
			continue
		}
		messageRunes := []rune(message)
		if len(messageRunes) > 220 {
			message = string(messageRunes[:220]) + "…"
		}
		label := core.GetSourceDescription(source)
		parts = append(parts, fmt.Sprintf("%s: %s", label, message))
	}
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return "搜索失败: " + parts[0]
	default:
		return "部分平台搜索失败: " + strings.Join(parts, "；")
	}
}
