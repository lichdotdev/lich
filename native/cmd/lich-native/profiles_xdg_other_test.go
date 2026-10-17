//go:build !linux && !windows

package main

import (
	"slices"
	"testing"
)

// TestMacOSHasOpenCodeProfilesAlone: Cursor CLI and Kiro CLI keep their logins
// in the Keychain, so their screens show no profile rows.
func TestMacOSHasOpenCodeProfilesAlone(t *testing.T) {
	if got := xdgProfileKinds(); !slices.Equal(got, []string{opencodeKind}) {
		t.Errorf("kinds = %v", got)
	}
	for _, kind := range []string{cursorKind, kiroKind} {
		if _, ok := profileProviderOf(kind); ok {
			t.Errorf("%s has a profile provider on macOS", kind)
		}
	}
}
