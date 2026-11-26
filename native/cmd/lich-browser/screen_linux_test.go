package main

import (
	"os"
	"slices"
	"strings"
)

const binaryName = "lich-browser"

// waylandEnv names a Wayland display the tests run lich-browser on with no X
// display (a headless compositor's socket, never the desktop's): its pages
// then draw off screen and send frames.
const waylandEnv = "LICH_BROWSER_TEST_WAYLAND"

// noScreen is why no page can show here, empty when one can.
func noScreen() string {
	if os.Getenv("DISPLAY") == "" && os.Getenv(waylandEnv) == "" {
		return "no X display and no " + waylandEnv
	}
	return ""
}

// offscreen: the tests run on Wayland, where a tab has no surface (xid 0)
// and sends frames instead.
func offscreen() bool { return os.Getenv(waylandEnv) != "" }

// browserEnv is the environment lich-browser runs in: the X display and
// never the desktop's Wayland, or waylandEnv's Wayland and no X.
func browserEnv() []string {
	wayland := os.Getenv(waylandEnv)
	if wayland == "" {
		return append(os.Environ(), "WAYLAND_DISPLAY=/nonexistent/wl-none")
	}
	env := slices.DeleteFunc(os.Environ(), func(kv string) bool { return strings.HasPrefix(kv, "DISPLAY=") })
	return append(env, "WAYLAND_DISPLAY="+wayland)
}
