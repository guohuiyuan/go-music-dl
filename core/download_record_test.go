package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

// newDownloadDedupTestDB 给每个用例一份独立的 SQLite 配置库。
func newDownloadDedupTestDB(t *testing.T) {
	t.Helper()

	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "settings.db"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)
}

func writeDownloadDedupFile(t *testing.T, dir, relPath string) string {
	t.Helper()

	filePath := filepath.Join(dir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("mkdir for %s: %v", relPath, err)
	}
	if err := os.WriteFile(filePath, []byte("audio"), 0644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
	return filePath
}

func TestDownloadDedupIndexUsesSQLiteAndSurvivesHistoryClear(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "settings.db"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)

	if err := SaveDownloadRecord("Song\nTitle", "Artist\rName", "source", DownloadStatusSuccess, ""); err != nil {
		t.Fatalf("SaveDownloadRecord: %v", err)
	}

	dedupSet, err := LoadDownloadDedupSet()
	if err != nil {
		t.Fatalf("LoadDownloadDedupSet: %v", err)
	}
	song := &model.Song{Name: "SongTitle", Artist: "ArtistName"}
	if !IsSongDownloaded(song, dedupSet) {
		t.Fatal("successful record should be available through the SQLite de-duplication index")
	}

	if err := ClearDownloadRecords(); err != nil {
		t.Fatalf("ClearDownloadRecords: %v", err)
	}
	records, err := GetDownloadRecords()
	if err != nil {
		t.Fatalf("GetDownloadRecords: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("download history length = %d, want 0", len(records))
	}

	dedupSet, err = LoadDownloadDedupSet()
	if err != nil {
		t.Fatalf("LoadDownloadDedupSet after clear: %v", err)
	}
	if !IsSongDownloaded(song, dedupSet) {
		t.Fatal("clearing visible history must not clear the SQLite de-duplication index")
	}
}

func TestGetDownloadRecordPageReturnsStablePagesAndTotal(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "settings.db"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)

	for _, name := range []string{"First", "Second", "Third"} {
		if err := SaveDownloadRecord(name, "Artist", "qq", DownloadStatusSuccess, ""); err != nil {
			t.Fatalf("SaveDownloadRecord(%q): %v", name, err)
		}
	}

	records, total, err := GetDownloadRecordPage(2, 1)
	if err != nil {
		t.Fatalf("GetDownloadRecordPage: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if len(records) != 1 || records[0].Name != "Second" {
		t.Fatalf("page 2 = %#v, want Second", records)
	}
}

func TestDownloadRecordPersistsFileRelPath(t *testing.T) {
	newDownloadDedupTestDB(t)

	if err := SaveDownloadRecordWithRelPath("Song", "Artist", "qq", DownloadStatusSuccess, "", `Artist\Song.flac`); err != nil {
		t.Fatalf("SaveDownloadRecordWithRelPath: %v", err)
	}
	records, err := GetDownloadRecords()
	if err != nil {
		t.Fatalf("GetDownloadRecords: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %#v, want one record", records)
	}
	record, err := GetDownloadRecord(records[0].ID)
	if err != nil {
		t.Fatalf("GetDownloadRecord: %v", err)
	}
	if record.RelPath != "Artist/Song.flac" {
		t.Fatalf("RelPath = %q, want normalized relative path", record.RelPath)
	}
	relPath, err := ResolveDownloadRecordRelPath(record)
	if err != nil {
		t.Fatalf("ResolveDownloadRecordRelPath: %v", err)
	}
	if relPath != record.RelPath {
		t.Fatalf("resolved RelPath = %q, want %q", relPath, record.RelPath)
	}
}

func TestDownloadRecordResolvesLegacyRelPathFromDedupIndex(t *testing.T) {
	newDownloadDedupTestDB(t)

	if err := SaveDownloadRecord("Legacy Song", "Artist", "qq", DownloadStatusSuccess, ""); err != nil {
		t.Fatalf("SaveDownloadRecord: %v", err)
	}
	if err := SaveDownloadDedupEntry("Legacy Song", "Artist", "Artist/Legacy Song.flac"); err != nil {
		t.Fatalf("SaveDownloadDedupEntry: %v", err)
	}
	records, err := GetDownloadRecords()
	if err != nil || len(records) != 1 {
		t.Fatalf("GetDownloadRecords: records=%#v err=%v", records, err)
	}
	relPath, err := ResolveDownloadRecordRelPath(records[0])
	if err != nil {
		t.Fatalf("ResolveDownloadRecordRelPath: %v", err)
	}
	if relPath != "Artist/Legacy Song.flac" {
		t.Fatalf("resolved legacy RelPath = %q", relPath)
	}
}

// 文件被删掉（含在 NAS 后台直接删）以后，这首歌不能再被判成「已下载」，否则再点
// 下载只会返回「跳过」。
func TestDeletedDownloadFileStopsBeingSkipped(t *testing.T) {
	newDownloadDedupTestDB(t)
	downloadDir := t.TempDir()

	const relPath = "Artist - Song.flac"
	filePath := writeDownloadDedupFile(t, downloadDir, relPath)

	if err := SaveDownloadDedupEntry("Song", "Artist", relPath); err != nil {
		t.Fatalf("SaveDownloadDedupEntry: %v", err)
	}

	song := &model.Song{Name: "Song", Artist: "Artist"}
	index, err := LoadDownloadDedupSet()
	if err != nil {
		t.Fatalf("LoadDownloadDedupSet: %v", err)
	}
	if !IsSongStillDownloaded(song, index, downloadDir) {
		t.Fatal("文件还在磁盘上时应当判定为已下载")
	}
	if got := SongFileRelPath(song, index); got != relPath {
		t.Fatalf("SongFileRelPath = %q, want %q", got, relPath)
	}

	if err := os.Remove(filePath); err != nil {
		t.Fatalf("remove downloaded file: %v", err)
	}

	index, err = LoadDownloadDedupSet()
	if err != nil {
		t.Fatalf("LoadDownloadDedupSet after delete: %v", err)
	}
	if IsSongStillDownloaded(song, index, downloadDir) {
		t.Fatal("文件已删除，不应再判定为已下载")
	}

	index, err = LoadDownloadDedupSet()
	if err != nil {
		t.Fatalf("LoadDownloadDedupSet after recycle: %v", err)
	}
	if IsSongDownloaded(song, index) {
		t.Fatal("发现文件被删除后，去重记录应当一起回收")
	}
}

// 没有记录文件路径的旧记录按「仍在本地」处理，交由上层用本地曲库索引反查。
func TestLegacyDedupEntryWithoutRelPathStaysDownloaded(t *testing.T) {
	newDownloadDedupTestDB(t)

	if err := SaveDownloadDedupEntry("Song", "Artist", ""); err != nil {
		t.Fatalf("SaveDownloadDedupEntry: %v", err)
	}

	song := &model.Song{Name: "Song", Artist: "Artist"}
	index, err := LoadDownloadDedupSet()
	if err != nil {
		t.Fatalf("LoadDownloadDedupSet: %v", err)
	}
	if !IsSongStillDownloaded(song, index, t.TempDir()) {
		t.Fatal("旧记录没有路径，无法校验磁盘时不应误判为可重新下载")
	}
}

func TestDownloadLookupFindsLegacyTraditionalChineseKey(t *testing.T) {
	index := DownloadDedupIndex{
		"久石讓 - Summer 久石讓": "久石讓 - Summer 久石讓.flac",
	}
	song := &model.Song{Name: "Summer 久石让", Artist: "久石让"}

	if !IsSongDownloaded(song, index) {
		t.Fatal("simplified song should match a legacy traditional Chinese key")
	}
	if got := SongFileRelPath(song, index); got != "久石讓 - Summer 久石讓.flac" {
		t.Fatalf("SongFileRelPath = %q", got)
	}
}

func TestForgetDownloadedSongClearsDedupEntry(t *testing.T) {
	newDownloadDedupTestDB(t)
	song := &model.Song{Name: "Song", Artist: "Artist"}

	if err := SaveDownloadDedupEntry("Song", "Artist", "Artist - Song.flac"); err != nil {
		t.Fatalf("SaveDownloadDedupEntry: %v", err)
	}
	if err := ForgetDownloadedSong("Song", "Artist", "Artist - Song.flac"); err != nil {
		t.Fatalf("ForgetDownloadedSong: %v", err)
	}

	index, err := LoadDownloadDedupSet()
	if err != nil {
		t.Fatalf("LoadDownloadDedupSet: %v", err)
	}
	if IsSongDownloaded(song, index) {
		t.Fatal("清除后不应再命中去重记录")
	}

	// 歌名/歌手对不上时，靠文件路径也要能清掉。
	if err := SaveDownloadDedupEntry("Song", "Artist", "Artist - Song.flac"); err != nil {
		t.Fatalf("SaveDownloadDedupEntry: %v", err)
	}
	if err := ForgetDownloadedSong("Other", "Someone", "Artist - Song.flac"); err != nil {
		t.Fatalf("ForgetDownloadedSong by rel path: %v", err)
	}
	index, err = LoadDownloadDedupSet()
	if err != nil {
		t.Fatalf("LoadDownloadDedupSet after relpath forget: %v", err)
	}
	if IsSongDownloaded(song, index) {
		t.Fatal("按文件路径也应该能回收去重记录")
	}
}

func TestPruneDownloadDedupDropsMissingFilesAndBackfillsPath(t *testing.T) {
	newDownloadDedupTestDB(t)

	if err := SaveDownloadDedupEntry("Keep", "Artist", "Artist - Keep.flac"); err != nil {
		t.Fatalf("SaveDownloadDedupEntry(Keep): %v", err)
	}
	if err := SaveDownloadDedupEntry("Drop", "Artist", "Artist - Drop.flac"); err != nil {
		t.Fatalf("SaveDownloadDedupEntry(Drop): %v", err)
	}
	// 旧记录：只有歌名+歌手，没有文件路径。
	if err := SaveDownloadDedupEntry("Legacy", "Artist", ""); err != nil {
		t.Fatalf("SaveDownloadDedupEntry(Legacy): %v", err)
	}

	keep := map[string]string{
		SongKey(&model.Song{Name: "Keep", Artist: "Artist"}):   "Artist - Keep.flac",
		SongKey(&model.Song{Name: "Legacy", Artist: "Artist"}): "Artist - Legacy.flac",
	}
	removed, err := PruneDownloadDedup(t.TempDir(), keep)
	if err != nil {
		t.Fatalf("PruneDownloadDedup: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1 (只有 Drop 的文件不在本地)", removed)
	}

	index, err := LoadDownloadDedupSet()
	if err != nil {
		t.Fatalf("LoadDownloadDedupSet: %v", err)
	}
	if IsSongDownloaded(&model.Song{Name: "Drop", Artist: "Artist"}, index) {
		t.Fatal("文件已不在下载目录，记录应当被回收")
	}
	if !IsSongDownloaded(&model.Song{Name: "Keep", Artist: "Artist"}, index) {
		t.Fatal("文件还在本地，记录必须保留")
	}
	if got := SongFileRelPath(&model.Song{Name: "Legacy", Artist: "Artist"}, index); got != "Artist - Legacy.flac" {
		t.Fatalf("旧记录的文件路径应被回填，got %q", got)
	}
}

func TestPruneDownloadDedupKeepsRecordCreatedAfterScan(t *testing.T) {
	newDownloadDedupTestDB(t)
	downloadDir := t.TempDir()

	const relPath = "Artist - New.flac"
	writeDownloadDedupFile(t, downloadDir, relPath)
	if err := SaveDownloadDedupEntry("New", "Artist", relPath); err != nil {
		t.Fatalf("SaveDownloadDedupEntry: %v", err)
	}

	// 模拟扫描快照生成后才完成下载，因此 keep 中还没有这首新歌。
	removed, err := PruneDownloadDedup(downloadDir, nil)
	if err != nil {
		t.Fatalf("PruneDownloadDedup: %v", err)
	}
	if removed != 0 {
		t.Fatalf("removed = %d, want 0", removed)
	}

	index, err := LoadDownloadDedupSet()
	if err != nil {
		t.Fatalf("LoadDownloadDedupSet: %v", err)
	}
	if !IsSongDownloaded(&model.Song{Name: "New", Artist: "Artist"}, index) {
		t.Fatal("磁盘上仍存在的并发下载记录不能被旧扫描快照回收")
	}
}
