//go:build !linux && !windows

package xdgprofile

// providersHere are the providers with link-home profiles on macOS: opencode
// alone, whose data is xdg-basedir on every platform. Cursor CLI and Kiro CLI
// keep their logins in the macOS Keychain under fixed names, which one login
// session shares between every profile (docs/providers/, Profiles).
var providersHere = []*Provider{OpenCode}
