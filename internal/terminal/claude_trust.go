package terminal

import (
	"log/slog"

	"github.com/omartelo/lich/internal/claudeprofile"
	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/providers"
)

// inheritClaudeTrust carries a trust answer the person gave in Personal into
// the profile the spawn is about to run in (claudeprofile.InheritTrust), so
// Claude Code does not open on its "trust this folder?" question in a folder
// trusted after the profile was made. That question is the first screen of a
// session lich opened unattended, and its preselected answer exits. A failure
// costs the session the question, never its spawn.
func inheritClaudeTrust(kind, cwd, profileDir string) {
	if kind != providers.Claude || profileDir == "" || cwd == "" {
		return
	}
	top, origin := project.CheckoutRoots(cwd)
	if err := claudeprofile.InheritTrust(profileDir, top, origin); err != nil {
		slog.Warn("carry folder trust into Claude Code's config", "cwd", cwd, "err", err)
	}
}
