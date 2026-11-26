package main

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1 x11
#cgo LDFLAGS: -lm
*/
import "C"

import "syscall"

const engineFailed = "GTK cannot open the X display"

// stderrOnto points fd at stderr; Dup2 is missing on linux/arm64.
func stderrOnto(fd int) error { return syscall.Dup3(2, fd, 0) }
