package providers

import (
	"slices"
	"strings"
	"testing"
)

// hostOSes are the OSes lich ships for; each provider has an install or a
// named gap on every one.
var hostOSes = []string{"linux", "darwin", "windows"}

func TestEveryProviderHasAnInstallOrAGapOnEveryOS(t *testing.T) {
	for _, goos := range hostOSes {
		for _, p := range Registry {
			r, has := recipes[goos][p.ID]
			gap := installGaps[goos][p.ID]
			switch {
			case has && gap != "":
				t.Errorf("%s on %s has an install and a gap", p.ID, goos)
			case !has && gap == "":
				t.Errorf("%s on %s has neither an install nor a gap", p.ID, goos)
			case has && (len(r.needs) == 0 || r.args[0] != r.needs[0]):
				t.Errorf("%s on %s runs %q but needs %v", p.ID, goos, r.args[0], r.needs)
			}
		}
	}
}

func TestNoInstallAsksForAdministrator(t *testing.T) {
	for goos, byID := range recipes {
		for id, r := range byID {
			line := strings.Join(r.args, " ")
			for _, word := range []string{"sudo", "doas", "pkexec", "runas", "-Verb RunAs"} {
				if strings.Contains(line, word) {
					t.Errorf("%s on %s: %q", id, goos, line)
				}
			}
		}
	}
}

// The command lines are the vendors' own, pinned here word for word: a change
// to one is a change to what runs on someone's machine.
func TestEachInstallIsTheVendorsCommand(t *testing.T) {
	want := map[string]map[string][]string{
		"linux": {
			Claude:      {"bash", "-c", "curl -fsSL https://claude.ai/install.sh | bash"},
			Codex:       {"sh", "-c", "curl -fsSL https://chatgpt.com/codex/install.sh | sh"},
			Antigravity: {"bash", "-c", "curl -fsSL https://antigravity.google/cli/install.sh | bash"},
			OpenCode:    {"bash", "-c", "curl -fsSL https://opencode.ai/v2/install | bash"},
			OMP:         {"sh", "-c", "curl -fsSL https://omp.sh/install | sh"},
			Crush:       {"npm", "install", "-g", "@charmland/crush"},
			Cursor:      {"bash", "-c", "curl https://cursor.com/install -fsS | bash"},
			Kiro:        {"bash", "-c", "curl -fsSL https://cli.kiro.dev/install | bash"},
		},
		"darwin": {
			Claude:      {"bash", "-c", "curl -fsSL https://claude.ai/install.sh | bash"},
			Codex:       {"sh", "-c", "curl -fsSL https://chatgpt.com/codex/install.sh | sh"},
			Antigravity: {"bash", "-c", "curl -fsSL https://antigravity.google/cli/install.sh | bash"},
			OpenCode:    {"bash", "-c", "curl -fsSL https://opencode.ai/v2/install | bash"},
			OMP:         {"sh", "-c", "curl -fsSL https://omp.sh/install | sh"},
			Crush:       {"brew", "install", "charmbracelet/tap/crush"},
			Cursor:      {"bash", "-c", "curl https://cursor.com/install -fsS | bash"},
			Kiro:        {"bash", "-c", "curl -fsSL https://cli.kiro.dev/install | bash"},
		},
		"windows": {
			Claude:      ps("irm https://claude.ai/install.ps1 | iex"),
			Codex:       ps("irm https://chatgpt.com/codex/install.ps1 | iex"),
			Antigravity: ps("irm https://antigravity.google/cli/install.ps1 | iex"),
			OMP:         ps("irm https://omp.sh/install.ps1 | iex"),
			Crush:       {"npm", "install", "-g", "@charmland/crush"},
			Cursor:      ps("irm 'https://cursor.com/install?win32=true' | iex"),
		},
	}
	for goos, byID := range want {
		for id, args := range byID {
			if got := recipes[goos][id].args; !slices.Equal(got, args) {
				t.Errorf("%s on %s: %q, want %q", id, goos, got, args)
			}
		}
		if len(recipes[goos]) != len(byID) {
			t.Errorf("%s has %d installs, the test pins %d", goos, len(recipes[goos]), len(byID))
		}
	}
}

func ps(command string) []string {
	return []string{"powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", command}
}
