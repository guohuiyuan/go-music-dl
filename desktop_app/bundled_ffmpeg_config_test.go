package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureBundledFFmpegFromNativeLibraryDirSetsEnv(t *testing.T) {
	dir := t.TempDir()
	ffmpeg := filepath.Join(dir, "libffmpeg.so")
	ffprobe := filepath.Join(dir, "libffprobe.so")
	if err := os.WriteFile(ffmpeg, []byte("ffmpeg"), 0755); err != nil {
		t.Fatalf("write ffmpeg: %v", err)
	}
	if err := os.WriteFile(ffprobe, []byte("ffprobe"), 0755); err != nil {
		t.Fatalf("write ffprobe: %v", err)
	}

	t.Setenv("MUSIC_DL_FFMPEG", "")
	t.Setenv("MUSIC_DL_FFPROBE", "")
	t.Setenv("PATH", "")
	t.Setenv("LD_LIBRARY_PATH", "")

	if err := configureBundledFFmpegFromNativeLibraryDir(dir); err != nil {
		t.Fatalf("configureBundledFFmpegFromNativeLibraryDir error: %v", err)
	}
	if got := os.Getenv("MUSIC_DL_FFMPEG"); got != ffmpeg {
		t.Fatalf("MUSIC_DL_FFMPEG = %q, want %q", got, ffmpeg)
	}
	if got := os.Getenv("MUSIC_DL_FFPROBE"); got != ffprobe {
		t.Fatalf("MUSIC_DL_FFPROBE = %q, want %q", got, ffprobe)
	}
	if got := os.Getenv("PATH"); !strings.HasPrefix(got, dir) {
		t.Fatalf("PATH = %q, want prefix %q", got, dir)
	}
	if got := os.Getenv("LD_LIBRARY_PATH"); !strings.HasPrefix(got, dir) {
		t.Fatalf("LD_LIBRARY_PATH = %q, want prefix %q", got, dir)
	}
}

func TestConfigureBundledFFmpegFromNativeLibraryDirRequiresBothTools(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "libffmpeg.so"), []byte("ffmpeg"), 0755); err != nil {
		t.Fatalf("write ffmpeg: %v", err)
	}

	if err := configureBundledFFmpegFromNativeLibraryDir(dir); err == nil {
		t.Fatal("configureBundledFFmpegFromNativeLibraryDir should fail without libffprobe.so")
	}
}

func TestConfigureBundledFFmpegFromExtractDirSetsEnv(t *testing.T) {
	dir := t.TempDir()
	ffmpeg := filepath.Join(dir, "ffmpeg")
	ffprobe := filepath.Join(dir, "ffprobe")
	if err := os.WriteFile(ffmpeg, []byte("ffmpeg"), 0755); err != nil {
		t.Fatalf("write ffmpeg: %v", err)
	}
	if err := os.WriteFile(ffprobe, []byte("ffprobe"), 0755); err != nil {
		t.Fatalf("write ffprobe: %v", err)
	}

	t.Setenv("MUSIC_DL_FFMPEG", "")
	t.Setenv("MUSIC_DL_FFPROBE", "")
	t.Setenv("PATH", "")
	t.Setenv("LD_LIBRARY_PATH", "")

	if err := configureBundledFFmpegFromExtractDir(dir); err != nil {
		t.Fatalf("configureBundledFFmpegFromExtractDir error: %v", err)
	}
	if got := os.Getenv("MUSIC_DL_FFMPEG"); got != ffmpeg {
		t.Fatalf("MUSIC_DL_FFMPEG = %q, want %q", got, ffmpeg)
	}
	if got := os.Getenv("MUSIC_DL_FFPROBE"); got != ffprobe {
		t.Fatalf("MUSIC_DL_FFPROBE = %q, want %q", got, ffprobe)
	}
	if got := os.Getenv("PATH"); !strings.HasPrefix(got, dir) {
		t.Fatalf("PATH = %q, want prefix %q", got, dir)
	}
	if got := os.Getenv("LD_LIBRARY_PATH"); !strings.HasPrefix(got, dir) {
		t.Fatalf("LD_LIBRARY_PATH = %q, want prefix %q", got, dir)
	}
}
