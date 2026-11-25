//go:build !linux

package browser

// findLichBrowser is none: lich-browser holds pages on Linux only until
// WebView2 and WKWebView land on the same wire (docs/lich-browser.md).
func findLichBrowser(string) *lichBrowser { return nil }
