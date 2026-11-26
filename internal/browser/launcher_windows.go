package browser

// findLichBrowser is the lich-browser.exe beside this lich, with the
// WebView2Loader.dll it loads beside it (docs/lich-browser.md).
func findLichBrowser(logDir string) *lichBrowser {
	return lichBrowserBeside(logDir, "build it with native/build-windows.sh")
}
