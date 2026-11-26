//go:build !linux && !windows && !darwin

// Command lich-browser holds every page lich shows (docs/lich-browser.md); it
// has no web engine on this system, and the backend keeps Chrome here.
package main

import "log"

func main() {
	log.Fatal("lich-browser: no web engine on this system yet")
}
