package core

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	DownloadStatusSuccess = "success"
	DownloadStatusSkipped = "skipped"
	DownloadStatusFailed  = "failed"
)

// DownloadRecord keeps the user-visible download history in SQLite.
type DownloadRecord struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"size:512;not null;index"`
	Artist    string    `gorm:"size:512;not null;index"`
	Source    string    `gorm:"size:64;not null"`
	Status    string    `gorm:"size:32;not null;index"`
	Error     string    `gorm:"size:1024"`
	RelPath   string    `gorm:"size:1024"`
	CreatedAt time.Time `gorm:"autoCreateTime;index"`
}

// DownloadDedupEntry is intentionally separate from the visible history. Clearing
// the history therefore does not make previously downloaded songs downloadable again.
// RelPath 记录这首歌落在下载目录内的相对路径：文件被删除后要靠它回收这条记录，
// 否则再下载会被判定为「已下载」而跳过。
type DownloadDedupEntry struct {
	SongKey   string    `gorm:"primaryKey;size:1024"`
	Name      string    `gorm:"size:512;not null"`
	Artist    string    `gorm:"size:512;not null"`
	RelPath   string    `gorm:"size:1024"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// DownloadDedupIndex 是去重索引的内存视图：key 为 SongKey，value 为记录到的
// 下载目录内相对路径（空字符串表示旧数据，路径未知）。
type DownloadDedupIndex map[string]string

func initDownloadRecordTable() error {
	if err := ensureConfigDB(); err != nil {
		return err
	}
	return configDB.AutoMigrate(&DownloadRecord{}, &DownloadDedupEntry{})
}

// SaveDownloadRecord persists one download outcome and records successful songs in
// the durable de-duplication index. Control characters are removed before writing.
func SaveDownloadRecord(name, artist, source, status, errStr string) error {
	return SaveDownloadRecordWithRelPath(name, artist, source, status, errStr, "")
}

// SaveDownloadRecordWithRelPath 同 SaveDownloadRecord，额外记录文件在下载目录内的
// 相对路径，供后续校验文件是否还在磁盘上。
func SaveDownloadRecordWithRelPath(name, artist, source, status, errStr, relPath string) error {
	if err := initDownloadRecordTable(); err != nil {
		return err
	}

	name = cleanDownloadRecordText(name)
	artist = cleanDownloadRecordText(artist)
	relPath = cleanDownloadRelPath(relPath)
	record := DownloadRecord{
		Name:    name,
		Artist:  artist,
		Source:  cleanDownloadRecordText(source),
		Status:  cleanDownloadRecordText(status),
		Error:   cleanDownloadRecordText(errStr),
		RelPath: relPath,
	}

	return configDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		if record.Status != DownloadStatusSuccess {
			return nil
		}

		return saveDownloadDedupEntry(tx, record.Name, record.Artist, relPath)
	})
}

// SaveDownloadDedupEntry records a song as already available locally without
// adding an item to the user-visible download history.
func SaveDownloadDedupEntry(name, artist, relPath string) error {
	if err := initDownloadRecordTable(); err != nil {
		return err
	}
	return saveDownloadDedupEntry(configDB, name, artist, relPath)
}

func saveDownloadDedupEntry(db *gorm.DB, name, artist, relPath string) error {
	name = cleanDownloadRecordText(name)
	artist = cleanDownloadRecordText(artist)
	relPath = cleanDownloadRelPath(relPath)
	entry := DownloadDedupEntry{
		SongKey: songKeyFromParts(name, artist),
		Name:    name,
		Artist:  artist,
		RelPath: relPath,
	}
	// 路径未知时不能覆盖已有路径，只有拿到了真实路径才回填。
	if relPath == "" {
		return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entry).Error
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "song_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"rel_path"}),
	}).Create(&entry).Error
}

func cleanDownloadRecordText(value string) string {
	return strings.TrimSpace(stripControl(value))
}

// cleanDownloadRelPath 归一化下载目录内的相对路径：去掉控制字符、统一成斜杠、
// 去掉开头斜杠，保证跨平台存进 SQLite 的形态一致。
func cleanDownloadRelPath(relPath string) string {
	relPath = strings.ReplaceAll(strings.TrimSpace(stripControl(relPath)), "\\", "/")
	relPath = strings.TrimSpace(relPath)
	for strings.HasPrefix(relPath, "/") {
		relPath = strings.TrimPrefix(relPath, "/")
	}
	return relPath
}

// GetDownloadRecords returns the most recent 200 user-visible download records.
func GetDownloadRecords() ([]DownloadRecord, error) {
	records, _, err := GetDownloadRecordPage(1, 200)
	return records, err
}

// GetDownloadRecordPage returns one page of user-visible download records and
// the total number of records available for pagination.
func GetDownloadRecordPage(page, pageSize int) ([]DownloadRecord, int64, error) {
	if err := initDownloadRecordTable(); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	var total int64
	if err := configDB.Model(&DownloadRecord{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []DownloadRecord
	err := configDB.Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error
	return records, total, err
}

// GetDownloadRecord returns one user-visible history row by its stable database ID.
func GetDownloadRecord(id uint) (DownloadRecord, error) {
	if err := initDownloadRecordTable(); err != nil {
		return DownloadRecord{}, err
	}
	var record DownloadRecord
	err := configDB.Where("id = ?", id).First(&record).Error
	return record, err
}

// ResolveDownloadRecordRelPath returns the file path recorded with the history
// row, falling back to the durable de-duplication index for records created by
// older versions that did not persist RelPath directly.
func ResolveDownloadRecordRelPath(record DownloadRecord) (string, error) {
	if relPath := cleanDownloadRelPath(record.RelPath); relPath != "" {
		return relPath, nil
	}
	index, err := LoadDownloadDedupSet()
	if err != nil {
		return "", err
	}
	_, relPath, _ := lookupSongKey(&model.Song{Name: record.Name, Artist: record.Artist}, index)
	return cleanDownloadRelPath(relPath), nil
}

// ClearDownloadRecords clears only the history displayed in the UI. The durable
// de-duplication index is retained so the download decision remains correct.
func ClearDownloadRecords() error {
	if err := initDownloadRecordTable(); err != nil {
		return err
	}
	return configDB.Where("1 = 1").Delete(&DownloadRecord{}).Error
}

// SongKey generates a stable de-duplication key from artist and title.
func SongKey(song *model.Song) string {
	artist := normalizeVariantText(cleanDownloadRecordText(song.Artist))
	name := normalizeVariantText(cleanDownloadRecordText(song.Name))
	if artist == "" {
		artist = "Unknown"
	}
	if name == "" {
		name = "Unknown"
	}
	return artist + " - " + name
}

func songKeyFromParts(name, artist string) string {
	return SongKey(&model.Song{Name: name, Artist: artist})
}

func lookupSongKey(song *model.Song, index DownloadDedupIndex) (string, string, bool) {
	if index == nil {
		return "", "", false
	}
	key := SongKey(song)
	if relPath, ok := index[key]; ok {
		return key, relPath, true
	}

	normalizedKey := normalizeVariantText(key)
	for existingKey, relPath := range index {
		if normalizeVariantText(existingKey) == normalizedKey {
			return existingKey, relPath, true
		}
	}
	return "", "", false
}

func stripControl(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r >= 0x20 && r != 0x7f {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

// DownloadDirOrDefault 返回实际使用的下载目录（空值回落到默认目录）。
func DownloadDirOrDefault(outDir string) string {
	dir := strings.TrimSpace(outDir)
	if dir == "" {
		dir = DefaultWebDownloadDir
	}
	return filepath.Clean(dir)
}

// inspectDownloadFile 返回文件是否存在以及判断是否明确。权限错误、临时 I/O
// 故障或挂载异常返回 known=false，调用方必须保守处理，不能当作文件已删除。
func inspectDownloadFile(outDir, relPath string) (exists bool, known bool) {
	relPath = cleanDownloadRelPath(relPath)
	if relPath == "" {
		return false, false
	}
	info, err := os.Stat(filepath.Join(DownloadDirOrDefault(outDir), filepath.FromSlash(relPath)))
	if err == nil {
		return !info.IsDir(), true
	}
	if os.IsNotExist(err) {
		return false, true
	}
	return false, false
}

// DownloadFileExists 判断下载目录内的相对路径是否仍然指向一个真实文件。
func DownloadFileExists(outDir, relPath string) bool {
	exists, _ := inspectDownloadFile(outDir, relPath)
	return exists
}

// SongFileRelPath 返回去重索引里记录的文件相对路径（"" 表示没有记录或路径未知）。
func SongFileRelPath(song *model.Song, index DownloadDedupIndex) string {
	_, relPath, _ := lookupSongKey(song, index)
	return relPath
}

// IsSongDownloaded 只查去重索引，不校验磁盘。需要判断「现在还能不能跳过」时用
// IsSongStillDownloaded。
func IsSongDownloaded(song *model.Song, index DownloadDedupIndex) bool {
	_, _, exists := lookupSongKey(song, index)
	return exists
}

// IsSongStillDownloaded 在去重索引命中的基础上校验文件是否还在下载目录里。
// 记录里没有路径（旧数据）时按「仍在」处理，由上层用本地曲库索引反查。
// 一旦确认文件已被删除，会顺手回收这条去重记录，避免以后一直被判定为已下载。
func IsSongStillDownloaded(song *model.Song, index DownloadDedupIndex, outDir string) bool {
	if index == nil {
		return false
	}
	key, relPath, exists := lookupSongKey(song, index)
	if !exists {
		return false
	}
	if cleanDownloadRelPath(relPath) == "" {
		return true
	}
	exists, known := inspectDownloadFile(outDir, relPath)
	if !known || exists {
		return true
	}
	_ = forgetDownloadedSongKeys(key)
	delete(index, key)
	return false
}

// ForgetDownloadedSong 清理去重索引中与这首歌相关的记录（按歌名+歌手，或按文件
// 相对路径）。删除本地文件后调用，这样再下载时不会因为记录残留而被跳过。
func ForgetDownloadedSong(name, artist, relPath string) error {
	if err := initDownloadRecordTable(); err != nil {
		return err
	}
	keys := []string{songKeyFromParts(name, artist)}
	return forgetDownloadedSongWhere(keys, cleanDownloadRelPath(relPath))
}

func forgetDownloadedSongKeys(keys ...string) error {
	if err := initDownloadRecordTable(); err != nil {
		return err
	}
	return forgetDownloadedSongWhere(keys, "")
}

func forgetDownloadedSongWhere(keys []string, relPath string) error {
	conditions := make([]string, 0, 2)
	args := make([]interface{}, 0, 2)

	cleanKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		if strings.TrimSpace(key) != "" {
			cleanKeys = append(cleanKeys, key)
		}
	}
	if len(cleanKeys) > 0 {
		conditions = append(conditions, "song_key IN ?")
		args = append(args, cleanKeys)
	}
	if relPath != "" {
		conditions = append(conditions, "rel_path = ?")
		args = append(args, relPath)
	}
	if len(conditions) == 0 {
		return nil
	}
	return configDB.Where(strings.Join(conditions, " OR "), args...).Delete(&DownloadDedupEntry{}).Error
}

// LoadDownloadDedupSet loads the SQLite de-duplication index. On first use after
// upgrading, it migrates existing successful history rows into the new index.
func LoadDownloadDedupSet() (DownloadDedupIndex, error) {
	if err := initDownloadRecordTable(); err != nil {
		return nil, err
	}

	var entries []DownloadDedupEntry
	if err := configDB.Find(&entries).Error; err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		var records []DownloadRecord
		if err := configDB.Select("name", "artist").Where("status = ?", DownloadStatusSuccess).Find(&records).Error; err != nil {
			return nil, err
		}
		if len(records) > 0 {
			entries = make([]DownloadDedupEntry, 0, len(records))
			for _, record := range records {
				entries = append(entries, DownloadDedupEntry{
					SongKey: songKeyFromParts(record.Name, record.Artist),
					Name:    cleanDownloadRecordText(record.Name),
					Artist:  cleanDownloadRecordText(record.Artist),
				})
			}
			if err := configDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&entries).Error; err != nil {
				return nil, err
			}
		}
	}

	index := make(DownloadDedupIndex, len(entries))
	for _, entry := range entries {
		if key := cleanDownloadRecordText(entry.SongKey); key != "" {
			index[key] = cleanDownloadRelPath(entry.RelPath)
		}
	}
	return index, nil
}

// PruneDownloadDedup 用一次全量扫盘结果对账去重索引：keep 里没有的记录说明文件
// 已经不在下载目录里，整条回收；keep 里有记录的顺手把文件路径回填/更新。
// 返回被回收的记录条数。
//
// 调用方必须先确认下载目录真实存在，否则目录未挂载时会把整张索引清空。
func PruneDownloadDedup(outDir string, keep map[string]string) (int, error) {
	if err := initDownloadRecordTable(); err != nil {
		return 0, err
	}

	var entries []DownloadDedupEntry
	if err := configDB.Find(&entries).Error; err != nil {
		return 0, err
	}

	removed := 0
	for _, entry := range entries {
		relPath, exists := keep[entry.SongKey]
		if !exists {
			// 扫描和下载可能并发：新记录未必在这轮扫描快照里。删除前再看
			// 一次磁盘，只要文件明确存在，或状态无法判断，就保留记录。
			fileExists, fileKnown := inspectDownloadFile(outDir, entry.RelPath)
			if fileExists || !fileKnown {
				continue
			}
			if err := configDB.Where("song_key = ?", entry.SongKey).Delete(&DownloadDedupEntry{}).Error; err != nil {
				return removed, err
			}
			removed++
			continue
		}
		if relPath != "" && cleanDownloadRelPath(entry.RelPath) != cleanDownloadRelPath(relPath) {
			// 文件被改名或换目录后，把路径同步过来，下次校验才不会误判。
			if err := configDB.Model(&DownloadDedupEntry{}).
				Where("song_key = ?", entry.SongKey).
				Update("rel_path", cleanDownloadRelPath(relPath)).Error; err != nil {
				return removed, err
			}
		}
	}
	return removed, nil
}

func CountSkippable(queue []model.Song, index DownloadDedupIndex, outDir string) int {
	count := 0
	for i := range queue {
		if IsSongStillDownloaded(&queue[i], index, outDir) {
			count++
		}
	}
	return count
}

func DownloadWithDedupCheck(song *model.Song, outDir string, withCover, withLyrics bool, dedupSet DownloadDedupIndex) (*DownloadedSong, error) {
	return DownloadWithDedupCheckWithTemplate(song, outDir, withCover, withLyrics, "", dedupSet)
}

func DownloadWithDedupCheckWithTemplate(song *model.Song, outDir string, withCover, withLyrics bool, filenameTemplate string, dedupSet DownloadDedupIndex) (*DownloadedSong, error) {
	key := SongKey(song)
	if IsSongStillDownloaded(song, dedupSet, outDir) {
		_, relPath, _ := lookupSongKey(song, dedupSet)
		relPath = cleanDownloadRelPath(relPath)
		_ = SaveDownloadRecordWithRelPath(song.Name, song.Artist, song.Source, DownloadStatusSkipped, "", relPath)
		savedPath := ""
		if relPath != "" {
			savedPath = filepath.Join(DownloadDirOrDefault(outDir), filepath.FromSlash(relPath))
		}
		return &DownloadedSong{Skipped: true, Filename: key, SavedPath: savedPath}, nil
	}

	var (
		result *DownloadedSong
		dlErr  error
	)
	if filenameTemplate == "" {
		result, dlErr = SaveSongToFile(song, outDir, withCover, withLyrics)
	} else {
		result, dlErr = SaveSongToFileWithTemplate(song, outDir, withCover, withLyrics, filenameTemplate)
	}
	if dlErr != nil {
		_ = SaveDownloadRecord(song.Name, song.Artist, song.Source, DownloadStatusFailed, dlErr.Error())
		return result, dlErr
	}

	// 记下文件落在下载目录里的相对路径，下次判定「是否已下载」时会用它校验磁盘。
	relPath := ""
	if result != nil {
		relPath = relativeDownloadPath(outDir, result.SavedPath)
	}
	_ = SaveDownloadRecordWithRelPath(song.Name, song.Artist, song.Source, DownloadStatusSuccess, "", relPath)
	if dedupSet != nil {
		dedupSet[key] = relPath
	}
	return result, nil
}

// relativeDownloadPath 把落盘后的绝对路径换算成下载目录内的相对路径。
func relativeDownloadPath(outDir, savedPath string) string {
	savedPath = strings.TrimSpace(savedPath)
	if savedPath == "" {
		return ""
	}
	rel, err := filepath.Rel(DownloadDirOrDefault(outDir), savedPath)
	if err != nil {
		return ""
	}
	return cleanDownloadRelPath(rel)
}
