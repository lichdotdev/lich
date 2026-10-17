//go:build !linux && !windows

package main

// xdgProfileProviders are the link-home providers with profiles here: opencode
// alone on macOS (internal/xdgprofile).
var xdgProfileProviders = []profileProvider{opencodeProfileProvider}
