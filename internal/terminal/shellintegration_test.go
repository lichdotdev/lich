package terminal

import (
	"slices"
	"testing"

	"github.com/lichdotdev/lich/internal/providers"
)

// Only a shell session takes the integration: a provider is a full-screen
// program that draws its own screen, and its binary is no shell to point at
// scripts.
func TestShellIntegrationReachesShellSessionsOnly(t *testing.T) {
	t.Parallel()
	spec := ptySpec{bin: "/bin/bash", args: []string{"-l"}, env: []string{"A=1"}}
	got := withShellIntegration(spec, KindShell, "/cfg/lich/shell-integration")
	if !slices.Contains(got.args, "--rcfile") {
		t.Errorf("shell session args = %q, want the integration's --rcfile", got.args)
	}
	for _, c := range []struct {
		name, kind, dir string
	}{
		{"provider", providers.Claude, "/cfg/lich/shell-integration"},
		{"no integration installed", KindShell, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := withShellIntegration(spec, c.kind, c.dir)
			if !slices.Equal(got.args, spec.args) || !slices.Equal(got.env, spec.env) {
				t.Errorf("spec changed: %q %q", got.args, got.env)
			}
		})
	}
}
