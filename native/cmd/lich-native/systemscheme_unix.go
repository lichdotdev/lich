//go:build !linux && !windows && !darwin

package main

import "context"

// watchSystemDark reports dark: the BSDs have no OS color scheme lich reads.
func watchSystemDark(context.Context, func(dark bool)) bool { return true }
