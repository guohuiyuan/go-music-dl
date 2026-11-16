//go:build !windows

package web

func revealDownloadFileNative(string) (bool, error) {
	return false, nil
}
