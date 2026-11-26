package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework QuartzCore -framework JavaScriptCore -framework CoreGraphics
*/
import "C"

import "syscall"

const engineFailed = "AppKit cannot start"

// stderrOnto points fd at stderr.
func stderrOnto(fd int) error { return syscall.Dup2(2, fd) }
