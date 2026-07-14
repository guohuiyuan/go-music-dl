package web

import (
	"bytes"
	"encoding/json"
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

func RegisterMusicRoutes(api *gin.RouterGroup) {

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
							}
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
							}
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
							}
						}
					}
				}(src)
			}
			wg.Wait()

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
			sodaInst := soda.New(cookie)
			info, sErr := sodaInst.GetDownloadInfo(&model.Song{ID: id, Source: src})
			if sErr != nil {
				c.JSON(200, gin.H{"valid": false})
				return
			}
			urlStr = info.URL
		} else {
			fn := core.GetDownloadFunc(src)
			if fn == nil {
				c.JSON(200, gin.H{"valid": false})
				return
			}
			urlStr, err = fn(&model.Song{ID: id, Source: src, Extra: extra})
			if err != nil || urlStr == "" {
				c.JSON(200, gin.H{"valid": false})
				return
			}
		}

		req, reqErr := core.BuildSourceRequest("GET", urlStr, src, "bytes=0-1")
		if reqErr != nil {
			c.JSON(200, gin.H{"valid": false})
			return
		}

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)

		valid := false
		var size int64 = 0

		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == 200 || resp.StatusCode == 206 {
				valid = true
				cr := resp.Header.Get("Content-Range")
				if parts := strings.Split(cr, "/"); len(parts) == 2 {
					size, _ = strconv.ParseInt(parts[1], 10, 64)
				} else {
					size = resp.ContentLength
				}
			}
		}

		bitrate := "-"
		if valid && size > 0 {
			dur, _ := strconv.Atoi(durStr)
			if dur > 0 {
				kbps := int((size * 8) / int64(dur) / 1000)
				bitrate = fmt.Sprintf("%d kbps", kbps)
			}
		}

		c.JSON(200, gin.H{
			"valid":   valid,
			"url":     urlStr,
			"size":    core.FormatSize(size),
			"bitrate": bitrate,
		})
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
			result, err := core.SaveSongToFileWithTemplate(tempSong, settings.DownloadDir, embedMeta, embedMeta, settings.DownloadFilenameTemplate)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
				return
			}

			payload := gin.H{
				"status":   "ok",
				"saved":    true,
				"path":     result.SavedPath,
				"filename": result.Filename,
			}
			if result.Warning != "" {
				payload["warning"] = result.Warning
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
			if result.Warning != "" {
				c.Header("X-MusicDL-Warning", result.Warning)
			}

			setDownloadHeader(c, result.Filename)
			c.Data(200, result.ContentType, result.Data)
			return
		}

		if source == "soda" {
			cookie := core.CM.Get("soda")
			sodaInst := soda.New(cookie)
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
			finalData, err := soda.DecryptAudio(encryptedData, info.PlayAuth)
			if err != nil {
				c.String(500, "Decrypt failed")
				return
			}
			ext := core.DetectAudioExt(finalData)
			filename := core.BuildDownloadFilename(tempSong, ext, settings.DownloadFilenameTemplate)
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
			if err := rangeFetch.WriteTo(c.Writer); err != nil {
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
		if streamPlayback {
			contentType := strings.TrimSpace(strings.ToLower(resp.Header.Get("Content-Type")))
			if contentType == "" || strings.HasPrefix(contentType, "application/octet-stream") {
				c.Header("Content-Type", core.AudioMimeByExt(ext))
			}
		} else {
			setDownloadHeader(c, filename)
		}
		c.Status(resp.StatusCode)
		io.Copy(c.Writer, resp.Body)
	}
	api.GET("/download", downloadHandler)
	api.POST("/download", downloadHandler)

	downloadLRCHandler := func(c *gin.Context) {
		song := lyricSongFromQuery(c)
		name := song.Name
		artist := song.Artist
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
		lrc = formatLyricForMode(lrc, c.DefaultQuery("format", "auto"))
		c.Header("X-Lyric-Format", classifyLyricFormat(lrc))

		filename := fmt.Sprintf("%s - %s.lrc", name, artist)
		if saveLocal {
			saveWebAssetResponse(c, filename, []byte(lrc))
			return
		}
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
