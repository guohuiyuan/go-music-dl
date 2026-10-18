package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

type downloadContainerRequest struct {
	RequestID string `json:"request_id"`
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Source    string `json:"source"`
	Link      string `json:"link"`
	Embed     bool   `json:"embed"`
}

func loadDownloadContainerSongs(req downloadContainerRequest) ([]model.Song, error) {
	switch req.Kind {
	case "collection":
		id, err := strconv.ParseUint(req.ID, 10, 32)
		if err != nil || id == 0 {
			return nil, fmt.Errorf("无效的本地歌单 ID")
		}
		collection, err := loadCollection(req.ID)
		if err != nil {
			return nil, fmt.Errorf("本地歌单不存在")
		}
		return loadCollectionSongs(collection)
	case collectionContentPlaylist, collectionContentAlbum:
		source := strings.TrimSpace(req.Source)
		if source == "" || isLocalMusicSource(source) || strings.TrimSpace(req.ID) == "" {
			return nil, fmt.Errorf("缺少平台或歌单/专辑 ID")
		}
		collection := &Collection{Kind: collectionKindImported, ContentType: req.Kind, Source: source, ExternalID: req.ID, Link: req.Link}
		return loadImportedCollectionSongs(collection)
	default:
		return nil, fmt.Errorf("不支持的下载类型")
	}
}

func registerDownloadContainerRoute(api *gin.RouterGroup, queue *core.DownloadQueue) {
	api.POST("/api/downloads/queue/container", func(c *gin.Context) {
		if !allowSameOriginWrite(c) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
		var req downloadContainerRequest
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.RequestID) == "" || len(req.RequestID) > 128 {
			c.JSON(400, gin.H{"error": "无效的下载请求"})
			return
		}
		settings := core.GetWebSettings()
		songs, err := loadDownloadContainerSongs(req)
		if err != nil {
			c.JSON(400, gin.H{"error": "获取完整曲目失败：" + err.Error()})
			return
		}
		if len(songs) == 0 {
			c.JSON(400, gin.H{"error": "歌单或专辑中没有可下载的歌曲"})
			return
		}
		if len(songs) > 20000 {
			c.JSON(400, gin.H{"error": "曲目超过 20000 首，请拆分歌单后下载；本次未加入队列"})
			return
		}
		inputs := make([]core.DownloadTaskInput, 0, len(songs))
		local := 0
		for _, song := range songs {
			if isLocalMusicSource(song.Source) {
				local++
				continue
			}
			if strings.TrimSpace(song.ID) == "" || strings.TrimSpace(song.Source) == "" {
				c.JSON(400, gin.H{"error": "部分曲目缺少 ID 或音源，本次未加入队列"})
				return
			}
			inputs = append(inputs, core.DownloadTaskInput{Song: song, Directory: core.DownloadDirOrDefault(settings.DownloadDir), FilenameTemplate: settings.DownloadFilenameTemplate, Embed: req.Embed})
		}
		if len(inputs) == 0 {
			c.JSON(400, gin.H{"error": "歌单中均为已有本地音乐，无需重复下载"})
			return
		}
		added, err := queue.Enqueue(req.RequestID, inputs)
		if err != nil {
			c.JSON(500, gin.H{"error": "加入队列失败：" + err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"added": added, "existing": len(inputs) - added, "total": len(songs), "skipped_local": local})
	})
}
