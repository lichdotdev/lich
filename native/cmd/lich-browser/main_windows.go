// Command lich-browser holds every page lich shows: the backend starts it and
// talks to it over stdin and stdout, one JSON object per line
// (docs/lich-browser.md, browserwire), and the window makes each page's
// window a child of its own over the pane. stderr is its log. Its pages are
// WebView2's, loaded through the WebView2Loader.dll beside it.
package main

/*
#cgo CXXFLAGS: -std=c++17 -I${SRCDIR}/../../third_party/webview2
#cgo LDFLAGS: -lole32 -loleaut32 -lshlwapi -luser32 -luuid
#include <stdlib.h>
#include "browser.h"
*/
import "C"

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"github.com/lichdotdev/lich/browserwire"
)

// WebView2 calls back on the thread that made its views, which runs the
// message loop: main's goroutine stays on the process's first thread.
func init() { runtime.LockOSThread() }

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetPrefix("lich-browser: ")
	folder, err := dataFolder()
	if err != nil {
		log.Fatalf("WebView2's data folder: %v", err)
	}
	cfolder := C.CString(folder)
	failed := C.lich_browser_init(cfolder)
	C.free(unsafe.Pointer(cfolder))
	if failed != nil {
		log.Fatalf("WebView2: %s", C.GoString(failed))
	}
	the = newBrowser(newWire(os.Stdout))
	go func() {
		if err := the.wire.read(os.Stdin, func(req browserwire.Request) { onMain(func() { the.handle(req) }) }); err != nil {
			log.Printf("stdin: %v", err)
		}
		onMain(func() { C.lich_browser_quit() })
	}()
	C.lich_browser_main()
}

// dataFolder is where WebView2 keeps its browser's data. Every page is
// InPrivate, so nothing of a page's stays there; every lich-browser shares
// it, and so one browser process.
func dataFolder() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "lich", "webview2")
	return dir, os.MkdirAll(dir, 0o755)
}
