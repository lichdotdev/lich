//go:build windows

package main

import (
	"slices"
	"testing"
)

// TestWindowsHasOpenCodeAndCursorProfiles: Kiro CLI has no profiles on
// Windows, so its screen shows no profile rows.
func TestWindowsHasOpenCodeAndCursorProfiles(t *testing.T) {
	if got := xdgProfileKinds(); !slices.Equal(got, []string{opencodeKind, cursorKind}) {
		t.Errorf("kinds = %v", got)
	}
	if _, ok := profileProviderOf(kiroKind); ok {
		t.Error("Kiro CLI has a profile provider on Windows")
	}
}
