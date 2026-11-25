// Command lich-browser holds every page lich shows: the backend starts it and
// talks to it over stdin and stdout, one JSON object per line
// (docs/lich-browser.md, browserwire), and the window places each page's X
// window over its pane. stderr is its log.
package main

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1 x11
#cgo LDFLAGS: -lm
#include <stdlib.h>
#include "browser_linux.h"
*/
import "C"

import (
	"log"
	"os"
	"runtime"
	"runtime/cgo"
	"syscall"
	"unsafe"

	"github.com/lichdotdev/lich/browserwire"
)

// GTK runs on the thread that called gtk_init, and main's goroutine starts on
// the process's first thread: it stays there.
func init() { runtime.LockOSThread() }

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetPrefix("lich-browser: ")
	out, err := wireOut()
	if err != nil {
		log.Fatalf("stdout: %v", err)
	}
	if C.lich_browser_init() == 0 {
		log.Fatal("GTK cannot open the X display")
	}
	downloads, err := os.MkdirTemp("", "lich-browser-downloads-")
	if err != nil {
		log.Fatalf("downloads: %v", err)
	}
	defer os.RemoveAll(downloads)
	dir := C.CString(downloads)
	C.lich_browser_set_downloads(dir)
	C.free(unsafe.Pointer(dir))

	the = newBrowser(newWire(out))
	go func() {
		if err := the.wire.read(os.Stdin, func(req browserwire.Request) { onGTK(func() { the.handle(req) }) }); err != nil {
			log.Printf("stdin: %v", err)
		}
		onGTK(func() { C.lich_browser_quit() })
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
	if err := syscall.Dup3(2, 1, 0); err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), "wire"), nil
}

// onGTK runs work on GTK's thread, after the work queued before it.
func onGTK(work func()) {
	C.lich_browser_invoke(C.uintptr_t(cgo.NewHandle(work)))
}

//export lichBrowserRun
func lichBrowserRun(handle C.uintptr_t) {
	h := cgo.Handle(handle)
	work := h.Value().(func())
	h.Delete()
	work()
}
