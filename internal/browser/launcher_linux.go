package browser

import (
	"log/slog"
	"os"
)

// findLichBrowser is the lich-browser beside this lich, when there is an X
// display for its pages: the window embeds them as X windows, so Wayland
// without XWayland keeps Chrome (docs/lich-browser.md).
func findLichBrowser(logDir string) *lichBrowser {
	if os.Getenv("DISPLAY") == "" {
		slog.Info("browser: pages run in Chrome, no X display for lich-browser's")
		return nil
	}
	return lichBrowserBeside(logDir, "build it from native/")
}
