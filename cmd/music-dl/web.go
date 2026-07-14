package main

import (
	"github.com/guohuiyuan/go-music-dl/internal/web"
	"github.com/spf13/cobra"
)

var port string
var noBrowser bool
var desktopMode bool

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "启动 Web 服务模式",
	Run: func(cmd *cobra.Command, args []string) {
		if desktopMode {
			web.StartDesktop(port)
			return
		}
		web.Start(port, !noBrowser)
	},
}

func init() {
	webCmd.Flags().StringVarP(&port, "port", "p", "8080", "服务端口")
	webCmd.Flags().BoolVar(&noBrowser, "no-browser", false, "不自动打开浏览器")
	webCmd.Flags().BoolVar(&desktopMode, "desktop", false, "桌面内嵌模式")
	_ = webCmd.Flags().MarkHidden("desktop")
	rootCmd.AddCommand(webCmd)
}
