//go:build !linux

package main

// xdgProfileProviders is empty: the backend keeps no link-home profiles off
// Linux (internal/xdgprofile).
var xdgProfileProviders []profileProvider
