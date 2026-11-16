//go:build windows

package web

import (
	"fmt"
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	windowsCoInitApartmentThreaded = 0x2
	windowsCoInitDisableOle1DDE    = 0x4
	windowsRPCChangedMode          = 0x80010106
)

var (
	windowsShell32                   = windows.NewLazySystemDLL("shell32.dll")
	windowsOle32                     = windows.NewLazySystemDLL("ole32.dll")
	windowsProcSHParseDisplayName    = windowsShell32.NewProc("SHParseDisplayName")
	windowsProcSHOpenFolderAndSelect = windowsShell32.NewProc("SHOpenFolderAndSelectItems")
	windowsProcCoInitializeEx        = windowsOle32.NewProc("CoInitializeEx")
	windowsProcCoUninitialize        = windowsOle32.NewProc("CoUninitialize")
	windowsProcCoTaskMemFree         = windowsOle32.NewProc("CoTaskMemFree")
)

// revealDownloadFileNative asks the Windows shell to select a file in Explorer.
// Unlike passing /select,<path> through explorer.exe, this avoids command-line
// quoting and reports real HRESULT failures instead of a false success toast.
func revealDownloadFileNative(path string) (bool, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return true, fmt.Errorf("resolve file path: %w", err)
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	initHR, _, _ := windowsProcCoInitializeEx.Call(
		0,
		windowsCoInitApartmentThreaded|windowsCoInitDisableOle1DDE,
	)
	if windowsHRESULTFailed(initHR) && uint32(initHR) != windowsRPCChangedMode {
		return true, windowsShellError("CoInitializeEx", initHR)
	}
	if !windowsHRESULTFailed(initHR) {
		defer func() {
			_, _, _ = windowsProcCoUninitialize.Call()
		}()
	}

	pathPtr, err := windows.UTF16PtrFromString(absolute)
	if err != nil {
		return true, fmt.Errorf("encode file path: %w", err)
	}

	var (
		pidl       uintptr
		attributes uint32
	)
	parseHR, _, _ := windowsProcSHParseDisplayName.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(unsafe.Pointer(&pidl)),
		0,
		uintptr(unsafe.Pointer(&attributes)),
	)
	runtime.KeepAlive(pathPtr)
	if windowsHRESULTFailed(parseHR) {
		return true, windowsShellError("SHParseDisplayName", parseHR)
	}
	if pidl == 0 {
		return true, fmt.Errorf("SHParseDisplayName returned an empty item identifier")
	}
	defer func() {
		_, _, _ = windowsProcCoTaskMemFree.Call(pidl)
	}()

	selectHR, _, _ := windowsProcSHOpenFolderAndSelect.Call(pidl, 0, 0, 0)
	if windowsHRESULTFailed(selectHR) {
		return true, windowsShellError("SHOpenFolderAndSelectItems", selectHR)
	}
	return true, nil
}

func windowsHRESULTFailed(hr uintptr) bool {
	return int32(uint32(hr)) < 0
}

func windowsShellError(operation string, hr uintptr) error {
	return fmt.Errorf("%s failed: HRESULT 0x%08X", operation, uint32(hr))
}
