package providers

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

// packageManager is a package manager lich knows how to ask for git, gh and
// glab: its binary, the command line before the package, and each tool's
// package under it.
type packageManager struct {
	bin, command string
	packages     map[string]string
}

// packageManagers are tried in order, the first one on PATH with a package
// for the tool answering. The names are each index's own (read 2026-11-22):
// Arch's extra, Debian's and Ubuntu's main, Fedora's, openSUSE's, Homebrew's
// core and winget's. Debian and openSUSE carry no glab, which Homebrew or the
// tool's page then serves.
var packageManagers = []packageManager{
	{"pacman", "sudo pacman -S --needed ", map[string]string{"git": "git", "gh": "github-cli", "glab": "glab"}},
	{"apt", "sudo apt install ", map[string]string{"git": "git", "gh": "gh"}},
	{"dnf", "sudo dnf install ", map[string]string{"git": "git", "gh": "gh", "glab": "glab"}},
	{"zypper", "sudo zypper install ", map[string]string{"git": "git", "gh": "gh"}},
	{"brew", "brew install ", map[string]string{"git": "git", "gh": "gh", "glab": "glab"}},
	{"winget", "winget install --exact --id ", map[string]string{"git": "Git.Git", "gh": "GitHub.cli", "glab": "GLab.GLab"}},
}

// ToolInstall answers toolInstall over RPC.
func (s *Service) ToolInstall(_ context.Context, opts api.BinaryOptions) (string, error) {
	return s.toolInstall(opts.Bin), nil
}

// toolInstall is the command that installs tool with the first package
// manager on PATH that has it, "" when none does.
func (s *Service) toolInstall(tool string) string {
	for _, m := range packageManagers {
		pkg, ok := m.packages[tool]
		if !ok {
			continue
		}
		if _, err := s.lookPath(m.bin); err == nil {
			return m.command + pkg
		}
	}
	return ""
}
