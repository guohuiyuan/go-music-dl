package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStandardLRCForExport(t *testing.T) {
	raw := "\uFEFF[ti:test]\r\n" +
		"[00:01.00]你[00:01.50]好[00:02.00]\n" +
		"[00:01.00]hello[00:02.00]\r" +
		"[00:03.00][00:10.00]世界[00:04.00]"
	want := "[ti:test]\r\n" +
		"[00:01.00]你好\r\n" +
		"[00:01.00]hello\r\n" +
		"[00:03.00][00:10.00]世界\r\n"

	if got := StandardLRCForExport(raw); got != want {
		t.Fatalf("StandardLRCForExport() mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestSaveDownloadedSongLyricSidecarWhenEnabled(t *testing.T) {
	setLyricSidecarSettingForTest(t, true)
	outDir := t.TempDir()
	result := &DownloadedSong{
		Data:     []byte("audio"),
		Filename: filepath.Join("Artist", "Album", "Song - Artist.flac"),
		Lyric:    "[00:01.00]你[00:01.50]好[00:02.00]",
	}

	saved, err := saveDownloadedSongToFile(result, outDir)
	if err != nil {
		t.Fatal(err)
	}

	wantPath := filepath.Join(outDir, "Artist", "Album", "Song - Artist.lrc")
	if saved.LyricPath != wantPath {
		t.Fatalf("LyricPath = %q, want %q", saved.LyricPath, wantPath)
	}
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "[00:01.00]你好\r\n"; got != want {
		t.Fatalf("sidecar lyric = %q, want %q", got, want)
	}
}

func TestSaveDownloadedSongLyricSidecarDisabled(t *testing.T) {
	setLyricSidecarSettingForTest(t, false)
	outDir := t.TempDir()
	result := &DownloadedSong{
		Data:     []byte("audio"),
		Filename: "Song.mp3",
		Lyric:    "[00:01.00]line",
	}

	saved, err := saveDownloadedSongToFile(result, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if saved.LyricPath != "" {
		t.Fatalf("LyricPath = %q, want empty", saved.LyricPath)
	}
	if _, err := os.Stat(filepath.Join(outDir, "Song.lrc")); !os.IsNotExist(err) {
		t.Fatalf("sidecar should not exist, stat error = %v", err)
	}
}

func TestSaveDownloadedSongLyricSidecarExplicitlyDisabled(t *testing.T) {
	setLyricSidecarSettingForTest(t, true)
	outDir := t.TempDir()
	result := &DownloadedSong{
		Data:     []byte("audio"),
		Filename: "Staged.mp3",
		Lyric:    "[00:01.00]should not be exported",
	}

	saved, err := saveDownloadedSongToFileWithLyricSidecar(result, outDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if saved.LyricPath != "" {
		t.Fatalf("LyricPath = %q, want empty", saved.LyricPath)
	}
	if _, err := os.Stat(filepath.Join(outDir, "Staged.lrc")); !os.IsNotExist(err) {
		t.Fatalf("sidecar should not exist, stat error = %v", err)
	}
}

func TestSaveDownloadedSongLyricSidecarPreservesExistingFile(t *testing.T) {
	setLyricSidecarSettingForTest(t, true)
	outDir := t.TempDir()
	lyricPath := filepath.Join(outDir, "Song.lrc")
	if err := os.WriteFile(lyricPath, []byte("user lyric"), 0644); err != nil {
		t.Fatal(err)
	}
	result := &DownloadedSong{
		Data:     []byte("audio"),
		Filename: "Song.mp3",
		Lyric:    "[00:01.00]new lyric",
	}

	saved, err := saveDownloadedSongToFile(result, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if saved.LyricPath != lyricPath {
		t.Fatalf("LyricPath = %q, want %q", saved.LyricPath, lyricPath)
	}
	if !strings.Contains(saved.Warning, "kept existing file") {
		t.Fatalf("warning = %q, want existing file warning", saved.Warning)
	}
	data, err := os.ReadFile(lyricPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "user lyric" {
		t.Fatalf("existing lyric was overwritten: %q", data)
	}
}

func setLyricSidecarSettingForTest(t *testing.T, enabled bool) {
	t.Helper()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(t.TempDir(), "settings.db"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)
	if err := SaveWebSettings(WebSettings{DownloadLyricsToLocal: enabled}); err != nil {
		t.Fatalf("SaveWebSettings: %v", err)
	}
}
