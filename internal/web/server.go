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
	// [新增] 引入 soda 包
	"github.com/guohuiyuan/music-lib/soda"
)

//go:embed templates/*
var templateFS embed.FS

func Start(port string) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 使用 embed.FS 加载模板
	tmpl := template.Must(template.New("").ParseFS(templateFS, "templates/*.html"))
	r.SetHTMLTemplate(tmpl)

	// 服务 favicon 图标
	r.GET("/icon.png", func(c *gin.Context) {
		c.FileFromFS("templates/icon.png", http.FS(templateFS))
	})

	// 首页：传递所有可用源给前端，用于生成复选框
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"AllSources": core.GetAllSourceNames(),
		})
	})

	// 搜索处理
	r.GET("/search", func(c *gin.Context) {
		keyword := c.Query("q")
		// 获取用户勾选的源 (Gin 会自动将同名参数解析为 slice)
		sources := c.QueryArray("sources")

		// 默认全选逻辑：如果用户什么都没选(直接回车)，由于前端默认勾选，
		// 这里主要处理异常情况，或者在此处再次兜底
		if len(sources) == 0 {
			sources = core.GetAllSourceNames()
		}

		songs, err := core.SearchAndFilter(keyword, sources)
		if err != nil {
			fmt.Printf("搜索失败: %v\n", err)
		}

		// 为模板准备格式化数据
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
			"AllSources": core.GetAllSourceNames(), // 保持复选框列表
			"Selected":   sources,                  // 用于回显勾选状态
		})
	})

	// 下载/播放接口
	r.GET("/download", func(c *gin.Context) {
		id := c.Query("id")
		source := c.Query("source")
		songName := c.Query("name")
		artist := c.Query("artist")

		if id == "" || source == "" {
			c.String(http.StatusBadRequest, "缺少必要参数")
			return
		}

		if songName == "" { songName = "Unknown" }
		if artist == "" { artist = "Unknown" }
		filename := generateFilename(songName, artist)
		encodedFilename := url.QueryEscape(filename)
		encodedFilename = strings.ReplaceAll(encodedFilename, "+", "%20")
		contentDisposition := fmt.Sprintf("attachment; filename=\"music.mp3\"; filename*=utf-8''%s", encodedFilename)

		// [新增] Soda 音乐特殊处理：内存下载 -> 解密 -> 播放
		if source == "soda" {
			// 1. 获取包含 Auth 的信息
			tempSong := &model.Song{ID: id, Source: source}
			info, err := soda.GetDownloadInfo(tempSong)
			if err != nil {
				c.String(http.StatusInternalServerError, "获取Soda信息失败: %v", err)
				return
			}

			// 2. 下载加密文件
			resp, err := http.Get(info.URL)
			if err != nil {
				c.String(http.StatusBadGateway, "下载源文件失败: %v", err)
				return
			}
			defer resp.Body.Close()
			
			encryptedData, err := io.ReadAll(resp.Body)
			if err != nil {
				c.String(http.StatusInternalServerError, "读取数据失败: %v", err)
				return
			}

			// 3. 解密
			decryptedData, err := soda.DecryptAudio(encryptedData, info.PlayAuth)
			if err != nil {
				c.String(http.StatusInternalServerError, "解密失败: %v", err)
				return
			}

			// 4. 返回解密后的数据 (支持 Range)
			c.Header("Content-Disposition", contentDisposition)
			// 使用 http.ServeContent 自动处理 Range/Content-Length/MIME
			http.ServeContent(c.Writer, c.Request, filename, time.Now(), bytes.NewReader(decryptedData))
			return
		}

		// --- 以下为通用源代理逻辑 (不变) ---
		tempSong := &model.Song{ID: id, Source: source, Name: songName, Artist: artist}
		downloadUrl, err := core.GetDownloadURL(tempSong)
		if err != nil {
			c.String(http.StatusInternalServerError, "获取下载链接失败: %v", err)
			return
		}

		if downloadUrl == "" {
			c.String(http.StatusBadRequest, "无效的下载链接")
			return
		}

		// 获取远程文件流 (代理下载)
		resp, err := http.Get(downloadUrl)
		if err != nil {
			c.String(http.StatusInternalServerError, "获取源文件失败: %v", err)
			return
		}
		defer resp.Body.Close()

		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Disposition", contentDisposition)
		c.Header("Content-Transfer-Encoding", "binary")
		
		// 将流传输给用户
		// 【关键修改】：第二个参数 (contentLength) 必须传 -1
		// 传 -1 会告诉 Gin 使用 "Transfer-Encoding: chunked"
		// 浏览器就会乖乖接收所有数据，直到连接正常关闭，而不会去核对字节数
		c.DataFromReader(http.StatusOK, -1, "application/octet-stream", resp.Body, map[string]string{
			"Content-Disposition": contentDisposition,
		})
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
	// 简单的文件名清洗，防止非法字符
	clean := func(s string) string {
		// Windows 文件名非法字符: \ / : * ? " < > |
		illegalChars := []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|"}
		result := s
		for _, char := range illegalChars {
			// 简单替换
			for i := 0; i < len(result); i++ {
				if i+len(char) <= len(result) && result[i:i+len(char)] == char {
					result = result[:i] + "_" + result[i+len(char):]
				}
			}
		}
		return result
	}

	safeName := clean(name)
	safeArtist := clean(artist)
	return fmt.Sprintf("%s - %s.mp3", safeArtist, safeName)
}
