//go:build !windows

package core

import "os/exec"

func HideCommandWindow(cmd *exec.Cmd) {
	// 非 Windows 平台不需要处理。
}
