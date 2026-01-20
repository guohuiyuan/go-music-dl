package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	// 引入基础定义
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"

	// 引入具体的源
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/kuwo"
	"github.com/guohuiyuan/music-lib/migu"
	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/fivesing"
	"github.com/guohuiyuan/music-lib/jamendo"
	"github.com/guohuiyuan/music-lib/joox"
	"github.com/guohuiyuan/music-lib/qianqian"
	"github.com/guohuiyuan/music-lib/soda"
)

// 定义搜索函数类型
type SearchFunc func(keyword string) ([]model.Song, error)

// SourceMap 管理所有支持的源
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

// GetAllSourceNames 获取所有源的名称列表（固定顺序）
func GetAllSourceNames() []string {
	// 返回固定的源顺序，确保 Web 界面和 CLI 的一致性
	return []string{
		"netease",  // 网易云音乐
		"qq",       // QQ音乐
		"kugou",    // 酷狗音乐
		"kuwo",     // 酷我音乐
		"migu",     // 咪咕音乐
		"fivesing", // 5sing
		"jamendo",  // Jamendo
		"joox",     // JOOX
		"qianqian", // 千千音乐
		"soda",     // Soda音乐
		"bilibili", // Bilibili（放在最后，通常不推荐使用）
	}
}

// GetDefaultSourceNames 获取默认启用的源名称列表（排除 bilibili, joox, jamendo, fivesing）
func GetDefaultSourceNames() []string {
	allSources := GetAllSourceNames()
	var defaultSources []string
	excluded := map[string]bool{
		"bilibili": true,
		"joox":     true,
		"jamendo":  true,
		"fivesing": true,
	}
	
	for _, source := range allSources {
		if !excluded[source] {
			defaultSources = append(defaultSources, source)
		}
	}
	return defaultSources
}

// GetSourceDescription 获取音乐源的描述信息
func GetSourceDescription(source string) string {
	descriptions := map[string]string{
		"netease":  "网易云音乐 - 中国领先的在线音乐平台，以个性化推荐和社区氛围著称",
		"qq":       "QQ音乐 - 腾讯旗下音乐平台，拥有海量正版音乐资源",
		"kugou":    "酷狗音乐 - 中国知名的数字音乐交互服务提供商，以音效和K歌功能见长",
		"kuwo":     "酷我音乐 - 提供高品质音乐播放和下载服务，专注于无损音乐",
		"migu":     "咪咕音乐 - 中国移动旗下音乐平台，拥有丰富的正版音乐版权",
		"fivesing": "5sing - 中国原创音乐基地，专注于原创音乐和翻唱作品",
		"jamendo":  "Jamendo - 国际免费音乐平台，提供 Creative Commons 许可的音乐",
		"joox":     "JOOX - 腾讯在东南亚推出的音乐流媒体服务",
		"qianqian": "千千音乐 - 百度旗下音乐平台，前身为千千静听",
		"soda":     "Soda音乐 - 抖音旗下音乐平台，提供高品质音乐流媒体服务",
		"bilibili": "Bilibili - 中国知名视频弹幕网站，包含大量用户上传的音乐内容",
	}
	
	if desc, exists := descriptions[source]; exists {
		return desc
	}
	return "未知音乐源"
}

// SearchAndFilter 支持指定源搜索 + 并发处理
func SearchAndFilter(keyword string, selectedSources []string) ([]model.Song, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allSongs []model.Song

	// 如果未指定源，默认全选
	if len(selectedSources) == 0 {
		selectedSources = GetAllSourceNames()
	}

	// 按照固定顺序处理源，确保结果的一致性
	for _, sourceName := range selectedSources {
		searchFunc, exists := SourceMap[sourceName]
		if !exists {
			continue
		}

		wg.Add(1)
		go func(src string, sFunc SearchFunc) {
			defer wg.Done()
			
			// 调用具体的搜索
			songs, err := sFunc(keyword)
			if err != nil {
				fmt.Printf("搜索源 %s 失败: %v\n", src, err)
				return
			}

			// 标记来源
			for i := range songs {
				songs[i].Source = src
			}

			mu.Lock()
			allSongs = append(allSongs, songs...)
			mu.Unlock()
		}(sourceName, searchFunc)
	}

	wg.Wait()
	return allSongs, nil
}

// GetDownloadURL 根据源获取下载链接
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

// DownloadSong CLI使用的下载函数
func DownloadSong(song *model.Song) error {
	return DownloadSongWithCover(song, false)
}

// DownloadSongWithCover 下载歌曲，可选下载封面
func DownloadSongWithCover(song *model.Song, downloadCover bool) error {
	// 清洗文件名
	filename := fmt.Sprintf("%s - %s.mp3", song.Artist, song.Name)
	filename = sanitizeFilename(filename)

	saveDir := "downloads"
	if _, err := os.Stat(saveDir); os.IsNotExist(err) {
		os.Mkdir(saveDir, 0755)
	}
	filePath := filepath.Join(saveDir, filename)

	// [新增] 针对 Soda 源的特殊处理：使用专用下载器（含解密）
	if song.Source == "soda" {
		fmt.Println("检测到 Soda 音乐，正在下载并解密...")
		// 注意：Soda 下载的是 m4a/mp4，这里强制保存为 mp3 后缀可能不太严谨，但为了兼容性暂且如此
		// 或者可以在 soda.Download 内部处理
		err := soda.Download(song, filePath)
		if err != nil {
			return err
		}
		// Soda 下载完音频后，如果需要封面，单独处理
		if downloadCover && song.Cover != "" {
			coverName := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".jpg"
			coverPath := filepath.Join(saveDir, coverName)
			_ = downloadCoverImage(song.Cover, coverPath)
		}
		return nil
	}

	// --- 以下为通用源的下载逻辑 (不变) ---
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

	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.Write(data)
	if err != nil {
		return err
	}

	if downloadCover && song.Cover != "" {
		coverName := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".jpg"
		coverPath := filepath.Join(saveDir, coverName)
		_ = downloadCoverImage(song.Cover, coverPath)
	}

	return nil
}

// downloadCoverImage 下载封面图片
func downloadCoverImage(coverURL, destPath string) error {
	data, err := utils.Get(coverURL)
	if err != nil {
		return err
	}
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = out.Write(data)
	return err
}

// 简单的文件名清洗工具
func sanitizeFilename(name string) string {
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalid {
		name = strings.ReplaceAll(name, char, "_")
	}
	return name
}
