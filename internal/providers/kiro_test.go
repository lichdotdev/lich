package providers

import (
	"path/filepath"
	"testing"
)

// KIRO_HOME moves the whole root, which is what every reader of ~/.kiro was
// missing (issue #766).
func TestKiroHomeFollowsTheVariable(t *testing.T) {
	home := t.TempDir()
	explicit := t.TempDir()

	t.Setenv(KiroHomeVar, "")
	if got, want := KiroHome(home), filepath.Join(home, ".kiro"); got != want {
		t.Errorf("with KIRO_HOME unset = %q, want %q", got, want)
	}

	t.Setenv(KiroHomeVar, explicit)
	if got := KiroHome(home); got != explicit {
		t.Errorf("with KIRO_HOME set = %q, want %q", got, explicit)
	}
}

// A relative KIRO_HOME is ignored, not resolved against lich's working
// directory, which is not the CLI's.
func TestKiroHomeIgnoresARelativeVariable(t *testing.T) {
	home := t.TempDir()
	t.Setenv(KiroHomeVar, filepath.Join("relative", "kiro"))
	if got, want := KiroHome(home), filepath.Join(home, ".kiro"); got != want {
		t.Errorf("a relative KIRO_HOME = %q, want %q", got, want)
	}
}

// The agent lich installs lands under KIRO_HOME, where Kiro looks for it.
func TestKiroAgentPathIsUnderKiroHome(t *testing.T) {
	explicit := t.TempDir()
	t.Setenv(KiroHomeVar, explicit)
	got, err := KiroAgentPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(explicit, "agents", KiroAgentName+".json"); got != want {
		t.Errorf("KiroAgentPath() = %q, want %q", got, want)
	}
}
