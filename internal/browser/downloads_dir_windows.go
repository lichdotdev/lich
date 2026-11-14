//go:build windows

package browser

import "golang.org/x/sys/windows"

// downloadDir is the Downloads known folder, which a user can move away from
// %USERPROFILE%\Downloads.
func downloadDir() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_Downloads, 0)
}
