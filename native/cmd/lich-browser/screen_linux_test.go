package main

import "os"

const binaryName = "lich-browser"

// noScreen is why no page can show here, empty when one can.
func noScreen() string {
	if os.Getenv("DISPLAY") == "" {
		return "no X display"
	}
	return ""
}
