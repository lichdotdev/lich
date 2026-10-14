//go:build !linux

package xdgprofile

// supported is whether the providers keep their logins under an XDG base
// directory on this platform: macOS and Windows put them elsewhere.
const supported = false
