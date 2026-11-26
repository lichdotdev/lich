package main

import "os"

const binaryName = "lich-browser"

// noScreen is empty: a macOS session always has one.
func noScreen() string { return "" }

// offscreen: every tab has a surface the window shows.
func offscreen() bool { return false }

func browserEnv() []string { return os.Environ() }
