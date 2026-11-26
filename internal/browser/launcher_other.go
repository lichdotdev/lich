//go:build !linux && !windows

package browser

// findLichBrowser is none: lich-browser holds pages on Linux and Windows
// only until WKWebView lands on the same wire (docs/lich-browser.md).
func findLichBrowser(string) *lichBrowser { return nil }
