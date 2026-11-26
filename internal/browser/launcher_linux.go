package browser

import (
	"log/slog"
	"os"
)

// findLichBrowser is the lich-browser beside this lich, when there is a
// display for its pages: X, where the window embeds them, or Wayland, where
// lich-browser draws them off screen and sends their frames
// (docs/lich-browser.md).
func findLichBrowser(logDir string) *lichBrowser {
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		slog.Info("browser: pages run in Chrome, no display for lich-browser's")
		return nil
	}
	return lichBrowserBeside(logDir, "build it from native/")
}
