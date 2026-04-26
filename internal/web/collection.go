package web

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var db *gorm.DB

const (
	legacyFavoritesDBFile = "data/favorites.db"

	collectionKindManual   = "manual"
	collectionKindImported = "imported"

	collectionContentPlaylist = "playlist"
	collectionContentAlbum    = "album"
)

var (
	playlistDetailFuncProvider = core.GetPlaylistDetailFunc
	albumDetailFuncProvider    = core.GetAlbumDetailFunc
	parsePlaylistFuncProvider  = core.GetParsePlaylistFunc
	parseAlbumFuncProvider     = core.GetParseAlbumFunc
)

// Collection stores local entries shown in "My Collections".
// Manual collections persist songs in SavedSong. Imported entries only keep
// metadata and fetch songs on demand from the upstream source.
type Collection struct {
	ID          uint        `gorm:"primaryKey" json:"id"`
	Name        string      `gorm:"not null" json:"name"`
	Description string      `json:"description"`
	Cover       string      `json:"cover"`
	Kind        string      `gorm:"not null;default:manual" json:"kind"`
	ContentType string      `gorm:"column:content_type;not null;default:playlist" json:"content_type"`
	Source      string      `gorm:"not null;default:local" json:"source"`
	ExternalID  string      `json:"external_id"`
	Link        string      `json:"link"`
	Creator     string      `json:"creator"`
	TrackCount  int         `json:"track_count"`
	CreatedAt   time.Time   `json:"created_at"`
	SavedSongs  []SavedSong `gorm:"constraint:OnDelete:CASCADE;" json:"-"`
}

type SavedSong struct {
	ID           uint      `gorm:"primaryKey" json:"db_id"`
	CollectionID uint      `gorm:"uniqueIndex:idx_col_song_src" json:"collection_id"`
	SongID       string    `gorm:"uniqueIndex:idx_col_song_src;not null" json:"song_id"`
	Source       string    `gorm:"uniqueIndex:idx_col_song_src;not null" json:"source"`
	Extra        string    `json:"extra"`
	Name         string    `json:"name"`
	Artist       string    `json:"artist"`
	Cover        string    `json:"cover"`
	Duration     int       `json:"duration"`
	AddedAt      time.Time `json:"added_at"`
}

type importCollectionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Cover       string `json:"cover"`
	Creator     string `json:"creator"`
	TrackCount  int    `json:"track_count"`
	Source      string `json:"source"`
	ExternalID  string `json:"external_id"`
	Link        string `json:"link"`
	ContentType string `json:"content_type"`
}

func (c Collection) normalizedKind() string {
	if strings.TrimSpace(c.Kind) == collectionKindImported {
		return collectionKindImported
	}
	return collectionKindManual
}

func (c Collection) normalizedContentType() string {
	if strings.TrimSpace(c.ContentType) == collectionContentAlbum {
		return collectionContentAlbum
	}
	return collectionContentPlaylist
}

func (c Collection) normalizedSource() string {
	source := strings.TrimSpace(c.Source)
	if source == "" || source == "local" {
		if c.normalizedKind() == collectionKindImported {
			return ""
		}
		return "local"
	}
	return source
}

func (c Collection) isImported() bool {
	return c.normalizedKind() == collectionKindImported
}

func (c Collection) isManual() bool {
	return !c.isImported()
}

func (c Collection) editable() bool {
	return c.isManual()
}

func (c Collection) displayCover() string {
	if strings.TrimSpace(c.Cover) != "" {
		return c.Cover
	}
	return fmt.Sprintf("https://picsum.photos/seed/col_%d/400/400", c.ID)
}

func (c Collection) displayCreator() string {
	if c.isImported() {
		if creator := strings.TrimSpace(c.Creator); creator != "" {
			return creator
		}
		if c.normalizedContentType() == collectionContentAlbum {
			return "外部导入专辑"
		}
		return "外部导入歌单"
	}
	return "我自己"
}

func (c Collection) originalLink() string {
	if link := strings.TrimSpace(c.Link); link != "" {
		return link
	}
	source := c.normalizedSource()
	if !c.isImported() || source == "" || strings.TrimSpace(c.ExternalID) == "" {
		return ""
	}
	return core.GetOriginalLink(source, c.ExternalID, c.normalizedContentType())
}

func (c Collection) playlistCard() model.Playlist {
	trackCount := c.TrackCount
	if c.isManual() {
		trackCount = countSavedSongs(c.ID)
	}

	extra := map[string]string{
		"collection_kind": c.normalizedKind(),
		"content_type":    c.normalizedContentType(),
		"editable":        fmt.Sprintf("%t", c.editable()),
	}
	if remoteSource := c.normalizedSource(); c.isImported() && remoteSource != "" {
		extra["remote_source"] = remoteSource
	}

	return model.Playlist{
		ID:          fmt.Sprint(c.ID),
		Name:        c.Name,
		Description: c.Description,
		Cover:       c.displayCover(),
		Creator:     c.displayCreator(),
		TrackCount:  trackCount,
		Source:      "local",
		Link:        c.originalLink(),
		Extra:       extra,
	}
}

func InitDB() {
	dbPath := filepath.Clean(core.ConfigDBPath())
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		panic("Failed to create SQLite directory: " + err.Error())
	}

	var err error
	db, err = gorm.Open(sqlite.Open(dbPath+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to SQLite: " + err.Error())
	}

	if err := db.AutoMigrate(&Collection{}, &SavedSong{}); err != nil {
		panic("Failed to migrate database: " + err.Error())
	}

	if err := migrateLegacyFavorites(dbPath); err != nil {
		panic("Failed to migrate legacy favorites database: " + err.Error())
	}
	if err := backfillCollectionDefaults(); err != nil {
		panic("Failed to normalize collection defaults: " + err.Error())
	}
}

func CloseDB() {
	if db != nil {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

func legacyFavoritesDBPath() string {
	if path := strings.TrimSpace(os.Getenv("MUSIC_DL_FAVORITES_DB")); path != "" {
		return path
	}
	return legacyFavoritesDBFile
}

func migrateLegacyFavorites(unifiedPath string) error {
	legacyPath := filepath.Clean(legacyFavoritesDBPath())
	if legacyPath == "" || legacyPath == filepath.Clean(unifiedPath) {
		return nil
	}

	if _, err := os.Stat(legacyPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var collectionCount int64
	if err := db.Model(&Collection{}).Count(&collectionCount).Error; err != nil {
		return err
	}
	if collectionCount > 0 {
		return nil
	}

	legacyDB, err := gorm.Open(sqlite.Open(legacyPath+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"), &gorm.Config{})
	if err != nil {
		return err
	}
	sqlDB, err := legacyDB.DB()
	if err != nil {
		return err
	}

	if !legacyDB.Migrator().HasTable(&Collection{}) {
		_ = sqlDB.Close()
		return nil
	}

	var collections []Collection
	if err := legacyDB.Order("id ASC").Find(&collections).Error; err != nil {
		_ = sqlDB.Close()
		return err
	}

	var savedSongs []SavedSong
	if legacyDB.Migrator().HasTable(&SavedSong{}) {
		if err := legacyDB.Order("id ASC").Find(&savedSongs).Error; err != nil {
			_ = sqlDB.Close()
			return err
		}
	}

	if len(collections) == 0 && len(savedSongs) == 0 {
		if err := sqlDB.Close(); err != nil {
			return err
		}
		return removeLegacyFavoritesFiles(legacyPath)
	}

	if err := sqlDB.Close(); err != nil {
		return err
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if len(collections) > 0 {
			if err := tx.Create(&collections).Error; err != nil {
				return err
			}
		}
		if len(savedSongs) > 0 {
			for i := range savedSongs {
				savedSongs[i].ID = 0
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&savedSongs).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	return removeLegacyFavoritesFiles(legacyPath)
}

func backfillCollectionDefaults() error {
	statements := []struct {
		query string
		args  []interface{}
	}{
		{
			query: "UPDATE collections SET kind = ? WHERE kind = '' OR kind IS NULL",
			args:  []interface{}{collectionKindManual},
		},
		{
			query: "UPDATE collections SET content_type = ? WHERE content_type = '' OR content_type IS NULL",
			args:  []interface{}{collectionContentPlaylist},
		},
		{
			query: "UPDATE collections SET source = ? WHERE source = '' OR source IS NULL",
			args:  []interface{}{"local"},
		},
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt.query, stmt.args...).Error; err != nil {
			return err
		}
	}
	return nil
}

func removeLegacyFavoritesFiles(legacyPath string) error {
	candidates := []string{
		legacyPath,
		legacyPath + "-shm",
		legacyPath + "-wal",
		legacyPath + "-journal",
	}
	for _, candidate := range candidates {
		if err := os.Remove(candidate); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func countSavedSongs(collectionID uint) int {
	var count int64
	_ = db.Model(&SavedSong{}).Where("collection_id = ?", collectionID).Count(&count).Error
	return int(count)
}

func loadCollection(collectionID string) (*Collection, error) {
	var collection Collection
	if err := db.First(&collection, collectionID).Error; err != nil {
		return nil, err
	}
	return &collection, nil
}

func loadCollectionSongs(collection *Collection) ([]model.Song, error) {
	if collection == nil {
		return nil, fmt.Errorf("collection is nil")
	}
	if collection.isImported() {
		return loadImportedCollectionSongs(collection)
	}
	return loadSavedSongs(collection.ID)
}

func loadSavedSongs(collectionID uint) ([]model.Song, error) {
	var savedSongs []SavedSong
	if err := db.Where("collection_id = ?", collectionID).Order("id DESC").Find(&savedSongs).Error; err != nil {
		return nil, err
	}

	songs := make([]model.Song, 0, len(savedSongs))
	for _, ss := range savedSongs {
		songs = append(songs, model.Song{
			ID:       ss.SongID,
			Source:   ss.Source,
			Name:     ss.Name,
			Artist:   ss.Artist,
			Cover:    ss.Cover,
			Duration: ss.Duration,
			Extra:    decodeSongExtraMap(ss.Extra),
		})
	}
	return songs, nil
}

func loadImportedCollectionSongs(collection *Collection) ([]model.Song, error) {
	if collection == nil || !collection.isImported() {
		return nil, fmt.Errorf("collection is not imported")
	}

	source := collection.normalizedSource()
	if source == "" {
		return nil, fmt.Errorf("missing imported source")
	}

	externalID := strings.TrimSpace(collection.ExternalID)
	link := collection.originalLink()
	contentType := collection.normalizedContentType()

	switch contentType {
	case collectionContentAlbum:
		if externalID != "" {
			if fn := albumDetailFuncProvider(source); fn != nil {
				if songs, err := fn(externalID); err == nil && len(songs) > 0 {
					return ensureSongSource(songs, source), nil
				}
			}
		}
		if link != "" {
			if fn := parseAlbumFuncProvider(source); fn != nil {
				if _, songs, err := fn(link); err == nil {
					return ensureSongSource(songs, source), nil
				}
			}
		}
	default:
		if externalID != "" {
			if fn := playlistDetailFuncProvider(source); fn != nil {
				if songs, err := fn(externalID); err == nil && len(songs) > 0 {
					return ensureSongSource(songs, source), nil
				}
			}
		}
		if link != "" {
			if fn := parsePlaylistFuncProvider(source); fn != nil {
				if _, songs, err := fn(link); err == nil {
					return ensureSongSource(songs, source), nil
				}
			}
		}
	}

	return nil, fmt.Errorf("failed to fetch imported %s songs", contentType)
}

func ensureSongSource(songs []model.Song, source string) []model.Song {
	for i := range songs {
		if strings.TrimSpace(songs[i].Source) == "" {
			songs[i].Source = source
		}
	}
	return songs
}

func decodeSongExtraMap(raw string) map[string]string {
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
			extra[key] = fmt.Sprintf("%.0f", v)
		case bool:
			if v {
				extra[key] = "true"
			} else {
				extra[key] = "false"
			}
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

func decodeSongExtraObject(raw string) interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var extra interface{}
	if err := json.Unmarshal([]byte(raw), &extra); err != nil {
		return raw
	}
	return extra
}

func buildImportedCollection(req importCollectionRequest) (*Collection, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Cover = strings.TrimSpace(req.Cover)
	req.Creator = strings.TrimSpace(req.Creator)
	req.Source = strings.TrimSpace(req.Source)
	req.ExternalID = strings.TrimSpace(req.ExternalID)
	req.Link = strings.TrimSpace(req.Link)
	req.ContentType = strings.TrimSpace(req.ContentType)

	if req.ContentType != collectionContentPlaylist && req.ContentType != collectionContentAlbum {
		return nil, fmt.Errorf("invalid content_type")
	}
	if req.Source == "" || req.Source == "local" {
		return nil, fmt.Errorf("invalid source")
	}
	if req.ExternalID == "" {
		return nil, fmt.Errorf("missing external_id")
	}
	if req.Name == "" {
		if req.ContentType == collectionContentAlbum {
			req.Name = "导入专辑"
		} else {
			req.Name = "导入歌单"
		}
	}
	if req.Link == "" {
		req.Link = core.GetOriginalLink(req.Source, req.ExternalID, req.ContentType)
	}

	return &Collection{
		Name:        req.Name,
		Description: req.Description,
		Cover:       req.Cover,
		Kind:        collectionKindImported,
		ContentType: req.ContentType,
		Source:      req.Source,
		ExternalID:  req.ExternalID,
		Link:        req.Link,
		Creator:     req.Creator,
		TrackCount:  req.TrackCount,
	}, nil
}

func collectionSongsJSON(collection *Collection) ([]gin.H, error) {
	if collection == nil {
		return nil, fmt.Errorf("collection is nil")
	}

	if collection.isImported() {
		songs, err := loadImportedCollectionSongs(collection)
		if err != nil {
			return nil, err
		}
		resp := make([]gin.H, 0, len(songs))
		for _, song := range songs {
			resp = append(resp, gin.H{
				"collection_id": collection.ID,
				"id":            song.ID,
				"source":        song.Source,
				"extra":         song.Extra,
				"name":          song.Name,
				"artist":        song.Artist,
				"album":         song.Album,
				"album_id":      song.AlbumID,
				"cover":         song.Cover,
				"duration":      song.Duration,
				"link":          song.Link,
			})
		}
		return resp, nil
	}

	var savedSongs []SavedSong
	if err := db.Where("collection_id = ?", collection.ID).Order("id DESC").Find(&savedSongs).Error; err != nil {
		return nil, err
	}

	resp := make([]gin.H, 0, len(savedSongs))
	for _, s := range savedSongs {
		resp = append(resp, gin.H{
			"db_id":         s.ID,
			"collection_id": s.CollectionID,
			"id":            s.SongID,
			"source":        s.Source,
			"extra":         decodeSongExtraObject(s.Extra),
			"name":          s.Name,
			"artist":        s.Artist,
			"cover":         s.Cover,
			"duration":      s.Duration,
			"added_at":      s.AddedAt,
		})
	}
	return resp, nil
}

func RegisterCollectionRoutes(api *gin.RouterGroup) {
	api.GET("/my_collections", func(c *gin.Context) {
		var collections []Collection
		if err := db.Order("id DESC").Find(&collections).Error; err != nil {
			renderIndex(c, nil, nil, "我的本地歌单", nil, "获取本地歌单失败", "playlist", "", "", "", true, "", nil)
			return
		}

		playlists := make([]model.Playlist, 0, len(collections))
		for _, collection := range collections {
			playlists = append(playlists, collection.playlistCard())
		}

		renderIndex(c, nil, playlists, "我的本地歌单", nil, "", "playlist", "", "", "", true, "", nil)
	})

	api.GET("/collection", func(c *gin.Context) {
		id := c.Query("id")
		if id == "" {
			renderIndex(c, nil, nil, "", nil, "缺少参数", "song", "", "", "", false, "", nil)
			return
		}

		collection, err := loadCollection(id)
		if err != nil {
			renderIndex(c, nil, nil, "", nil, "本地歌单不存在", "song", "", "", "", false, "", nil)
			return
		}

		songs, err := loadCollectionSongs(collection)
		errMsg := ""
		if err != nil {
			errMsg = fmt.Sprintf("获取歌单歌曲失败: %v", err)
		}

		renderIndex(c, songs, nil, "", nil, errMsg, "song", collection.originalLink(), id, collection.Name, false, collection.normalizedKind(), nil)
	})

	colAPI := api.Group("/collections")

	colAPI.GET("", func(c *gin.Context) {
		var collections []Collection

		query := db.Order("id DESC")
		if c.Query("include_imported") != "1" {
			query = query.Where("kind = ? OR kind = '' OR kind IS NULL", collectionKindManual)
		}

		if err := query.Find(&collections).Error; err != nil {
			c.JSON(500, gin.H{"error": "获取歌单失败"})
			return
		}
		c.JSON(200, collections)
	})

	colAPI.POST("", func(c *gin.Context) {
		var req Collection
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
			c.JSON(400, gin.H{"error": "参数错误，必须提供歌单名"})
			return
		}

		req.Name = strings.TrimSpace(req.Name)
		req.Description = strings.TrimSpace(req.Description)
		req.Cover = strings.TrimSpace(req.Cover)
		req.Kind = collectionKindManual
		req.ContentType = collectionContentPlaylist
		req.Source = "local"
		req.ExternalID = ""
		req.Link = ""
		req.Creator = ""
		req.TrackCount = 0

		if err := db.Create(&req).Error; err != nil {
			c.JSON(500, gin.H{"error": "创建失败: " + err.Error()})
			return
		}
		c.JSON(200, gin.H{"id": req.ID, "name": req.Name})
	})

	colAPI.POST("/import", func(c *gin.Context) {
		var req importCollectionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "参数错误"})
			return
		}

		collection, err := buildImportedCollection(req)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		var existing Collection
		err = db.Where(
			"kind = ? AND content_type = ? AND source = ? AND external_id = ?",
			collectionKindImported,
			collection.ContentType,
			collection.Source,
			collection.ExternalID,
		).First(&existing).Error
		if err == nil {
			c.JSON(200, gin.H{
				"id":        existing.ID,
				"name":      existing.Name,
				"duplicate": true,
			})
			return
		}
		if err != gorm.ErrRecordNotFound {
			c.JSON(500, gin.H{"error": "导入失败: " + err.Error()})
			return
		}

		if err := db.Create(collection).Error; err != nil {
			c.JSON(500, gin.H{"error": "导入失败: " + err.Error()})
			return
		}
		c.JSON(200, gin.H{"id": collection.ID, "name": collection.Name})
	})

	colAPI.PUT("/:id", func(c *gin.Context) {
		id := c.Param("id")

		existing, err := loadCollection(id)
		if err != nil {
			c.JSON(404, gin.H{"error": "歌单不存在"})
			return
		}
		if existing.isImported() {
			c.JSON(400, gin.H{"error": "外部导入歌单/专辑不支持编辑，请删除后重新导入"})
			return
		}

		var req Collection
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
			c.JSON(400, gin.H{"error": "参数错误"})
			return
		}

		if err := db.Model(&Collection{}).Where("id = ?", id).Updates(map[string]interface{}{
			"name":        strings.TrimSpace(req.Name),
			"description": strings.TrimSpace(req.Description),
			"cover":       strings.TrimSpace(req.Cover),
		}).Error; err != nil {
			c.JSON(500, gin.H{"error": "更新失败"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	colAPI.DELETE("/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := db.Delete(&Collection{}, id).Error; err != nil {
			c.JSON(500, gin.H{"error": "删除失败"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	colAPI.GET("/:id/songs", func(c *gin.Context) {
		collection, err := loadCollection(c.Param("id"))
		if err != nil {
			c.JSON(404, gin.H{"error": "歌单不存在"})
			return
		}

		resp, err := collectionSongsJSON(collection)
		if err != nil {
			c.JSON(500, gin.H{"error": "获取歌曲失败: " + err.Error()})
			return
		}
		c.JSON(200, resp)
	})

	colAPI.POST("/:id/songs", func(c *gin.Context) {
		collection, err := loadCollection(c.Param("id"))
		if err != nil {
			c.JSON(404, gin.H{"error": "歌单不存在"})
			return
		}
		if collection.isImported() {
			c.JSON(400, gin.H{"error": "外部导入歌单/专辑不保存歌曲明细，不能直接加入歌曲"})
			return
		}

		var req struct {
			SongID   string      `json:"id" binding:"required"`
			Source   string      `json:"source" binding:"required"`
			Name     string      `json:"name"`
			Artist   string      `json:"artist"`
			Cover    string      `json:"cover"`
			Duration int         `json:"duration"`
			Extra    interface{} `json:"extra"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "参数错误，缺少 id 或 source"})
			return
		}

		extraStr := ""
		if req.Extra != nil {
			if b, err := json.Marshal(req.Extra); err == nil {
				extraStr = string(b)
			}
		}

		song := SavedSong{
			CollectionID: collection.ID,
			SongID:       req.SongID,
			Source:       req.Source,
			Name:         req.Name,
			Artist:       req.Artist,
			Cover:        req.Cover,
			Duration:     req.Duration,
			Extra:        extraStr,
		}

		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&song).Error; err != nil {
			c.JSON(500, gin.H{"error": "添加失败: " + err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	colAPI.DELETE("/:id/songs", func(c *gin.Context) {
		collection, err := loadCollection(c.Param("id"))
		if err != nil {
			c.JSON(404, gin.H{"error": "歌单不存在"})
			return
		}
		if collection.isImported() {
			c.JSON(400, gin.H{"error": "外部导入歌单/专辑没有本地歌曲明细可删除"})
			return
		}

		songID := c.Query("id")
		source := c.Query("source")
		if songID == "" || source == "" {
			c.JSON(400, gin.H{"error": "需要通过 query 传递 id 和 source"})
			return
		}

		if err := db.Where(
			"collection_id = ? AND song_id = ? AND source = ?",
			collection.ID,
			songID,
			source,
		).Delete(&SavedSong{}).Error; err != nil {
			c.JSON(500, gin.H{"error": "删除失败"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})
}
