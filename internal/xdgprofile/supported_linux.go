//go:build linux

package xdgprofile

// providersHere are the providers with link-home profiles on Linux: all three
// keep their logins under an XDG base directory.
var providersHere = []*Provider{OpenCode, Cursor, Kiro}
