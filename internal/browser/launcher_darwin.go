package browser

// findLichBrowser is the lich-browser beside this lich, in Lich.app's
// Contents/MacOS: the window shows its pages as layers it exports
// (docs/lich-browser.md).
func findLichBrowser(logDir string) *lichBrowser {
	return lichBrowserBeside(logDir, "build it from native/")
}
