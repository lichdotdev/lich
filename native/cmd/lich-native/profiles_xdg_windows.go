//go:build windows

package main

// xdgProfileProviders are the link-home providers with profiles here: Kiro CLI
// has none on Windows (internal/xdgprofile).
var xdgProfileProviders = []profileProvider{opencodeProfileProvider, cursorProfileProvider}
