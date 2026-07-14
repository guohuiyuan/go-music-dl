package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestBuildDownloadFilenameUsesTemplate(t *testing.T) {
	song := &model.Song{
		ID:     "12345",
		Source: "netease",
		Name:   "没地址的信",
		Artist: "阮俊霖",
		Album:  "专辑/测试",
	}

	tests := []struct {
		name     string
		template string
		ext      string
		want     string
	}{
		{
			name:     "default template appends extension",
			template: "",
			ext:      "mp3",
			want:     "没地址的信 - 阮俊霖.mp3",
		},
		{
			name:     "custom template can create subdirectories",
			template: "{artist}/{album}/{name}",
			ext:      "flac",
			want:     filepath.Join("阮俊霖", "专辑_测试", "没地址的信.flac"),
		},
		{
			name:     "extension token controls extension position in subdirectory template",
			template: "{artist}/{album}/{name} - {artist}.{ext}",
			ext:      "flac",
			want:     filepath.Join("阮俊霖", "专辑_测试", "没地址的信 - 阮俊霖.flac"),
		},
		{
			name:     "path traversal segments are ignored",
			template: "../{artist}/./{name}.{ext}",
			ext:      "m4a",
			want:     filepath.Join("阮俊霖", "没地址的信.m4a"),
		},
		{
			name:     "flat template still works",
			template: "{source}-{id}-{name}.{ext}",
			ext:      "m4a",
			want:     "netease-12345-没地址的信.m4a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildDownloadFilename(song, tt.ext, tt.template); got != tt.want {
				t.Fatalf("BuildDownloadFilename() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSaveDownloadedSongToFileCreatesTemplateSubdirectories(t *testing.T) {
	dir := t.TempDir()
	result := &DownloadedSong{
		Data:     []byte("audio"),
		Filename: filepath.Join("阮俊霖", "专辑", "没地址的信.flac"),
	}

	saved, err := saveDownloadedSongToFile(result, dir)
	if err != nil {
		t.Fatal(err)
	}

	wantPath := filepath.Join(dir, "阮俊霖", "专辑", "没地址的信.flac")
	if saved.SavedPath != wantPath {
		t.Fatalf("SavedPath = %q, want %q", saved.SavedPath, wantPath)
	}
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "audio" {
		t.Fatalf("saved data = %q, want audio", string(data))
	}
}
