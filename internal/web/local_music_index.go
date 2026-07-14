package web

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"gorm.io/gorm/clause"
)// LocalMusicIndex 是下载目录的搜索索引行。磁盘文件仍是唯一真相，
// 该表只用于加速对大量本地文件的关键词搜索。主键沿用 base64(相对路径)。
type LocalMusicIndex struct {
	ID        string    `gorm:"column:id;primaryKey"`
	RelPath   string    `gorm:"column:rel_path;uniqueIndex;not null"`
	Name      string    `gorm:"column:name;index"`
	Artist    string    `gorm:"column:artist;index"`
	Album     string    `gorm:"column:album;index"`
	Duration  int       `gorm:"column:duration"`
	Size      int64     `gorm:"column:size"`
	Ext       string    `gorm:"column:ext"`
	Cover     string    `gorm:"column:cover"`
	HasCover  bool      `gorm:"column:has_cover"`
	HasLyric  bool      `gorm:"column:has_lyric"`
	ModTime   time.Time `gorm:"column:mod_time"`
	ScannedAt time.Time `gorm:"column:scanned_at;index"`
}

func (LocalMusicIndex) TableName() string { return "local_music_index" }

func containsLocalSource(sources []string) bool {
	for _, s := range sources {
		if isLocalMusicSource(s) {
			return true
		}
	}
	return false
}

func localMusicIndexExtra(row *LocalMusicIndex) map[string]string {
	extra := map[string]string{
		"local_music": "true",
		"file_id":     row.ID,
		"rel_path":    row.RelPath,
		"ext":         row.Ext,
	}
	if row.HasCover {
		extra["cover"] = "true"
	}
	if row.HasLyric {
		extra["lyric"] = "true"
	}
	return extra
}

func localMusicTrackToIndexRow(track *localMusicTrack, scannedAt time.Time) LocalMusicIndex {
	hasCover := strings.TrimSpace(track.Cover) != ""
	hasLyric := false
	if track.Extra != nil {
		if track.Extra["cover"] == "true" {
			hasCover = true
		}
		hasLyric = track.Extra["lyric"] == "true"
	}
	return LocalMusicIndex{
		ID:        track.ID,
		RelPath:   track.RelPath,
		Name:      track.Name,
		Artist:    track.Artist,
		Album:     track.Album,
		Duration:  track.Duration,
		Size:      track.Size,
		Ext:       track.Ext,
		Cover:     track.Cover,
		HasCover:  hasCover,
		HasLyric:  hasLyric,
		ModTime:   track.modTime,
		ScannedAt: scannedAt,
	}
}

// syncLocalMusicIndex 全量扫描下载目录并把结果 upsert 进索引表，
// 同时清扫掉本轮未出现（文件已消失）的行。
func syncLocalMusicIndex() error {
	if db == nil {
		return nil
	}
	tracks, _, _, err := scanLocalMusicTracks()
	if err != nil {
		return err
	}

	runStart := time.Now()
	rows := make([]LocalMusicIndex, 0, len(tracks))
	for _, track := range tracks {
		if track == nil {
			continue
		}
		rows = append(rows, localMusicTrackToIndexRow(track, runStart))
	}

	if len(rows) > 0 {
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"rel_path", "name", "artist", "album", "duration", "size",
				"ext", "cover", "has_cover", "has_lyric", "mod_time", "scanned_at",
			}),
		}).CreateInBatches(rows, 200).Error; err != nil {
			return err
		}
	}

	// 清扫本轮未刷新的行（对应已删除/移动的文件）。
	return db.Where("scanned_at < ?", runStart).Delete(&LocalMusicIndex{}).Error
}

// syncLocalMusicIndexAsync 在后台跑一次全量同步，不阻塞调用方（启动时用）。
func syncLocalMusicIndexAsync() {
	go func() {
		_ = syncLocalMusicIndex()
	}()
}

// upsertLocalMusicIndexRow 针对单个文件做定向 upsert（上传后用）。
func upsertLocalMusicIndexRow(track *localMusicTrack) {
	if db == nil || track == nil {
		return
	}
	row := localMusicTrackToIndexRow(track, time.Now())
	_ = db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"rel_path", "name", "artist", "album", "duration", "size",
			"ext", "cover", "has_cover", "has_lyric", "mod_time", "scanned_at",
		}),
	}).Create(&row).Error
}

// deleteLocalMusicIndexRow 删除单个索引行（删除文件后用）。
func deleteLocalMusicIndexRow(id string) {
	if db == nil || strings.TrimSpace(id) == "" {
		return
	}
	_ = db.Delete(&LocalMusicIndex{}, "id = ?", id).Error
}

// localMusicSearchSongs 在索引表里按关键词搜索本地歌曲。对返回的每行做
// os.Stat 校验，已不在磁盘上的（删除/移动）一律剔除，保证已删除本地音乐
// 不会出现在搜索结果里。
func localMusicSearchSongs(keyword string, limit int) []model.Song {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || db == nil {
		return nil
	}
	if limit <= 0 {
		limit = 200
	}

	like := "%" + keyword + "%"
	var rows []LocalMusicIndex
	if err := db.Where("name LIKE ? OR artist LIKE ? OR album LIKE ?", like, like, like).
		Order("mod_time DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil
	}

	rootAbs, err := filepath.Abs(localMusicDownloadDir())
	if err != nil {
		rootAbs = ""
	}

	songs := make([]model.Song, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		if rootAbs != "" {
			absPath := filepath.Join(rootAbs, filepath.FromSlash(row.RelPath))
			if info, statErr := os.Stat(absPath); statErr != nil || info.IsDir() {
				deleteLocalMusicIndexRow(row.ID)
				continue
			}
		}
		cover := row.Cover
		if cover == "" && row.HasCover {
			cover = RoutePrefix + "/local_music/cover?id=" + url.QueryEscape(row.ID)
		}
		songs = append(songs, model.Song{
			ID:       row.ID,
			Source:   localMusicSource,
			Name:     row.Name,
			Artist:   row.Artist,
			Album:    row.Album,
			Cover:    cover,
			Duration: row.Duration,
			Extra:    localMusicIndexExtra(row),
		})
	}
	return songs
}

// localCollectionSearchPlaylists 在本地歌单（Collection）里按名称/描述/创建者搜索，
// 返回 model.Playlist 卡片（Source=local），用于"歌单搜索 + 勾选 local"。
func localCollectionSearchPlaylists(keyword string) []model.Playlist {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || db == nil {
		return nil
	}
	like := "%" + keyword + "%"
	var collections []Collection
	if err := db.Where("name LIKE ? OR description LIKE ? OR creator LIKE ?", like, like, like).
		Order("id DESC").
		Find(&collections).Error; err != nil {
		return nil
	}
	playlists := make([]model.Playlist, 0, len(collections))
	for _, collection := range collections {
		playlists = append(playlists, collection.playlistCard())
	}
	return playlists
}

