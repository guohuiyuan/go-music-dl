package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/soda"
	"github.com/guohuiyuan/music-lib/kuwo"
)

//go:embed templates/*
var templateFS embed.FS

// 定义各源的伪装常量
const (
	UA_Common    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
	UA_Mobile    = "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13B143 Safari/601.1"
	Ref_Bilibili = "https://www.bilibili.com/"
	Ref_Migu     = "http://music.migu.cn/"
)

func Start(port string) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 使用 embed.FS 加载模板
	tmpl := template.Must(template.New("").ParseFS(templateFS, "templates/*.html"))
	r.SetHTMLTemplate(tmpl)

	// 服务 favicon
	r.GET("/icon.png", func(c *gin.Context) {
		c.FileFromFS("templates/icon.png", http.FS(templateFS))
	})

	// 首页
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"AllSources": core.GetAllSourceNames(),
		})
	})

	// 搜索接口
	r.GET("/search", func(c *gin.Context) {
		keyword := c.Query("q")
		sources := c.QueryArray("sources")

		if len(sources) == 0 {
			sources = core.GetAllSourceNames()
		}

		songs, err := core.SearchAndFilter(keyword, sources)
		if err != nil {
			fmt.Printf("搜索失败: %v\n", err)
		}

		type SongWithFormat struct {
			model.Song
			FormatDuration string
			FormatSize     string
		}

		var formattedSongs []SongWithFormat
		for _, song := range songs {
			formattedSongs = append(formattedSongs, SongWithFormat{
				Song:           song,
				FormatDuration: formatDuration(song.Duration),
				FormatSize:     formatSize(song.Size),
			})
		}

		c.HTML(http.StatusOK, "index.html", gin.H{
			"Result":     formattedSongs,
			"Keyword":    keyword,
			"AllSources": core.GetAllSourceNames(),
			"Selected":   sources,
		})
	})

	// [新增] 歌词接口
	r.GET("/lyric", func(c *gin.Context) {
		id := c.Query("id")
		source := c.Query("source")

		if id == "" || source == "" {
			c.String(http.StatusBadRequest, "[00:00.00] 缺少参数")
			return
		}

		var lrc string
		var err error

		// 根据源获取歌词
		switch source {
		case "soda":
			lrc, err = soda.GetLyric(id)
		case "kuwo":
			lrc, err = kuwo.GetLyric(id)
		// 后续可以继续添加 case "netease": ...
		default:
			lrc = "[00:00.00] 暂不支持该源歌词"
		}

		if err != nil {
			fmt.Printf("获取歌词失败: %v\n", err)
			c.String(http.StatusOK, "[00:00.00] 获取歌词失败")
			return
		}

		if lrc == "" {
			lrc = "[00:00.00] 暂无歌词"
		}

		// 直接返回 LRC 文本
		c.String(http.StatusOK, lrc)
	})

	// 下载/播放接口 (改为内存播放以支持进度条拖动)
	r.GET("/download", func(c *gin.Context) {
		id := c.Query("id")
		source := c.Query("source")
		songName := c.Query("name")
		artist := c.Query("artist")

		if id == "" || source == "" {
			c.String(http.StatusBadRequest, "缺少必要参数")
			return
		}

		// 准备文件名
		if songName == "" { songName = "Unknown" }
		if artist == "" { artist = "Unknown" }
		filename := generateFilename(songName, artist)
		encodedFilename := url.QueryEscape(filename)
		encodedFilename = strings.ReplaceAll(encodedFilename, "+", "%20")
		contentDisposition := fmt.Sprintf("attachment; filename=\"music.mp3\"; filename*=utf-8''%s", encodedFilename)

		var finalData []byte

		// === 分支处理：获取音频数据到内存 ===
		if source == "soda" {
			// --- Soda 逻辑: 下载 -> 解密 ---
			tempSong := &model.Song{ID: id, Source: source}
			info, err := soda.GetDownloadInfo(tempSong)
			if err != nil {
				c.String(http.StatusInternalServerError, "获取Soda信息失败: %v", err)
				return
			}

			// 下载加密数据
			// 注意：Soda 下载也需要 UA
			req, _ := http.NewRequest("GET", info.URL, nil)
			req.Header.Set("User-Agent", UA_Common)
			
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				c.String(http.StatusBadGateway, "下载Soda文件失败: %v", err)
				return
			}
			defer resp.Body.Close()

			encryptedData, err := io.ReadAll(resp.Body)
			if err != nil {
				c.String(http.StatusInternalServerError, "读取Soda数据失败: %v", err)
				return
			}

			// 解密
			finalData, err = soda.DecryptAudio(encryptedData, info.PlayAuth)
			if err != nil {
				c.String(http.StatusInternalServerError, "解密失败: %v", err)
				return
			}

		} else {
			// --- 通用逻辑: 获取 URL -> 下载 (带Header) ---
			tempSong := &model.Song{ID: id, Source: source, Name: songName, Artist: artist}
			downloadUrl, err := core.GetDownloadURL(tempSong)
			if err != nil {
				c.String(http.StatusInternalServerError, "获取链接失败: %v", err)
				return
			}
			if downloadUrl == "" {
				c.String(http.StatusBadRequest, "无有效下载链接")
				return
			}

			// 构造请求 (带防盗链 Header)
			req, err := http.NewRequest("GET", downloadUrl, nil)
			if err != nil {
				c.String(http.StatusInternalServerError, "构造请求失败: %v", err)
				return
			}

			// 添加伪装 Header
			switch source {
			case "bilibili":
				req.Header.Set("User-Agent", UA_Common)
				req.Header.Set("Referer", Ref_Bilibili)
			case "migu":
				req.Header.Set("User-Agent", UA_Mobile)
				req.Header.Set("Referer", Ref_Migu)
			case "kuwo":
				req.Header.Set("User-Agent", UA_Common)
			default:
				req.Header.Set("User-Agent", UA_Common)
			}

			// 执行下载
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				c.String(http.StatusBadGateway, "请求源文件失败: %v", err)
				return
			}
			defer resp.Body.Close()

			// 将全部数据读入内存
			finalData, err = io.ReadAll(resp.Body)
			if err != nil {
				c.String(http.StatusInternalServerError, "读取源数据失败: %v", err)
				return
			}
		}

		// === 统一响应：使用 ServeContent 支持拖动 ===
		
		// 1. 设置下载文件名 Header
		c.Header("Content-Disposition", contentDisposition)
		
		// 2. 使用 http.ServeContent
		// 这个函数会自动处理 Range 头、Content-Length 和 Content-Type
		// 让浏览器可以随意拖动进度条
		http.ServeContent(c.Writer, c.Request, filename, time.Now(), bytes.NewReader(finalData))
	})

	fmt.Printf("Web started at http://localhost:%s\n", port)
	r.Run(":" + port)
}

// 辅助函数：格式化时长
func formatDuration(seconds int) string {
	if seconds == 0 {
		return "-"
	}
	min := seconds / 60
	sec := seconds % 60
	return fmt.Sprintf("%02d:%02d", min, sec)
}

// 辅助函数：格式化大小
func formatSize(size int64) string {
	if size == 0 {
		return "-"
	}
	mb := float64(size) / 1024 / 1024
	return fmt.Sprintf("%.2f MB", mb)
}

// 辅助函数：生成文件名
func generateFilename(name, artist string) string {
	clean := func(s string) string {
		illegalChars := []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|"}
		result := s
		for _, char := range illegalChars {
			result = strings.ReplaceAll(result, char, "_")
		}
		return result
	}
	return fmt.Sprintf("%s - %s.mp3", clean(artist), clean(name))
}
