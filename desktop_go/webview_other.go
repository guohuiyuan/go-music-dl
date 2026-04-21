//go:build !windows

package main

import (
	webview "github.com/Ghibranalj/webview_go"
	"github.com/guohuiyuan/go-music-dl/internal/web"
)

func main() {
	go web.Start("37777", false)

	w := webview.New(false)
	w.SetTitle("music-dl-desktop-go")
	w.SetSize(1350, 780, webview.Hint(webview.HintNone))
	w.Navigate("http://localhost:37777/music/")

	w.Run()
}
