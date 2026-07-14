package core

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dhowden/tag"
	"github.com/guohuiyuan/music-lib/model"
)

func TestEmbedSongMetadataWritesReadableMP3ID3v23LyricsAndCover(t *testing.T) {
	audioData := []byte{0xff, 0xfb, 0x90, 0x64, 0x00, 0x00, 0x00, 0x00}
	lyric := "[00:01.00]歌词测试"
	cover := []byte{0xff, 0xd8, 0xff, 0xd9}

	embedded, err := EmbedSongMetadata(audioData, &model.Song{
		Name:   "测试歌",
		Artist: "测试歌手",
		Album:  "测试专辑",
		Ext:    "mp3",
	}, lyric, cover, "image/jpeg")
	if err != nil {
		t.Fatalf("EmbedSongMetadata() error = %v", err)
	}
	if bytes.HasPrefix(embedded, audioData) {
		t.Fatal("embedded data should prepend an ID3 tag before MP3 audio frames")
	}
	if !bytes.HasSuffix(embedded, audioData) {
		t.Fatal("embedded data should preserve original MP3 audio frames")
	}

	metadata, err := tag.ReadFrom(bytes.NewReader(embedded))
	if err != nil {
		t.Fatalf("ReadFrom(embedded): %v", err)
	}
	if metadata.Format() != tag.ID3v2_3 {
		t.Fatalf("metadata format = %v, want ID3v2.3", metadata.Format())
	}
	if metadata.Title() != "测试歌" {
		t.Fatalf("metadata title = %q, want 测试歌", metadata.Title())
	}
	if metadata.Artist() != "测试歌手" {
		t.Fatalf("metadata artist = %q, want 测试歌手", metadata.Artist())
	}
	if metadata.Album() != "测试专辑" {
		t.Fatalf("metadata album = %q, want 测试专辑", metadata.Album())
	}
	if metadata.Lyrics() != lyric {
		t.Fatalf("metadata lyrics = %q, want %q", metadata.Lyrics(), lyric)
	}
	if picture := metadata.Picture(); picture == nil || !bytes.Equal(picture.Data, cover) {
		t.Fatalf("metadata picture = %#v, want embedded cover bytes", picture)
	}
}

func TestEmbedSongMetadataReplacesExistingMP3ID3Tag(t *testing.T) {
	audioData := []byte{0xff, 0xfb, 0x90, 0x64}
	first, err := EmbedSongMetadata(audioData, &model.Song{Name: "旧歌", Artist: "旧歌手", Album: "旧专辑", Ext: "mp3"}, "旧歌词", nil, "")
	if err != nil {
		t.Fatalf("first EmbedSongMetadata() error = %v", err)
	}

	second, err := EmbedSongMetadata(first, &model.Song{Name: "新歌", Artist: "新歌手", Album: "新专辑", Ext: "mp3"}, "新歌词", nil, "")
	if err != nil {
		t.Fatalf("second EmbedSongMetadata() error = %v", err)
	}

	metadata, err := tag.ReadFrom(bytes.NewReader(second))
	if err != nil {
		t.Fatalf("ReadFrom(second): %v", err)
	}
	if metadata.Title() != "新歌" || metadata.Artist() != "新歌手" || metadata.Album() != "新专辑" || metadata.Lyrics() != "新歌词" {
		t.Fatalf("metadata = %q/%q/%q/%q, want 新歌/新歌手/新专辑/新歌词", metadata.Title(), metadata.Artist(), metadata.Album(), metadata.Lyrics())
	}
	if !bytes.HasSuffix(second, audioData) {
		t.Fatal("re-embedded data should keep the original MP3 audio frames once")
	}
}

func TestEmbedSongMetadataPreservesExistingMP3MetadataWhenMissing(t *testing.T) {
	audioData := []byte{0xff, 0xfb, 0x90, 0x64}
	cover := []byte{0xff, 0xd8, 0xff, 0xd9}
	first, err := EmbedSongMetadata(audioData, &model.Song{Name: "旧歌", Artist: "旧歌手", Album: "旧专辑", Ext: "mp3"}, "旧歌词", cover, "image/jpeg")
	if err != nil {
		t.Fatalf("first EmbedSongMetadata() error = %v", err)
	}

	second, err := EmbedSongMetadata(first, &model.Song{Name: "新歌", Artist: "新歌手", Ext: "mp3"}, "", nil, "")
	if err != nil {
		t.Fatalf("second EmbedSongMetadata() error = %v", err)
	}

	metadata, err := tag.ReadFrom(bytes.NewReader(second))
	if err != nil {
		t.Fatalf("ReadFrom(second): %v", err)
	}
	if metadata.Title() != "新歌" || metadata.Artist() != "新歌手" {
		t.Fatalf("metadata title/artist = %q/%q, want 新歌/新歌手", metadata.Title(), metadata.Artist())
	}
	if metadata.Album() != "旧专辑" {
		t.Fatalf("metadata album = %q, want preserved 旧专辑", metadata.Album())
	}
	if metadata.Lyrics() != "旧歌词" {
		t.Fatalf("metadata lyrics = %q, want preserved 旧歌词", metadata.Lyrics())
	}
	if picture := metadata.Picture(); picture == nil || !bytes.Equal(picture.Data, cover) {
		t.Fatalf("metadata picture = %#v, want preserved cover bytes", picture)
	}
}

func TestEmbedSongMetadataPreservesUntouchedMP3ID3Frames(t *testing.T) {
	audioData := []byte{0xff, 0xfb, 0x90, 0x64}

	var existingFrames bytes.Buffer
	existingFrames.Write(id3v23Frame("TCON", id3TextFramePayload("Rock")))
	existingFrames.Write(id3v23Frame("TRCK", id3TextFramePayload("7")))
	frameData := existingFrames.Bytes()
	tagSize := id3SynchsafeSize(len(frameData))
	tagged := make([]byte, 0, 10+len(frameData)+len(audioData))
	tagged = append(tagged, 'I', 'D', '3', 0x03, 0x00, 0x00)
	tagged = append(tagged, tagSize[:]...)
	tagged = append(tagged, frameData...)
	tagged = append(tagged, audioData...)

	embedded, err := EmbedSongMetadata(tagged, &model.Song{Name: "New Title", Artist: "New Artist", Album: "New Album", Ext: "mp3"}, "New lyric", nil, "")
	if err != nil {
		t.Fatalf("EmbedSongMetadata() error = %v", err)
	}

	metadata, err := tag.ReadFrom(bytes.NewReader(embedded))
	if err != nil {
		t.Fatalf("ReadFrom(embedded): %v", err)
	}
	if metadata.Title() != "New Title" || metadata.Artist() != "New Artist" || metadata.Album() != "New Album" {
		t.Fatalf("metadata title/artist/album = %q/%q/%q, want New Title/New Artist/New Album", metadata.Title(), metadata.Artist(), metadata.Album())
	}
	if metadata.Genre() != "Rock" {
		t.Fatalf("metadata genre = %q, want preserved Rock", metadata.Genre())
	}
	track, _ := metadata.Track()
	if track != 7 {
		t.Fatalf("metadata track = %d, want preserved 7", track)
	}
}

func TestEmbedSongMetadataPreservesFLACMetadataWhenAddingAlbum(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	inPath := filepath.Join(t.TempDir(), "source.flac")
	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-hide_banner",
		"-loglevel",
		"error",
		"-f",
		"lavfi",
		"-i",
		"anullsrc=r=8000:cl=mono",
		"-t",
		"0.05",
		"-metadata",
		"title=Old Title",
		"-metadata",
		"artist=Old Artist",
		"-metadata",
		"album=Old Album",
		"-metadata",
		"genre=Jazz",
		inPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg create flac failed: %v, output: %s", err, string(out))
	}

	audioData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("ReadFile(source.flac): %v", err)
	}
	embedded, err := EmbedSongMetadata(audioData, &model.Song{Album: "New Album", Ext: "flac"}, "", nil, "")
	if err != nil {
		t.Fatalf("EmbedSongMetadata() error = %v", err)
	}

	metadata, err := tag.ReadFrom(bytes.NewReader(embedded))
	if err != nil {
		t.Fatalf("ReadFrom(embedded): %v", err)
	}
	if metadata.Title() != "Old Title" || metadata.Artist() != "Old Artist" {
		t.Fatalf("metadata title/artist = %q/%q, want preserved Old Title/Old Artist", metadata.Title(), metadata.Artist())
	}
	if metadata.Album() != "New Album" {
		t.Fatalf("metadata album = %q, want New Album", metadata.Album())
	}
	if metadata.Genre() != "Jazz" {
		t.Fatalf("metadata genre = %q, want preserved Jazz", metadata.Genre())
	}
}
