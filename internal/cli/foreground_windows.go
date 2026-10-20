//go:build windows

package cli

import "golang.org/x/sys/windows"

// asfwAny is ASFW_ANY, (DWORD)-1: any process may take the foreground.
const asfwAny = 0xFFFFFFFF

var allowSetForegroundWindow = windows.NewLazySystemDLL("user32.dll").NewProc("AllowSetForegroundWindow")

// allowWindowForeground hands this process's right to the foreground on to
// lich's window: `lich focus` started by a click (a toast, an editor) holds
// it, the window raising the card does not, and Windows only flashes its
// taskbar button for a window that takes the front without one.
func allowWindowForeground() {
	_, _, _ = allowSetForegroundWindow.Call(asfwAny) // best effort: without it the window flashes instead
}
