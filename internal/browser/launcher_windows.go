package browser

import (
	"log/slog"
	"os"

	"github.com/lichdotdev/lich/internal/restart"
)

// findLichBrowser is the lich-browser.exe beside this lich, with the
// WebView2Loader.dll it loads beside it (docs/lich-browser.md).
func findLichBrowser(logDir string) *lichBrowser {
	path, err := restart.Companion(os.Getenv, BrowserEnv, "lich-browser", "build it with native/build-windows.sh")
	if err != nil {
		slog.Info("browser: pages run in Chrome", "why", err)
		return nil
	}
	return &lichBrowser{spawn: func() (*wireConn, error) { return spawnProcess(path, logDir) }}
}
