package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func configureBundledFFmpegFromNativeLibraryDir(nativeLibraryDir string) error {
	return configureBundledFFmpegTools(nativeLibraryDir, "libffmpeg.so", "libffprobe.so")
}

func configureBundledFFmpegFromExtractDir(dir string) error {
	return configureBundledFFmpegTools(dir, "ffmpeg", "ffprobe")
}

func configureBundledFFmpegTools(dir, ffmpegName, ffprobeName string) error {
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "" || dir == "." {
		return fmt.Errorf("empty native library dir")
	}

	ffmpegPath := filepath.Join(dir, ffmpegName)
	ffprobePath := filepath.Join(dir, ffprobeName)
	if err := validateBundledTool(ffmpegPath); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}
	if err := validateBundledTool(ffprobePath); err != nil {
		return fmt.Errorf("ffprobe: %w", err)
	}

	_ = os.Setenv("MUSIC_DL_FFMPEG", ffmpegPath)
	_ = os.Setenv("MUSIC_DL_FFPROBE", ffprobePath)
	prependPathDir(dir)
	prependEnvPathDir("LD_LIBRARY_PATH", dir)
	return nil
}

func validateBundledTool(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%q is a directory", path)
	}
	_ = os.Chmod(path, 0755)
	return nil
}

func prependPathDir(dir string) {
	prependEnvPathDir("PATH", dir)
}

func prependEnvPathDir(name string, dir string) {
	current := os.Getenv(name)
	for _, entry := range filepath.SplitList(current) {
		if filepath.Clean(entry) == dir {
			return
		}
	}
	if current == "" {
		_ = os.Setenv(name, dir)
		return
	}
	_ = os.Setenv(name, dir+string(os.PathListSeparator)+current)
}
