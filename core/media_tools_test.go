package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveMediaToolPathUsesAbsoluteEnvPath(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "ffmpeg")
	if err := os.WriteFile(tool, []byte("test"), 0755); err != nil {
		t.Fatalf("write tool: %v", err)
	}

	t.Setenv(ffmpegEnvName, tool)
	got, err := ResolveFFmpegPath()
	if err != nil {
		t.Fatalf("ResolveFFmpegPath error: %v", err)
	}
	if got != tool {
		t.Fatalf("ResolveFFmpegPath = %q, want %q", got, tool)
	}
}

func TestResolveMediaToolPathRejectsMissingAbsoluteEnvPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "ffprobe")

	t.Setenv(ffprobeEnvName, missing)
	if _, err := ResolveFFprobePath(); err == nil {
		t.Fatal("ResolveFFprobePath should fail for a missing configured path")
	}
}
