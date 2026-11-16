//go:build windows

package web

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowsShellParsesChinesePathWithSpaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "歌手 名称", "最真的梦 (女声深情版) - 七分可可.m4a")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}

	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	var (
		pidl       uintptr
		attributes uint32
	)
	hr, _, _ := windowsProcSHParseDisplayName.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(unsafe.Pointer(&pidl)),
		0,
		uintptr(unsafe.Pointer(&attributes)),
	)
	if windowsHRESULTFailed(hr) {
		t.Fatalf("SHParseDisplayName failed: HRESULT 0x%08X", uint32(hr))
	}
	if pidl == 0 {
		t.Fatal("SHParseDisplayName returned an empty item identifier")
	}
	_, _, _ = windowsProcCoTaskMemFree.Call(pidl)
}
