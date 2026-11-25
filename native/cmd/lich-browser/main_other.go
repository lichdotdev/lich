//go:build !linux

// Command lich-browser holds every page lich shows (docs/lich-browser.md); it
// runs on Linux only so far, and the backend keeps Chrome elsewhere.
package main

import "log"

func main() {
	log.Fatal("lich-browser: no web engine on this system yet")
}
