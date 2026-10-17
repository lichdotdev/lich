//go:build windows

package profilestore

import (
	"strings"

	"github.com/lichdotdev/lich/internal/shquote"
)

// LoginLine is the PowerShell line that runs `bin args` as a profile,
// PowerShell being the shell a Windows session runs: the variables that would
// override its login removed and HomeVar (and Pinned) set to value, or the
// bare command for Personal (value empty). The change stays with that shell,
// which is the card opened for it.
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
	var set strings.Builder
	for _, v := range s.assignments(value) {
		set.WriteString("$env:" + v[0] + " = " + shquote.QuotePwsh(v[1]) + "; ")
	}
	return "Remove-Item " + strings.Join(removed, ",") + " -ErrorAction SilentlyContinue; " + set.String() + login
}
