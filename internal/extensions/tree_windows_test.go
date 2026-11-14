//go:build windows

package extensions

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

func processGone(t *testing.T, pid int) bool {
	t.Helper()
	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return true
	}
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(process)
	event, err := windows.WaitForSingleObject(process, 0)
	if err != nil {
		t.Fatal(err)
	}
	return event == windows.WAIT_OBJECT_0
}
