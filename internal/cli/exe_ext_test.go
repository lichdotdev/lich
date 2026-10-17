package cli

import (
	"runtime"
	"testing"
)

// Windows runs only what carries .exe, so a companion beside lich.exe is
// lich-native.exe.
func TestCompanionFileNameCarriesTheExecutableExtension(t *testing.T) {
	t.Parallel()
	want := "lich-native"
	if runtime.GOOS == "windows" {
		want = "lich-native.exe"
	}
	if got := executableName(nativeBinary); got != want {
		t.Fatalf("executableName(%q) = %q, want %q", nativeBinary, got, want)
	}
}
