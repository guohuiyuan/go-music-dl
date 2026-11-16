package core

import (
	"bytes"
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
		Extra:  map[string]string{"album_artist": "群星/合辑"},
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
			want:     "阮俊霖 - 没地址的信.mp3",
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
			name:     "album artist token uses source metadata and sanitizes subdirectory",
			template: "{album_artist}/{artist}/{name}",
			ext:      "flac",
			want:     filepath.Join("群星_合辑", "阮俊霖", "没地址的信.flac"),
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

func TestBuildDownloadFilenameAlbumArtistFallsBackToArtist(t *testing.T) {
	song := &model.Song{
		Name:   "没地址的信",
		Artist: "阮俊霖",
		Extra:  map[string]string{"albumArtist": "  "},
	}
	got := BuildDownloadFilename(song, "mp3", "{album_artist} - {name}")
	want := "阮俊霖 - 没地址的信.mp3"
	if got != want {
		t.Fatalf("BuildDownloadFilename() = %q, want %q", got, want)
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

func TestLiveMiguHighestQualityDownload(t *testing.T) {
	cookie := os.Getenv("MIGU_TEST_COOKIE")
	if cookie == "" {
		t.Skip("set MIGU_TEST_COOKIE to run the live Migu download test")
	}

	CM.SetAll(map[string]string{"migu": cookie})
	t.Cleanup(func() { CM.SetAll(map[string]string{"migu": ""}) })

	song := &model.Song{
		Source:  "migu",
		ID:      "600919000009811300|2|Z3D",
		AlbumID: "1138997327",
		Extra: map[string]string{
			"content_id":    "600919000009811300",
			"resource_type": "2",
			"format_type":   "Z3D",
			"copyright_id":  "6005861HZLK",
			"song_id":       "1138997181",
			"album_id":      "1138997327",
		},
	}

	data, err := FetchDecryptedMiguAudio(song)
	if err != nil {
		t.Fatalf("FetchDecryptedMiguAudio() error = %v", err)
	}
	if len(data) < 16 {
		t.Fatalf("downloaded data is too short: %d", len(data))
	}
	if !bytes.HasPrefix(data, []byte("RIFF")) &&
		!bytes.HasPrefix(data, []byte("fLaC")) &&
		!bytes.HasPrefix(data, []byte("ID3")) &&
		!(data[0] == 0xFF && data[1]&0xE0 == 0xE0) {
		t.Fatalf("downloaded data is not a supported audio stream")
	}
}
