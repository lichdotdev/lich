//go:build !linux && !windows && !darwin

package browser

// findLichBrowser is none: lich-browser has no web engine on this system
// (docs/lich-browser.md).
func findLichBrowser(string) *lichBrowser { return nil }
