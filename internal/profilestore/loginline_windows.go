//go:build windows

package profilestore

import (
	"strings"

	"github.com/omartelo/lich/internal/shquote"
)

// LoginLine is the PowerShell line that runs `bin args` as a profile,
// PowerShell being the shell a Windows session runs: the variables that would
// override its login removed and HomeVar set to value, or the bare command for
// Personal (value empty). The change stays with that shell, which is the card
// opened for it.
func (s Spec) LoginLine(bin, args, value string) string {
	login := "& " + shquote.QuotePwsh(bin) + " " + args
	if value == "" {
		return login
	}
	unset := s.Unset()
	removed := make([]string, len(unset))
	for i, name := range unset {
		removed[i] = "Env:" + name
	}
	return "Remove-Item " + strings.Join(removed, ",") + " -ErrorAction SilentlyContinue; " +
		"$env:" + s.HomeVar + " = " + shquote.QuotePwsh(value) + "; " + login
}
