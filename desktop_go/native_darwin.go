//go:build darwin

package main

/*
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
void music_dl_install_edit_menu(void);
char *music_dl_pick_folder(void);
*/
import "C"

import (
	webview "github.com/Ghibranalj/webview_go"
	"unsafe"
)

func configureNativeDesktop(w webview.WebView) error {
	C.music_dl_install_edit_menu()
	// Bind callbacks execute on the WebView UI thread, as required by AppKit.
	return w.Bind("musicDlPickDownloadFolder", func() string {
		path := C.music_dl_pick_folder()
		if path == nil {
			return ""
		}
		defer C.free(unsafe.Pointer(path))
		return C.GoString(path)
	})
}
