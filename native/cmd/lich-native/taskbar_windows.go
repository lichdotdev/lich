package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var setAppUserModelID = windows.NewLazySystemDLL("shell32.dll").NewProc("SetCurrentProcessExplicitAppUserModelID")

// claimTaskbarIdentity names this process with the AppUserModelID lich's Start
// Menu shortcut carries (build/windows/lich.iss). Without it Windows groups the
// window by executable, lich-native.exe, so it opens as a button of its own
// beside the pinned lich instead of under it. The dev window's class keeps it
// apart from the daily driver's button.
func claimTaskbarIdentity(class string) error {
	id, err := syscall.UTF16PtrFromString(appUserModelIDPrefix + class)
	if err != nil {
		return err
	}
	if err := setAppUserModelID.Find(); err != nil {
		return err
	}
	hr, _, _ := setAppUserModelID.Call(uintptr(unsafe.Pointer(id)))
	if hr != 0 {
		return fmt.Errorf("SetCurrentProcessExplicitAppUserModelID: HRESULT %#x", hr)
	}
	return nil
}
