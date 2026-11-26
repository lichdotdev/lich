//go:build !linux && !darwin

package browser

// findLichBrowser is none: lich-browser holds pages on Linux and macOS until
// WebView2 lands on the same wire (docs/lich-browser.md).
func findLichBrowser(string) *lichBrowser { return nil }
