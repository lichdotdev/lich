// Package shquote quotes a string for a shell command line. lich composes
// several of those — the terminal-editor invocation handed to a session
// (internal/system), on a POSIX shell and on Windows, where the session's shell
// is PowerShell; and the hook, MCP and login commands it writes for providers.
// All of them interpolate a path or a binary name the user or an agent chose.
// One copy of each rule, so a metacharacter that survives one call site cannot
// survive the other.
package shquote

import "strings"

// Quote returns s single-quoted, safe against embedded single quotes. It
// assumes an sh/bash/zsh-style shell; PowerShell, which a Windows session runs,
// spells the escape differently — see QuotePwsh.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// QuotePwsh returns s single-quoted for PowerShell, safe against embedded
// single quotes. PowerShell expands nothing inside a single-quoted string — no
// $variable, no `backtick escape, no subexpression — so doubling the quote is
// the whole rule, and unlike cmd.exe (which lich no longer opens a session in)
// there is no character it cannot express.
func QuotePwsh(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
