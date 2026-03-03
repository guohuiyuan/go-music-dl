package web

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
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

func encodeSyncSafeInt(v int) [4]byte {
	if v < 0 {
		v = 0
	}
	return [4]byte{
		byte((v >> 21) & 0x7F),
		byte((v >> 14) & 0x7F),
		byte((v >> 7) & 0x7F),
		byte(v & 0x7F),
	}
}

func decodeSyncSafeInt(b []byte) int {
	if len(b) < 4 {
		return 0
	}
	return int(b[0]&0x7F)<<21 | int(b[1]&0x7F)<<14 | int(b[2]&0x7F)<<7 | int(b[3]&0x7F)
}

func stripID3v2Prefix(audioData []byte) []byte {
	if len(audioData) < 10 || string(audioData[:3]) != "ID3" {
		return audioData
	}
	tagSize := decodeSyncSafeInt(audioData[6:10])
	total := 10 + tagSize
	if audioData[5]&0x10 != 0 {
		total += 10
	}
	if total <= 0 || total > len(audioData) {
		return audioData
	}
	return audioData[total:]
}

func buildID3v24Frame(frameID string, payload []byte) []byte {
	if len(frameID) != 4 || len(payload) == 0 {
		return nil
	}
	var frame bytes.Buffer
	frame.WriteString(frameID)
	sz := encodeSyncSafeInt(len(payload))
	frame.Write(sz[:])
	frame.Write([]byte{0x00, 0x00})
	frame.Write(payload)
	return frame.Bytes()
}

func buildTextFrame(frameID string, text string) []byte {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	payload := append([]byte{0x03}, []byte(text)...)
	return buildID3v24Frame(frameID, payload)
}

func normalizeCoverMime(coverMime string) string {
	coverMime = strings.TrimSpace(strings.ToLower(coverMime))
	if coverMime == "" {
		return "image/jpeg"
	}
	if strings.Contains(coverMime, "png") {
		return "image/png"
	}
	if strings.Contains(coverMime, "webp") {
		return "image/webp"
	}
	if strings.Contains(coverMime, "gif") {
		return "image/gif"
	}
	return "image/jpeg"
}

func fetchBytesWithMime(urlStr string, source string) ([]byte, string, error) {
	req, err := core.BuildSourceRequest("GET", urlStr, source, "")
	if err != nil {
		return nil, "", err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" && len(data) > 0 {
		contentType = http.DetectContentType(data)
	}

	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	return data, contentType, nil
}

func embedSongMetadata(audioData []byte, song *model.Song, lyric string, coverData []byte, coverMime string) ([]byte, error) {
	audioData = stripID3v2Prefix(audioData)

	var frames bytes.Buffer

	if frame := buildTextFrame("TIT2", song.Name); len(frame) > 0 {
		frames.Write(frame)
	}
	if frame := buildTextFrame("TPE1", song.Artist); len(frame) > 0 {
		frames.Write(frame)
	}

	lyric = strings.TrimSpace(lyric)
	if lyric != "" {
		payload := make([]byte, 0, 8+len(lyric))
		payload = append(payload, 0x03)
		payload = append(payload, []byte("chi")...)
		payload = append(payload, 0x00)
		payload = append(payload, []byte(lyric)...)
		if frame := buildID3v24Frame("USLT", payload); len(frame) > 0 {
			frames.Write(frame)
		}
	}

	if len(coverData) > 0 {
		mime := normalizeCoverMime(coverMime)
		payload := make([]byte, 0, len(mime)+4+len(coverData))
		payload = append(payload, 0x03)
		payload = append(payload, []byte(mime)...)
		payload = append(payload, 0x00)
		payload = append(payload, 0x03)
		payload = append(payload, 0x00)
		payload = append(payload, coverData...)
		if frame := buildID3v24Frame("APIC", payload); len(frame) > 0 {
			frames.Write(frame)
		}
	}

	if frames.Len() == 0 {
		return audioData, nil
	}

	var tag bytes.Buffer
	tag.WriteString("ID3")
	tag.WriteByte(0x04)
	tag.WriteByte(0x00)
	tag.WriteByte(0x00)
	size := encodeSyncSafeInt(frames.Len())
	tag.Write(size[:])
	tag.Write(frames.Bytes())

	return append(tag.Bytes(), audioData...), nil
}

func RegisterMusicRoutes(api *gin.RouterGroup) {

	api.GET("/", func(c *gin.Context) {
		renderIndex(c, nil, nil, "", nil, "", "song", "", "", "", false)
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

		renderIndex(c, nil, allPlaylists, "🔥 每日推荐", sources, "", "playlist", "", "", "", false)
	})

	api.GET("/search", func(c *gin.Context) {
		keyword := strings.TrimSpace(c.Query("q"))
		searchType := c.DefaultQuery("type", "song")
		sources := c.QueryArray("sources")

		if len(sources) == 0 {
			if searchType == "playlist" {
				sources = core.GetPlaylistSourceNames()
			} else {
				sources = core.GetDefaultSourceNames()
			}
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

		renderIndex(c, allSongs, allPlaylists, keyword, sources, errorMsg, searchType, "", "", "", false)
	})

	api.GET("/playlist", func(c *gin.Context) {
		id := c.Query("id")
		src := c.Query("source")
		if id == "" || src == "" {
			renderIndex(c, nil, nil, "", nil, "缺少参数", "song", "", "", "", false)
			return
		}
		fn := core.GetPlaylistDetailFunc(src)
		if fn == nil {
			renderIndex(c, nil, nil, "", nil, "该源不支持查看歌单详情", "song", "", "", "", false)
			return
		}
		songs, err := fn(id)
		errMsg := ""
		if err != nil {
			errMsg = fmt.Sprintf("获取歌单失败: %v", err)
		}
		playlistLink := core.GetOriginalLink(src, id, "playlist")
		renderIndex(c, songs, nil, "", []string{src}, errMsg, "song", playlistLink, "", "", false)
	})

	api.GET("/inspect", func(c *gin.Context) {
		id := c.Query("id")
		src := c.Query("source")
		durStr := c.Query("duration")

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
			urlStr, err = fn(&model.Song{ID: id, Source: src})
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

		type candidate struct {
			song    model.Song
			score   float64
			durDiff int
		}
		var wg sync.WaitGroup
		var mu sync.Mutex
		var candidates []candidate

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
			go func(s string) {
				defer wg.Done()

				res, err := fn(keyword)
				if (err != nil || len(res) == 0) && artist != "" {
					res, _ = fn(name)
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
					candidates = append(candidates, candidate{song: cand, score: score, durDiff: durDiff})
					mu.Unlock()
				}
			}(src)
		}

		wg.Wait()
		if len(candidates) == 0 {
			c.JSON(404, gin.H{"error": "no match"})
			return
		}

		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].score == candidates[j].score {
				return candidates[i].durDiff < candidates[j].durDiff
			}
			return candidates[i].score > candidates[j].score
		})

		var selected *model.Song
		var selectedScore float64
		for _, cand := range candidates {
			ok := core.ValidatePlayable(&cand.song)
			if ok {
				tmp := cand.song
				selected = &tmp
				selectedScore = cand.score
				break
			}
		}
		if selected == nil {
			c.JSON(404, gin.H{"error": "no playable match"})
			return
		}

		c.JSON(200, gin.H{
			"id":       selected.ID,
			"name":     selected.Name,
			"artist":   selected.Artist,
			"album":    selected.Album,
			"duration": selected.Duration,
			"source":   selected.Source,
			"cover":    selected.Cover,
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
		embedMeta := c.Query("embed") == "1" && strings.TrimSpace(c.GetHeader("Range")) == ""

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

		tempSong := &model.Song{ID: id, Source: source, Name: name, Artist: artist, Cover: coverURL}
		filename := fmt.Sprintf("%s - %s.mp3", name, artist)

		if embedMeta {
			var audioData []byte
			if source == "soda" {
				cookie := core.CM.Get("soda")
				sodaInst := soda.New(cookie)
				info, err := sodaInst.GetDownloadInfo(tempSong)
				if err != nil {
					c.String(502, "Soda info error")
					return
				}
				encryptedData, _, err := fetchBytesWithMime(info.URL, "soda")
				if err != nil {
					c.String(502, "Soda stream error")
					return
				}
				audioData, err = soda.DecryptAudio(encryptedData, info.PlayAuth)
				if err != nil {
					c.String(500, "Decrypt failed")
					return
				}
			} else {
				dlFunc := core.GetDownloadFunc(source)
				if dlFunc == nil {
					c.String(400, "Unknown source")
					return
				}

				downloadURL, err := dlFunc(tempSong)
				if err != nil {
					c.String(404, "Failed to get URL")
					return
				}

				audioData, _, err = fetchBytesWithMime(downloadURL, source)
				if err != nil {
					c.String(502, "Upstream stream error")
					return
				}
			}

			var lyric string
			if lyricFn := core.GetLyricFunc(source); lyricFn != nil {
				lyric, _ = lyricFn(&model.Song{ID: id, Source: source})
			}

			var coverData []byte
			var coverMime string
			if coverURL != "" {
				coverData, coverMime, _ = fetchBytesWithMime(coverURL, source)
			}

			finalData, err := embedSongMetadata(audioData, tempSong, lyric, coverData, coverMime)
			if err != nil {
				finalData = audioData
			}

			setDownloadHeader(c, filename)
			c.Data(200, "audio/mpeg", finalData)
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
		c.String(200, "[00:00.00] 暂无歌词")
	})
}
