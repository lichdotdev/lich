package browser

import (
	"log/slog"
	"os"

	"github.com/lichdotdev/lich/internal/restart"
)

// findLichBrowser is the lich-browser beside this lich, in Lich.app's
// Contents/MacOS: the window shows its pages as layers it exports
// (docs/lich-browser.md).
func findLichBrowser(logDir string) *lichBrowser {
	path, err := restart.Companion(os.Getenv, BrowserEnv, browserBinary, buildBrowser)
	if err != nil {
		slog.Info("browser: pages run in Chrome", "why", err)
		return nil
	}
	return &lichBrowser{spawn: func() (*wireConn, error) { return spawnProcess(path, logDir) }}
}
