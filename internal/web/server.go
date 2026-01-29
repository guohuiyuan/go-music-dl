package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os/exec" // [新增] 用于执行系统命令
	"runtime" // [新增] 用于判断操作系统
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/fivesing"
	"github.com/guohuiyuan/music-lib/joox"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/kuwo"
	"github.com/guohuiyuan/music-lib/migu"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qianqian"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/soda"
	"github.com/guohuiyuan/music-lib/utils" // 保持用于 HTTP 请求
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

// [新增] 本地辅助函数：打开浏览器
func openBrowser(url string) {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin": // macOS
		cmd = "open"
	default: // linux
		cmd = "xdg-open"
	}
	args = append(args, url)
	_ = exec.Command(cmd, args...).Start()
}

func Start(port string) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	tmpl := template.Must(template.New("").ParseFS(templateFS, "templates/*.html"))
	r.SetHTMLTemplate(tmpl)

	r.GET("/icon.png", func(c *gin.Context) {
		c.FileFromFS("templates/icon.png", http.FS(templateFS))
	})

	r.GET("/", func(c *gin.Context) {
		allSources := core.GetAllSourceNames()
		sourceDescriptions := make(map[string]string)
		for _, source := range allSources {
			sourceDescriptions[source] = core.GetSourceDescription(source)
		}

		c.HTML(http.StatusOK, "index.html", gin.H{
			"AllSources":         allSources,
			"DefaultSources":     core.GetDefaultSourceNames(),
			"SourceDescriptions": sourceDescriptions,
		})
	})

	r.GET("/search", func(c *gin.Context) {
		keyword := c.Query("q")
		sources := c.QueryArray("sources")

		if len(sources) == 0 {
			sources = core.GetDefaultSourceNames()
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
				FormatDuration: song.FormatDuration(),
				FormatSize:     song.FormatSize(),
			})
		}

		allSources := core.GetAllSourceNames()
		sourceDescriptions := make(map[string]string)
		for _, source := range allSources {
			sourceDescriptions[source] = core.GetSourceDescription(source)
		}

		c.HTML(http.StatusOK, "index.html", gin.H{
			"Result":             formattedSongs,
			"Keyword":            keyword,
			"AllSources":         allSources,
			"DefaultSources":     core.GetDefaultSourceNames(),
			"SourceDescriptions": sourceDescriptions,
			"Selected":           sources,
		})
	})

	// 获取歌词文本 (用于播放器显示)
	r.GET("/lyric", func(c *gin.Context) {
		id := c.Query("id")
		source := c.Query("source")
		lrc, err := fetchLyrics(id, source)
		if err != nil {
			fmt.Printf("获取歌词失败: %v\n", err)
			c.String(http.StatusOK, "[00:00.00] 获取歌词失败")
			return
		}
		if lrc == "" {
			lrc = "[00:00.00] 暂无歌词"
		}
		c.String(http.StatusOK, lrc)
	})

	// 下载歌词文件 (.lrc)
	r.GET("/download_lrc", func(c *gin.Context) {
		id := c.Query("id")
		source := c.Query("source")
		songName := c.Query("name")
		artist := c.Query("artist")

		lrc, err := fetchLyrics(id, source)
		if err != nil || lrc == "" {
			c.String(http.StatusNotFound, "歌词未找到")
			return
		}

		filename := fmt.Sprintf("%s - %s.lrc", artist, songName)
		setDownloadHeader(c, filename)
		c.String(http.StatusOK, lrc)
	})

	// 下载封面图片 (.jpg)
	r.GET("/download_cover", func(c *gin.Context) {
		coverUrl := c.Query("url")
		songName := c.Query("name")
		artist := c.Query("artist")

		if coverUrl == "" {
			c.String(http.StatusBadRequest, "无封面链接")
			return
		}

		// 下载图片数据
		data, err := utils.Get(coverUrl, utils.WithHeader("User-Agent", UA_Common))
		if err != nil {
			c.String(http.StatusBadGateway, "下载封面失败: %v", err)
			return
		}

		filename := fmt.Sprintf("%s - %s.jpg", artist, songName)
		setDownloadHeader(c, filename)
		c.Data(http.StatusOK, "image/jpeg", data)
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

		if songName == "" {
			songName = "Unknown"
		}
		if artist == "" {
			artist = "Unknown"
		}

		tempSong := &model.Song{ID: id, Source: source, Name: songName, Artist: artist}
		filename := tempSong.Filename()

		var finalData []byte

		if source == "soda" {
			// Soda 下载流程
			info, err := soda.GetDownloadInfo(tempSong)
			if err != nil {
				c.String(http.StatusInternalServerError, "获取Soda信息失败: %v", err)
				return
			}
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
			finalData, err = soda.DecryptAudio(encryptedData, info.PlayAuth)
			if err != nil {
				c.String(http.StatusInternalServerError, "解密失败: %v", err)
				return
			}
		} else {
			// 通用下载流程
			downloadUrl, err := core.GetDownloadURL(tempSong)
			if err != nil || downloadUrl == "" {
				c.String(http.StatusInternalServerError, "获取链接失败: %v", err)
				return
			}
			req, err := http.NewRequest("GET", downloadUrl, nil)
			if err != nil {
				c.String(http.StatusInternalServerError, "构造请求失败: %v", err)
				return
			}
			// 设置伪装头
			switch source {
			case "bilibili":
				req.Header.Set("User-Agent", UA_Common)
				req.Header.Set("Referer", Ref_Bilibili)
			case "migu":
				req.Header.Set("User-Agent", UA_Mobile)
				req.Header.Set("Referer", Ref_Migu)
			default:
				req.Header.Set("User-Agent", UA_Common)
			}
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				c.String(http.StatusBadGateway, "请求源文件失败: %v", err)
				return
			}
			defer resp.Body.Close()
			finalData, err = io.ReadAll(resp.Body)
			if err != nil {
				c.String(http.StatusInternalServerError, "读取源数据失败: %v", err)
				return
			}
		}

		setDownloadHeader(c, filename)
		http.ServeContent(c.Writer, c.Request, filename, time.Now(), bytes.NewReader(finalData))
	})

	urlStr := "http://localhost:" + port
	fmt.Printf("Web started at %s\n", urlStr)

	// [新增] 启动 Goroutine 自动打开浏览器
	go func() {
		time.Sleep(500 * time.Millisecond) // 稍微等待服务器启动
		fmt.Println("正在尝试自动打开浏览器...")
		openBrowser(urlStr)
	}()

	r.Run(":" + port)
}

// 辅助函数：统一设置下载头
func setDownloadHeader(c *gin.Context, filename string) {
	encodedFilename := url.QueryEscape(filename)
	encodedFilename = strings.ReplaceAll(encodedFilename, "+", "%20")
	contentDisposition := fmt.Sprintf("attachment; filename=\"%s\"; filename*=utf-8''%s", encodedFilename, encodedFilename)
	c.Header("Content-Disposition", contentDisposition)
}

// 辅助函数：统一获取歌词逻辑
func fetchLyrics(id, source string) (string, error) {
	var lrc string
	var err error
	song := &model.Song{ID: id, Source: source}

	switch source {
	case "soda":
		lrc, err = soda.GetLyrics(song)
	case "kuwo":
		lrc, err = kuwo.GetLyrics(song)
	case "netease":
		lrc, err = netease.GetLyrics(song)
	case "qq":
		lrc, err = qq.GetLyrics(song)
	case "kugou":
		lrc, err = kugou.GetLyrics(song)
	case "qianqian":
		lrc, err = qianqian.GetLyrics(song)
	case "migu":
		lrc, err = migu.GetLyrics(song)
	case "joox":
		lrc, err = joox.GetLyrics(song)
	case "fivesing":
		lrc, err = fivesing.GetLyrics(song)
	default:
		return "", nil
	}
	return lrc, err
}
