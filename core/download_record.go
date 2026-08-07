package core

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/mozillazg/go-pinyin"
	"gorm.io/gorm/clause"
)

// ==========================================
// 下载记录（持久化到 SQLite）
// ==========================================

// ImportedSong 导入成功解析的歌曲（替代 成功解析.txt）
type ImportedSong struct {
	ID        uint      `gorm:"primaryKey"`
	Key       string    `gorm:"size:1024;not null;uniqueIndex"`
	CreatedAt time.Time `gorm:"autoCreateTime;index"`
}

// DownloadLog 下载记录（替代 下载记录.txt/跳过下载.txt/下载失败.txt）
type DownloadLog struct {
	ID        uint      `gorm:"primaryKey"`
	Key       string    `gorm:"size:1024;not null;index"`
	Status    string    `gorm:"size:32;not null;index"` // success / skipped / failed
	Error     string    `gorm:"size:1024"`
	CreatedAt time.Time `gorm:"autoCreateTime;index"`
}

func initImportTables() error {
	if err := ensureConfigDB(); err != nil {
		return err
	}
	return configDB.AutoMigrate(&ImportedSong{}, &DownloadLog{})
}

// migrateFromTxt 将现有 txt 文件导入 SQLite（仅首次运行）
func migrateFromTxt() error {
	if err := initImportTables(); err != nil {
		return err
	}
	dataDir := filepath.Dir(resolveAllSongsFilePath())

	if count, _ := countTable(&ImportedSong{}); count == 0 {
		if lines, err := readLines(filepath.Join(dataDir, "成功解析.txt")); err == nil && len(lines) > 0 {
			var batch []ImportedSong
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || line == "(无)" {
					continue
				}
				batch = append(batch, ImportedSong{Key: line})
			}
			if len(batch) > 0 {
				configDB.CreateInBatches(batch, 500)
			}
		}
	}

	if count, _ := countTable(&DownloadLog{}); count == 0 {
		for _, pair := range [][2]string{{"下载记录.txt", "success"}, {"跳过下载.txt", "skipped"}, {"下载失败.txt", "failed"}} {
			if lines, err := readLines(filepath.Join(dataDir, pair[0])); err == nil && len(lines) > 0 {
				var batch []DownloadLog
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" || line == "(无)" {
						continue
					}
					batch = append(batch, DownloadLog{Key: line, Status: pair[1]})
				}
				if len(batch) > 0 {
					configDB.CreateInBatches(batch, 500)
				}
			}
		}
	}
	return nil
}

func countTable(model interface{}) (int64, error) {
	var count int64
	err := configDB.Model(model).Count(&count).Error
	return count, err
}

func readLines(filePath string) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

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

// GetAllDownloadRecords 返回全部下载记录（无条数限制，用于导出 CSV）。
func GetAllDownloadRecords() ([]DownloadRecord, error) {
	if err := initDownloadRecordTable(); err != nil {
		return nil, err
	}
	var records []DownloadRecord
	err := configDB.Order("created_at DESC").Find(&records).Error
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

// SongKey 生成用于查重的 key："artist - name"（归一化后），并剔除控制字符防止污染行式记录。
func SongKey(song *model.Song) string {
	return NormalizeKey(song.Artist, song.Name)
}

// NormalizeKey 生成归一化去重 key："artist - name"。
// 归一化规则：空白压缩、英文大小写折叠、歌手分隔符（、，,/ _）统一为 &、多歌手排序。
func NormalizeKey(artist, name string) string {
	a := normalizeArtist(artist)
	n := normalizeName(name)
	if a == "" {
		a = "unknown"
	}
	if n == "" {
		n = "unknown"
	}
	return a + " - " + n
}

// NormalizeStoredKey 将已存储的 "artist - name" key 归一化（兼容旧格式数据）。
func NormalizeStoredKey(key string) string {
	// 先做字符级 NFKC + 长横线归一，便于定位 " - " 分隔符（全角空格/长横线/全角破折号场景）
	normKey := strings.NewReplacer("–", "-", "—", "-", "―", "-").Replace(toNFKC(key))
	if i := strings.Index(normKey, " - "); i >= 0 {
		return NormalizeKey(normKey[:i], normKey[i+3:])
	}
	return NormalizeKey(normKey, "")
}

// normalizeArtist 归一化歌手名：按启用的跳过规则处理。
func normalizeArtist(s string) string {
	s = stripControl(strings.TrimSpace(s))
	if ruleOn("nfkc") {
		s = toNFKC(s)
	}
	if ruleOn("tc") {
		s = toSimplified(s)
	}
	if ruleOn("ws") {
		s = strings.Join(strings.Fields(s), " ")
	}
	if ruleOn("sep") {
		// 有空格包围的斜杠/反斜杠/下划线视为歌手分隔符
		s = strings.ReplaceAll(s, " / ", "&")
		s = strings.ReplaceAll(s, " \\ ", "&")
		s = strings.ReplaceAll(s, " _ ", "&")
		// 中文顿号、中英文逗号统一为歌手分隔符
		s = strings.ReplaceAll(s, "、", "&")
		s = strings.ReplaceAll(s, "，", "&")
		s = strings.ReplaceAll(s, ",", "&")
	}
	if ruleOn("semi") {
		// 分号也视为歌手分隔符（钟镇涛、彭健新 vs 钟镇涛;彭健新）
		s = strings.ReplaceAll(s, ";", "&")
		s = strings.ReplaceAll(s, "；", "&")
	}
	if ruleOn("ndash") {
		// 长横线统一为普通连字符
		s = strings.NewReplacer("–", "-", "—", "-", "―", "-").Replace(s)
	}
	if ruleOn("feat") {
		// feat./ft. 视为歌手分隔符（A feat. B → A&B）
		s = strings.ReplaceAll(s, "feat.", "&")
		s = strings.ReplaceAll(s, "ft.", "&")
		s = strings.ReplaceAll(s, "feat ", "&")
		s = strings.ReplaceAll(s, "feat", "&")
	}
	if ruleOn("heyu") {
		// 「和」「与」两侧均为汉字时视为歌手分隔符（孙楠和韩红 → 孙楠&韩红）
		s = replaceCJKConjunction(s)
	}
	if ruleOn("underscore") {
		// 汉字_汉字 的下划线视为歌手分隔符（张智霖_许秋怡 → 张智霖&许秋怡）
		s = replaceCJKUnderscore(s)
	}
	if ruleOn("boundary") {
		// 字母-汉字边界视为歌手分隔符（F.I.R.飞儿乐团、G.E.M.邓紫棋、阿YueYue）
		s = splitLetterHanBoundary(s)
	}
	if ruleOn("band") {
		// 剥离歌手名「乐团/乐队/合唱团」后缀（八三夭乐团 → 八三夭）
		s = stripBandSuffix(s)
	}
	if ruleOn("cjk") {
		// 若空格分隔的每一段都是 CJK 字符，则空格视为歌手分隔符（如 "海鸣威 吴琼"）
		segs := strings.Split(s, " ")
		if len(segs) > 1 {
			allCJK := true
			for _, seg := range segs {
				if !isAllCJK(seg) {
					allCJK = false
					break
				}
			}
			if allCJK {
				s = strings.Join(segs, "&")
			}
		}
	}
	if ruleOn("sep") {
		// 清理 & 两侧空白
		s = strings.ReplaceAll(s, " & ", "&")
		s = strings.ReplaceAll(s, "& ", "&")
		s = strings.ReplaceAll(s, " &", "&")
	}
	if ruleOn("lower") {
		s = strings.ToLower(s)
	}
	if ruleOn("sort") {
		// 按 & 拆分、去空白、排序、合并（消除歌手顺序差异）
		parts := strings.Split(s, "&")
		trimmed := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				trimmed = append(trimmed, p)
			}
		}
		if len(trimmed) == 0 {
			return ""
		}
		sort.Strings(trimmed)
		return strings.Join(trimmed, "&")
	}
	return s
}

// isAllCJK 判断字符串是否全部由 CJK（汉字）字符组成。
func isAllCJK(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.Is(unicode.Han, r) {
			return false
		}
	}
	return true
}

// ==========================================
// 同音字匹配（拼音变体）
// ==========================================

var (
	runePinyinCache   = map[rune]string{}
	runePinyinCacheMu sync.RWMutex
)

// runePinyin 返回单个汉字的拼音（无声调，取第一个读音），非汉字原样保留（小写）。
func runePinyin(r rune) string {
	if !unicode.Is(unicode.Han, r) {
		return strings.ToLower(string(r))
	}
	runePinyinCacheMu.RLock()
	if p, ok := runePinyinCache[r]; ok {
		runePinyinCacheMu.RUnlock()
		return p
	}
	runePinyinCacheMu.RUnlock()

	py := pinyin.LazyPinyin(string(r), pinyin.NewArgs())
	p := ""
	if len(py) > 0 {
		p = py[0]
	}
	runePinyinCacheMu.Lock()
	runePinyinCache[r] = p
	runePinyinCacheMu.Unlock()
	return p
}

// toPinyin 将字符串转为拼音串（汉字→拼音，非汉字原样小写）。
// 先应用同音字规则内的异体/通假字归一（如 藉→借），再转拼音。
func toPinyin(s string) string {
	var b strings.Builder
	for _, r := range s {
		if v, ok := homophoneVariantMap[r]; ok {
			r = v
		}
		b.WriteString(runePinyin(r))
	}
	return b.String()
}

// homophoneVariantMap 同音字规则内的异体/通假字归一（转拼音前替换为常用字）。
// 用于 go-pinyin 首读音不是常用读音的多音字案例（如 藉 jiè → 借）。
var homophoneVariantMap = map[rune]rune{
	'藉': '借',
}

// toPinyinKey 生成拼音变体 key：pinyin(artist) - pinyin(name)。
func toPinyinKey(artist, name string) string {
	return toPinyin(normalizeArtist(artist)) + " - " + toPinyin(normalizeName(name))
}

var (
	pinyinSetMu sync.RWMutex
	pinyinSet   map[string]struct{}
)

// buildPinyinSet 基于归一化集合构建拼音变体集合。
func buildPinyinSet(set map[string]struct{}) {
	ps := make(map[string]struct{}, len(set))
	for k := range set {
		if i := strings.Index(k, " - "); i >= 0 {
			ps[toPinyinKey(k[:i], k[i+3:])] = struct{}{}
		}
	}
	pinyinSetMu.Lock()
	pinyinSet = ps
	pinyinSetMu.Unlock()
}

// clearPinyinSet 清空拼音变体集合（规则关闭时释放内存）。
func clearPinyinSet() {
	pinyinSetMu.Lock()
	pinyinSet = nil
	pinyinSetMu.Unlock()
}

// normalizeName 归一化歌名：按启用的跳过规则处理（不做歌手分隔符替换，避免误伤）。
func normalizeName(s string) string {
	s = stripControl(strings.TrimSpace(s))
	if ruleOn("nfkc") {
		s = toNFKC(s)
	}
	if ruleOn("tc") {
		s = toSimplified(s)
	}
	if ruleOn("ws") {
		s = strings.Join(strings.Fields(s), " ")
	}
	if ruleOn("punct") {
		// 歌名标点统一：. · • 〜 ～ ~ 视为空白
		s = strings.NewReplacer(".", " ", "·", " ", "•", " ", "〜", " ", "～", " ", "~", " ").Replace(s)
		s = strings.Join(strings.Fields(s), " ")
	}
	if ruleOn("semi") {
		// 分号视为空白（歌名场景）
		s = strings.NewReplacer(";", " ", "；", " ").Replace(s)
	}
	if ruleOn("bang") {
		// 感叹/疑问标点直接删除（再见亦是泪!? → 再见亦是泪）
		s = strings.NewReplacer("!", "", "?", "", "！", "", "？", "").Replace(s)
	}
	if ruleOn("trailunder") {
		// 剥离歌名末尾连续下划线（再见亦是泪!_ → 再见亦是泪!）
		s = strings.TrimRight(s, "_")
	}
	if ruleOn("ndash") {
		// 长横线统一为普通连字符
		s = strings.NewReplacer("–", "-", "—", "-", "―", "-").Replace(s)
	}
	if ruleOn("paren") {
		// 剥离括号及其内容：(...) （...） [...] 【...】
		s = stripParentheses(s)
	}
	if ruleOn("cjkspace") {
		// 歌名全为汉字时删除内部空格（爱多一次 痛多一次 → 爱多一次痛多一次）
		if isAllCJK(strings.ReplaceAll(s, " ", "")) && strings.Contains(s, " ") {
			s = strings.ReplaceAll(s, " ", "")
		}
	}
	if ruleOn("seq") {
		// 剥离歌名前序号（01 - 十年 → 十年）
		s = stripLeadingSeq(s)
	}
	if ruleOn("lower") {
		s = strings.ToLower(s)
	}
	return s
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

// replaceCJKConjunction 将两侧均为汉字的「和」「与」替换为 &（视为歌手分隔符）。
func replaceCJKConjunction(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if (r == '和' || r == '与') && i > 0 && i < len(runes)-1 &&
			unicode.Is(unicode.Han, runes[i-1]) && unicode.Is(unicode.Han, runes[i+1]) {
			b.WriteRune('&')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// replaceCJKUnderscore 将两侧均为汉字的下划线替换为 &（张智霖_许秋怡 → 张智霖&许秋怡）。
// 开头/结尾的下划线（如 相依为命_、_R）不受影响。
func replaceCJKUnderscore(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if r == '_' && i > 0 && i < len(runes)-1 &&
			unicode.Is(unicode.Han, runes[i-1]) && unicode.Is(unicode.Han, runes[i+1]) {
			b.WriteRune('&')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// splitLetterHanBoundary 在字母与汉字边界插入 &（F.I.R.飞儿乐团 → F.I.R.&飞儿乐团）。
// 处理：英汉相邻（阿YueYue）、英汉被标点隔开（G.E.M.邓紫棋）、英汉被空格隔开（G.E.M. 邓紫棋）。
// 遇已有分隔符 & 停止回溯，避免破坏既有拆分（孙建平&Sweet Style）。
func splitLetterHanBoundary(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		if i > 0 {
			curHan := unicode.Is(unicode.Han, r)
			curEng := isEng(r)
			if curHan || curEng {
				// 往前找最近的有效字符（跳过标点/空格，遇 & 停止）
				prevType := -1 // -1 none, 0 han, 1 eng
				for j := i - 1; j >= 0; j-- {
					c := runes[j]
					if c == '&' {
						break
					}
					if unicode.Is(unicode.Han, c) {
						prevType = 0
						break
					}
					if isEng(c) {
						prevType = 1
						break
					}
				}
				if (prevType == 1 && curHan) || (prevType == 0 && curEng) {
					b.WriteRune('&')
					continue
				}
			}
			if r == ' ' && i < len(runes)-1 {
				// 空格两侧跨英汉边界
				prevType := -1
				for j := i - 1; j >= 0; j-- {
					c := runes[j]
					if c == '&' {
						break
					}
					if unicode.Is(unicode.Han, c) {
						prevType = 0
						break
					}
					if isEng(c) {
						prevType = 1
						break
					}
				}
				if (prevType == 1 && firstIsHan(runes[i+1:])) || (prevType == 0 && firstIsEng(runes[i+1:])) {
					b.WriteRune('&')
					continue
				}
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}

// isEng 判断是否为 ASCII 字母或数字。
func isEng(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// firstIsHan 检查后续字符中首个有效字符是否为汉字（遇 & 停止，遇字母返回 false）。
func firstIsHan(rs []rune) bool {
	for _, c := range rs {
		if c == '&' {
			return false
		}
		if unicode.Is(unicode.Han, c) {
			return true
		}
		if isEng(c) {
			return false
		}
	}
	return false
}

// firstIsEng 检查后续字符中首个有效字符是否为字母/数字（遇 & 停止，遇汉字返回 false）。
func firstIsEng(rs []rune) bool {
	for _, c := range rs {
		if c == '&' {
			return false
		}
		if isEng(c) {
			return true
		}
		if unicode.Is(unicode.Han, c) {
			return false
		}
	}
	return false
}

// stripBandSuffix 剥离每个歌手段末尾的「乐团/乐队/合唱团」后缀（八三夭乐团 → 八三夭）。
func stripBandSuffix(s string) string {
	parts := strings.Split(s, "&")
	for i, p := range parts {
		for _, suffix := range []string{"乐团", "乐队", "合唱团"} {
			if len(p) > len(suffix) && strings.HasSuffix(p, suffix) {
				p = strings.TrimSpace(strings.TrimSuffix(p, suffix))
				break
			}
		}
		parts[i] = p
	}
	return strings.Join(parts, "&")
}

// stripParentheses 剥离歌名中的括号及其内容（半角/全角/方括号/书名号）。
func stripParentheses(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '(', '（', '[', '【', '「':
			depth++
			if depth > 1 {
				b.WriteRune(r)
			}
		case ')', '）', ']', '】', '」':
			if depth > 1 {
				b.WriteRune(r)
			}
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// stripLeadingSeq 剥离歌名前序号（如 "01 - "、"01."、"01、"、"01 "、"01 十年"）。
func stripLeadingSeq(s string) string {
	s2 := strings.TrimSpace(s)
	for len(s2) > 0 && s2[0] >= '0' && s2[0] <= '9' {
		// 找到数字串结束位置（长度须 ≤4，避免误伤年份类歌名）
		i := 0
		for i < len(s2) && s2[i] >= '0' && s2[i] <= '9' {
			i++
		}
		if i > 4 {
			return s2
		}
		rest := strings.TrimLeft(s2[i:], " ")
		if rest == "" {
			return s2
		}
		// 若紧跟分隔符（- . 、 _ ： :），剥掉分隔符及其后空白
		r0 := []rune(rest)[0]
		if r0 == '-' || r0 == '.' || r0 == '、' || r0 == '_' || r0 == '：' || r0 == ':' {
			rest = strings.TrimLeft(string([]rune(rest)[1:]), " ")
		}
		s2 = strings.TrimSpace(rest)
	}
	return s2
}

// ==========================================
// 跳过规则管理（用于 key 归一化）
// ==========================================

var (
	skipRulesMu  sync.RWMutex
	skipRulesSet = map[string]bool{}
)

// SetSkipRules 设置启用的跳过规则（逗号分隔，如 "ws,sep,cjk,sort,lower,tc,punct,nfkc"）。
// 未在列表中的规则视为关闭。
func SetSkipRules(rules string) {
	enabled := make(map[string]bool)
	for _, r := range strings.Split(rules, ",") {
		r = strings.TrimSpace(r)
		if r != "" {
			enabled[r] = true
		}
	}
	skipRulesMu.Lock()
	skipRulesSet = enabled
	skipRulesMu.Unlock()
}

// ruleOn 判断某条跳过规则是否启用。
func ruleOn(name string) bool {
	skipRulesMu.RLock()
	defer skipRulesMu.RUnlock()
	return skipRulesSet[name]
}

// syncSkipRules 从 WebSettings 同步当前启用的跳过规则。
func syncSkipRules() {
	SetSkipRules(GetWebSettings().SkipRules)
}

// toNFKC 执行全半角/空白归一（Unicode 全半角归一规则）。
// 全角 ASCII（U+FF01-U+FF5E）转半角；全角空格（U+3000）与各种特殊空白转普通空格。
func toNFKC(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == 0x3000 || r == 0x00a0 || r == 0x2007 || r == 0x202f || r == 0xfeff:
			b.WriteByte(' ')
		case r >= 0xff01 && r <= 0xff5e:
			b.WriteRune(r - 0xfee0)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// traditionalToSimplifiedMap 常用繁体字→简体字映射（覆盖常见字，避免引入外部转换库）。
var traditionalToSimplifiedMap = map[rune]rune{
	'葉': '叶', '蒨': '倩', '來': '来', '這': '这', '們': '们', '會': '会', '為': '为',
	'說': '说', '話': '话', '還': '还', '沒': '没', '個': '个', '與': '与', '後': '后',
	'時': '时', '間': '间', '裏': '里', '裡': '里', '聽': '听', '見': '见', '樣': '样',
	'點': '点', '書': '书', '愛': '爱', '車': '车', '紅': '红', '綠': '绿', '藍': '蓝',
	'黃': '黄', '張': '张', '劉': '刘', '陳': '陈', '楊': '杨', '萬': '万', '億': '亿',
	'數': '数', '學': '学', '習': '习', '無': '无', '錢': '钱', '東': '东', '風': '风',
	'雲': '云', '電': '电', '臺': '台', '灣': '湾', '國': '国', '園': '园', '圓': '圆',
	'員': '员', '師': '师', '帥': '帅', '帶': '带', '幫': '帮', '幹': '干', '寫': '写',
	'對': '对', '錯': '错', '發': '发', '頭': '头', '飛': '飞', '馬': '马', '魚': '鱼',
	'鳥': '鸟', '雞': '鸡', '鴨': '鸭', '貓': '猫', '龍': '龙', '鳳': '凤', '鵬': '鹏',
	'關': '关', '開': '开', '閉': '闭', '問': '问', '題': '题', '體': '体', '門': '门',
	'從': '从', '經': '经', '過': '过', '邊': '边', '變': '变', '麼': '么', '讓': '让',
	'請': '请', '謝': '谢', '誰': '谁', '兩': '两', '雙': '双', '華': '华', '蘭': '兰',
	'術': '术', '衛': '卫', '視': '视', '覺': '觉', '願': '愿', '靈': '灵', '驚': '惊',
	'驗': '验', '難': '难', '離': '离', '雖': '虽', '隨': '随', '陽': '阳', '陰': '阴',
	'陸': '陆', '險': '险', '隊': '队', '陣': '阵', '際': '际', '隱': '隐', '儲': '储',
	'優': '优', '傷': '伤', '傳': '传', '價': '价', '儀': '仪', '傑': '杰', '備': '备',
	'創': '创', '剛': '刚', '劍': '剑', '勁': '劲', '動': '动', '務': '务', '勝': '胜',
	'勞': '劳', '勢': '势', '區': '区', '參': '参', '號': '号', '補': '补', '裝': '装',
	'語': '语', '誠': '诚', '誤': '误', '課': '课', '調': '调', '論': '论', '談': '谈',
	'講': '讲', '讀': '读', '記': '记', '設': '设', '試': '试', '詩': '诗', '詳': '详',
	'認': '认', '誌': '志', '質': '质', '賴': '赖', '贏': '赢', '趙': '赵', '趕': '赶',
	'週': '周', '進': '进', '遠': '远', '適': '适', '選': '选', '鄉': '乡', '鄭': '郑',
	'鐘': '钟', '鏡': '镜', '長': '长', '閒': '闲', '階': '阶', '靜': '静', '韓': '韩',
	'類': '类', '顯': '显', '騎': '骑', '麥': '麦', '麵': '面',
}

// toSimplified 将繁体字转换为简体字（仅转换映射表中的常用字）。
func toSimplified(s string) string {
	var b strings.Builder
	for _, r := range s {
		if simp, ok := traditionalToSimplifiedMap[r]; ok {
			b.WriteRune(simp)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// IsSongDownloaded 检查歌曲是否已存在（精确匹配 + 可选的歌手包含匹配）。
func IsSongDownloaded(song *model.Song, allSongsSet map[string]struct{}) bool {
	if allSongsSet == nil {
		return false
	}
	key := SongKey(song)
	if _, exists := allSongsSet[key]; exists {
		return true
	}
	// 歌手包含匹配：下载方歌手名包含曲库方歌手（或反之）且歌名相同即命中
	if ruleOn("contain") {
		name := normalizeName(song.Name)
		if name == "" || name == "unknown" {
			return false
		}
		artist := normalizeArtist(song.Artist)
		if artist == "" || artist == "unknown" {
			return false
		}
		suffix := " - " + name
		for k := range allSongsSet {
			if !strings.HasSuffix(k, suffix) {
				continue
			}
			i := strings.Index(k, " - ")
			if i < 0 {
				continue
			}
			if artistsOverlap(normalizeArtist(k[:i]), artist) {
				return true
			}
		}
	}
	// 同音字匹配：歌手与歌名拼音相同即命中
	if ruleOn("homophone") {
		pinyinSetMu.RLock()
		ps := pinyinSet
		pinyinSetMu.RUnlock()
		if ps != nil {
			if _, ok := ps[toPinyinKey(song.Artist, song.Name)]; ok {
				return true
			}
		}
	}
	// 不下 Live 版：下载歌曲名含 Live 标记时，剥离标记后查原版是否已存在
	if ruleOn("nolive") {
		name := normalizeName(song.Name)
		stripped := stripLiveMarkers(name)
		if stripped != name && stripped != "" && stripped != "unknown" {
			artist := normalizeArtist(song.Artist)
			candKey := artist + " - " + stripped
			if _, ok := allSongsSet[candKey]; ok {
				return true
			}
		}
	}
	return false
}

// stripLiveMarkers 剥离歌名中的 Live 版本标记（live、live版、现场版、演唱会版 等）。
func stripLiveMarkers(name string) string {
	s := name
	for _, m := range []string{
		" live版", " live", " (live)", "(live)", "现场版", "演唱会版", "live版",
	} {
		s = strings.ReplaceAll(s, m, "")
	}
	return strings.TrimSpace(s)
}

// artistsOverlap 判断两组歌手（& 分隔）是否存在包含关系：a 是 b 的子集或 b 是 a 的子集。
func artistsOverlap(a, b string) bool {
	as := splitArtists(a)
	bs := splitArtists(b)
	return containsAll(as, bs) || containsAll(bs, as)
}

// splitArtists 按 & 拆分歌手列表并去空白。
func splitArtists(s string) []string {
	parts := strings.Split(s, "&")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// containsAll 判断 sub 的所有元素都在 super 中。
func containsAll(super, sub []string) bool {
	if len(sub) == 0 {
		return true
	}
	for _, s := range sub {
		found := false
		for _, t := range super {
			if t == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
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

// LoadDownloadDedupSet 加载下载去重集合：从 SQLite 查询，不再读取 txt 文件。
func LoadDownloadDedupSet() (map[string]struct{}, error) {
	if err := initImportTables(); err != nil {
		return make(map[string]struct{}), err
	}
	syncSkipRules()
	set := make(map[string]struct{})

	var keys []string
	configDB.Model(&ImportedSong{}).Pluck("key", &keys)
	for _, k := range keys {
		set[NormalizeStoredKey(k)] = struct{}{}
	}
	keys = nil
	configDB.Model(&DownloadLog{}).Where("status = ?", "success").Pluck("key", &keys)
	for _, k := range keys {
		set[NormalizeStoredKey(k)] = struct{}{}
	}

	// 同音字规则开启时构建拼音变体集合；关闭时释放
	if ruleOn("homophone") {
		buildPinyinSet(set)
	} else {
		clearPinyinSet()
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
	syncSkipRules()
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

		key := NormalizeKey(artist, name)
		successLines = append(successLines, key)
		result.Imported++

		if result.Imported <= 5 {
			result.Samples = append(result.Samples, key)
		}
	}

	dataDir := filepath.Dir(resolveAllSongsFilePath())
	result.DataDir = dataDir

	// 写入 SQLite
	if len(successLines) > 0 {
		if err := initImportTables(); err != nil {
			return result
		}
		var batch []ImportedSong
		for _, k := range successLines {
			batch = append(batch, ImportedSong{Key: k})
		}
		for i := 0; i < len(batch); i += 500 {
			end := i + 500
			if end > len(batch) {
				end = len(batch)
			}
			configDB.Clauses(clause.OnConflict{DoNothing: true}).Create(batch[i:end])
		}
	}

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

// AppendLogLine 记录一条下载日志到 SQLite。
func AppendLogLine(filename, line string) error {
	if err := initImportTables(); err != nil {
		return err
	}
	status := "success"
	if strings.Contains(filename, "跳过") {
		status = "skipped"
	} else if strings.Contains(filename, "失败") || strings.Contains(filename, "fail") {
		status = "failed"
	}
	return configDB.Create(&DownloadLog{Key: stripControl(line), Status: status}).Error
}

// ClearAllDownloadLogs 清空所有下载日志记录。
func ClearAllDownloadLogs() error {
	if err := initImportTables(); err != nil {
		return err
	}
	return configDB.Where("1 = 1").Delete(&DownloadLog{}).Error
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
		_ = SaveDownloadRecord(song.Name, song.Artist, song.Source, DownloadStatusSkipped, "已存在")
		_ = AppendLogLine("跳过下载.txt", key)
		return &DownloadedSong{Skipped: true, Filename: key}, nil
	}

	// 1.5 超长文件名检查（不含扩展名，按字节数）
	if ruleOn("longname") {
		maxLen := GetWebSettings().MaxFilenameLen
		if maxLen <= 0 {
			maxLen = DefaultMaxFilenameLen
		}
		fname := BuildDownloadFilename(song, "", filenameTemplate)
		if len(fname) > maxLen {
			_ = SaveDownloadRecord(song.Name, song.Artist, song.Source, DownloadStatusSkipped, "文件名超长")
			_ = AppendLogLine("跳过下载.txt", key+"（文件名超长 "+fmt.Sprint(len(fname))+"字节）")
			return &DownloadedSong{Skipped: true, Filename: key}, nil
		}
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
