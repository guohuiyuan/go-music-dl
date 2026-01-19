package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/guohuiyuan/go-music-dl/internal/cli"
)

// 全局配置变量
var (
	showVersion bool
	keyword     string
	urlStr      string
	playlist    string
	sources     []string
	number      int
	outDir      string
	proxy       string
	verbose     bool
	withLyrics  bool
	withCover   bool
	noMerge     bool
	filter      string
	play        bool
)

var rootCmd = &cobra.Command{
	Use:   "music-dl [OPTIONS]",
	Short: "Search and download music from netease, qq, kugou, baidu and xiami.",
	Example: `  music-dl -k "周杰伦"
  music-dl web`, // 增加 web 子命令提示
	Run: func(cmd *cobra.Command, args []string) {
		if showVersion {
			fmt.Println("music-dl version v1.0.0")
			return
		}

		// 优先处理 Web 模式（虽然通常它是子命令，但也可以通过 flag 触发逻辑，这里我们保留子命令方式，但也兼容直接运行）
		// 如果有关键字，进入搜索模式
		if keyword != "" {
			// 默认源
			if len(sources) == 0 {
				sources = []string{"netease", "qq", "kugou", "kuwo", "migu"} // 排除 bilibili 除非显式指定? 或者默认带上
			}
			cli.Run(keyword, sources, outDir, number, withCover)
			return
		}

		// 如果有 URL
		if urlStr != "" {
			fmt.Println("🚀 URL 下载功能开发中: ", urlStr)
			return
		}
		
		// 如果没有参数，启动交互式 CLI
		fmt.Println("🎵 欢迎使用 Go Music DL 交互式命令行")
		fmt.Println("   输入 'q' 退出程序")
		fmt.Println("   或直接输入歌名/歌手进行搜索")
		fmt.Println()
		cli.RunInteractive()
	},
}

func init() {
	// 绑定 Flags
	rootCmd.Flags().BoolVar(&showVersion, "version", false, "Show the version and exit.")
	rootCmd.Flags().StringVarP(&keyword, "keyword", "k", "", "搜索关键字，歌名和歌手同时输入可以提高匹配")
	rootCmd.Flags().StringVarP(&urlStr, "url", "u", "", "通过指定的歌曲URL下载音乐")
	rootCmd.Flags().StringVarP(&playlist, "playlist", "p", "", "通过指定的歌单URL下载音乐")
	rootCmd.Flags().StringSliceVarP(&sources, "source", "s", []string{"netease", "qq", "kugou", "kuwo", "migu"}, "Supported music source")
	rootCmd.Flags().IntVarP(&number, "number", "n", 10, "Number of search results")
	rootCmd.Flags().StringVarP(&outDir, "outdir", "o", ".", "Output directory")
	rootCmd.Flags().StringVarP(&proxy, "proxy", "x", "", "Proxy (e.g. http://127.0.0.1:1087)")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose mode")
	rootCmd.Flags().BoolVar(&withLyrics, "lyrics", false, "同时下载歌词")
	rootCmd.Flags().BoolVar(&withCover, "cover", false, "同时下载封面")
	rootCmd.Flags().BoolVar(&noMerge, "nomerge", false, "不对搜索结果列表排序和去重")
	rootCmd.Flags().StringVar(&filter, "filter", "", "按文件大小和歌曲时长过滤搜索结果")
	rootCmd.Flags().BoolVar(&play, "play", false, "开启下载后自动播放功能")
}
