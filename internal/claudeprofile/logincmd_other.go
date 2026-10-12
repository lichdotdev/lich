//go:build !windows

package claudeprofile

import (
	"strings"

	"github.com/omartelo/lich/internal/shquote"
)

// loginCommandLine is the POSIX shell line that logs a profile in: the
// variables that would override its login unset and its directory exported, or
// the bare command for Personal (dir empty).
func loginCommandLine(bin, dir string) string {
	login := shquote.Quote(bin) + " auth login --claudeai"
	if dir == "" {
		return login
	}
	var b strings.Builder
	b.WriteString("env")
	for _, name := range tokenVars {
		b.WriteString(" -u " + name)
	}
	b.WriteString(" CLAUDE_CONFIG_DIR=" + shquote.Quote(dir) + " " + login)
	return b.String()
}
