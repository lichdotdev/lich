package browser

import (
	"log/slog"
	"os"

	"github.com/lichdotdev/lich/internal/restart"
)

// BrowserEnv pins the lich-browser to start, by path, as restart.NativeEnv
// pins the window.
const BrowserEnv = "LICH_BROWSER"

// findLichBrowser is the lich-browser beside this lich, when there is an X
// display for its pages: the window embeds them as X windows, so Wayland
// without XWayland keeps Chrome (docs/lich-browser.md).
func findLichBrowser(logDir string) *lichBrowser {
	if os.Getenv("DISPLAY") == "" {
		slog.Info("browser: pages run in Chrome, no X display for lich-browser's")
		return nil
	}
	path, err := restart.Companion(os.Getenv, BrowserEnv, "lich-browser", "build it from native/")
	if err != nil {
		slog.Info("browser: pages run in Chrome", "why", err)
		return nil
	}
	return &lichBrowser{spawn: func() (*wireConn, error) { return spawnProcess(path, logDir) }}
}
