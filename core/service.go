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

// GetAllSourceNames 获取所有源的名称列表
func GetAllSourceNames() []string {
	keys := make([]string, 0, len(SourceMap))
	for k := range SourceMap {
		keys = append(keys, k)
	}
	return keys
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
	// 1. 获取下载链接
	url, err := GetDownloadURL(song)
	if err != nil {
		return fmt.Errorf("获取下载链接失败: %v", err)
	}

	if url == "" {
		return fmt.Errorf("该歌曲无下载链接")
	}

	// 2. 生成文件名: "歌手 - 歌名.mp3"
	// 清洗文件名中的非法字符
	filename := fmt.Sprintf("%s - %s.mp3", song.Artist, song.Name)
	filename = sanitizeFilename(filename)

	// 创建保存目录 (可选)
	saveDir := "downloads"
	if _, err := os.Stat(saveDir); os.IsNotExist(err) {
		os.Mkdir(saveDir, 0755)
	}
	filePath := filepath.Join(saveDir, filename)

	// 3. 使用 music-lib 的 utils.Get 下载
	// 这样可以复用 music-lib 中已经封装好的 Header 处理逻辑
	data, err := utils.Get(url)
	if err != nil {
		return fmt.Errorf("下载失败: %v", err)
	}

	// 4. 写入文件
	out, err := os.Create(filePath)
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
