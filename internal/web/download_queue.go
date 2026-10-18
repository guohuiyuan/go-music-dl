package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

type queueSongRequest struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Name   string `json:"name"`
	Artist string `json:"artist"`
	Album  string `json:"album"`
	Cover  string `json:"cover"`
	Extra  string `json:"extra"`
	Embed  bool   `json:"embed"`
}

func RegisterDownloadQueueRoutes(api *gin.RouterGroup, queue *core.DownloadQueue) {
	api.GET("/api/downloads/queue", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		if page < 1 {
			page = 1
		}
		tasks, total, counts, err := queue.List(page, 50, c.Query("status"))
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"tasks": tasks, "total": total, "counts": counts, "page": page, "total_pages": (total + 49) / 50})
	})
	api.POST("/api/downloads/queue", func(c *gin.Context) {
		if !allowSameOriginWrite(c) {
			c.JSON(403, gin.H{"error": "forbidden"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<20)
		var req struct {
			RequestID string             `json:"request_id"`
			Songs     []queueSongRequest `json:"songs"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || len(req.Songs) == 0 || len(req.Songs) > 2000 || strings.TrimSpace(req.RequestID) == "" || len(req.RequestID) > 128 {
			c.JSON(400, gin.H{"error": "无效的下载批次（每次最多 2000 首）"})
			return
		}
		settings := core.GetWebSettings()
		inputs := make([]core.DownloadTaskInput, 0, len(req.Songs))
		for _, s := range req.Songs {
			if strings.TrimSpace(s.ID) == "" || strings.TrimSpace(s.Source) == "" || isLocalMusicSource(s.Source) {
				c.JSON(400, gin.H{"error": "歌曲缺少 ID 或音源，或属于本地音乐"})
				return
			}
			inputs = append(inputs, core.DownloadTaskInput{Song: model.Song{ID: s.ID, Source: s.Source, Name: s.Name, Artist: s.Artist, Album: s.Album, Cover: s.Cover, Extra: parseSongExtraQuery(s.Extra)}, Directory: core.DownloadDirOrDefault(settings.DownloadDir), FilenameTemplate: settings.DownloadFilenameTemplate, Embed: s.Embed})
		}
		added, err := queue.Enqueue(req.RequestID, inputs)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"added": added, "existing": len(inputs) - added})
	})
	api.POST("/api/downloads/queue/:id/:action", func(c *gin.Context) {
		if !allowSameOriginWrite(c) {
			c.JSON(403, gin.H{"error": "forbidden"})
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid task id"})
			return
		}
		switch c.Param("action") {
		case "retry":
			err = queue.Retry(uint(id))
		case "cancel":
			err = queue.Cancel(uint(id))
		default:
			c.Status(404)
			return
		}
		if err != nil {
			c.JSON(409, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})
}

func processQueuedDownload(input core.DownloadTaskInput) (core.DownloadTaskResult, error) {
	index, err := core.LoadDownloadDedupSet()
	if err != nil {
		return core.DownloadTaskResult{}, err
	}
	song := input.Song
	resolveLegacyDedupForSong(index, &song)
	result, err := core.DownloadWithDedupCheckWithTemplate(&song, input.Directory, input.Embed, input.Embed, input.FilenameTemplate, index)
	if err != nil {
		return core.DownloadTaskResult{}, err
	}
	outcome := core.DownloadTaskResult{Status: core.DownloadStatusSuccess, Path: result.SavedPath, Warning: result.Warning}
	if result.Skipped {
		outcome.Status = core.DownloadStatusSkipped
		return outcome, nil
	}
	indexSavedLocalMusicFile(result, input.Directory)
	settings := core.GetWebSettings()
	if core.WebDAVConfigured(settings) {
		if err := core.UploadSongToWebDAV(settings, result.Filename, result.Data); err != nil {
			outcome.Warning += " WebDAV: " + err.Error()
		}
	}
	return outcome, nil
}
