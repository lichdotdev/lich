// On Windows lich-browser's pages are WebView2's, loaded through the
// WebView2Loader.dll beside it, and the window makes each page's window a
// child of its own over the pane.
package main

/*
#cgo CXXFLAGS: -std=c++17 -I${SRCDIR}/../../third_party/webview2
#cgo LDFLAGS: -lole32 -loleaut32 -lshlwapi -luser32 -luuid
#include <stdlib.h>
#include "browser.h"
*/
import "C"

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"
)

// startEngine loads WebView2 with its data folder.
func startEngine() error {
	folder, err := dataFolder()
	if err != nil {
		return fmt.Errorf("WebView2's data folder: %w", err)
	}
	cfolder := C.CString(folder)
	failed := C.lich_browser_init(cfolder)
	C.free(unsafe.Pointer(cfolder))
	if failed != nil {
		return fmt.Errorf("WebView2: %s", C.GoString(failed))
	}
	return nil
}

// wireOut is stdout itself on Windows.
func wireOut() (*os.File, error) { return os.Stdout, nil }

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
