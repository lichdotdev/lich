//go:build linux || windows || darwin

// Command lich-browser holds every page lich shows: the backend starts it and
// talks to it over stdin and stdout, one JSON object per line
// (docs/lich-browser.md, browserwire), and the window shows each page's
// surface over its pane. stderr is its log.
package main

/*
#include "browser.h"
*/
import "C"

import (
	"log"
	"os"
	"runtime"

	"github.com/lichdotdev/lich/browserwire"
)

// The engine runs on the thread that started it (GTK, AppKit, WebView2's
// message loop), and main's goroutine starts on the process's first thread:
// it stays there.
func init() { runtime.LockOSThread() }

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetPrefix("lich-browser: ")
	out, err := wireOut()
	if err != nil {
		log.Fatalf("stdout: %v", err)
	}
	if err := startEngine(); err != nil {
		log.Fatal(err)
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
