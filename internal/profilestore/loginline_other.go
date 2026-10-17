//go:build !windows

package profilestore

import (
	"strings"

	"github.com/lichdotdev/lich/internal/shquote"
)

// LoginLine is the POSIX shell line that runs `bin args` as a profile: the
// variables that would override its login unset and HomeVar (and Pinned)
// exported as value, or the bare command for Personal (value empty).
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
	for _, v := range s.assignments(value) {
		b.WriteString(" " + v[0] + "=" + shquote.Quote(v[1]))
	}
	b.WriteString(" " + login)
	return b.String()
}
