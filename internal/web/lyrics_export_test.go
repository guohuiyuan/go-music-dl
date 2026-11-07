package web

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

func TestLyricDownloadFilenameUsesAudioFilenameTemplate(t *testing.T) {
	initCollectionDBForTest(t)
	if err := core.SaveWebSettings(core.WebSettings{
		DownloadFilenameTemplate: "{artist} - {name}",
	}); err != nil {
		t.Fatalf("SaveWebSettings: %v", err)
	}

	song := &model.Song{Name: "Song", Artist: "Artist", Album: "Album"}
	if got, want := lyricDownloadFilename(song), filepath.Join("Artist - Song.lrc"); got != want {
		t.Fatalf("lyricDownloadFilename() = %q, want %q", got, want)
	}
}

func TestSaveWebAssetRelativeToLocalUsesFilenameTemplate(t *testing.T) {
	initCollectionDBForTest(t)
	downloadDir := t.TempDir()
	if err := core.SaveWebSettings(core.WebSettings{
		DownloadDir:              downloadDir,
		DownloadFilenameTemplate: "{artist}/{album}/{name}.{ext}",
	}); err != nil {
		t.Fatalf("SaveWebSettings: %v", err)
	}

	song := &model.Song{Name: "Song", Artist: "Artist", Album: "Album"}
	filename := core.BuildDownloadFilename(song, "lrc", core.GetWebSettings().DownloadFilenameTemplate)
	data := []byte("[00:01.00]line\r\n")
	savedPath, savedFilename, err := saveWebAssetRelativeToLocal(filename, data)
	if err != nil {
		t.Fatal(err)
	}

	wantFilename := filepath.Join("Artist", "Album", "Song.lrc")
	if savedFilename != wantFilename {
		t.Fatalf("savedFilename = %q, want %q", savedFilename, wantFilename)
	}
	if savedPath != filepath.Join(downloadDir, wantFilename) {
		t.Fatalf("savedPath = %q, want %q", savedPath, filepath.Join(downloadDir, wantFilename))
	}
	got, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("saved lyric = %q, want %q", got, data)
	}

	if _, _, err := saveWebAssetRelativeToLocal(filename, []byte("different")); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing lyric error = %v, want os.ErrExist", err)
	}
	got, err = os.ReadFile(savedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("existing lyric was overwritten: %q", got)
	}
}

func TestSaveWebAssetRelativeToLocalRejectsEscapingPath(t *testing.T) {
	initCollectionDBForTest(t)
	if _, _, err := saveWebAssetRelativeToLocal("../outside.lrc", []byte("lyric")); err == nil {
		t.Fatal("path traversal should be rejected")
	}
}

func TestLocalMusicLyricSidecarFilenameUsesActualAudioPath(t *testing.T) {
	track := &localMusicTrack{
		Name:     "Metadata Title",
		Artist:   "Metadata Artist",
		Filename: "Audio File.flac",
		RelPath:  "Folder/Audio File.flac",
	}
	if got, want := localMusicLyricSidecarFilename(track), "Folder/Audio File.lrc"; got != want {
		t.Fatalf("localMusicLyricSidecarFilename() = %q, want %q", got, want)
	}
}
