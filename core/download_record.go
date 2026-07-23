package core

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/guohuiyuan/music-lib/model"
)

// ==========================================
// 下载记录（持久化到 SQLite）
// ==========================================

type DownloadRecord struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"size:512;not null;index"`
	Artist    string    `gorm:"size:512;not null;index"`
	Source    string    `gorm:"size:64;not null"`
	Status    string    `gorm:"size:32;not null;index"` // success / skipped / failed
	Error     string    `gorm:"size:1024"`
	CreatedAt time.Time `gorm:"autoCreateTime;index"`
}

func initDownloadRecordTable() error {
	if err := ensureConfigDB(); err != nil {
		return err
	}
	return configDB.AutoMigrate(&DownloadRecord{})
}

// SaveDownloadRecord 保存一条下载记录到数据库。
func SaveDownloadRecord(name, artist, source, status, errStr string) error {
	if err := initDownloadRecordTable(); err != nil {
		return err
	}
	record := DownloadRecord{
		Name:   strings.TrimSpace(name),
		Artist: strings.TrimSpace(artist),
		Source: strings.TrimSpace(source),
		Status: status,
		Error:  strings.TrimSpace(errStr),
	}
	return configDB.Create(&record).Error
}

// GetDownloadRecords 返回最近的下载记录（默认最多 200 条，按时间倒序）。
func GetDownloadRecords() ([]DownloadRecord, error) {
	if err := initDownloadRecordTable(); err != nil {
		return nil, err
	}
	var records []DownloadRecord
	err := configDB.Order("created_at DESC").Limit(200).Find(&records).Error
	return records, err
}

// ClearDownloadRecords 清空所有下载记录。
func ClearDownloadRecords() error {
	if err := initDownloadRecordTable(); err != nil {
		return err
	}
	return configDB.Where("1 = 1").Delete(&DownloadRecord{}).Error
}

// ==========================================
// "全部文件.txt" — 下载去重依据
// ==========================================

var (
	allSongsFileMu   sync.RWMutex
	allSongsFilePath string // 按需惰性初始化
	allSongsFileOnce sync.Once
)

// resolveAllSongsFilePath 返回软件根目录下 "全部文件.txt" 的完整路径。
// 优先级：MUSIC_DL_DATA_DIR 环境变量 → 工作目录 → 可执行文件所在目录。
func resolveAllSongsFilePath() string {
	allSongsFileOnce.Do(func() {
		// 1. 环境变量（Rust 桌面版传入的 exe 所在目录）
		if envDir := strings.TrimSpace(os.Getenv("MUSIC_DL_DATA_DIR")); envDir != "" {
			if info, statErr := os.Stat(envDir); statErr == nil && info.IsDir() {
				allSongsFilePath = filepath.Join(envDir, "全部文件.txt")
				return
			}
		}
		// 2. 工作目录（Rust 桌面版通过 current_dir 设为 %LOCALAPPDATA%/go-music-dl/）
		if wd, err := os.Getwd(); err == nil {
			if info, statErr := os.Stat(wd); statErr == nil && info.IsDir() {
				allSongsFilePath = filepath.Join(wd, "全部文件.txt")
				return
			}
		}
		// 3. 兜底：可执行文件所在目录
		if exe, err := os.Executable(); err == nil {
			dir := filepath.Dir(exe)
			if info, statErr := os.Stat(dir); statErr == nil && info.IsDir() {
				allSongsFilePath = filepath.Join(dir, "全部文件.txt")
				return
			}
		}
		// 最终兜底
		allSongsFilePath = "全部文件.txt"
	})
	return allSongsFilePath
}

// AllSongsFilePath 返回 "全部文件.txt" 的路径（导出，Web 端可能要用）。
func AllSongsFilePath() string {
	return resolveAllSongsFilePath()
}

// DataDir 返回 "全部文件.txt" 所在目录（即 exe 所在目录或当前目录）。
func DataDir() string {
	return filepath.Dir(resolveAllSongsFilePath())
}

// LoadAllSongsSet 读取 "全部文件.txt"，返回 artist - name 的集合用于快速查重。
// 文件格式：每行一条记录，形如 "artist - name"。
func LoadAllSongsSet() (map[string]struct{}, error) {
	path := resolveAllSongsFilePath()

	allSongsFileMu.RLock()
	defer allSongsFileMu.RUnlock()

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]struct{}), nil
		}
		return nil, err
	}
	defer f.Close()

	set := make(map[string]struct{})
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		set[line] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return set, err
	}
	return set, nil
}

// AppendToAllSongs 将一首歌写入 "全部文件.txt"（追加一行 "artist - name"）。
func AppendToAllSongs(artist, name string) error {
	path := resolveAllSongsFilePath()

	allSongsFileMu.Lock()
	defer allSongsFileMu.Unlock()

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法写入全部文件.txt: %w", err)
	}
	defer f.Close()

	line := fmt.Sprintf("%s - %s\n", strings.TrimSpace(artist), strings.TrimSpace(name))
	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("写入全部文件.txt 失败: %w", err)
	}
	return nil
}

// SongKey 生成用于查重的 key："artist - name"，并剔除控制字符防止污染行式记录。
func SongKey(song *model.Song) string {
	artist := stripControl(strings.TrimSpace(song.Artist))
	name := stripControl(strings.TrimSpace(song.Name))
	if artist == "" {
		artist = "Unknown"
	}
	if name == "" {
		name = "Unknown"
	}
	return artist + " - " + name
}

// stripControl 剔除字符串中的控制字符（\n \r \t \0 等），保留空格和可见字符。
func stripControl(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0x20 && r != 0x7f {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// IsSongDownloaded 检查歌曲是否已存在于 "全部文件.txt" 中。
func IsSongDownloaded(song *model.Song, allSongsSet map[string]struct{}) bool {
	if allSongsSet == nil {
		return false
	}
	key := SongKey(song)
	_, exists := allSongsSet[key]
	return exists
}

// readLinesSet 读取一个文本文件，每行作为一个 key 返回集合。
// 文件不存在时返回空集合（不报错）。
func readLinesSet(filePath string) (map[string]struct{}, error) {
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]struct{}), nil
		}
		return nil, err
	}
	defer f.Close()

	set := make(map[string]struct{})
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		set[line] = struct{}{}
	}
	return set, scanner.Err()
}

// LoadDownloadDedupSet 加载下载去重集合：合并 成功解析.txt + 下载记录.txt。
// 文件不存在时返回空集合（不报错），其他读取错误会返回。
func LoadDownloadDedupSet() (map[string]struct{}, error) {
	dataDir := filepath.Dir(resolveAllSongsFilePath())
	set := make(map[string]struct{})

	// 加载 成功解析.txt（导入的本地曲库）
	s, err := readLinesSet(filepath.Join(dataDir, "成功解析.txt"))
	if err != nil {
		return set, fmt.Errorf("读取成功解析.txt 失败: %w", err)
	}
	for k := range s {
		set[k] = struct{}{}
	}

	// 加载 下载记录.txt（通过本工具成功下载的记录）
	s, err = readLinesSet(filepath.Join(dataDir, "下载记录.txt"))
	if err != nil {
		return set, fmt.Errorf("读取下载记录.txt 失败: %w", err)
	}
	for k := range s {
		set[k] = struct{}{}
	}

	return set, nil
}

// CountSkippable 统计待下载队列中有多少首已在去重集合中。
func CountSkippable(queue []model.Song, dedupSet map[string]struct{}) int {
	count := 0
	for _, s := range queue {
		if IsSongDownloaded(&s, dedupSet) {
			count++
		}
	}
	return count
}

// ==========================================
// 导入已有曲库（从目录列表文件解析）
// ==========================================

// ImportDirectoryListingResult 导入结果
type ImportDirectoryListingResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Total    int      `json:"total"`
	Samples  []string `json:"samples,omitempty"` // 前几条导入的记录示例
	DataDir  string   `json:"dataDir"`           // 文件生成目录
}

// parseFileNameToArtistName 从文件名中尝试解析 artist 和 name。
// 文件名不含扩展名和路径，例如 "10y0 - Lemon (翻自 时代少年团)"。
// 返回值：artist, name, ok。
func parseFileNameToArtistName(filename string) (string, string, bool) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return "", "", false
	}

	// 策略1: 按 " - "（空格-短横-空格）分割 —— 最可靠
	if idx := strings.Index(filename, " - "); idx > 0 {
		artist := strings.TrimSpace(filename[:idx])
		name := strings.TrimSpace(filename[idx+3:])
		if artist != "" && name != "" {
			return artist, name, true
		}
	}

	// 策略2: 按最后一个 " - "（带空格）分割
	if idx := strings.LastIndex(filename, " - "); idx > 0 {
		artist := strings.TrimSpace(filename[:idx])
		name := strings.TrimSpace(filename[idx+3:])
		if artist != "" && name != "" {
			return artist, name, true
		}
	}

	// 策略3: 按最后一个 "-" 分割，且右侧包含中文（处理 A-Lin-天若有情 这类文件）
	if idx := strings.LastIndex(filename, "-"); idx > 0 {
		right := strings.TrimSpace(filename[idx+1:])
		left := strings.TrimSpace(filename[:idx])
		if left != "" && right != "" && containsChinese(right) {
			return left, right, true
		}
	}

	// 策略4: 文件名中只有一个 "-"（无空格），直接按它分割
	// 处理纯英文 artist-name 如 "2Someone-Star Unkind..."
	if strings.Count(filename, "-") == 1 && !strings.Contains(filename, " - ") {
		if idx := strings.Index(filename, "-"); idx > 0 {
			artist := strings.TrimSpace(filename[:idx])
			name := strings.TrimSpace(filename[idx+1:])
			if artist != "" && name != "" {
				return artist, name, true
			}
		}
	}

	return "", "", false
}

// containsChinese 判断字符串是否包含中文字符。
func containsChinese(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// importDirectoryListingFromLines 从行列表解析并写入 "全部文件.txt"，
// importDirectoryListingFromLines 解析行列表，生成 "成功解析.txt" 和 "不能匹配.txt"。
// 不再写入 "全部文件.txt"，仅做解析统计。
func importDirectoryListingFromLines(lines []string) *ImportDirectoryListingResult {
	result := &ImportDirectoryListingResult{}
	var successLines []string
	var failLines []string

	for _, line := range lines {
		originalLine := strings.TrimSpace(line)
		if originalLine == "" {
			continue
		}

		base := filepath.Base(originalLine)
		ext := filepath.Ext(base)
		nameOnly := strings.TrimSuffix(base, ext)

		artist, name, ok := parseFileNameToArtistName(nameOnly)
		if !ok {
			result.Skipped++
			failLines = append(failLines, originalLine)
			continue
		}

		key := artist + " - " + name
		successLines = append(successLines, key)
		result.Imported++

		if result.Imported <= 5 {
			result.Samples = append(result.Samples, key)
		}
	}

	dataDir := filepath.Dir(resolveAllSongsFilePath())
	if err := writeLinesFile(filepath.Join(dataDir, "成功解析.txt"), successLines); err != nil {
		result.Skipped += result.Imported
		result.Imported = 0
	}
	if err := writeLinesFile(filepath.Join(dataDir, "不能匹配.txt"), failLines); err != nil {
		// 不影响已解析数据，只记录跳过
	}

	result.DataDir = dataDir

	return result
}

// writeLinesFile 将行列表写入指定文件（覆盖模式），空列表写 "(无)"。
func writeLinesFile(path string, lines []string) error {
	if len(lines) == 0 {
		return os.WriteFile(path, []byte("(无)\n"), 0644)
	}
	var sb strings.Builder
	for _, line := range lines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// AppendLogLine 追加一行到指定日志文件（在 DataDir 下），用于累计记录。
func AppendLogLine(filename, line string) error {
	path := filepath.Join(DataDir(), filename)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}

// ClearLogFile 清空指定日志文件（在 DataDir 下）。
func ClearLogFile(filename string) error {
	path := filepath.Join(DataDir(), filename)
	return os.WriteFile(path, []byte{}, 0644)
}

// ImportDirectoryListing 读取目录列表文件，解析生成 "成功解析.txt" 和 "不能匹配.txt"。
func ImportDirectoryListing(filePath string) (*ImportDirectoryListingResult, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法打开文件: %w", err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件出错: %w", err)
	}

	return importDirectoryListingFromLines(lines), nil
}

// ImportDirectoryListingFromContent 从文本内容（由前端上传）解析生成。
func ImportDirectoryListingFromContent(content string) (*ImportDirectoryListingResult, error) {
	lines := strings.Split(content, "\n")
	return importDirectoryListingFromLines(lines), nil
}

const (
	DownloadStatusSuccess = "success"
	DownloadStatusSkipped = "skipped"
	DownloadStatusFailed  = "failed"
)

// ==========================================
// 导入歌曲片段 — 搜索完整版
// ==========================================

// ClipImportResult 歌曲片段导入结果
type ClipImportResult struct {
	Total   int          `json:"total"`   // 文件总行数
	Matched int          `json:"matched"` // 成功匹配数
	Songs   []model.Song `json:"songs"`   // 匹配到的歌曲列表
}

// ClipProgress 进度回调，current 为已处理的歌曲数，total 为总解析数，
// song 为当前正在搜索的歌曲名，matched 为已匹配数，noMatch 为未匹配数。
type ClipProgress func(current, total, matched, noMatch int, song string, itemMatched bool)

// ImportSongClips 解析目录列表文件，对每首可解析的歌曲搜索指定的音乐源，
// 返回相似度 >= threshold 的匹配结果。sources 为空时搜索全部源。
// onProgress 可选，每次搜索完一首歌后回调。
func ImportSongClips(content string, sources []string, threshold float64, onProgress ClipProgress) (*ClipImportResult, error) {
	lines := strings.Split(content, "\n")
	if len(sources) == 0 {
		sources = GetAllSourceNames()
	}
	if threshold <= 0 || threshold > 1 {
		threshold = 0.75
	}

	result := &ClipImportResult{}
	seen := make(map[string]bool) // 按 "source:id" 去重
	processed := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		base := filepath.Base(line)
		ext := filepath.Ext(base)
		nameOnly := strings.TrimSuffix(base, ext)

		parsedName, parsedArtist, ok := parseFileNameToArtistName(nameOnly)
		if !ok {
			continue
		}
		result.Total++
		processed++

		// 搜索各音乐源，取最佳匹配
		songMatched := false
		var bestScore float64 = -1
		var bestSong *model.Song

		for _, src := range sources {
			if src == "" {
				continue
			}
			searchFn := GetSearchFunc(src)
			if searchFn == nil {
				continue
			}

			keyword := parsedName
			if parsedArtist != "" {
				keyword = parsedName + " " + parsedArtist
			}

			results, err := searchFn(keyword)
			if err != nil || len(results) == 0 {
				continue
			}

			for _, song := range results {
				score := CalcSongSimilarity(parsedName, parsedArtist, song.Name, song.Artist)
				if score >= threshold && score > bestScore {
					bestScore = score
					bestSong = &song
				}
			}
		}

		if bestSong != nil {
			// 按 song ID 去重（跨源），避免同一首歌曲被重复添加
			if !seen[bestSong.ID] {
				seen[bestSong.ID] = true
				songMatched = true
				result.Songs = append(result.Songs, *bestSong)
				result.Matched++
			}
		}

		// 进度回调（在搜索之后，携带当前歌曲的匹配结果）
		if onProgress != nil {
			onProgress(processed, result.Total, result.Matched, processed-result.Matched, parsedArtist+" - "+parsedName, songMatched)
		}
	}

	return result, nil
}

// DownloadWithDedupCheck 带去重检查的下载函数：先查 "全部文件.txt"，
// 已存在则跳过并记录 "skipped"；否则下载、记录 "success"/"failed"。
// allSongsSet 从 LoadAllSongsSet() 获取，批量下载时复用可避免重复读文件。
func DownloadWithDedupCheck(song *model.Song, outDir string, withCover, withLyrics bool, allSongsSet map[string]struct{}) (*DownloadedSong, error) {
	return DownloadWithDedupCheckWithTemplate(song, outDir, withCover, withLyrics, "", allSongsSet)
}

// DownloadWithDedupCheckWithTemplate 同 DownloadWithDedupCheck，支持自定义文件名模板。
func DownloadWithDedupCheckWithTemplate(song *model.Song, outDir string, withCover, withLyrics bool, filenameTemplate string, allSongsSet map[string]struct{}) (*DownloadedSong, error) {
	key := SongKey(song)

	// 1. 去重检查
	if IsSongDownloaded(song, allSongsSet) {
		_ = SaveDownloadRecord(song.Name, song.Artist, song.Source, DownloadStatusSkipped, "")
		_ = AppendLogLine("跳过下载.txt", key)
		return &DownloadedSong{Skipped: true, Filename: key}, nil
	}

	// 2. 执行下载
	var result *DownloadedSong
	var dlErr error
	if filenameTemplate == "" {
		result, dlErr = SaveSongToFile(song, outDir, withCover, withLyrics)
	} else {
		result, dlErr = SaveSongToFileWithTemplate(song, outDir, withCover, withLyrics, filenameTemplate)
	}

	// 3. 记录结果
	if dlErr != nil {
		_ = SaveDownloadRecord(song.Name, song.Artist, song.Source, DownloadStatusFailed, dlErr.Error())
		_ = AppendLogLine("下载失败.txt", key+"  ("+dlErr.Error()+")")
		return result, dlErr
	}

	_ = SaveDownloadRecord(song.Name, song.Artist, song.Source, DownloadStatusSuccess, "")
	_ = AppendLogLine("下载记录.txt", key)
	// 更新内存集合，确保同一批次内的后续下载能正确去重
	if allSongsSet != nil {
		allSongsSet[key] = struct{}{}
	}

	return result, nil
}
