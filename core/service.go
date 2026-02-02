package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"

	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/fivesing"
	"github.com/guohuiyuan/music-lib/jamendo"
	"github.com/guohuiyuan/music-lib/joox"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/kuwo"
	"github.com/guohuiyuan/music-lib/migu"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qianqian"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/soda"
)

// 定义搜索函数类型
type SearchFunc func(keyword string) ([]model.Song, error)

// [新增] 定义歌单搜索函数类型
type SearchPlaylistFunc func(keyword string) ([]model.Playlist, error)

var SourceMap = map[string]SearchFunc{
	"netease":  netease.Search,
	"qq":       qq.Search,
	"kugou":    kugou.Search,
	"kuwo":     kuwo.Search,
	"migu":     migu.Search,
	"bilibili": bilibili.Search,
	"fivesing": fivesing.Search,
	"jamendo":  jamendo.Search,
	"joox":     joox.Search,
	"qianqian": qianqian.Search,
	"soda":     soda.Search,
}

// [新增] 歌单搜索映射
var PlaylistSourceMap = map[string]SearchPlaylistFunc{
	"netease":  netease.SearchPlaylist,
	"qq":       qq.SearchPlaylist,
	"kugou":    kugou.SearchPlaylist,
	"kuwo":     kuwo.SearchPlaylist,
	"bilibili": bilibili.SearchPlaylist,
	"soda":     soda.SearchPlaylist,
	"fivesing": fivesing.SearchPlaylist,
}

func GetAllSourceNames() []string {
	return []string{
		"netease", "qq", "kugou", "kuwo", "migu",
		"fivesing", "jamendo", "joox", "qianqian", "soda", "bilibili",
	}
}

// [新增] 获取支持歌单搜索的源列表
func GetPlaylistSourceNames() []string {
	return []string{
		"netease", "qq", "kugou", "kuwo", "bilibili", "soda", "fivesing",
	}
}

func GetDefaultSourceNames() []string {
	allSources := GetAllSourceNames()
	var defaultSources []string
	excluded := map[string]bool{
		"bilibili": true, "joox": true, "jamendo": true, "fivesing": true,
	}
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
		"soda":     "Soda音乐",
		"bilibili": "Bilibili",
	}
	if desc, exists := descriptions[source]; exists {
		return desc
	}
	return "未知音乐源"
}

// [新增] 歌单搜索入口
func SearchPlaylist(keyword string, selectedSources []string) ([]model.Playlist, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allPlaylists []model.Playlist

	if len(selectedSources) == 0 {
		selectedSources = GetPlaylistSourceNames()
	}

	for _, sourceName := range selectedSources {
		searchFunc, exists := PlaylistSourceMap[sourceName]
		if !exists {
			continue
		}

		wg.Add(1)
		go func(src string, sFunc SearchPlaylistFunc) {
			defer wg.Done()
			lists, err := sFunc(keyword)
			if err == nil {
				for i := range lists {
					lists[i].Source = src
				}
				mu.Lock()
				allPlaylists = append(allPlaylists, lists...)
				mu.Unlock()
			}
		}(sourceName, searchFunc)
	}
	wg.Wait()
	return allPlaylists, nil
}

// GetDownloadURL ... (保持不变)
func GetDownloadURL(song *model.Song) (string, error) {
	switch song.Source {
	case "netease":
		return netease.GetDownloadURL(song)
	case "qq":
		return qq.GetDownloadURL(song)
	case "kugou":
		return kugou.GetDownloadURL(song)
	case "kuwo":
		return kuwo.GetDownloadURL(song)
	case "migu":
		return migu.GetDownloadURL(song)
	case "bilibili":
		return bilibili.GetDownloadURL(song)
	case "fivesing":
		return fivesing.GetDownloadURL(song)
	case "jamendo":
		return jamendo.GetDownloadURL(song)
	case "joox":
		return joox.GetDownloadURL(song)
	case "qianqian":
		return qianqian.GetDownloadURL(song)
	case "soda":
		return soda.GetDownloadURL(song)
	default:
		return "", fmt.Errorf("不支持的源: %s", song.Source)
	}
}

// GetLyrics ... (保持不变)
func GetLyrics(song *model.Song) (string, error) {
	switch song.Source {
	case "netease":
		return netease.GetLyrics(song)
	case "qq":
		return qq.GetLyrics(song)
	case "kugou":
		return kugou.GetLyrics(song)
	case "kuwo":
		return kuwo.GetLyrics(song)
	case "migu":
		return migu.GetLyrics(song)
	case "bilibili":
		return bilibili.GetLyrics(song)
	case "fivesing":
		return fivesing.GetLyrics(song)
	case "jamendo":
		return jamendo.GetLyrics(song)
	case "joox":
		return joox.GetLyrics(song)
	case "qianqian":
		return qianqian.GetLyrics(song)
	case "soda":
		return soda.GetLyrics(song)
	default:
		return "", fmt.Errorf("不支持的源: %s", song.Source)
	}
}

func DownloadSong(song *model.Song) error {
	return DownloadSongWithOptions(song, "downloads", false, false)
}

func DownloadSongWithOptions(song *model.Song, saveDir string, downloadCover bool, downloadLyrics bool) error {
	filename := song.Filename()
	if saveDir == "" {
		saveDir = "downloads"
	}
	if _, err := os.Stat(saveDir); os.IsNotExist(err) {
		os.MkdirAll(saveDir, 0755)
	}

	filePath := filepath.Join(saveDir, filename)
	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))

	if song.Source == "soda" {
		if err := soda.Download(song, filePath); err != nil {
			return err
		}
	} else {
		url, err := GetDownloadURL(song)
		if err != nil {
			return fmt.Errorf("获取下载链接失败: %v", err)
		}
		if url == "" {
			return fmt.Errorf("该歌曲无下载链接")
		}
		data, err := utils.Get(url)
		if err != nil {
			return fmt.Errorf("下载失败: %v", err)
		}
		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return err
		}
	}

	if downloadCover && song.Cover != "" {
		_ = downloadFile(song.Cover, filepath.Join(saveDir, baseName+".jpg"))
	}
	if downloadLyrics {
		lrc, err := GetLyrics(song)
		if err == nil && lrc != "" {
			_ = os.WriteFile(filepath.Join(saveDir, baseName+".lrc"), []byte(lrc), 0644)
		}
	}
	return nil
}

func DownloadSongWithCover(song *model.Song, downloadCover bool) error {
	return DownloadSongWithOptions(song, "downloads", downloadCover, false)
}

func downloadFile(url, destPath string) error {
	data, err := utils.Get(url)
	if err != nil {
		return err
	}
	return os.WriteFile(destPath, data, 0644)
}
