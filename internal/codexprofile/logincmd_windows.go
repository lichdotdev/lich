//go:build windows

package codexprofile

import (
	"strings"

	"github.com/omartelo/lich/internal/shquote"
)

// loginCommandLine is the PowerShell line that logs a profile in, PowerShell
// being the shell a Windows session runs: the variables that would override
// its login removed and its directory set, or the bare command for Personal
// (dir empty). The change stays with that shell, which is the card opened for it.
func loginCommandLine(bin, dir string) string {
	login := "& " + shquote.QuotePwsh(bin) + " login"
	if dir == "" {
		return login
	}
	removed := make([]string, len(overridingVars))
	for i, name := range overridingVars {
		removed[i] = "Env:" + name
	}
	return "Remove-Item " + strings.Join(removed, ",") + " -ErrorAction SilentlyContinue; " +
		"$env:CODEX_HOME = " + shquote.QuotePwsh(dir) + "; " + login
}
