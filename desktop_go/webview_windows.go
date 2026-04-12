//go:build windows

package main

import (
	"log"

	"github.com/guohuiyuan/go-music-dl/internal/web"
	"github.com/jchv/go-webview2"
)

func main() {
	go web.Start("37777", false)

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "go-music-dl-desktop",
			Width:  1350,
			Height: 780,
			IconId: 2, // icon resource id
			Center: true,
		},
	})
	if w == nil {
		log.Fatalln("Failed to load webview.")
	}
	defer w.Destroy()
	w.SetSize(1350, 780, webview2.Hint(webview2.HintNone))
	w.Navigate("http://localhost:37777/music/")
	w.Run()
}
