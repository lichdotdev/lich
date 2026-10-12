//go:build !linux

package main

import "context"

// watchSystemDark reports dark: the window reads no OS color scheme outside
// Linux yet. ponytail: Windows' AppsUseLightTheme and macOS's
// AppleInterfaceStyle are where System would read it.
func watchSystemDark(context.Context, func(dark bool)) bool { return true }
