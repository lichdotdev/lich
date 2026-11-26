package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework QuartzCore -framework JavaScriptCore -framework CoreGraphics
#include "browser.h"
*/
import "C"

import "syscall"

// startEngine starts AppKit, which cannot fail to.
func startEngine() error {
	C.lich_browser_init()
	return nil
}

// stderrOnto points fd at stderr.
func stderrOnto(fd int) error { return syscall.Dup2(2, fd) }
