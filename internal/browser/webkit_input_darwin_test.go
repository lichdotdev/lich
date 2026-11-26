package browser

import "testing"

// paneInputReachesThePage: on macOS the page is a layer of lich-browser's.
func paneInputReachesThePage(t *testing.T, s *Service, f *fakeLichBrowser) {
	t.Helper()
	paneInputGoesOverTheWire(t, s, f)
}
