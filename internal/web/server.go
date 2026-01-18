package web

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

//go:embed templates/*
var templateFS embed.FS

func Start(port string) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 使用 embed.FS 加载模板
	tmpl := template.Must(template.New("").ParseFS(templateFS, "templates/*.html"))
	r.SetHTMLTemplate(tmpl)

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

	// 下载接口：统一文件名逻辑
	r.GET("/download", func(c *gin.Context) {
		// 获取参数
		id := c.Query("id")
		source := c.Query("source")
		songName := c.Query("name")
		artist := c.Query("artist")

		if id == "" || source == "" {
			c.String(http.StatusBadRequest, "缺少必要参数")
			return
		}

		// 创建临时歌曲对象
		tempSong := &model.Song{
			ID:     id,
			Source: source,
			Name:   songName,
			Artist: artist,
		}

		// 获取下载链接
		downloadUrl, err := core.GetDownloadURL(tempSong)
		if err != nil {
			c.String(http.StatusInternalServerError, "获取下载链接失败: %v", err)
			return
		}

		if downloadUrl == "" {
			c.String(http.StatusBadRequest, "无效的下载链接")
			return
		}

		// 如果歌名或歌手为空，使用默认值
		if songName == "" {
			songName = "Unknown"
		}
		if artist == "" {
			artist = "Unknown"
		}

		// 1. 构建文件名: "歌手 - 歌名.mp3"
		filename := generateFilename(songName, artist)

		// 2. 处理 URL 编码 (解决中文乱码的关键)
		// QueryEscape 会把空格转为 +，为了美观我们把 + 换回 %20
		encodedFilename := url.QueryEscape(filename)
		encodedFilename = strings.ReplaceAll(encodedFilename, "+", "%20")

		// 3. 获取远程文件流 (代理下载)
		resp, err := http.Get(downloadUrl)
		if err != nil {
			c.String(http.StatusInternalServerError, "获取源文件失败: %v", err)
			return
		}
		defer resp.Body.Close()

		// 4. 设置强制下载 Header (最强兼容性写法)
		// filename="..." 兼容旧浏览器
		// filename*=utf-8''... 兼容现代浏览器并支持中文
		contentDisposition := fmt.Sprintf("attachment; filename=\"music.mp3\"; filename*=utf-8''%s", encodedFilename)
		
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Disposition", contentDisposition)
		c.Header("Content-Transfer-Encoding", "binary")
		
		// 5. 将流传输给用户
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
