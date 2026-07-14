package web

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dhowden/tag"
	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"gorm.io/gorm/clause"
)

const (
	localMusicSource       = "local"
	legacyLocalMusicSource = "local-file"
	localMusicScanCacheTTL = 10 * time.Second
)

var localMusicDownloadDirProvider = func() string {
	return core.GetWebSettings().DownloadDir
}

var localMusicAudioExts = map[string]struct{}{
	".aac":  {},
	".flac": {},
	".m4a":  {},
	".mp3":  {},
	".ogg":  {},
	".wav":  {},
	".wma":  {},
}

var localMusicCoverExts = []string{".jpg", ".jpeg", ".png", ".webp", ".bmp", ".gif"}
var localMusicLyricExts = []string{".lrc", ".txt", ".lyric"}
var localMusicMetaCacheMu sync.RWMutex
var localMusicMetaCache = make(map[string]*localMusicTrack)
var localMusicScanCacheMu sync.RWMutex
var localMusicScanCache localMusicScanSnapshot
var localMusicScanRefreshMu sync.Mutex
var localMusicScanRefreshInFlight bool

type localMusicScanSnapshot struct {
	Dir       string
	Tracks    []*localMusicTrack
	Exists    bool
	Err       error
	ScannedAt time.Time
}

type localMusicTrack struct {
	ID           string            `json:"id"`
	Source       string            `json:"source"`
	Name         string            `json:"name"`
	Artist       string            `json:"artist"`
	Album        string            `json:"album"`
	Cover        string            `json:"cover"`
	Duration     int               `json:"duration"`
	Filename     string            `json:"filename"`
	RelPath      string            `json:"rel_path"`
	Ext          string            `json:"ext"`
	Size         int64             `json:"size"`
	SizeText     string            `json:"size_text"`
	ModifiedAt   time.Time         `json:"modified_at"`
	Missing      []string          `json:"missing"`
	AlreadyAdded bool              `json:"already_added,omitempty"`
	Extra        map[string]string `json:"extra"`

	absPath string
	modTime time.Time
}

func RegisterLocalMusicRoutes(api *gin.RouterGroup) {
	api.GET("/local_music_page", func(c *gin.Context) {
		errMsg := ""
		tracks := []*localMusicTrack{}
		if snapshot, ok := cachedLocalMusicScanSnapshot(localMusicDownloadDir(), false); ok {
			tracks = snapshot.Tracks
			if snapshot.Err != nil {
				errMsg = "加载本地音乐失败: " + snapshot.Err.Error()
			} else if !snapshot.Exists {
				errMsg = "本地下载目录不存在，可上传音乐或在设置中调整下载目录"
			}
		}

		renderIndex(c, localMusicTracksToSongs(tracks), nil, "", nil, errMsg, "local_music", "", "", "", false, "", nil)
	})

	api.GET("/local_music", func(c *gin.Context) {
		forceRefresh := c.Query("refresh") == "1" || c.Query("force") == "1"
		tracks, dir, exists, err, refreshing, scannedAt := scanLocalMusicTracksCached(forceRefresh)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		offset := parseLocalMusicRangeInt(c.Query("offset"), 0)
		limit := parseLocalMusicRangeInt(c.Query("limit"), 0)
		pageTracks := paginateLocalMusicTracks(tracks, offset, limit)
		markAlreadyAddedLocalTracks(c.Query("collection_id"), pageTracks)
		c.JSON(http.StatusOK, gin.H{
			"download_dir": filepath.ToSlash(dir),
			"exists":       exists,
			"tracks":       pageTracks,
			"total":        len(tracks),
			"offset":       offset,
			"limit":        limit,
			"has_more":     offset+len(pageTracks) < len(tracks),
			"refreshing":   refreshing,
			"scanned_at":   scannedAt,
		})
	})

	localMusicCoverHandler := func(c *gin.Context) {
		track, err := localMusicTrackByID(c.Query("id"))
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		saveLocal := wantsSaveLocal(c)
		if saveLocal && !allowSaveLocalRequest(c) {
			return
		}

		data, mimeType, ext, err := readLocalMusicCover(track)
		if err != nil || len(data) == 0 {
			c.Status(http.StatusNotFound)
			return
		}
		if saveLocal {
			saveWebAssetResponse(c, localMusicCoverFilename(track, ext), data)
			return
		}
		if c.Query("download") == "1" {
			setDownloadHeader(c, localMusicCoverFilename(track, ext))
		}
		c.Header("Cache-Control", "public, max-age=21600")
		c.Data(http.StatusOK, mimeType, data)
	}
	api.GET("/local_music/cover", localMusicCoverHandler)
	api.POST("/local_music/cover", localMusicCoverHandler)

	api.POST("/local_music/upload", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的音乐文件"})
			return
		}

		track, err := saveUploadedLocalMusic(file)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		upsertLocalMusicIndexRow(track)

		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"track":  track,
		})
	})

	api.DELETE("/local_music", func(c *gin.Context) {
		if err := deleteLocalMusicTrack(c.Query("id")); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	colAPI := api.Group("/collections")
	colAPI.POST("/:id/local_music", func(c *gin.Context) {
		collection, err := loadCollection(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "歌单不存在"})
			return
		}
		if collection.isImported() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "外部导入歌单/专辑不支持直接添加本地音乐"})
			return
		}

		var req struct {
			ID string `json:"id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少本地音乐 ID"})
			return
		}

		track, err := localMusicTrackByID(req.ID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "本地音乐不存在或已不在下载目录内"})
			return
		}

		extra, _ := json.Marshal(track.Extra)
		song := SavedSong{
			CollectionID: collection.ID,
			SongID:       track.ID,
			Source:       localMusicSource,
			Extra:        string(extra),
			Name:         track.Name,
			Artist:       track.Artist,
			Cover:        track.Cover,
			Duration:     track.Duration,
			AddedAt:      time.Now(),
		}

		tx := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&song)
		if tx.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "添加失败: " + tx.Error.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"duplicate": tx.RowsAffected == 0,
			"song":      song,
		})
	})

	colAPI.POST("/:id/local_music/batch", func(c *gin.Context) {
		collection, err := loadCollection(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "歌单不存在"})
			return
		}
		if collection.isImported() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "外部导入歌单/专辑不支持直接添加本地音乐"})
			return
		}

		var req struct {
			IDs []string `json:"ids" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少本地音乐 ID 列表"})
			return
		}

		songs := make([]SavedSong, 0, len(req.IDs))
		failed := 0
		for _, id := range req.IDs {
			track, err := localMusicTrackByID(id)
			if err != nil {
				failed++
				continue
			}
			extra, _ := json.Marshal(track.Extra)
			songs = append(songs, SavedSong{
				CollectionID: collection.ID,
				SongID:       track.ID,
				Source:       localMusicSource,
				Extra:        string(extra),
				Name:         track.Name,
				Artist:       track.Artist,
				Cover:        track.Cover,
				Duration:     track.Duration,
				AddedAt:      time.Now(),
			})
		}

		added := 0
		if len(songs) > 0 {
			tx := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&songs)
			if tx.Error != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "批量添加失败: " + tx.Error.Error()})
				return
			}
			added = int(tx.RowsAffected)
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"requested": len(req.IDs),
			"added":     added,
			"duplicate": len(songs) - added,
			"failed":    failed,
		})
	})
}

func isLocalMusicSource(source string) bool {
	source = strings.TrimSpace(source)
	return source == localMusicSource || source == legacyLocalMusicSource
}

func localMusicTracksToSongs(tracks []*localMusicTrack) []model.Song {
	songs := make([]model.Song, 0, len(tracks))
	for _, track := range tracks {
		if track == nil {
			continue
		}
		songs = append(songs, model.Song{
			ID:       track.ID,
			Source:   localMusicSource,
			Name:     track.Name,
			Artist:   track.Artist,
			Album:    track.Album,
			Cover:    track.Cover,
			Duration: track.Duration,
			Extra:    track.Extra,
		})
	}
	return songs
}

func scanLocalMusicTracks() ([]*localMusicTrack, string, bool, error) {
	dir := localMusicDownloadDir()
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*localMusicTrack{}, dir, false, nil
		}
		return nil, dir, false, err
	}
	if !info.IsDir() {
		return nil, dir, false, fmt.Errorf("本地下载路径不是目录: %s", dir)
	}

	rootAbs, err := filepath.Abs(dir)
	if err != nil {
		return nil, dir, true, err
	}

	tracks := make([]*localMusicTrack, 0)
	err = filepath.WalkDir(rootAbs, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || strings.HasPrefix(entry.Name(), ".") {
				if path != rootAbs {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !isLocalMusicAudioFile(path) {
			return nil
		}

		track, err := buildLocalMusicTrackFast(rootAbs, path)
		if err == nil {
			tracks = append(tracks, track)
		}
		return nil
	})
	if err != nil {
		return nil, dir, true, err
	}

	sort.SliceStable(tracks, func(i, j int) bool {
		if !tracks[i].modTime.Equal(tracks[j].modTime) {
			return tracks[i].modTime.After(tracks[j].modTime)
		}
		return strings.ToLower(tracks[i].RelPath) < strings.ToLower(tracks[j].RelPath)
	})

	return tracks, dir, true, nil
}

func scanLocalMusicTracksCached(force bool) ([]*localMusicTrack, string, bool, error, bool, time.Time) {
	dir := localMusicDownloadDir()
	if !force {
		if snapshot, ok := cachedLocalMusicScanSnapshot(dir, false); ok {
			if time.Since(snapshot.ScannedAt) < localMusicScanCacheTTL {
				return snapshot.Tracks, snapshot.Dir, snapshot.Exists, snapshot.Err, false, snapshot.ScannedAt
			}
			if snapshot.Err == nil {
				refreshLocalMusicScanAsync(dir)
				return snapshot.Tracks, snapshot.Dir, snapshot.Exists, snapshot.Err, true, snapshot.ScannedAt
			}
		}
	}

	tracks, scanDir, exists, err := scanLocalMusicTracks()
	snapshot := localMusicScanSnapshot{
		Dir:       scanDir,
		Tracks:    cloneLocalMusicTrackSlice(tracks),
		Exists:    exists,
		Err:       err,
		ScannedAt: time.Now(),
	}
	storeLocalMusicScanSnapshot(snapshot)
	return cloneLocalMusicTrackSlice(tracks), scanDir, exists, err, false, snapshot.ScannedAt
}

func refreshLocalMusicScanAsync(dir string) {
	localMusicScanRefreshMu.Lock()
	if localMusicScanRefreshInFlight {
		localMusicScanRefreshMu.Unlock()
		return
	}
	localMusicScanRefreshInFlight = true
	localMusicScanRefreshMu.Unlock()

	go func() {
		defer func() {
			localMusicScanRefreshMu.Lock()
			localMusicScanRefreshInFlight = false
			localMusicScanRefreshMu.Unlock()
		}()

		tracks, scanDir, exists, err := scanLocalMusicTracks()
		if filepath.Clean(scanDir) != filepath.Clean(dir) {
			return
		}
		storeLocalMusicScanSnapshot(localMusicScanSnapshot{
			Dir:       scanDir,
			Tracks:    cloneLocalMusicTrackSlice(tracks),
			Exists:    exists,
			Err:       err,
			ScannedAt: time.Now(),
		})
	}()
}

func cachedLocalMusicScanSnapshot(dir string, freshOnly bool) (localMusicScanSnapshot, bool) {
	localMusicScanCacheMu.RLock()
	snapshot := localMusicScanCache
	localMusicScanCacheMu.RUnlock()

	if strings.TrimSpace(snapshot.Dir) == "" || filepath.Clean(snapshot.Dir) != filepath.Clean(dir) {
		return localMusicScanSnapshot{}, false
	}
	if freshOnly && time.Since(snapshot.ScannedAt) >= localMusicScanCacheTTL {
		return localMusicScanSnapshot{}, false
	}
	snapshot.Tracks = cloneLocalMusicTrackSlice(snapshot.Tracks)
	return snapshot, true
}

func storeLocalMusicScanSnapshot(snapshot localMusicScanSnapshot) {
	snapshot.Tracks = cloneLocalMusicTrackSlice(snapshot.Tracks)
	localMusicScanCacheMu.Lock()
	localMusicScanCache = snapshot
	localMusicScanCacheMu.Unlock()
}

func invalidateLocalMusicScanCache() {
	localMusicScanCacheMu.Lock()
	localMusicScanCache = localMusicScanSnapshot{}
	localMusicScanCacheMu.Unlock()
}

func parseLocalMusicRangeInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 0 {
		return fallback
	}
	if value > 1000 {
		return 1000
	}
	return value
}

func paginateLocalMusicTracks(tracks []*localMusicTrack, offset int, limit int) []*localMusicTrack {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(tracks) {
		return []*localMusicTrack{}
	}
	if limit <= 0 || offset+limit > len(tracks) {
		limit = len(tracks) - offset
	}
	return tracks[offset : offset+limit]
}

func markAlreadyAddedLocalTracks(collectionID string, tracks []*localMusicTrack) {
	if strings.TrimSpace(collectionID) == "" || len(tracks) == 0 || db == nil {
		return
	}

	collection, err := loadCollection(collectionID)
	if err != nil || collection.isImported() {
		return
	}

	ids := make([]string, 0, len(tracks))
	for _, track := range tracks {
		ids = append(ids, track.ID)
	}

	var saved []SavedSong
	if err := db.Where(
		"collection_id = ? AND source IN ? AND song_id IN ?",
		collection.ID,
		[]string{localMusicSource, legacyLocalMusicSource},
		ids,
	).Find(&saved).Error; err != nil {
		return
	}

	added := make(map[string]struct{}, len(saved))
	for _, song := range saved {
		added[song.SongID] = struct{}{}
	}
	for _, track := range tracks {
		_, track.AlreadyAdded = added[track.ID]
	}
}

func localMusicDownloadDir() string {
	dir := strings.TrimSpace(localMusicDownloadDirProvider())
	if dir == "" {
		dir = core.DefaultWebDownloadDir
	}
	return filepath.Clean(dir)
}

func isLocalMusicAudioFile(path string) bool {
	_, ok := localMusicAudioExts[strings.ToLower(filepath.Ext(path))]
	return ok
}

func buildLocalMusicTrackFast(rootAbs string, audioPath string) (*localMusicTrack, error) {
	track, err := buildLocalMusicTrackFallback(rootAbs, audioPath)
	if err != nil {
		return nil, err
	}
	if cached := getCachedLocalMusicTrack(rootAbs, track.RelPath, track.Size, track.modTime); cached != nil {
		cached.absPath = track.absPath
		cached.modTime = track.modTime
		return cached, nil
	}
	return buildLocalMusicTrack(rootAbs, audioPath)
}

func buildLocalMusicTrackFallback(rootAbs string, audioPath string) (*localMusicTrack, error) {
	absPath, err := filepath.Abs(audioPath)
	if err != nil {
		return nil, err
	}
	if !isPathInside(rootAbs, absPath) {
		return nil, errors.New("path is outside local music dir")
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() || !isLocalMusicAudioFile(absPath) {
		return nil, errors.New("not a supported audio file")
	}

	rel, err := filepath.Rel(rootAbs, absPath)
	if err != nil {
		return nil, err
	}
	rel = filepath.ToSlash(rel)

	filename := info.Name()
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	fallbackName := strings.TrimSuffix(filename, filepath.Ext(filename))
	id := encodeLocalMusicID(rel)
	extra := map[string]string{
		"local_music": "true",
		"file_id":     id,
		"filename":    filename,
		"rel_path":    rel,
		"ext":         ext,
		"size":        strconv.FormatInt(info.Size(), 10),
	}
	cover := ""
	if _, _, ok := localMusicExactSidecarFile(absPath, localMusicCoverExts); ok {
		cover = RoutePrefix + "/local_music/cover?id=" + url.QueryEscape(id)
		extra["cover"] = "true"
		extra["cover_source"] = "sidecar"
	}
	if _, _, ok := localMusicExactSidecarFile(absPath, localMusicLyricExts); ok {
		extra["lyric"] = "true"
		extra["lyric_source"] = "sidecar"
	}

	return &localMusicTrack{
		ID:         id,
		Source:     localMusicSource,
		Name:       strings.TrimSpace(fallbackName),
		Artist:     "未知歌手",
		Album:      "",
		Cover:      cover,
		Duration:   0,
		Filename:   filename,
		RelPath:    rel,
		Ext:        ext,
		Size:       info.Size(),
		SizeText:   core.FormatSize(info.Size()),
		ModifiedAt: info.ModTime(),
		Missing:    []string{"title", "artist", "album"},
		Extra:      extra,
		absPath:    absPath,
		modTime:    info.ModTime(),
	}, nil
}

func buildLocalMusicTrack(rootAbs string, audioPath string) (*localMusicTrack, error) {
	absPath, err := filepath.Abs(audioPath)
	if err != nil {
		return nil, err
	}
	if !isPathInside(rootAbs, absPath) {
		return nil, errors.New("path is outside local music dir")
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() || !isLocalMusicAudioFile(absPath) {
		return nil, errors.New("not a supported audio file")
	}

	rel, err := filepath.Rel(rootAbs, absPath)
	if err != nil {
		return nil, err
	}
	rel = filepath.ToSlash(rel)

	filename := info.Name()
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	if cached := getCachedLocalMusicTrack(rootAbs, rel, info.Size(), info.ModTime()); cached != nil {
		cached.absPath = absPath
		cached.modTime = info.ModTime()
		return cached, nil
	}
	fallbackName := strings.TrimSuffix(filename, filepath.Ext(filename))
	name := ""
	artist := ""
	album := ""
	hasEmbeddedCover := false
	hasEmbeddedLyric := false

	if file, err := os.Open(absPath); err == nil {
		if metadata, readErr := tag.ReadFrom(file); readErr == nil {
			name = strings.TrimSpace(metadata.Title())
			artist = strings.TrimSpace(metadata.Artist())
			album = strings.TrimSpace(metadata.Album())
			if picture := metadata.Picture(); picture != nil && len(picture.Data) > 0 {
				hasEmbeddedCover = true
			}
			hasEmbeddedLyric = strings.TrimSpace(metadata.Lyrics()) != ""
		}
		_ = file.Close()
	}

	missing := make([]string, 0, 3)
	if strings.TrimSpace(name) == "" {
		name = fallbackName
		missing = append(missing, "title")
	}
	if strings.TrimSpace(artist) == "" {
		artist = "未知歌手"
		missing = append(missing, "artist")
	}
	if strings.TrimSpace(album) == "" {
		missing = append(missing, "album")
	}

	id := encodeLocalMusicID(rel)
	extra := map[string]string{
		"local_music": "true",
		"file_id":     id,
		"filename":    filename,
		"rel_path":    rel,
		"ext":         ext,
		"size":        strconv.FormatInt(info.Size(), 10),
	}
	if album != "" {
		extra["album"] = album
	}

	cover := ""
	if hasEmbeddedCover {
		cover = RoutePrefix + "/local_music/cover?id=" + url.QueryEscape(id)
		extra["cover"] = "true"
		extra["cover_source"] = "embedded"
	} else if _, _, ok := localMusicSidecarFile(absPath, localMusicCoverExts); ok {
		cover = RoutePrefix + "/local_music/cover?id=" + url.QueryEscape(id)
		extra["cover"] = "true"
		extra["cover_source"] = "sidecar"
	}

	if hasEmbeddedLyric {
		extra["lyric"] = "true"
		extra["lyric_source"] = "embedded"
	} else if _, _, ok := localMusicSidecarFile(absPath, localMusicLyricExts); ok {
		extra["lyric"] = "true"
		extra["lyric_source"] = "sidecar"
	}

	track := &localMusicTrack{
		ID:         id,
		Source:     localMusicSource,
		Name:       strings.TrimSpace(name),
		Artist:     strings.TrimSpace(artist),
		Album:      strings.TrimSpace(album),
		Cover:      cover,
		Duration:   0,
		Filename:   filename,
		RelPath:    rel,
		Ext:        ext,
		Size:       info.Size(),
		SizeText:   core.FormatSize(info.Size()),
		ModifiedAt: info.ModTime(),
		Missing:    missing,
		Extra:      extra,
		absPath:    absPath,
		modTime:    info.ModTime(),
	}
	if probe, err := probeLocalMusicTrack(track); err == nil && probe != nil {
		applyLocalProbeResult(track, probe)
	}
	cacheLocalMusicTrack(rootAbs, track)
	return track, nil
}

func getCachedLocalMusicTrack(rootAbs string, relPath string, size int64, modTime time.Time) *localMusicTrack {
	localMusicMetaCacheMu.RLock()
	defer localMusicMetaCacheMu.RUnlock()
	track := localMusicMetaCache[localMusicMetaCacheKey(rootAbs, relPath)]
	if track == nil || track.Size != size || !track.modTime.Equal(modTime) {
		return nil
	}
	return cloneLocalMusicTrack(track)
}

func cacheLocalMusicTrack(rootAbs string, track *localMusicTrack) {
	if track == nil || strings.TrimSpace(track.RelPath) == "" {
		return
	}
	localMusicMetaCacheMu.Lock()
	localMusicMetaCache[localMusicMetaCacheKey(rootAbs, track.RelPath)] = cloneLocalMusicTrack(track)
	localMusicMetaCacheMu.Unlock()
}

func localMusicMetaCacheKey(rootAbs string, relPath string) string {
	root, err := filepath.Abs(rootAbs)
	if err != nil {
		root = rootAbs
	}
	return filepath.Clean(root) + "|" + filepath.ToSlash(relPath)
}

func cloneLocalMusicTrack(track *localMusicTrack) *localMusicTrack {
	if track == nil {
		return nil
	}
	next := *track
	if track.Missing != nil {
		next.Missing = append([]string(nil), track.Missing...)
	}
	if track.Extra != nil {
		next.Extra = make(map[string]string, len(track.Extra))
		for key, value := range track.Extra {
			next.Extra[key] = value
		}
	}
	return &next
}

func cloneLocalMusicTrackSlice(tracks []*localMusicTrack) []*localMusicTrack {
	if len(tracks) == 0 {
		return []*localMusicTrack{}
	}
	cloned := make([]*localMusicTrack, 0, len(tracks))
	for _, track := range tracks {
		if track == nil {
			continue
		}
		cloned = append(cloned, cloneLocalMusicTrack(track))
	}
	return cloned
}

func localMusicTrackByID(id string) (*localMusicTrack, error) {
	rel, err := decodeLocalMusicID(id)
	if err != nil {
		return nil, err
	}
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return nil, errors.New("empty local music id")
	}

	cleanRel := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(cleanRel) || cleanRel == "." || cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) {
		return nil, errors.New("invalid local music path")
	}

	rootAbs, err := filepath.Abs(localMusicDownloadDir())
	if err != nil {
		return nil, err
	}
	audioPath := filepath.Join(rootAbs, cleanRel)
	absPath, err := filepath.Abs(audioPath)
	if err != nil {
		return nil, err
	}
	if !isPathInside(rootAbs, absPath) {
		return nil, errors.New("local music path escaped root")
	}

	return buildLocalMusicTrack(rootAbs, absPath)
}

func encodeLocalMusicID(relPath string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(filepath.ToSlash(relPath)))
}

func decodeLocalMusicID(id string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(id))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func isPathInside(rootAbs string, targetAbs string) bool {
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func saveUploadedLocalMusic(file *multipart.FileHeader) (*localMusicTrack, error) {
	filename, err := sanitizeLocalMusicUploadName(file.Filename)
	if err != nil {
		return nil, err
	}

	dir := localMusicDownloadDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	rootAbs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	dstPath := uniqueLocalMusicPath(rootAbs, filename)

	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return nil, err
	}
	_, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil {
		_ = os.Remove(dstPath)
		return nil, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dstPath)
		return nil, closeErr
	}

	track, err := buildLocalMusicTrack(rootAbs, dstPath)
	if err == nil {
		invalidateLocalMusicScanCache()
	}
	return track, err
}

func sanitizeLocalMusicUploadName(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	ext := strings.ToLower(filepath.Ext(name))
	if _, ok := localMusicAudioExts[ext]; !ok {
		return "", fmt.Errorf("仅支持 mp3、flac、m4a、ogg、wav、wma、aac 音频文件")
	}

	base := strings.TrimSuffix(name, filepath.Ext(name))
	base = strings.TrimSpace(utils.SanitizeFilename(base))
	if base == "" {
		base = "local-music"
	}
	return base + ext, nil
}

func uniqueLocalMusicPath(dir string, filename string) string {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	candidate := filepath.Join(dir, filename)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	for i := 1; ; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

type localProbeResult struct {
	Duration int
	Bitrate  int
	Title    string
	Artist   string
	Album    string
}

func probeLocalMusicTrack(track *localMusicTrack) (*localProbeResult, error) {
	if track == nil || strings.TrimSpace(track.absPath) == "" {
		return nil, errors.New("empty local music track")
	}
	ffprobePath, err := core.ResolveFFprobePath()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(ffprobePath, "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", track.absPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var payload struct {
		Format struct {
			Duration string            `json:"duration"`
			BitRate  string            `json:"bit_rate"`
			Tags     map[string]string `json:"tags"`
		} `json:"format"`
		Streams []struct {
			CodecType string            `json:"codec_type"`
			Duration  string            `json:"duration"`
			BitRate   string            `json:"bit_rate"`
			Tags      map[string]string `json:"tags"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, err
	}

	result := &localProbeResult{
		Duration: secondsFromProbe(payload.Format.Duration),
		Bitrate:  kbpsFromProbe(payload.Format.BitRate),
		Title:    probeTag(payload.Format.Tags, "title"),
		Artist:   probeTag(payload.Format.Tags, "artist"),
		Album:    probeTag(payload.Format.Tags, "album"),
	}

	for _, stream := range payload.Streams {
		if stream.CodecType != "audio" {
			continue
		}
		if result.Duration <= 0 {
			result.Duration = secondsFromProbe(stream.Duration)
		}
		if result.Bitrate <= 0 {
			result.Bitrate = kbpsFromProbe(stream.BitRate)
		}
		if result.Title == "" {
			result.Title = probeTag(stream.Tags, "title")
		}
		if result.Artist == "" {
			result.Artist = probeTag(stream.Tags, "artist")
		}
		if result.Album == "" {
			result.Album = probeTag(stream.Tags, "album")
		}
		break
	}

	return result, nil
}

func applyLocalProbeResult(track *localMusicTrack, probe *localProbeResult) {
	if track == nil || probe == nil {
		return
	}
	if track.Extra == nil {
		track.Extra = make(map[string]string)
	}
	if probe.Duration > 0 {
		track.Duration = probe.Duration
		track.Extra["duration"] = strconv.Itoa(probe.Duration)
	}
	if probe.Title != "" && containsString(track.Missing, "title") {
		track.Name = probe.Title
		track.Extra["title"] = probe.Title
		track.Missing = removeString(track.Missing, "title")
	}
	if probe.Artist != "" && containsString(track.Missing, "artist") {
		track.Artist = probe.Artist
		track.Extra["artist"] = probe.Artist
		track.Missing = removeString(track.Missing, "artist")
	}
	if probe.Album != "" && containsString(track.Missing, "album") {
		track.Album = probe.Album
		track.Extra["album"] = probe.Album
		track.Missing = removeString(track.Missing, "album")
	}
	if probe.Bitrate > 0 {
		track.Extra["bitrate"] = strconv.Itoa(probe.Bitrate)
	}
}

func secondsFromProbe(raw string) int {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || value <= 0 {
		return 0
	}
	return int(value + 0.5)
}

func kbpsFromProbe(raw string) int {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return 0
	}
	return int(value / 1000)
}

func probeTag(tags map[string]string, key string) string {
	if len(tags) == 0 {
		return ""
	}
	for k, v := range tags {
		if strings.EqualFold(strings.TrimSpace(k), key) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func localMusicSidecarFile(audioPath string, exts []string) (string, string, bool) {
	basePath := strings.TrimSuffix(audioPath, filepath.Ext(audioPath))
	for _, ext := range exts {
		candidate := basePath + ext
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, ext, true
		}
	}

	dir := filepath.Dir(audioPath)
	baseName := filepath.Base(basePath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		entryExt := strings.ToLower(filepath.Ext(entry.Name()))
		if !containsString(exts, entryExt) {
			continue
		}
		entryBase := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if strings.EqualFold(entryBase, baseName) {
			return filepath.Join(dir, entry.Name()), entryExt, true
		}
	}
	return "", "", false
}

func localMusicExactSidecarFile(audioPath string, exts []string) (string, string, bool) {
	basePath := strings.TrimSuffix(audioPath, filepath.Ext(audioPath))
	for _, ext := range exts {
		candidate := basePath + ext
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, ext, true
		}
	}
	return "", "", false
}

func readLocalMusicPicture(audioPath string) (*tag.Picture, error) {
	file, err := os.Open(audioPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	metadata, err := tag.ReadFrom(file)
	if err != nil {
		return nil, err
	}
	return metadata.Picture(), nil
}

func readLocalMusicLyrics(audioPath string) (string, error) {
	file, err := os.Open(audioPath)
	if err == nil {
		metadata, readErr := tag.ReadFrom(file)
		_ = file.Close()
		if readErr == nil {
			if lyrics := strings.TrimSpace(metadata.Lyrics()); lyrics != "" {
				return lyrics, nil
			}
		}
	} else {
		return "", err
	}

	sidecarPath, _, ok := localMusicSidecarFile(audioPath, localMusicLyricExts)
	if !ok {
		return "", errors.New("local lyric not found")
	}
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		return "", err
	}
	lyrics := strings.TrimSpace(string(data))
	if lyrics == "" {
		return "", errors.New("local lyric is empty")
	}
	return lyrics, nil
}

func readLocalMusicCover(track *localMusicTrack) ([]byte, string, string, error) {
	if track == nil {
		return nil, "", "", errors.New("empty local music track")
	}

	picture, err := readLocalMusicPicture(track.absPath)
	if err == nil && picture != nil && len(picture.Data) > 0 {
		mimeType := strings.TrimSpace(picture.MIMEType)
		if mimeType == "" {
			mimeType = "image/jpeg"
		}
		return picture.Data, mimeType, imageExtByMime(mimeType), nil
	}

	sidecarPath, ext, ok := localMusicSidecarFile(track.absPath, localMusicCoverExts)
	if !ok {
		return nil, "", "", errors.New("local cover not found")
	}
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		return nil, "", "", err
	}
	mimeType := localImageMimeByExt(ext)
	return data, mimeType, ext, nil
}

func localImageMimeByExt(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".gif":
		return "image/gif"
	default:
		if mimeType := mime.TypeByExtension(ext); strings.HasPrefix(mimeType, "image/") {
			return mimeType
		}
		return "image/jpeg"
	}
}

func imageExtByMime(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/bmp", "image/x-ms-bmp":
		return ".bmp"
	case "image/gif":
		return ".gif"
	default:
		return ".jpg"
	}
}

func localMusicCoverFilename(track *localMusicTrack, ext string) string {
	if strings.TrimSpace(ext) == "" {
		ext = ".jpg"
	}
	name := strings.TrimSpace(track.Name)
	if name == "" {
		name = strings.TrimSuffix(track.Filename, filepath.Ext(track.Filename))
	}
	artist := strings.TrimSpace(track.Artist)
	if artist == "" {
		artist = "Unknown"
	}
	return utils.SanitizeFilename(fmt.Sprintf("%s - %s%s", name, artist, ext))
}

func localMusicLyricFilename(track *localMusicTrack) string {
	name := strings.TrimSpace(track.Name)
	if name == "" {
		name = strings.TrimSuffix(track.Filename, filepath.Ext(track.Filename))
	}
	artist := strings.TrimSpace(track.Artist)
	if artist == "" {
		artist = "Unknown"
	}
	return utils.SanitizeFilename(fmt.Sprintf("%s - %s.lrc", name, artist))
}

func serveLocalMusicLyric(c *gin.Context, song *model.Song, download bool, saveLocal ...bool) {
	if song == nil {
		c.String(http.StatusNotFound, "Lyric not found")
		return
	}
	track, err := localMusicTrackByID(song.ID)
	if err != nil {
		if download {
			c.String(http.StatusNotFound, "Lyric not found")
		} else {
			c.String(http.StatusOK, "[00:00.00] 纯音乐 / 无歌词")
		}
		return
	}

	lyrics, err := readLocalMusicLyrics(track.absPath)
	if err != nil || strings.TrimSpace(lyrics) == "" {
		if download {
			c.String(http.StatusNotFound, "Lyric not found")
		} else {
			c.String(http.StatusOK, "[00:00.00] 纯音乐 / 无歌词")
		}
		return
	}

	lyrics = formatLyricForMode(lyrics, c.DefaultQuery("format", "auto"))
	c.Header("X-Lyric-Format", classifyLyricFormat(lyrics))
	if download {
		if len(saveLocal) > 0 && saveLocal[0] {
			saveWebAssetResponse(c, localMusicLyricFilename(track), []byte(lyrics))
			return
		}
		setDownloadHeader(c, localMusicLyricFilename(track))
	}
	c.String(http.StatusOK, lyrics)
}

func inspectLocalMusicFile(id string, duration string) (gin.H, error) {
	track, err := localMusicTrackByID(id)
	if err != nil {
		return gin.H{"valid": false}, err
	}

	if probe, err := probeLocalMusicTrack(track); err == nil && probe != nil {
		applyLocalProbeResult(track, probe)
	}
	if rootAbs, err := filepath.Abs(localMusicDownloadDir()); err == nil {
		cacheLocalMusicTrack(rootAbs, track)
	}

	bitrate := "-"
	if kbps, _ := strconv.Atoi(track.Extra["bitrate"]); kbps > 0 {
		bitrate = fmt.Sprintf("%d kbps", kbps)
	} else if seconds := track.Duration; seconds > 0 && track.Size > 0 {
		bitrate = fmt.Sprintf("%d kbps", int((track.Size*8)/int64(seconds)/1000))
	} else if seconds, _ := strconv.Atoi(strings.TrimSpace(duration)); seconds > 0 && track.Size > 0 {
		bitrate = fmt.Sprintf("%d kbps", int((track.Size*8)/int64(seconds)/1000))
	}

	return gin.H{
		"valid":    true,
		"url":      "",
		"size":     track.SizeText,
		"bitrate":  bitrate,
		"duration": track.Duration,
		"song": gin.H{
			"id":       track.ID,
			"source":   track.Source,
			"name":     track.Name,
			"artist":   track.Artist,
			"album":    track.Album,
			"cover":    track.Cover,
			"duration": track.Duration,
			"extra":    track.Extra,
		},
	}, nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	filtered := values[:0]
	for _, value := range values {
		if value != target {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func deleteLocalMusicTrack(id string) error {
	track, err := localMusicTrackByID(id)
	if err != nil {
		return errors.New("本地音乐不存在或已不在下载目录内")
	}
	// 硬删除：删磁盘文件 + 删索引行。收藏歌单里的引用条目保留，
	// 之后在歌单详情页会显示为失效，可换源到在线源。
	if err := os.Remove(track.absPath); err != nil {
		return err
	}
	deleteLocalMusicIndexRow(track.ID)
	invalidateLocalMusicScanCache()
	return nil
}

func serveLocalMusicDownload(c *gin.Context, id string, saveLocal bool) {
	track, err := localMusicTrackByID(id)
	if err != nil {
		c.String(http.StatusNotFound, "Local music not found")
		return
	}

	if saveLocal {
		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"saved":    true,
			"path":     track.absPath,
			"filename": track.Filename,
		})
		return
	}

	file, err := os.Open(track.absPath)
	if err != nil {
		c.String(http.StatusNotFound, "Local music not found")
		return
	}
	defer file.Close()

	c.Header("Content-Type", localAudioMimeByExt(track.Ext))
	setDownloadHeader(c, track.Filename)
	http.ServeContent(c.Writer, c.Request, track.Filename, track.modTime, file)
}

func localAudioMimeByExt(ext string) string {
	switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
	case "aac":
		return "audio/aac"
	case "wav":
		return "audio/wav"
	default:
		return core.AudioMimeByExt(ext)
	}
}
