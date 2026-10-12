//go:build windows

package claudeprofile

import (
	"strings"

	"github.com/omartelo/lich/internal/shquote"
)

// loginCommandLine is the PowerShell line that logs a profile in, PowerShell
// being the shell a Windows session runs: the variables that would override
// its login removed and its directory set, or the bare command for Personal
// (dir empty). The change stays with that shell, which is the card opened for it.
func loginCommandLine(bin, dir string) string {
	login := "& " + shquote.QuotePwsh(bin) + " auth login --claudeai"
	if dir == "" {
		return login
	}
	removed := make([]string, len(tokenVars))
	for i, name := range tokenVars {
		removed[i] = "Env:" + name
	}
	return "Remove-Item " + strings.Join(removed, ",") + " -ErrorAction SilentlyContinue; " +
		"$env:CLAUDE_CONFIG_DIR = " + shquote.QuotePwsh(dir) + "; " + login
}
