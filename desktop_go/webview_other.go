//go:build !windows

package main

import (
	"context"
	"log"

	webview "github.com/Ghibranalj/webview_go"
	"github.com/guohuiyuan/go-music-dl/internal/appshell"
)

func main() {
	if _, err := prepareDesktopDataDir(); err != nil {
		log.Fatalf("desktop data directory setup failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), appshell.ReadyTimeout)
	defer cancel()

	target, err := appshell.StartDesktopServerAndWait(ctx, appshell.DefaultPort)
	if err != nil {
		log.Printf("desktop server startup probe failed: %v", err)
		target = appshell.StartupErrorDataURL(err.Error(), target)
	}

	w := webview.New(false)
	defer w.Destroy()
	if err := configureNativeDesktop(w); err != nil {
		log.Printf("native desktop setup failed: %v", err)
	}
	w.SetTitle("music-dl-desktop-go")
	w.SetSize(1350, 780, webview.Hint(webview.HintNone))
	w.Navigate(target)

	w.Run()
}
