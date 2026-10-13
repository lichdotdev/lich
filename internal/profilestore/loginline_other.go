//go:build !windows

package profilestore

import (
	"strings"

	"github.com/omartelo/lich/internal/shquote"
)

// LoginLine is the POSIX shell line that runs `bin args` as a profile: the
// variables that would override its login unset and HomeVar exported as value,
// or the bare command for Personal (value empty).
func (s Spec) LoginLine(bin, args, value string) string {
	login := shquote.Quote(bin) + " " + args
	if value == "" {
		return login
	}
	var b strings.Builder
	b.WriteString("env")
	for _, name := range s.Unset() {
		b.WriteString(" -u " + name)
	}
	b.WriteString(" " + s.HomeVar + "=" + shquote.Quote(value) + " " + login)
	return b.String()
}
