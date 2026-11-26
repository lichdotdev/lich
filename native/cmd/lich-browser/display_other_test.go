//go:build !linux

package main

// noDisplay is false: a Windows or macOS session always has one.
func noDisplay() bool { return false }
