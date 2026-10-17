package i18n

import (
	"syscall"
	"unsafe"
)

// muiLanguageName asks GetUserPreferredUILanguages for tags ("pt-BR")
// rather than LANGIDs.
const muiLanguageName = 0x8

var preferredUILanguages = syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserPreferredUILanguages")

// systemTag is the first of the user's display languages, which is what the
// Windows UI is drawn in; the regional format (GetUserDefaultLocaleName) can
// be another language.
func systemTag() string {
	var count, size uint32
	if r, _, _ := preferredUILanguages.Call(muiLanguageName, uintptr(unsafe.Pointer(&count)), 0, uintptr(unsafe.Pointer(&size))); r == 0 || size == 0 {
		return ""
	}
	buf := make([]uint16, size)
	if r, _, _ := preferredUILanguages.Call(muiLanguageName, uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r == 0 {
		return ""
	}
	// buf is a double-NUL-terminated list; the first entry ends at the first NUL.
	return syscall.UTF16ToString(buf)
}
