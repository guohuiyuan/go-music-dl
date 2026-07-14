//go:build !android

package main

import "gioui.org/app"

func (a *desktopApp) requestStoragePermission(app.ViewEvent) {}

func (a *desktopApp) setPlaybackWakeLock(bool) {}
