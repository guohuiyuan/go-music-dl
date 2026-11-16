package core

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestEmbedSongMetadataSupportsFLACInMP4AndWebPCover(t *testing.T) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not available")
	}
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not available")
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "soda-lossless.mp4")
	createAudio := exec.Command(
		ffmpegPath,
		"-y",
		"-hide_banner",
		"-loglevel",
		"error",
		"-f",
		"lavfi",
		"-i",
		"anullsrc=r=44100:cl=stereo",
		"-t",
		"0.2",
		"-c:a",
		"flac",
		inputPath,
	)
	if out, err := createAudio.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg create FLAC-in-MP4 failed: %v, output: %s", err, out)
	}

	coverPath := filepath.Join(dir, "cover.webp")
	createCover := exec.Command(
		ffmpegPath,
		"-y",
		"-hide_banner",
		"-loglevel",
		"error",
		"-f",
		"lavfi",
		"-i",
		"color=c=red:s=32x32",
		"-frames:v",
		"1",
		coverPath,
	)
	if out, err := createCover.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg create WebP cover failed: %v, output: %s", err, out)
	}

	audioData, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("ReadFile(audio): %v", err)
	}
	coverData, err := os.ReadFile(coverPath)
	if err != nil {
		t.Fatalf("ReadFile(cover): %v", err)
	}

	embedded, err := EmbedSongMetadata(
		audioData,
		&model.Song{
			Name:   "最真的梦 (女声深情版)",
			Artist: "七分可可",
			Album:  "最真的梦 (女声深情版)",
			Ext:    "m4a",
		},
		"[00:01.00]test lyric",
		coverData,
		"image/webp",
	)
	if err != nil {
		t.Fatalf("EmbedSongMetadata() error = %v", err)
	}
	if len(embedded) < 8 || !bytes.Equal(embedded[4:8], []byte("ftyp")) {
		t.Fatalf("embedded data is not an MP4 container, prefix=%x", embedded[:min(16, len(embedded))])
	}

	outputPath := filepath.Join(dir, "embedded.m4a")
	if err := os.WriteFile(outputPath, embedded, 0o600); err != nil {
		t.Fatalf("WriteFile(embedded): %v", err)
	}
	probe, err := exec.Command(
		ffprobePath,
		"-v",
		"error",
		"-show_entries",
		"stream=codec_name,codec_type:format_tags=title,artist,album,lyrics",
		"-of",
		"json",
		outputPath,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("ffprobe failed: %v, output: %s", err, probe)
	}

	var metadata struct {
		Streams []struct {
			CodecName string `json:"codec_name"`
			CodecType string `json:"codec_type"`
		} `json:"streams"`
		Format struct {
			Tags map[string]string `json:"tags"`
		} `json:"format"`
	}
	if err := json.Unmarshal(probe, &metadata); err != nil {
		t.Fatalf("decode ffprobe JSON: %v", err)
	}
	codecs := make(map[string]string, len(metadata.Streams))
	for _, stream := range metadata.Streams {
		codecs[stream.CodecType] = stream.CodecName
	}
	if codecs["audio"] != "flac" {
		t.Fatalf("audio codec = %q, want flac; streams=%#v", codecs["audio"], metadata.Streams)
	}
	if codecs["video"] != "mjpeg" {
		t.Fatalf("cover codec = %q, want mjpeg; streams=%#v", codecs["video"], metadata.Streams)
	}
	for key, want := range map[string]string{
		"title":  "最真的梦 (女声深情版)",
		"artist": "七分可可",
		"album":  "最真的梦 (女声深情版)",
		"lyrics": "[00:01.00]test lyric",
	} {
		if got := strings.TrimSpace(metadata.Format.Tags[key]); got != want {
			t.Fatalf("format tag %s = %q, want %q; tags=%#v", key, got, want, metadata.Format.Tags)
		}
	}
}
