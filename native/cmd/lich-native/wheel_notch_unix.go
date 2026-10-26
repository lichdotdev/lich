//go:build !windows && !darwin

package main

import "os"

// wheelNotch is the pixels Gio reports for one notch of a mouse wheel: on
// Wayland the compositor's 15 times Gio's discrete scale of 10, on X11 Gio's
// scroll scale of 10 a button press. Gio opens Wayland whenever
// WAYLAND_DISPLAY is set, so the same check picks the size.
var wheelNotch = func() float32 {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return 150
	}
	return 10
}()
