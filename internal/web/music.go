package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
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

func RegisterMusicRoutes(api *gin.RouterGroup) {

	api.GET("/", func(c *gin.Context) {
		renderIndex(c, nil, nil, "", nil, "", "song", "", "", "", false, "", nil)
	})

	api.GET("/recommend", func(c *gin.Context) {
		sources := c.QueryArray("sources")
		if len(sources) == 0 {
			sources = []string{"netease", "qq", "kugou", "kuwo"}
		}

		var allPlaylists []model.Playlist
		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, src := range sources {
			fn := core.GetRecommendFunc(src)
			if fn == nil {
				continue
			}
			wg.Add(1)
			go func(s string) {
				defer wg.Done()
				res, err := fn()
				if err == nil && len(res) > 0 {
					mu.Lock()
					allPlaylists = append(allPlaylists, res...)
					mu.Unlock()
				}
			}(src)
		}
		wg.Wait()

		renderIndex(c, nil, allPlaylists, "每日推荐", sources, "", "playlist", "", "", "", false, "", nil)
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

	api.GET("/download", func(c *gin.Context) {
		id := c.Query("id")
		source := c.Query("source")
		name := c.Query("name")
		artist := c.Query("artist")
		coverURL := strings.TrimSpace(c.Query("cover"))
		noRangeRequest := strings.TrimSpace(c.GetHeader("Range")) == ""
		embedMeta := c.Query("embed") == "1" && noRangeRequest
		saveLocal := c.Query("save_local") == "1" && noRangeRequest
		settings := core.GetWebSettings()
		extra := parseSongExtraQuery(c.Query("extra"))

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

		tempSong := &model.Song{ID: id, Source: source, Name: name, Artist: artist, Cover: coverURL, Extra: extra}
		baseFilename := fmt.Sprintf("%s - %s", name, artist)

		if saveLocal {
			result, err := core.SaveSongToFile(tempSong, settings.DownloadDir, embedMeta, embedMeta)
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
			result, err := core.DownloadSongData(tempSong, true, true)
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
			filename := fmt.Sprintf("%s.%s", baseFilename, ext)
			setDownloadHeader(c, filename)
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
			ext = "mp3"
		}

		filename := fmt.Sprintf("%s.%s", baseFilename, ext)
		setDownloadHeader(c, filename)
		c.Status(resp.StatusCode)
		io.Copy(c.Writer, resp.Body)
	})

	api.GET("/download_lrc", func(c *gin.Context) {
		id := c.Query("id")
		src := c.Query("source")
		name := c.Query("name")
		artist := c.Query("artist")

		fn := core.GetLyricFunc(src)
		if fn == nil {
			c.String(404, "No support")
			return
		}

		lrc, err := fn(&model.Song{ID: id, Source: src})
		if err != nil || lrc == "" {
			c.String(404, "Lyric not found")
			return
		}

		filename := fmt.Sprintf("%s - %s.lrc", name, artist)
		setDownloadHeader(c, filename)
		c.String(200, lrc)
	})

	api.GET("/download_cover", func(c *gin.Context) {
		u := c.Query("url")
		if u == "" {
			return
		}
		resp, err := utils.Get(u, utils.WithHeader("User-Agent", core.UA_Common))
		if err == nil {
			filename := fmt.Sprintf("%s - %s.jpg", c.Query("name"), c.Query("artist"))
			setDownloadHeader(c, filename)
			c.Data(200, "image/jpeg", resp)
		}
	})

	api.GET("/lyric", func(c *gin.Context) {
		id := c.Query("id")
		src := c.Query("source")
		fn := core.GetLyricFunc(src)
		if fn != nil {
			lrc, _ := fn(&model.Song{ID: id, Source: src})
			if lrc != "" {
				c.String(200, lrc)
				return
			}
		}
		c.String(200, "[00:00.00] 閺嗗倹妫ゅ宀冪槤")
	})
}

type switchCandidate struct {
	song    model.Song
	score   float64
	durDiff int
}

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

	var sources []string
	if target != "" {
		sources = []string{target}
	} else {
		sources = core.GetAllSourceNames()
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var candidates []switchCandidate

	for _, src := range sources {
		if src == "" || src == current {
			continue
		}
		if src == "soda" || src == "fivesing" {
			continue
		}
		fn := core.GetSearchFunc(src)
		if fn == nil {
			continue
		}

		wg.Add(1)
		go func(s string, f func(string) ([]model.Song, error)) {
			defer wg.Done()

			res, err := f(keyword)
			if (err != nil || len(res) == 0) && artist != "" {
				res, _ = f(name)
			}
			if len(res) == 0 {
				return
			}

			limit := len(res)
			if limit > 8 {
				limit = 8
			}

			for i := 0; i < limit; i++ {
				cand := res[i]
				cand.Source = s
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

				mu.Lock()
				candidates = append(candidates, switchCandidate{song: cand, score: score, durDiff: durDiff})
				mu.Unlock()
			}
		}(src, fn)
	}

	wg.Wait()
	if len(candidates) == 0 {
		return nil, 0, fmt.Errorf("no match")
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].durDiff < candidates[j].durDiff
		}
		return candidates[i].score > candidates[j].score
	})

	for _, cand := range candidates {
		if core.ValidatePlayable(&cand.song) {
			tmp := cand.song
			return &tmp, cand.score, nil
		}
	}

	return nil, 0, fmt.Errorf("no playable match")
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
