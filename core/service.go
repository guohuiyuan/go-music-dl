package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/dhowden/tag"
	"github.com/guohuiyuan/music-lib/apple"
	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/fivesing"
	"github.com/guohuiyuan/music-lib/jamendo"
	"github.com/guohuiyuan/music-lib/joox"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/kuwo"
	"github.com/guohuiyuan/music-lib/migu"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qianqian"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/soda"
	"gorm.io/gorm"
)

var ErrFFmpegNotFound = errors.New("ffmpeg not found")

const (
	UA_Common    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
	UA_Mobile    = "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13B143 Safari/601.1"
	Ref_Netease  = "http://music.163.com/"
	Ref_Bilibili = "https://www.bilibili.com/"
	Ref_Migu     = "http://music.migu.cn/"
	CookieFile   = "data/cookies.json"
)

// ==========================================
// Cookie 管理系统
// ==========================================

type CookieManager struct {
	mu      sync.RWMutex
	cookies map[string]string
}

var CM = &CookieManager{cookies: make(map[string]string)}

func (m *CookieManager) Load() {
	if err := ensureConfigDB(); err != nil {
		return
	}

	var rows []cookieEntry
	if err := configDB.Order("source ASC").Find(&rows).Error; err != nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.cookies = make(map[string]string, len(rows))
	for _, row := range rows {
		m.cookies[row.Source] = row.Value
	}
}

func (m *CookieManager) Save() {
	if err := ensureConfigDB(); err != nil {
		return
	}

	m.mu.RLock()
	rows := make([]cookieEntry, 0, len(m.cookies))
	for source, value := range m.cookies {
		source = strings.TrimSpace(source)
		value = strings.TrimSpace(value)
		if source == "" || value == "" {
			continue
		}
		rows = append(rows, cookieEntry{Source: source, Value: value})
	}
	m.mu.RUnlock()

	_ = configDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&cookieEntry{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
	// 🌟 确保写入前目录存在
	os.MkdirAll("data", 0755)
}

func (m *CookieManager) Get(source string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cookies[source]
}

func (m *CookieManager) SetAll(c map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range c {
		if v == "" {
			delete(m.cookies, k)
		} else {
			m.cookies[k] = v
		}
	}
}

func (m *CookieManager) GetAll() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make(map[string]string)
	for k, v := range m.cookies {
		res[k] = v
	}
	return res
}

// ==========================================
// 工厂函数映射
// ==========================================

type SearchFunc func(keyword string) ([]model.Song, error)
type SearchPlaylistFunc func(keyword string) ([]model.Playlist, error)
type PlaylistCategoriesFunc func() ([]model.PlaylistCategory, error)
type CategoryPlaylistsFunc func(string, int, int) ([]model.Playlist, error)
type QRLoginCreateFunc func() (*model.QRLoginSession, error)
type QRLoginCheckFunc func(string) (*model.QRLoginResult, error)
type UserPlaylistsFunc func(page, limit int) ([]model.Playlist, error)

func GetSearchFunc(source string) SearchFunc {
	c := CM.Get(source)
	switch source {
	case "netease":
		return netease.New(c).Search
	case "qq":
		return qq.New(c).Search
	case "kugou":
		return kugou.New(c).Search
	case "kuwo":
		return kuwo.New(c).Search
	case "migu":
		return migu.New(c).Search
	case "bilibili":
		return bilibili.New(c).Search
	case "fivesing":
		return fivesing.New(c).Search
	case "jamendo":
		return jamendo.New(c).Search
	case "joox":
		return joox.New(c).Search
	case "qianqian":
		return qianqian.New(c).Search
	case "soda":
		return soda.New(c).Search
	case "apple":
		return apple.New(c).Search
	default:
		return nil
	}
}

func GetAlbumSearchFunc(source string) SearchPlaylistFunc {
	c := CM.Get(source)
	switch source {
	case "netease":
		return netease.New(c).SearchAlbum
	case "qq":
		return qq.New(c).SearchAlbum
	case "kugou":
		return kugou.New(c).SearchAlbum
	case "kuwo":
		return kuwo.New(c).SearchAlbum
	case "migu":
		return migu.New(c).SearchAlbum
	case "jamendo":
		return jamendo.New(c).SearchAlbum
	case "joox":
		return joox.New(c).SearchAlbum
	case "qianqian":
		return qianqian.New(c).SearchAlbum
	case "soda":
		return soda.New(c).SearchAlbum
	case "apple":
		return apple.New(c).SearchAlbum
	default:
		return nil
	}
}

func GetPlaylistSearchFunc(source string) SearchPlaylistFunc {
	c := CM.Get(source)
	switch source {
	case "netease":
		return netease.New(c).SearchPlaylist
	case "qq":
		return qq.New(c).SearchPlaylist
	case "kugou":
		return kugou.New(c).SearchPlaylist
	case "kuwo":
		return kuwo.New(c).SearchPlaylist
	case "migu":
		return migu.New(c).SearchPlaylist
	case "jamendo":
		return jamendo.New(c).SearchPlaylist
	case "joox":
		return joox.New(c).SearchPlaylist
	case "qianqian":
		return qianqian.New(c).SearchPlaylist
	case "bilibili":
		return bilibili.New(c).SearchPlaylist
	case "soda":
		return soda.New(c).SearchPlaylist
	case "fivesing":
		return fivesing.New(c).SearchPlaylist
	case "apple":
		return apple.New(c).SearchPlaylist
	default:
		return nil
	}
}

func GetAlbumDetailFunc(source string) func(string) ([]model.Song, error) {
	c := CM.Get(source)
	switch source {
	case "netease":
		return netease.New(c).GetAlbumSongs
	case "qq":
		return qq.New(c).GetAlbumSongs
	case "kugou":
		return kugou.New(c).GetAlbumSongs
	case "kuwo":
		return kuwo.New(c).GetAlbumSongs
	case "migu":
		return migu.New(c).GetAlbumSongs
	case "jamendo":
		return jamendo.New(c).GetAlbumSongs
	case "joox":
		return joox.New(c).GetAlbumSongs
	case "qianqian":
		return qianqian.New(c).GetAlbumSongs
	case "soda":
		return soda.New(c).GetAlbumSongs
	case "apple":
		return apple.New(c).GetAlbumSongs
	default:
		return nil
	}
}

func GetPlaylistDetailFunc(source string) func(string) ([]model.Song, error) {
	c := CM.Get(source)
	switch source {
	case "netease":
		return netease.New(c).GetPlaylistSongs
	case "qq":
		return qq.New(c).GetPlaylistSongs
	case "kugou":
		return kugou.New(c).GetPlaylistSongs
	case "kuwo":
		return kuwo.New(c).GetPlaylistSongs
	case "migu":
		return migu.New(c).GetPlaylistSongs
	case "jamendo":
		return jamendo.New(c).GetPlaylistSongs
	case "joox":
		return joox.New(c).GetPlaylistSongs
	case "qianqian":
		return qianqian.New(c).GetPlaylistSongs
	case "bilibili":
		return bilibili.New(c).GetPlaylistSongs
	case "soda":
		return soda.New(c).GetPlaylistSongs
	case "fivesing":
		return fivesing.New(c).GetPlaylistSongs
	case "apple":
		return apple.New(c).GetPlaylistSongs
	default:
		return nil
	}
}

func GetRecommendFunc(source string) func() ([]model.Playlist, error) {
	c := CM.Get(source)
	switch source {
	case "netease":
		return netease.New(c).GetRecommendedPlaylists
	case "qq":
		return qq.New(c).GetRecommendedPlaylists
	case "kugou":
		return kugou.New(c).GetRecommendedPlaylists
	case "kuwo":
		return kuwo.New(c).GetRecommendedPlaylists
	default:
		return nil
	}
}

func GetPlaylistCategoriesFunc(source string) PlaylistCategoriesFunc {
	c := CM.Get(source)
	switch source {
	case "netease":
		return netease.New(c).GetPlaylistCategories
	case "qq":
		return qq.New(c).GetPlaylistCategories
	case "kugou":
		return kugou.New(c).GetPlaylistCategories
	case "kuwo":
		return kuwo.New(c).GetPlaylistCategories
	case "migu":
		return migu.New(c).GetPlaylistCategories
	case "joox":
		return joox.New(c).GetPlaylistCategories
	case "qianqian":
		return qianqian.New(c).GetPlaylistCategories
	case "apple":
		return apple.New(c).GetPlaylistCategories
	default:
		return nil
	}
}

func GetCategoryPlaylistsFunc(source string) CategoryPlaylistsFunc {
	c := CM.Get(source)
	switch source {
	case "netease":
		return netease.New(c).GetCategoryPlaylists
	case "qq":
		return qq.New(c).GetCategoryPlaylists
	case "kugou":
		return kugou.New(c).GetCategoryPlaylists
	case "kuwo":
		return kuwo.New(c).GetCategoryPlaylists
	case "migu":
		return migu.New(c).GetCategoryPlaylists
	case "joox":
		return joox.New(c).GetCategoryPlaylists
	case "qianqian":
		return qianqian.New(c).GetCategoryPlaylists
	case "apple":
		return apple.New(c).GetCategoryPlaylists
	default:
		return nil
	}
}

func GetQRLoginCreateFunc(source string) QRLoginCreateFunc {
	switch source {
	case "netease":
		return netease.CreateQRLogin
	case "qq":
		return qq.CreateQRLogin
	case "qq_wx":
		return qq.CreateWXQRLogin
	case "kugou":
		return kugou.CreateQRLogin
	case "bilibili":
		return bilibili.CreateQRLogin
	case "soda":
		return soda.CreateQRLogin
	default:
		return nil
	}
}

func GetQRLoginCheckFunc(source string) QRLoginCheckFunc {
	switch source {
	case "netease":
		return netease.CheckQRLogin
	case "qq":
		return qq.CheckQRLogin
	case "qq_wx":
		return qq.CheckWXQRLogin
	case "kugou":
		return kugou.CheckQRLogin
	case "bilibili":
		return bilibili.CheckQRLogin
	case "soda":
		return soda.CheckQRLogin
	default:
		return nil
	}
}

func GetQRLoginSourceNames() []string {
	return []string{"netease", "qq", "qq_wx", "kugou", "bilibili", "soda"}
}

func GetUserPlaylistsFunc(source string) UserPlaylistsFunc {
	c := CM.Get(source)
	switch source {
	case "netease":
		return netease.New(c).GetUserPlaylists
	case "qq":
		return qq.New(c).GetUserPlaylists
	case "kugou":
		return kugou.New(c).GetUserPlaylists
	case "soda":
		return soda.New(c).GetUserPlaylists
	default:
		return nil
	}
}

func GetUserPlaylistSourceNames() []string {
	return []string{"netease", "qq", "kugou", "soda"}
}

func GetRecommendSourceNames() []string {
	return []string{"netease", "qq", "kugou", "kuwo"}
}

func GetDownloadFunc(source string) func(*model.Song) (string, error) {
	c := CM.Get(source)
	switch source {
	case "netease":
		return netease.New(c).GetDownloadURL
	case "qq":
		return qq.New(c).GetDownloadURL
	case "kugou":
		return kugou.New(c).GetDownloadURL
	case "kuwo":
		return kuwo.New(c).GetDownloadURL
	case "migu":
		return migu.New(c).GetDownloadURL
	case "soda":
		return soda.New(c).GetDownloadURL
	case "bilibili":
		return bilibili.New(c).GetDownloadURL
	case "fivesing":
		return fivesing.New(c).GetDownloadURL
	case "jamendo":
		return jamendo.New(c).GetDownloadURL
	case "joox":
		return joox.New(c).GetDownloadURL
	case "qianqian":
		return qianqian.New(c).GetDownloadURL
	case "apple":
		return apple.New(c).GetDownloadURL
	default:
		return nil
	}
}

func GetLyricFunc(source string) func(*model.Song) (string, error) {
	c := CM.Get(source)
	switch source {
	case "netease":
		return netease.New(c).GetLyrics
	case "qq":
		return qq.New(c).GetLyrics
	case "kugou":
		return kugou.New(c).GetLyrics
	case "kuwo":
		return kuwo.New(c).GetLyrics
	case "migu":
		return migu.New(c).GetLyrics
	case "soda":
		return soda.New(c).GetLyrics
	case "bilibili":
		return bilibili.New(c).GetLyrics
	case "fivesing":
		return fivesing.New(c).GetLyrics
	case "jamendo":
		return jamendo.New(c).GetLyrics
	case "joox":
		return joox.New(c).GetLyrics
	case "qianqian":
		return qianqian.New(c).GetLyrics
	case "apple":
		return apple.New(c).GetLyrics
	default:
		return nil
	}
}

func GetParseFunc(source string) func(string) (*model.Song, error) {
	c := CM.Get(source)
	switch source {
	case "netease":
		return netease.New(c).Parse
	case "qq":
		return qq.New(c).Parse
	case "kugou":
		return kugou.New(c).Parse
	case "kuwo":
		return kuwo.New(c).Parse
	case "migu":
		return migu.New(c).Parse
	case "soda":
		return soda.New(c).Parse
	case "bilibili":
		return bilibili.New(c).Parse
	case "fivesing":
		return fivesing.New(c).Parse
	case "jamendo":
		return jamendo.New(c).Parse
	case "joox":
		return joox.New(c).Parse
	case "qianqian":
		return qianqian.New(c).Parse
	case "apple":
		return apple.New(c).Parse
	default:
		return nil
	}
}

func GetParsePlaylistFunc(source string) func(string) (*model.Playlist, []model.Song, error) {
	c := CM.Get(source)
	switch source {
	case "netease":
		return netease.New(c).ParsePlaylist
	case "qq":
		return qq.New(c).ParsePlaylist
	case "kugou":
		return kugou.New(c).ParsePlaylist
	case "kuwo":
		return kuwo.New(c).ParsePlaylist
	case "migu":
		return migu.New(c).ParsePlaylist
	case "jamendo":
		return jamendo.New(c).ParsePlaylist
	case "joox":
		return joox.New(c).ParsePlaylist
	case "qianqian":
		return qianqian.New(c).ParsePlaylist
	case "bilibili":
		return bilibili.New(c).ParsePlaylist
	case "soda":
		return soda.New(c).ParsePlaylist
	case "fivesing":
		return fivesing.New(c).ParsePlaylist
	case "apple":
		return apple.New(c).ParsePlaylist
	default:
		return nil
	}
}

func GetParseAlbumFunc(source string) func(string) (*model.Playlist, []model.Song, error) {
	c := CM.Get(source)
	switch source {
	case "netease":
		return netease.New(c).ParseAlbum
	case "qq":
		return qq.New(c).ParseAlbum
	case "kugou":
		return kugou.New(c).ParseAlbum
	case "kuwo":
		return kuwo.New(c).ParseAlbum
	case "migu":
		return migu.New(c).ParseAlbum
	case "jamendo":
		return jamendo.New(c).ParseAlbum
	case "joox":
		return joox.New(c).ParseAlbum
	case "qianqian":
		return qianqian.New(c).ParseAlbum
	case "soda":
		return soda.New(c).ParseAlbum
	case "apple":
		return apple.New(c).ParseAlbum
	default:
		return nil
	}
}

// ==========================================
// 辅助与解析方法
// ==========================================

func DetectSource(link string) string {
	if strings.Contains(link, "163.com") {
		return "netease"
	}
	if strings.Contains(link, "qq.com") {
		return "qq"
	}
	if strings.Contains(link, "5sing") {
		return "fivesing"
	}
	if strings.Contains(link, "kugou.com") {
		return "kugou"
	}
	if strings.Contains(link, "kuwo.cn") {
		return "kuwo"
	}
	if strings.Contains(link, "migu.cn") {
		return "migu"
	}
	if strings.Contains(link, "joox.com") {
		return "joox"
	}
	if strings.Contains(link, "bilibili.com") || strings.Contains(link, "b23.tv") {
		return "bilibili"
	}
	if strings.Contains(link, "douyin.com") || strings.Contains(link, "qishui") {
		return "soda"
	}
	if strings.Contains(link, "91q.com") {
		return "qianqian"
	}
	if strings.Contains(link, "jamendo.com") {
		return "jamendo"
	}
	if strings.Contains(link, "music.apple.com") || strings.Contains(link, "itunes.apple.com") {
		return "apple"
	}
	return ""
}

func GetOriginalLink(source, id, typeStr string) string {
	switch source {
	case "netease":
		if typeStr == "album" {
			return "https://music.163.com/#/album?id=" + id
		}
		if typeStr == "playlist" {
			return "https://music.163.com/#/playlist?id=" + id
		}
		return "https://music.163.com/#/song?id=" + id
	case "qq":
		if typeStr == "album" {
			return "https://y.qq.com/n/ryqq/albumDetail/" + id
		}
		if strings.HasPrefix(id, "profile:") {
			return "https://y.qq.com/n/ryqq/profile"
		}
		if typeStr == "playlist" {
			return "https://y.qq.com/n/ryqq/playlist/" + id
		}
		return "https://y.qq.com/n/ryqq/songDetail/" + id
	case "kugou":
		if typeStr == "album" {
			return "https://www.kugou.com/album/" + id + ".html"
		}
		if typeStr == "playlist" {
			return "https://www.kugou.com/yy/special/single/" + id + ".html"
		}
		return "https://www.kugou.com/song/#hash=" + id
	case "kuwo":
		if typeStr == "album" {
			return "http://www.kuwo.cn/album_detail/" + id
		}
		if typeStr == "playlist" {
			return "http://www.kuwo.cn/playlist_detail/" + id
		}
		return "http://www.kuwo.cn/play_detail/" + id
	case "migu":
		if typeStr == "album" {
			return "https://music.migu.cn/v3/music/album/" + id
		}
		if typeStr == "playlist" {
			return "https://music.migu.cn/v5/#/playlist?playlistId=" + id + "&playlistType=ordinary"
		}
		if typeStr == "song" {
			return "https://music.migu.cn/v3/music/song/" + id
		}
	case "jamendo":
		if typeStr == "album" {
			return "https://www.jamendo.com/album/" + id
		}
		if typeStr == "playlist" {
			return "https://www.jamendo.com/playlist/" + id
		}
		if typeStr == "song" {
			return "https://www.jamendo.com/track/" + id
		}
	case "joox":
		if typeStr == "album" {
			return "https://www.joox.com/hk/album/" + id
		}
		if typeStr == "playlist" {
			return "https://www.joox.com/hk/playlist/" + id
		}
		if typeStr == "song" {
			return "https://www.joox.com/hk/single/" + id
		}
	case "qianqian":
		if typeStr == "album" {
			return "https://music.91q.com/album/" + id
		}
		if typeStr == "playlist" {
			return "https://music.91q.com/songlist/" + id
		}
		if typeStr == "song" {
			return "https://music.91q.com/song/" + id
		}
	case "soda":
		if typeStr == "album" {
			return "https://www.qishui.com/share/album?album_id=" + id
		}
		if typeStr == "playlist" {
			return "https://www.qishui.com/playlist/" + id
		}
	case "bilibili":
		return "https://www.bilibili.com/video/" + id
	case "apple":
		if typeStr == "album" {
			return "https://music.apple.com/album/" + id
		}
		if typeStr == "playlist" {
			return "https://music.apple.com/playlist/" + id
		}
		return "https://music.apple.com/song/" + id
	case "fivesing":
		if typeStr == "playlist" {
			return "http://5sing.kugou.com/dj/" + id + ".html"
		}
		if strings.Contains(id, "/") {
			return "http://5sing.kugou.com/" + id + ".html"
		}
	}
	return ""
}

func BuildSourceRequest(method, urlStr, source, rangeHeader string) (*http.Request, error) {
	req, err := http.NewRequest(method, urlStr, nil)
	if err != nil {
		return nil, err
	}
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	req.Header.Set("User-Agent", UA_Common)
	if source == "bilibili" {
		req.Header.Set("Referer", Ref_Bilibili)
	}
	if source == "netease" {
		req.Header.Set("Referer", Ref_Netease)
	}
	if source == "migu" {
		req.Header.Set("User-Agent", UA_Mobile)
		req.Header.Set("Referer", Ref_Migu)
	}
	if source == "qq" {
		req.Header.Set("Referer", "http://y.qq.com")
	}
	if cookie := CM.Get(source); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	return req, nil
}

func ValidatePlayable(song *model.Song) bool {
	if song == nil || song.ID == "" || song.Source == "" {
		return false
	}
	if song.Source == "soda" || song.Source == "fivesing" {
		return false
	}
	fn := GetDownloadFunc(song.Source)
	if fn == nil {
		return false
	}
	urlStr, err := fn(&model.Song{ID: song.ID, Source: song.Source})
	if err != nil || urlStr == "" {
		return false
	}

	req, err := BuildSourceRequest("GET", urlStr, song.Source, "bytes=0-1")
	if err != nil {
		return false
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200 || resp.StatusCode == 206
}

// ==========================================
// 算法与通用工具
// ==========================================

func FormatSize(s int64) string {
	if s <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f MB", float64(s)/1024/1024)
}

func DetectAudioExt(data []byte) string {
	if ext := DetectAudioExtBySignature(data); ext != "" {
		return ext
	}
	return "mp3"
}

func DetectAudioExtBySignature(data []byte) string {
	if len(data) >= 16 && bytes.Equal(data[:16], []byte{0x30, 0x26, 0xB2, 0x75, 0x8E, 0x66, 0xCF, 0x11, 0xA6, 0xD9, 0x00, 0xAA, 0x00, 0x62, 0xCE, 0x6C}) {
		return "wma"
	}
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{'f', 'L', 'a', 'C'}) {
		return "flac"
	}
	if len(data) >= 3 && bytes.Equal(data[:3], []byte{'I', 'D', '3'}) {
		return "mp3"
	}
	if len(data) >= 2 && data[0] == 0xFF && (data[1]&0xE0) == 0xE0 {
		return "mp3"
	}
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{'O', 'g', 'g', 'S'}) {
		return "ogg"
	}
	if len(data) >= 12 && bytes.Equal(data[4:8], []byte{'f', 't', 'y', 'p'}) {
		return "m4a"
	}
	return ""
}

func DetectAudioExtByContentType(contentType string) string {
	contentType = strings.TrimSpace(strings.ToLower(contentType))
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	switch contentType {
	case "audio/flac", "audio/x-flac":
		return "flac"
	case "audio/x-ms-wma", "audio/wma", "video/x-ms-asf", "application/vnd.ms-asf":
		return "wma"
	case "audio/mpeg", "audio/mp3", "audio/x-mp3":
		return "mp3"
	case "audio/ogg", "application/ogg":
		return "ogg"
	case "audio/mp4", "audio/x-m4a", "audio/aac", "audio/aacp":
		return "m4a"
	default:
		return ""
	}
}

func AudioMimeByExt(ext string) string {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "wma":
		return "audio/x-ms-wma"
	case "flac":
		return "audio/flac"
	case "ogg":
		return "audio/ogg"
	case "m4a":
		return "audio/mp4"
	default:
		return "audio/mpeg"
	}
}

func IsDurationClose(a, b int) bool {
	if a <= 0 || b <= 0 {
		return true
	}
	diff := IntAbs(a - b)
	if diff <= 10 {
		return true
	}
	maxAllowed := int(float64(a) * 0.15)
	if maxAllowed < 10 {
		maxAllowed = 10
	}
	return diff <= maxAllowed
}

func IntAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func CalcSongSimilarity(name, artist, candName, candArtist string) float64 {
	nameA := NormalizeText(name)
	nameB := NormalizeText(candName)
	if nameA == "" || nameB == "" {
		return 0
	}
	nameSim := SimilarityScore(nameA, nameB)

	artistA := NormalizeText(artist)
	artistB := NormalizeText(candArtist)
	if artistA == "" || artistB == "" {
		return nameSim
	}

	artistSim := SimilarityScore(artistA, artistB)
	return nameSim*0.7 + artistSim*0.3
}

func NormalizeText(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.In(r, unicode.Han) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func SimilarityScore(a, b string) float64 {
	if a == b {
		return 1
	}
	if a == "" || b == "" {
		return 0
	}
	la := len([]rune(a))
	lb := len([]rune(b))
	maxLen := la
	if lb > maxLen {
		maxLen = lb
	}
	if maxLen == 0 {
		return 0
	}
	dist := LevenshteinDistance(a, b)
	if dist >= maxLen {
		return 0
	}
	return 1 - float64(dist)/float64(maxLen)
}

func LevenshteinDistance(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	la := len(ra)
	lb := len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 0
			if ra[i-1] != rb[j-1] {
				cost = 1
			}
			del := prev[j] + 1
			ins := cur[j-1] + 1
			sub := prev[j-1] + cost
			cur[j] = del
			if ins < cur[j] {
				cur[j] = ins
			}
			if sub < cur[j] {
				cur[j] = sub
			}
		}
		prev, cur = cur, prev
	}
	return prev[lb]
}

func OpenBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd, args = "cmd", []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default:
		cmd = "xdg-open"
	}
	args = append(args, url)
	_ = exec.Command(cmd, args...).Start()
}

func GetAllSourceNames() []string {
	return []string{"netease", "qq", "kugou", "kuwo", "migu", "fivesing", "jamendo", "joox", "qianqian", "soda", "bilibili", "apple"}
}

func GetPlaylistSourceNames() []string {
	return []string{"netease", "qq", "kugou", "kuwo", "migu", "jamendo", "joox", "qianqian", "bilibili", "soda", "fivesing", "apple"}
}

func GetAlbumSourceNames() []string {
	return []string{"netease", "qq", "kugou", "kuwo", "migu", "jamendo", "joox", "qianqian", "soda", "apple"}
}

func GetPlaylistCategorySourceNames() []string {
	return []string{"netease", "qq", "kugou", "kuwo", "migu", "qianqian", "joox", "apple"}
}

func GetDefaultSourceNames() []string {
	allSources := GetAllSourceNames()
	var defaultSources []string
	excluded := map[string]bool{"bilibili": true, "joox": true, "jamendo": true, "fivesing": true}
	for _, source := range allSources {
		if !excluded[source] {
			defaultSources = append(defaultSources, source)
		}
	}
	return defaultSources
}

func GetSourceDescription(source string) string {
	descriptions := map[string]string{
		"netease":  "网易云音乐",
		"qq":       "QQ音乐",
		"kugou":    "酷狗音乐",
		"kuwo":     "酷我音乐",
		"migu":     "咪咕音乐",
		"fivesing": "5sing",
		"jamendo":  "Jamendo (CC)",
		"joox":     "JOOX",
		"qianqian": "千千音乐",
		"soda":     "汽水音乐",
		"bilibili": "Bilibili",
		"apple":    "Apple Music",
	}
	if desc, exists := descriptions[source]; exists {
		return desc
	}
	return "未知音乐源"
}

// ==========================================
// ID3v2 元数据内嵌（支持 Web & CLI 下载）
// ==========================================

func stripID3v2Prefix(audioData []byte) []byte {
	if len(audioData) < 10 || string(audioData[:3]) != "ID3" {
		return audioData
	}
	tagSize, ok := decodeID3SynchsafeSize(audioData[6:10])
	if !ok {
		return audioData
	}
	total := 10 + tagSize
	if audioData[5]&0x10 != 0 {
		total += 10
	}
	if total <= 0 || total > len(audioData) {
		return audioData
	}
	return audioData[total:]
}

func decodeID3SynchsafeSize(data []byte) (int, bool) {
	if len(data) < 4 {
		return 0, false
	}
	if data[0]&0x80 != 0 || data[1]&0x80 != 0 || data[2]&0x80 != 0 || data[3]&0x80 != 0 {
		return 0, false
	}
	return int(data[0])<<21 | int(data[1])<<14 | int(data[2])<<7 | int(data[3]), true
}

func id3SynchsafeSize(size int) [4]byte {
	return [4]byte{
		byte((size >> 21) & 0x7F),
		byte((size >> 14) & 0x7F),
		byte((size >> 7) & 0x7F),
		byte(size & 0x7F),
	}
}

func id3UTF16LEText(value string) []byte {
	units := utf16.Encode([]rune(value))
	data := make([]byte, 0, 2+len(units)*2)
	data = append(data, 0xFF, 0xFE)
	for _, unit := range units {
		data = binary.LittleEndian.AppendUint16(data, unit)
	}
	return data
}

func id3TextFramePayload(value string) []byte {
	payload := []byte{0x01}
	payload = append(payload, id3UTF16LEText(value)...)
	return payload
}

func id3USLTPayload(lyric string) []byte {
	payload := []byte{0x01, 'e', 'n', 'g'}
	payload = append(payload, id3UTF16LEText("")...)
	payload = append(payload, 0x00, 0x00)
	payload = append(payload, id3UTF16LEText(lyric)...)
	return payload
}

func id3APICPayload(coverData []byte, coverMime string) []byte {
	mimeType := normalizeCoverMime(coverMime)
	payload := []byte{0x00}
	payload = append(payload, []byte(mimeType)...)
	payload = append(payload, 0x00, 0x03, 0x00)
	payload = append(payload, coverData...)
	return payload
}

func id3v23Frame(id string, payload []byte) []byte {
	if id == "" || len(payload) == 0 {
		return nil
	}
	frame := make([]byte, 0, 10+len(payload))
	frame = append(frame, []byte(id)...)
	frame = binary.BigEndian.AppendUint32(frame, uint32(len(payload)))
	frame = append(frame, 0x00, 0x00)
	frame = append(frame, payload...)
	return frame
}

func isID3v23FrameID(id []byte) bool {
	if len(id) != 4 {
		return false
	}
	for _, c := range id {
		if (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			continue
		}
		return false
	}
	return true
}

func preservedID3v23Frames(audioData []byte, replace map[string]bool) []byte {
	if len(audioData) < 10 || string(audioData[:3]) != "ID3" || audioData[3] != 0x03 {
		return nil
	}
	if audioData[5]&0x40 != 0 {
		return nil
	}
	tagSize, ok := decodeID3SynchsafeSize(audioData[6:10])
	if !ok {
		return nil
	}
	tagEnd := 10 + tagSize
	if tagEnd > len(audioData) {
		return nil
	}

	tagData := audioData[10:tagEnd]
	preserved := make([]byte, 0, len(tagData))
	for pos := 0; pos+10 <= len(tagData); {
		header := tagData[pos : pos+10]
		if bytes.Equal(header, make([]byte, 10)) {
			break
		}
		if !isID3v23FrameID(header[:4]) {
			break
		}
		frameSize := int(binary.BigEndian.Uint32(header[4:8]))
		if frameSize <= 0 || pos+10+frameSize > len(tagData) {
			break
		}
		frameID := string(header[:4])
		if !replace[frameID] {
			preserved = append(preserved, tagData[pos:pos+10+frameSize]...)
		}
		pos += 10 + frameSize
	}
	return preserved
}

func embedMP3ID3v23Metadata(audioData []byte, title, artist, album, lyric string, coverData []byte, coverMime string) ([]byte, error) {
	var frames bytes.Buffer
	replaceFrames := map[string]bool{}
	if title != "" {
		replaceFrames["TIT2"] = true
	}
	if artist != "" {
		replaceFrames["TPE1"] = true
	}
	if album != "" {
		replaceFrames["TALB"] = true
	}
	if lyric != "" {
		replaceFrames["USLT"] = true
	}
	if len(coverData) > 0 {
		replaceFrames["APIC"] = true
	}
	frames.Write(preservedID3v23Frames(audioData, replaceFrames))

	if title != "" {
		frames.Write(id3v23Frame("TIT2", id3TextFramePayload(title)))
	}
	if artist != "" {
		frames.Write(id3v23Frame("TPE1", id3TextFramePayload(artist)))
	}
	if album != "" {
		frames.Write(id3v23Frame("TALB", id3TextFramePayload(album)))
	}
	if lyric != "" {
		frames.Write(id3v23Frame("USLT", id3USLTPayload(lyric)))
	}
	if len(coverData) > 0 {
		frames.Write(id3v23Frame("APIC", id3APICPayload(coverData, coverMime)))
	}

	frameData := frames.Bytes()
	if len(frameData) == 0 {
		return audioData, nil
	}

	size := id3SynchsafeSize(len(frameData))
	out := make([]byte, 0, 10+len(frameData)+len(audioData))
	out = append(out, 'I', 'D', '3', 0x03, 0x00, 0x00)
	out = append(out, size[:]...)
	out = append(out, frameData...)
	out = append(out, stripID3v2Prefix(audioData)...)
	return out, nil
}

func normalizeCoverMime(coverMime string) string {
	coverMime = strings.TrimSpace(strings.ToLower(coverMime))
	if coverMime == "" {
		return "image/jpeg"
	}
	if strings.Contains(coverMime, "png") {
		return "image/png"
	}
	if strings.Contains(coverMime, "webp") {
		return "image/webp"
	}
	if strings.Contains(coverMime, "gif") {
		return "image/gif"
	}
	return "image/jpeg"
}

func FetchBytesWithMime(urlStr string, source string) ([]byte, string, error) {
	if fetch, handled, err := NewSourceRangeFetch(urlStr, source, ""); handled || err != nil {
		if err != nil {
			return nil, "", err
		}
		var buf bytes.Buffer
		if fetch.ContentLength > 0 && fetch.ContentLength <= int64(1<<(strconv.IntSize-1)-1) {
			buf.Grow(int(fetch.ContentLength))
		}
		if err := fetch.WriteTo(&buf); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), fetch.ContentType, nil
	}
	return fetchBytesSingle(urlStr, source)
}

func fetchBytesSingle(urlStr string, source string) ([]byte, string, error) {
	req, err := BuildSourceRequest("GET", urlStr, source, "")
	if err != nil {
		return nil, "", err
	}

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" && len(data) > 0 {
		contentType = http.DetectContentType(data)
	}

	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	return data, contentType, nil
}

type SourceRangeFetch struct {
	URL           string
	Source        string
	StatusCode    int
	ContentLength int64
	ContentRange  string
	ContentType   string
	Ext           string
	Start         int64
	End           int64
	Total         int64
}

func NewSourceRangeFetch(urlStr string, source string, rangeHeader string) (*SourceRangeFetch, bool, error) {
	req, err := BuildSourceRequest("GET", urlStr, source, "bytes=0-3")
	if err != nil {
		return nil, false, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return nil, false, nil
	}

	total, ok := parseContentRangeTotal(resp.Header.Get("Content-Range"))
	if !ok || total <= 0 {
		return nil, false, nil
	}
	if total > int64(1<<(strconv.IntSize-1)-1) {
		return nil, true, fmt.Errorf("download too large: %d bytes", total)
	}

	probeData, _ := io.ReadAll(resp.Body)
	ext := DetectAudioExtBySignature(probeData)
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}
	if ext == "" {
		ext = DetectAudioExtByContentType(contentType)
	}
	if ext != "" && (contentType == "" || strings.HasPrefix(strings.ToLower(contentType), "application/octet-stream")) {
		contentType = AudioMimeByExt(ext)
	}

	start, end, partial, ok := resolveRangeHeader(rangeHeader, total)
	if !ok {
		return nil, true, fmt.Errorf("invalid range: %s", rangeHeader)
	}

	fetch := &SourceRangeFetch{
		URL:           urlStr,
		Source:        source,
		StatusCode:    http.StatusOK,
		ContentLength: end - start + 1,
		ContentType:   contentType,
		Ext:           ext,
		Start:         start,
		End:           end,
		Total:         total,
	}
	if partial {
		fetch.StatusCode = http.StatusPartialContent
		fetch.ContentRange = fmt.Sprintf("bytes %d-%d/%d", start, end, total)
	}
	return fetch, true, nil
}

func (f *SourceRangeFetch) WriteTo(w io.Writer) error {
	if f == nil {
		return errors.New("nil range fetch")
	}
	return writeParallelRange(w, f.URL, f.Source, f.Start, f.End)
}

type rangeChunkJob struct {
	index int
	start int64
	end   int64
}

type rangeChunkResult struct {
	index       int
	data        []byte
	contentType string
	err         error
}

func writeParallelRange(w io.Writer, urlStr string, source string, start int64, end int64) error {
	if end < start {
		return nil
	}

	const firstChunkSize int64 = 32 * 1024
	const chunkSize int64 = 256 * 1024
	const maxConcurrentChunks = 16

	jobs := buildRangeChunkJobs(start, end, firstChunkSize, chunkSize)

	sem := make(chan struct{}, maxConcurrentChunks)
	results := make(chan rangeChunkResult, len(jobs))

	for _, job := range jobs {
		job := job
		go func() {
			sem <- struct{}{}
			chunk, chunkContentType, err := fetchRangeChunk(urlStr, source, job.start, job.end)
			<-sem
			results <- rangeChunkResult{index: job.index, data: chunk, contentType: chunkContentType, err: err}
		}()
	}

	next := 0
	pending := make(map[int]rangeChunkResult)
	for next < len(jobs) {
		result := <-results
		if result.err != nil {
			return result.err
		}
		pending[result.index] = result

		for {
			ready, ok := pending[next]
			if !ok {
				break
			}
			if _, err := w.Write(ready.data); err != nil {
				return err
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			delete(pending, next)
			next++
		}
	}

	return nil
}

func buildRangeChunkJobs(start int64, end int64, firstChunkSize int64, chunkSize int64) []rangeChunkJob {
	var jobs []rangeChunkJob
	firstEnd := start + firstChunkSize - 1
	if firstEnd > end {
		firstEnd = end
	}
	jobs = append(jobs, rangeChunkJob{index: len(jobs), start: start, end: firstEnd})
	for chunkStart := firstEnd + 1; chunkStart <= end; chunkStart += chunkSize {
		chunkEnd := chunkStart + chunkSize - 1
		if chunkEnd > end {
			chunkEnd = end
		}
		jobs = append(jobs, rangeChunkJob{index: len(jobs), start: chunkStart, end: chunkEnd})
	}
	return jobs
}

func fetchRangeChunk(urlStr string, source string, start int64, end int64) ([]byte, string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := BuildSourceRequest("GET", urlStr, source, fmt.Sprintf("bytes=%d-%d", start, end))
		if err != nil {
			return nil, "", err
		}

		client := &http.Client{Timeout: 90 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		data, readErr := io.ReadAll(resp.Body)
		contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
		if idx := strings.Index(contentType, ";"); idx >= 0 {
			contentType = strings.TrimSpace(contentType[:idx])
		}
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("range %d-%d returned status %d", start, end, resp.StatusCode)
			continue
		}
		if readErr != nil {
			lastErr = readErr
			continue
		}
		expected := int(end - start + 1)
		if len(data) != expected {
			lastErr = fmt.Errorf("range %d-%d returned %d bytes, want %d", start, end, len(data), expected)
			continue
		}
		return data, contentType, nil
	}
	return nil, "", lastErr
}

func parseContentRangeTotal(value string) (int64, bool) {
	parts := strings.Split(strings.TrimSpace(value), "/")
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "*" {
		return 0, false
	}
	total, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		return 0, false
	}
	return total, true
}

func resolveRangeHeader(value string, total int64) (int64, int64, bool, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, total - 1, false, true
	}
	if !strings.HasPrefix(strings.ToLower(value), "bytes=") {
		return 0, 0, false, false
	}

	spec := strings.TrimSpace(value[len("bytes="):])
	if strings.Contains(spec, ",") {
		return 0, 0, false, false
	}

	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false, false
	}

	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	var start, end int64
	var err error

	switch {
	case left == "" && right == "":
		return 0, 0, false, false
	case left == "":
		suffix, err := strconv.ParseInt(right, 10, 64)
		if err != nil || suffix <= 0 {
			return 0, 0, false, false
		}
		if suffix > total {
			suffix = total
		}
		start = total - suffix
		end = total - 1
	case right == "":
		start, err = strconv.ParseInt(left, 10, 64)
		if err != nil || start < 0 || start >= total {
			return 0, 0, false, false
		}
		end = total - 1
	default:
		start, err = strconv.ParseInt(left, 10, 64)
		if err != nil || start < 0 {
			return 0, 0, false, false
		}
		end, err = strconv.ParseInt(right, 10, 64)
		if err != nil || end < start {
			return 0, 0, false, false
		}
		if start >= total {
			return 0, 0, false, false
		}
		if end >= total {
			end = total - 1
		}
	}

	return start, end, true, true
}

func EmbedSongMetadata(audioData []byte, song *model.Song, lyric string, coverData []byte, coverMime string) ([]byte, error) {
	if len(audioData) == 0 {
		return nil, errors.New("empty audio data")
	}

	ext := DetectAudioExt(audioData)
	if song != nil && song.Ext != "" {
		songExt := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(song.Ext, ".")))
		switch songExt {
		case "mp3", "flac", "m4a", "wma":
			ext = songExt
		}
	}

	title := ""
	artist := ""
	album := ""
	if song != nil {
		title = strings.TrimSpace(song.Name)
		artist = strings.TrimSpace(song.Artist)
		album = strings.TrimSpace(song.Album)
	}
	lyric = strings.TrimSpace(lyric)
	coverMime = normalizeCoverMime(coverMime)
	incomingCover := len(coverData) > 0

	if existing, err := tag.ReadFrom(bytes.NewReader(audioData)); err == nil {
		if existingTitle := strings.TrimSpace(existing.Title()); title == "" && existingTitle != "" {
			title = existingTitle
		}
		if existingArtist := strings.TrimSpace(existing.Artist()); artist == "" && existingArtist != "" {
			artist = existingArtist
		}
		if existingAlbum := strings.TrimSpace(existing.Album()); album == "" && existingAlbum != "" {
			album = existingAlbum
		}
		if existingLyric := strings.TrimSpace(existing.Lyrics()); lyric == "" && existingLyric != "" {
			lyric = existingLyric
		}
		if ext == "mp3" && !incomingCover {
			if picture := existing.Picture(); picture != nil && len(picture.Data) > 0 {
				coverData = append([]byte(nil), picture.Data...)
				if picture.MIMEType != "" {
					coverMime = picture.MIMEType
				}
			}
		}
	}

	if ext != "mp3" && ext != "flac" && ext != "m4a" && ext != "wma" {
		return audioData, nil
	}
	if title == "" && artist == "" && album == "" && lyric == "" && len(coverData) == 0 {
		return audioData, nil
	}

	if ext == "mp3" {
		return embedMP3ID3v23Metadata(audioData, title, artist, album, lyric, coverData, coverMime)
	}

	return embedAudioMetadataByFFmpeg(audioData, ext, title, artist, album, lyric, coverData, coverMime)
}

func embedAudioMetadataByFFmpeg(audioData []byte, ext, title, artist, album, lyric string, coverData []byte, coverMime string) ([]byte, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, ErrFFmpegNotFound
	}

	inFile, err := os.CreateTemp("", "gomusicdl-in-*"+"."+ext)
	if err != nil {
		return nil, err
	}
	inPath := inFile.Name()
	defer os.Remove(inPath)
	if _, err := inFile.Write(audioData); err != nil {
		inFile.Close()
		return nil, err
	}
	inFile.Close()

	outFile, err := os.CreateTemp("", "gomusicdl-out-*"+"."+ext)
	if err != nil {
		return nil, err
	}
	outPath := outFile.Name()
	outFile.Close()
	defer os.Remove(outPath)

	args := []string{"-y", "-hide_banner", "-loglevel", "error", "-i", inPath}

	hasCover := len(coverData) > 0
	coverPath := ""
	if hasCover {
		coverExt := ".jpg"
		if strings.Contains(coverMime, "png") {
			coverExt = ".png"
		}
		coverFile, err := os.CreateTemp("", "gomusicdl-cover-*"+coverExt)
		if err != nil {
			return nil, err
		}
		coverPath = coverFile.Name()
		defer os.Remove(coverPath)
		if _, err := coverFile.Write(coverData); err != nil {
			coverFile.Close()
			return nil, err
		}
		coverFile.Close()
		args = append(args, "-i", coverPath)
	}

	if hasCover {
		args = append(args, "-map", "0:a:0", "-map", "1:v:0")
	} else {
		args = append(args, "-map", "0")
	}
	args = append(args, "-map_metadata", "0")

	if hasCover {
		args = append(args, "-c:a", "copy")
		args = append(args, "-c:v", "copy", "-disposition:v:0", "attached_pic", "-metadata:s:v:0", "title=Album cover", "-metadata:s:v:0", "comment=Cover (front)")
	} else {
		args = append(args, "-c", "copy")
	}

	if title != "" {
		args = append(args, "-metadata", "title="+title)
	}
	if artist != "" {
		args = append(args, "-metadata", "artist="+artist)
	}
	if album != "" {
		args = append(args, "-metadata", "album="+album)
	}
	if lyric != "" {
		args = append(args, "-metadata", "lyrics="+lyric)
	}

	if ext == "mp3" {
		args = append(args, "-id3v2_version", "3", "-write_id3v1", "1")
	}

	args = append(args, outPath)

	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg metadata embed failed: %v, output: %s", err, strings.TrimSpace(string(out)))
	}

	finalData, err := os.ReadFile(filepath.Clean(outPath))
	if err != nil {
		return nil, err
	}
	if len(finalData) == 0 {
		return nil, errors.New("embedded output is empty")
	}

	return finalData, nil
}
