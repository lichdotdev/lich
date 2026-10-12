package main

// The freedesktop appearance portal's color-scheme values.
const (
	portalNoPreference = 0
	portalPreferDark   = 1
)

// portalDark maps a color-scheme value to dark. "No preference" is light, as
// Chromium reads it for the web window's prefers-color-scheme.
func portalDark(scheme uint32) bool { return scheme == portalPreferDark }
