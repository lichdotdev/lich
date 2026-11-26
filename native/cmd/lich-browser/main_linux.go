package main

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1 x11
#cgo LDFLAGS: -lm
#include "browser.h"
*/
import "C"

import (
	"errors"
	"syscall"
)

func startEngine() error {
	if C.lich_browser_init() == 0 {
		return errors.New("GTK cannot open the display")
	}
	return nil
}

// stderrOnto points fd at stderr; Dup2 is missing on linux/arm64.
func stderrOnto(fd int) error { return syscall.Dup3(2, fd, 0) }
