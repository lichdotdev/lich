package main

import "strings"

// The freedesktop appearance portal's color-scheme values.
const (
	portalNoPreference = 0
	portalPreferDark   = 1
)

// portalDark maps a color-scheme value to dark. "No preference" is light.
func portalDark(scheme uint32) bool { return scheme == portalPreferDark }

// appsUseLightThemeDark maps Windows' AppsUseLightTheme DWORD to dark: 0 is
// the dark app mode, anything else light.
func appsUseLightThemeDark(value uint64) bool { return value == 0 }

// interfaceStyleDark maps the output of `defaults read -g AppleInterfaceStyle`
// to dark. macOS writes "Dark" in dark mode and removes the key in light.
func interfaceStyleDark(out string) bool { return strings.TrimSpace(out) == "Dark" }
