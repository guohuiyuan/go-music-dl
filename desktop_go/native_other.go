//go:build !darwin && !windows

package main

import webview "github.com/Ghibranalj/webview_go"

func configureNativeDesktop(w webview.WebView) error { return nil }
