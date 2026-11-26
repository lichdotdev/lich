//go:build linux || darwin

// Command lich-browser holds every page lich shows: the backend starts it and
// talks to it over stdin and stdout, one JSON object per line
// (docs/lich-browser.md, browserwire), and the window shows each page's
// surface over its pane. stderr is its log.
package main

/*
#include <stdlib.h>
#include "browser.h"
*/
import "C"

import (
	"log"
	"os"
	"runtime"
	"runtime/cgo"
	"syscall"

	"github.com/lichdotdev/lich/browserwire"
)

// GTK and AppKit run on the thread that started them, and main's goroutine
// starts on the process's first thread: it stays there.
func init() { runtime.LockOSThread() }

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetPrefix("lich-browser: ")
	out, err := wireOut()
	if err != nil {
		log.Fatalf("stdout: %v", err)
	}
	if C.lich_browser_init() == 0 {
		log.Fatal(engineFailed)
	}
	the = newBrowser(newWire(out))
	go func() {
		if err := the.wire.read(os.Stdin, func(req browserwire.Request) { onMain(func() { the.handle(req) }) }); err != nil {
			log.Printf("stdin: %v", err)
		}
		onMain(func() { C.lich_browser_quit() })
	}()
	C.lich_browser_main()
}

// wireOut is stdout for the wire alone: fd 1 then points at stderr, so a
// library that prints to stdout lands in the log and never in the middle of a
// line the backend reads.
func wireOut() (*os.File, error) {
	fd, err := syscall.Dup(1)
	if err != nil {
		return nil, err
	}
	if err := stderrOnto(1); err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), "wire"), nil
}

// onMain runs work on the engine's thread, after the work queued before it.
func onMain(work func()) {
	C.lich_browser_invoke(C.uintptr_t(cgo.NewHandle(work)))
}

//export lichBrowserRun
func lichBrowserRun(handle C.uintptr_t) {
	h := cgo.Handle(handle)
	work := h.Value().(func())
	h.Delete()
	work()
}
