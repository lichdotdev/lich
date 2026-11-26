package agentplugin

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// onlyBins turns on the providers in on alone, every one run as bin.
type onlyBins struct {
	bin string
	on  []string
}

func (b onlyBins) ProviderBin(string, string) string { return b.bin }

func (b onlyBins) ProviderEnabled(id string) (bool, error) { return slices.Contains(b.on, id), nil }

// problems records what the service reports on api.PluginProblemEvent.
func problems(s *Service) func() []Problem {
	var got []Problem
	s.SetEmit(func(name string, data any) {
		if name == api.PluginProblemEvent.Name {
			got = append(got, data.(Problem))
		}
	})
	return func() []Problem { return got }
}

// A pass unpacks a newer compatible release beside the one there, which a
// session started on it keeps reading, and new spawns take the newer one.
func TestEnsureUnpacksANewerReleaseAndKeepsTheOlder(t *testing.T) {
	s, _ := fileServerAt(t, "0.14.0", nil)
	s.lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	root := bundleHome(t)
	older := plantRelease(t, root, testVersion)

	s.Ensure()

	if _, version, ok := BundleDir(); !ok || version != "0.14.0" {
		t.Errorf("BundleDir version = (%q,%v), want the newer release", version, ok)
	}
	if _, err := os.Stat(filepath.Join(older, ".claude-plugin", "plugin.json")); err != nil {
		t.Errorf("the older release a session may still run was removed: %v", err)
	}
	if p := s.currentProblem(); p.Error != "" {
		t.Errorf("problem = %+v, want none", p)
	}
}

// A newer release that does not arrive costs nothing while one is there: the
// sessions keep loading it, and nothing is reported.
func TestEnsureOfflineWithAReleaseThereReportsNothing(t *testing.T) {
	s, _ := fileServerAt(t, "0.14.0", nil)
	s.tarballBase += "/missing"
	s.lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	plantRelease(t, bundleHome(t), testVersion)
	got := problems(s)

	s.Ensure()

	if p := s.currentProblem(); p.Error != "" || len(got()) != 0 {
		t.Errorf("problem = %+v, events %v, want none", p, got())
	}
}

// A first start that cannot unpack a release says so once, keeps saying so
// while it lasts without another event, and Retry clears it when the release
// arrives.
func TestEnsureReportsAMissingReleaseUntilRetryUnpacksIt(t *testing.T) {
	s, _ := fileServer(t, nil)
	s.lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	base := s.tarballBase
	s.tarballBase += "/missing"
	got := problems(s)

	s.Ensure()
	s.Ensure()
	p, _ := s.Problem(t.Context(), api.None{})
	if !p.Bundle || p.Error == "" || len(got()) != 1 {
		t.Fatalf("problem = %+v, events %v, want the release missing, reported once", p, got())
	}

	s.tarballBase = base
	if _, err := s.Retry(t.Context(), api.None{}); err != nil {
		t.Fatal(err)
	}
	if p := s.currentProblem(); p.Error != "" || p.Bundle {
		t.Errorf("problem after Retry = %+v, want none", p)
	}
	if events := got(); len(events) != 2 || events[1].Error != "" {
		t.Errorf("events %v, want the failure then its end", events)
	}
	if _, _, ok := BundleDir(); !ok {
		t.Error("Retry did not unpack the release")
	}
}

// Every provider the rig fakes: a pass installs the plugin into a provider the
// person turned on whose CLI is there, leaves one turned off or missing alone,
// and names one whose install fails.
func TestEnsureInstallsWhereThePersonTurnedItOn(t *testing.T) {
	for _, tc := range []struct {
		name      string
		on        []string
		missing   bool
		failOn    string
		wantCalls []string
		wantFail  []string
	}{
		{name: "Codex on", on: []string{providers.Codex}, wantCalls: pinnedCalls[providers.Codex]},
		{name: "Codex off", on: nil},
		{name: "Codex on without its CLI", on: []string{providers.Codex}, missing: true},
		{name: "Codex refusing the install", on: []string{providers.Codex}, failOn: "plugin add",
			wantCalls: pinnedCalls[providers.Codex], wantFail: []string{"Codex"}},
		{name: "Claude Code on, with nothing installed of the past", on: []string{providers.Claude}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
			t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
			s, calls := fakeCLI(t, tc.failOn)
			s.bins = onlyBins{bin: string(s.bins.(stubBin)), on: tc.on}
			if tc.missing {
				s.lookPath = func(string) (string, error) { return "", os.ErrNotExist }
			}

			s.Ensure()

			var installs []string
			for _, c := range calls() {
				if c != "plugin list --json" && !strings.HasPrefix(c, codexAppServer) {
					installs = append(installs, c)
				}
			}
			if !slices.Equal(installs, tc.wantCalls) {
				t.Errorf("calls %v, want %v", installs, tc.wantCalls)
			}
			if p := s.currentProblem(); !slices.Equal(p.Providers, tc.wantFail) || p.Bundle {
				t.Errorf("problem = %+v, want %v named", p, tc.wantFail)
			}
			if _, _, ok := BundleDir(); !ok {
				t.Error("the release sessions load was not unpacked")
			}
		})
	}
}

// A spawn asks for a pass only when its provider is on and missing the plugin,
// or the last pass failed.
func TestNeedsPass(t *testing.T) {
	s, _ := fileServer(t, nil)
	s.bins = onlyBins{on: []string{providers.Claude}}
	root := bundleHome(t)
	if !s.needsPass(providers.Claude) {
		t.Error("Claude Code on with no release: want a pass")
	}
	if s.needsPass(providers.Codex) || s.needsPass("") {
		t.Error("a provider turned off, or none: want no pass")
	}
	plantRelease(t, root, testVersion)
	if s.needsPass(providers.Claude) {
		t.Error("Claude Code with its release there: want no pass")
	}
	s.setProblem(Problem{Providers: []string{"Codex"}, Error: "refused"})
	if !s.needsPass("") {
		t.Error("after a failed pass: want another")
	}
}
