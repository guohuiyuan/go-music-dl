//go:build windows

package main

import (
	"context"
	"log"
	"syscall"
	"unsafe"

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
		//引导下载 webview2
		user32 := syscall.NewLazyDLL("user32.dll")

		title, _ := syscall.UTF16PtrFromString("Error!")
		text, _ := syscall.UTF16PtrFromString("打开Webview2失败！请下载相关Window组件后再试。\n按下Ctrl+C即可复制本窗口的文本,下载地址:https://developer.microsoft.com/microsoft-edge/webview2/")

		// 参数：父窗口句柄(0), 消息文本, 标题, 按钮类型(0=仅确定)
		user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0)
		log.Fatalln("Failed to load webview.")
	}
	defer w.Destroy()
	w.SetSize(1350, 780, webview2.Hint(webview2.HintNone))
	w.Navigate(target)
	w.Run()
}
