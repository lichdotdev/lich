package main

const binaryName = "lich-browser.exe"

// noScreen is empty: Windows always has a screen, and the build of a
// lich-browser.exe the tests make needs WebView2Loader.dll copied beside it
// (build-windows.sh) or builtEnv set.
func noScreen() string { return "" }
