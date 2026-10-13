//go:build !windows

package codexprofile

import (
	"strings"

	"github.com/omartelo/lich/internal/shquote"
)

// loginCommandLine is the POSIX shell line that logs a profile in: the
// variables that would override its login unset and its directory exported, or
// the bare command for Personal (dir empty).
func loginCommandLine(bin, dir string) string {
	login := shquote.Quote(bin) + " login"
	if dir == "" {
		return login
	}
	var b strings.Builder
	b.WriteString("env")
	for _, name := range overridingVars {
		b.WriteString(" -u " + name)
	}
	b.WriteString(" CODEX_HOME=" + shquote.Quote(dir) + " " + login)
	return b.String()
}
