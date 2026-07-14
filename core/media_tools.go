package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	ffmpegEnvName  = "MUSIC_DL_FFMPEG"
	ffprobeEnvName = "MUSIC_DL_FFPROBE"
	ffplayEnvName  = "MUSIC_DL_FFPLAY"
)

func ResolveFFmpegPath() (string, error) {
	return resolveMediaToolPath(ffmpegEnvName, "ffmpeg")
}

func ResolveFFprobePath() (string, error) {
	return resolveMediaToolPath(ffprobeEnvName, "ffprobe")
}

func resolveMediaToolPath(envName, toolName string) (string, error) {
	configured := strings.TrimSpace(os.Getenv(envName))
	if configured != "" {
		return validateConfiguredMediaTool(envName, configured)
	}
	return exec.LookPath(toolName)
}

func validateConfiguredMediaTool(envName, path string) (string, error) {
	if !filepath.IsAbs(path) {
		return exec.LookPath(path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s points to %q: %w", envName, path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s points to a directory: %q", envName, path)
	}
	return path, nil
}
