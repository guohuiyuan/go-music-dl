//go:build !windows

package main

import (
	"github.com/guohuiyuan/go-music-dl/internal/web"
	webview "github.com/webview/webview_go"
)

func main() {
	go web.Start("37777", false, web.FeatureFlags{
		VgChangeCover: false,
		VgChangeAudio: false,
		VgChangeLyric: false,
		VgExportVideo: false,
	})

	w := webview.New(false)
	w.SetTitle("go-music-dl-desktop")
	w.SetSize(1350, 780, webview.Hint(webview.HintNone))
	w.Navigate("http://localhost:37777/music/")

	w.Run()
}
