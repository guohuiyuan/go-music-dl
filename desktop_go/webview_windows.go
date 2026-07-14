//go:build windows

package main

import (
	"context"
	"log"

	"github.com/guohuiyuan/go-music-dl/internal/appshell"
	"github.com/jchv/go-webview2"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), appshell.ReadyTimeout)
	defer cancel()

	target, err := appshell.StartDesktopServerAndWait(ctx, appshell.DefaultPort)
	if err != nil {
		log.Printf("desktop server startup probe failed: %v", err)
		target = appshell.StartupErrorDataURL(err.Error(), target)
	}

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "music-dl-desktop-go",
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
	w.Navigate(target)
	w.Run()
}
