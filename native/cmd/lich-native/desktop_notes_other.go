//go:build !linux

package main

import "context"

// newDesktopNotes is the backend's system.Notify off Linux, until the window
// talks to Windows' and macOS' own notification centers.
func newDesktopNotes(_ context.Context, fallback noteSink, _ func(projectID, sessionID, token string), _ func(string)) noteSink {
	return fallback
}
