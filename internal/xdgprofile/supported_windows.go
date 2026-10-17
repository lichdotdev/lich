//go:build windows

package xdgprofile

// providersHere are the providers with link-home profiles on Windows. Kiro CLI
// is not one: it finds its data folder, the one holding data.sqlite3 and the
// login, through the known-folder API, which no variable moves (measured on
// 2.32.0, docs/ceilings.md).
var providersHere = []*Provider{OpenCode, Cursor}
