//go:build !darwin

package main

// consoleValue is the program's value as the engine's C side reads it: WebKitGTK
// turns the value itself into JSON.
const consoleValue = "v"
