//go:build !darwin

package browser

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/browserwire"
)

// paneInputReachesThePage: the X server hands the page the pane's pointer and
// keys, so none goes over the wire but the wheel Select sends.
func paneInputReachesThePage(t *testing.T, s *Service, f *fakeLichBrowser) {
	t.Helper()
	for _, input := range []error{
		mustNil(s.MouseInput(t.Context(), api.BrowserMouseOptions{ID: "s", Type: "move"})),
		mustNil(s.KeyInput(t.Context(), api.BrowserKeyOptions{ID: "s", Type: "down"})),
		mustNil(s.TextInput(t.Context(), api.BrowserTextOptions{ID: "s", Text: "x"})),
	} {
		if input != nil {
			t.Fatal(input)
		}
	}
	if n := len(f.called(browserwire.Mouse)) + len(f.called(browserwire.Key)) + len(f.called(browserwire.Text)); n != 0 {
		t.Fatalf("%d pane inputs reached lich-browser, which takes them from the X server", n)
	}
	if _, err := s.MouseInput(t.Context(), api.BrowserMouseOptions{ID: "s", Type: "wheel", X: 5, Y: 6, DY: 120}); err != nil {
		t.Fatal(err)
	}
	wheel := f.called(browserwire.Mouse)
	if len(wheel) != 1 {
		t.Fatalf("%d wheels reached lich-browser, want the one Select sends, which X gives the page none of", len(wheel))
	}
	if m := decodeParams[browserwire.MouseParams](t, wheel[0]); m.Kind != browserwire.MouseWheel || m.X != 5 || m.Y != 6 || m.DY != 120 {
		t.Fatalf("wheel %+v, want the pane's at 5,6 by 120", m)
	}
}

func TestWebkitPaneInputOfAPageDrawnOffScreenGoesOverTheWire(t *testing.T) {
	f := newFakeLichBrowser(t)
	f.frames = true
	s, _ := wkService(t, f)
	if _, err := s.Attach("s", Viewport{W: 900, H: 700, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	paneInputGoesOverTheWire(t, s, f)
}
