package main

import "runtime/debug"

// gcPercent is lower than Go's default of 100: with 15 sessions the window's live heap
// is ~580 MB, so the default lets it grow to ~1.16 GB before a collection, and the
// idle half is what the system swaps out. 50 cut the resident size by about a tenth in
// a rig at no CPU cost the profiles could see.
const gcPercent = 50

// tuneGC sets gcPercent unless GOGC already says what the person wants.
func tuneGC(getenv func(string) string) {
	if getenv("GOGC") == "" {
		debug.SetGCPercent(gcPercent)
	}
}
