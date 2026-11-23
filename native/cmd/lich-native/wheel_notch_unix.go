//go:build !windows && !darwin

package main

// wheelNotch is the pixels Gio reports for one notch of a mouse wheel: on
// Wayland the compositor's 15 times Gio's discrete scale of 10, and the same
// on X11 since lichdotdev/gio#24. The window prefers X11 for the web view
// (internal/webview), so on a Wayland desktop it runs under XWayland.
const wheelNotch float32 = 150
