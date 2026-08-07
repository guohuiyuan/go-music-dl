package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/soda"
)

func getSodaCookie() string {
	if cookie := getEnvCookie("SODA_COOKIE"); strings.TrimSpace(cookie) != "" {
		return strings.TrimSpace(cookie)
	}

	raw, err := os.ReadFile(filepath.Join("..", "qishui.md"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "cookie:") {
			return strings.TrimSpace(line[len("cookie:"):])
		}
	}
	return ""
}

func isMP4Audio(data []byte) bool {
	return len(data) >= 12 && string(data[4:8]) == "ftyp"
}

func TestSodaVIPStatusAndDownload(t *testing.T) {
	cookie := getSodaCookie()
	if cookie == "" {
		t.Skip("SODA_COOKIE or ../qishui.md cookie not set")
	}

	s := soda.New(cookie)
	song, err := s.Parse("https://qishui.douyin.com/s/iQeFw9cE/")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if song.ID != "7304719759323564095" {
		t.Fatalf("song ID = %s, want 7304719759323564095", song.ID)
	}
	if !song.IsVIP {
		t.Fatalf("song should be marked VIP, extra=%#v", song.Extra)
	}
	if song.URL == "" {
		t.Fatal("parsed VIP song should include a cookie-backed download URL")
	}
	if song.Duration < 175 || song.Duration > 185 {
		t.Fatalf("song duration = %d, want about 180 seconds", song.Duration)
	}
	if song.Size < 5*1024*1024 {
		t.Fatalf("download size = %d, looks like a preview stream", song.Size)
	}

	info, err := s.GetDownloadInfo(song)
	if err != nil {
		t.Fatalf("GetDownloadInfo error: %v", err)
	}
	t.Logf("selected Soda quality=%q format=%q bitrate=%d size=%d duration=%.1fs", info.Quality, info.Format, info.Bitrate, info.Size, info.Duration)
	if info.URL == "" || info.Size < 5*1024*1024 || info.Duration < 175 {
		t.Fatalf("selected stream looks incomplete: quality=%q size=%d duration=%.1f", info.Quality, info.Size, info.Duration)
	}
	if info.Quality == "" && info.Bitrate == 0 {
		t.Fatal("selected stream should expose quality or bitrate metadata")
	}

	isVip, err := s.IsVipAccount()
	if err != nil {
		t.Fatalf("IsVipAccount error: %v", err)
	}
	if !isVip {
		t.Fatal("Soda account should be detected as VIP for the provided cookie")
	}

	outputPath := filepath.Join(t.TempDir(), "soda-vip.m4a")
	if err := s.Download(song, outputPath); err != nil {
		t.Fatalf("Download error: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Read downloaded file error: %v", err)
	}
	if len(data) < 5*1024*1024 {
		t.Fatalf("downloaded file size = %d, looks incomplete", len(data))
	}
	if !isMP4Audio(data) {
		t.Fatalf("downloaded file does not look like an MP4/M4A audio file")
	}
}

func TestSodaLosslessTrackDownloadIsDecrypted(t *testing.T) {
	cookie := getSodaCookie()
	if cookie == "" {
		t.Skip("SODA_COOKIE or ../qishui.md cookie not set")
	}

	s := soda.New(cookie)
	song, err := s.Parse("https://www.qishui.com/track/7501674235158431760")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if song.ID != "7501674235158431760" {
		t.Fatalf("song ID = %s, want 7501674235158431760", song.ID)
	}

	info, err := s.GetDownloadInfo(song)
	if err != nil {
		t.Fatalf("GetDownloadInfo error: %v", err)
	}
	t.Logf("selected Soda lossless candidate quality=%q format=%q bitrate=%d size=%d duration=%.1fs", info.Quality, info.Format, info.Bitrate, info.Size, info.Duration)
	if strings.ToLower(info.Quality) != "lossless" {
		t.Fatalf("quality = %q, want lossless", info.Quality)
	}
	if info.Size < 40*1024*1024 || info.Duration < 230 {
		t.Fatalf("selected lossless stream looks incomplete: size=%d duration=%.1f", info.Size, info.Duration)
	}

	outputPath := filepath.Join(t.TempDir(), "soda-lossless.mp4")
	if err := s.Download(song, outputPath); err != nil {
		t.Fatalf("Download error: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Read downloaded file error: %v", err)
	}
	if int64(len(data)) != info.Size {
		t.Fatalf("downloaded file size = %d, want %d", len(data), info.Size)
	}
	if !isMP4Audio(data) {
		t.Fatal("downloaded lossless file is not decrypted into a playable MP4/M4A audio file")
	}
	assertFFmpegCanDecode(t, outputPath)
}

func TestSodaReportedLosslessTrackRefreshesCachedURL(t *testing.T) {
	cookie := getSodaCookie()
	if cookie == "" {
		t.Skip("SODA_COOKIE or ../qishui.md cookie not set")
	}

	s := soda.New(cookie)
	info, err := s.GetDownloadInfo(&model.Song{
		ID:       "7501674235158431760",
		Source:   "soda",
		URL:      "https://example.invalid/stale-preview.m4a#auth=stale",
		Ext:      "m4a",
		Size:     1024,
		Duration: 60,
		Bitrate:  128,
		Extra:    map[string]string{"quality": "medium"},
	})
	if err != nil {
		t.Fatalf("GetDownloadInfo error: %v", err)
	}
	t.Logf("refreshed Soda candidate quality=%q format=%q bitrate=%d size=%d duration=%.1fs", info.Quality, info.Format, info.Bitrate, info.Size, info.Duration)
	if strings.Contains(info.URL, "example.invalid") {
		t.Fatal("GetDownloadInfo returned the stale cached URL instead of refreshing track_v2")
	}
	if strings.ToLower(info.Quality) != "lossless" {
		t.Fatalf("quality = %q, want lossless", info.Quality)
	}
	if info.Size < 40*1024*1024 || info.Duration < 230 {
		t.Fatalf("refreshed lossless stream looks incomplete: size=%d duration=%.1f", info.Size, info.Duration)
	}
}

func assertFFmpegCanDecode(t *testing.T, path string) {
	t.Helper()

	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not found; skipping decode-level playback check")
	}
	cmd := exec.Command(ffmpeg, "-v", "error", "-t", "10", "-i", path, "-f", "null", "-")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ffmpeg could not decode downloaded audio: %v\n%s", err, trimTestOutput(output))
	}
	if decodedErrors := trimTestOutput(output); decodedErrors != "" {
		t.Fatalf("ffmpeg reported decode errors:\n%s", decodedErrors)
	}
}

func trimTestOutput(output []byte) string {
	text := strings.TrimSpace(string(output))
	if len(text) <= 4000 {
		return text
	}
	return text[:4000] + "\n... output truncated ..."
}
