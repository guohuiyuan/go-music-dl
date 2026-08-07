package main

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	kugouVIPArtistKeyword = "G.E.M. 邓紫棋"
	kugouVIPSongName      = "唯一"
)

func getEnvCookie(key string) string {
	b, err := os.ReadFile(".env")
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			return strings.TrimSpace(strings.TrimPrefix(line, key+"="))
		}
	}
	return ""
}

func getKugouDownloadCookie() string {
	if cookie := getEnvCookie("KUGOU_APP_COOKIE"); cookie != "" {
		return cookie
	}
	return getEnvCookie("KUGOU_COOKIE")
}

func isStandardFLAC(data []byte) bool {
	return len(data) >= 4 && bytes.Equal(data[:4], []byte{'f', 'L', 'a', 'C'})
}

func saveTempFLAC(prefix string, data []byte) (string, error) {
	f, err := os.CreateTemp("", prefix+"-*.flac")
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return "", err
	}

	return f.Name(), nil
}

func findKugouVIPTestSong(t *testing.T, k *kugou.Kugou) model.Song {
	t.Helper()

	songs, err := k.Search(kugouVIPArtistKeyword + " " + kugouVIPSongName)
	if err != nil {
		t.Skipf("Search error: %v", err)
	}
	if len(songs) == 0 {
		t.Skip("No songs found for keyword")
	}

	for _, song := range songs {
		name := strings.TrimSpace(song.Name)
		artist := strings.TrimSpace(song.Artist)
		if name != kugouVIPSongName {
			continue
		}
		if !strings.Contains(artist, "邓紫棋") && !strings.Contains(artist, "G.E.M.") {
			continue
		}
		return song
	}

	t.Skipf("No exact song match found for %s - %s", kugouVIPArtistKeyword, kugouVIPSongName)
	return model.Song{}
}

func getKugouHashSnapshot(song model.Song) map[string]string {
	snapshot := map[string]string{
		"hash":           song.Extra["hash"],
		"ogg_320_hash":   song.Extra["ogg_320_hash"],
		"ogg_128_hash":   song.Extra["ogg_128_hash"],
		"sq_hash":        song.Extra["sq_hash"],
		"file_hash":      song.Extra["file_hash"],
		"res_hash":       song.Extra["res_hash"],
		"mv_hash":        song.Extra["mv_hash"],
		"hq_hash":        song.Extra["hq_hash"],
		"audio_id":       song.Extra["audio_id"],
		"album_audio_id": song.Extra["album_audio_id"],
		"album_id":       song.Extra["album_id"],
	}

	if song.AlbumID != "" && snapshot["album_id"] == "" {
		snapshot["album_id"] = song.AlbumID
	}
	return snapshot
}

func TestNeteaseVIPStatusAndDownload(t *testing.T) {
	cookie := getEnvCookie("NETEASE_COOKIE")
	n := netease.New(cookie)

	parsedSong, err := n.Parse("https://music.163.com/#/song?id=29732995")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if parsedSong == nil {
		t.Fatalf("Parse returned nil song")
	}
	fmt.Printf("Parsed song %s by %s, ID: %s\n", parsedSong.Name, parsedSong.Artist, parsedSong.ID)

	if parsedSong.URL == "" {
		downloadURL, getErr := n.GetDownloadURL(parsedSong)
		if getErr != nil {
			t.Fatalf("GetDownloadURL error: %v", getErr)
		}
		parsedSong.URL = downloadURL
	}

	isVip, err := n.IsVipAccount()
	if err != nil {
		t.Fatalf("IsVipAccount error: %v", err)
	}
	fmt.Printf("Account IsVip: %v\n", isVip)

	audioData, err := utils.Get(
		parsedSong.URL,
		utils.WithHeader("Referer", netease.Referer),
		utils.WithHeader("Cookie", cookie),
	)
	if err != nil {
		t.Fatalf("Download error: %v", err)
	}
	if len(audioData) == 0 {
		t.Fatalf("Downloaded audio is empty")
	}
	fmt.Printf("Downloaded %d bytes from parsed URL\n", len(audioData))
	fmt.Printf("Raw header check: standard FLAC=%v (first4=%q)\n", isStandardFLAC(audioData), string(audioData[:4]))

	if isStandardFLAC(audioData) {
		flacPath, saveErr := saveTempFLAC("netease-raw", audioData)
		if saveErr != nil {
			t.Fatalf("Save temp FLAC error: %v", saveErr)
		}
		fmt.Printf("Raw FLAC saved to: %s\n", flacPath)
		fmt.Printf("Raw data is already standard FLAC, skip decrypt\n")
		return
	}

	if len(audioData) < 8 || string(audioData[:8]) != "CTENFDAM" {
		t.Logf("Raw data is not standard FLAC and does not match NCM header, skip decrypt")
		return
	}

	decrypted, outExt, err := netease.DecryptNCM(audioData)
	if err != nil {
		t.Fatalf("Decrypt NCM error: %v", err)
	}

	if len(decrypted) == 0 {
		t.Fatalf("Decrypted audio is empty")
	}
	fmt.Printf("Decrypted %d bytes, source=netease, ext=%s\n", len(decrypted), outExt)
	fmt.Printf("Decrypted header check: standard FLAC=%v (first4=%q)\n", isStandardFLAC(decrypted), string(decrypted[:4]))

	flacPath, saveErr := saveTempFLAC("netease-decrypted", decrypted)
	if saveErr != nil {
		t.Fatalf("Save decrypted temp FLAC error: %v", saveErr)
	}
	fmt.Printf("Decrypted audio saved to: %s\n", flacPath)
}

func TestBilibiliVIPStatusAndDownload(t *testing.T) {
	cookie := getEnvCookie("BILIBILI_COOKIE")
	b := bilibili.New(cookie)

	vipKeyword := "Jay Chou"
	songs, err := b.Search(vipKeyword)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	var vipSongID string
	if len(songs) > 0 {
		vipSongID = songs[0].ID
		fmt.Printf("Found Bilibili video %s by %s, ID: %s\n", songs[0].Name, songs[0].Artist, vipSongID)
	} else {
		t.Fatalf("No songs found for keyword: %s", vipKeyword)
	}

	isVip, err := b.IsVipAccount()
	if err != nil {
		t.Fatalf("IsVipAccount error: %v", err)
	}
	fmt.Printf("Bilibili Account IsVip: %v\n", isVip)

	song := &model.Song{
		ID:     vipSongID,
		Source: "bilibili",
	}

	url, err := b.GetDownloadURL(song)
	if err != nil {
		t.Logf("GetDownloadURL error: %v", err)
		return
	}

	fmt.Printf("Bilibili Song %s Download URL: %s\n", vipSongID, url[:120]+"...")
}

func TestBilibiliVIPHiRes(t *testing.T) {
	cookie := getEnvCookie("BILIBILI_COOKIE")
	b := bilibili.New(cookie)

	isVip, err := b.IsVipAccount()
	if err != nil {
		t.Fatalf("IsVipAccount error: %v", err)
	}
	fmt.Printf("Bilibili Account IsVip: %v\n", isVip)

	song := &model.Song{
		ID:     "BV1rp4y1e745|244954665",
		Source: "bilibili",
	}

	url, err := b.GetDownloadURL(song)
	if err != nil {
		t.Fatalf("Failed to fetch Hi-Res Bilibili stream: %v", err)
	}

	fmt.Printf("Bilibili Hi-Res Stream Download URL: %s\n", url[:120]+"...")
}

func TestQQVIPStatusAndDownload(t *testing.T) {
	cookie := getEnvCookie("QQ_COOKIE")
	q := qq.New(cookie)

	isVip, err := q.IsVipAccount()
	if err != nil {
		t.Fatalf("IsVipAccount error: %v", err)
	}
	fmt.Printf("QQ Account IsVip: %v\n", isVip)

	song := &model.Song{
		ID:     "0039MnYb0qxYhV",
		Source: "qq",
	}

	url, err := q.GetDownloadURL(song)
	if err != nil {
		t.Fatalf("Failed to fetch QQ stream: %v", err)
	}

	fmt.Printf("QQ Stream Download URL: %s\n", url[:120]+"...")
}

func TestKugouVIPSearchHashes(t *testing.T) {
	cookie := getEnvCookie("KUGOU_COOKIE")
	if cookie == "" {
		t.Skip("KUGOU_COOKIE not set")
	}
	k := kugou.New(cookie)

	song := findKugouVIPTestSong(t, k)
	hashes := getKugouHashSnapshot(song)
	fmt.Printf("Found Kugou song %s by %s, ID: %s\n", song.Name, song.Artist, song.ID)

	order := []string{"hash", "ogg_320_hash", "ogg_128_hash", "sq_hash", "file_hash", "res_hash", "mv_hash", "hq_hash", "audio_id", "album_id"}
	for _, key := range order {
		fmt.Printf("%s=%s\n", key, hashes[key])
	}

	for _, key := range []string{"hash", "sq_hash", "file_hash", "res_hash", "hq_hash", "audio_id", "album_id"} {
		if strings.TrimSpace(hashes[key]) == "" {
			t.Fatalf("expected %s in extra, got empty", key)
		}
	}
}

func TestKugouVIPStatusAndDownload(t *testing.T) {
	cookie := getKugouDownloadCookie()
	if cookie == "" {
		t.Skip("KUGOU_APP_COOKIE or KUGOU_COOKIE not set")
	}
	k := kugou.New(cookie)

	song := model.Song{
		ID:      "83368470292244265486BF864701222C",
		Name:    "唯一",
		Artist:  "G.E.M. 邓紫棋",
		Album:   "T.I.M.E.",
		AlbumID: "82564821",
		Source:  "kugou",
		Extra: map[string]string{
			"hash":           "83368470292244265486BF864701222C",
			"ogg_320_hash":   "878D0D6E847F402299D29D4295BC21CB",
			"ogg_128_hash":   "A76E5E3C04E98737DBDD0C8C550EDAE5",
			"sq_hash":        "83368470292244265486BF864701222C",
			"file_hash":      "AB05B8F658851282DCB2CBAD548AEB9B",
			"res_hash":       "983AA9AF17E1688CC70EDEAAF6256D5B",
			"mv_hash":        "4CDC3359A8D45114153D68F26AEF0A9A",
			"hq_hash":        "7DD13522B6C143A39E10507B74B0A876",
			"audio_id":       "281733352",
			"album_audio_id": "590964610",
			"album_id":       "82564821",
			"privilege":      "10",
		},
	}
	hashes := getKugouHashSnapshot(song)
	fmt.Printf("Found Kugou song %s by %s, ID: %s\n", song.Name, song.Artist, song.ID)

	candidates := []string{
		hashes["hash"],
		hashes["sq_hash"],
		hashes["hq_hash"],
		hashes["res_hash"],
		hashes["ogg_320_hash"],
		hashes["file_hash"],
		hashes["ogg_128_hash"],
	}
	fmt.Printf("Download candidate hashes: %v\n", candidates)

	url, err := k.GetDownloadURL(&song)
	if err != nil {
		t.Fatalf("GetDownloadURL error: %v", err)
	}

	isVip, err := k.IsVipAccount()
	if err != nil {
		t.Fatalf("IsVipAccount error: %v", err)
	}
	fmt.Printf("Kugou Account IsVip: %v\n", isVip)
	fmt.Printf("Kugou Download URL: %s\n", url[:min(120, len(url))])

	audioData, err := utils.Get(
		url,
		utils.WithHeader("Referer", "https://www.kugou.com/"),
		utils.WithHeader("Cookie", cookie),
	)
	if err != nil {
		t.Fatalf("Download error: %v", err)
	}
	if len(audioData) == 0 {
		t.Fatalf("Downloaded audio is empty")
	}

	ext := song.Ext
	if ext == "" {
		if strings.Contains(strings.ToLower(url), ".flac") {
			ext = "flac"
		} else {
			ext = "mp3"
		}
	}
	fmt.Printf("Downloaded %d bytes, ext=%s, standard FLAC=%v\n", len(audioData), ext, isStandardFLAC(audioData))

	lowerID := strings.ToLower(song.ID)
	lowerCandidates := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		lowerCandidates = append(lowerCandidates, strings.ToLower(candidate))
	}
	if !slices.Contains(lowerCandidates, lowerID) {
		t.Fatalf("song ID %s not found in candidate hashes %v", song.ID, candidates)
	}
}
