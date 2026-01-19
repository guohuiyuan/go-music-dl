package main

import (
	"github.com/spf13/cobra"
	"github.com/guohuiyuan/go-music-dl/internal/web"
)

var port string

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "启动 Web 服务模式",
	Run: func(cmd *cobra.Command, args []string) {
		web.Start(port)
	},
}

func init() {
	webCmd.Flags().StringVarP(&port, "port", "p", "8080", "服务端口")
	rootCmd.AddCommand(webCmd)
}
