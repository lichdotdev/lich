package browser

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/browserwire"
)

// paneInputReachesThePage: a page in the window takes no input of its own
// here, so the pane's goes over the wire, as the wire names it.
func paneInputReachesThePage(t *testing.T, s *Service, f *fakeLichBrowser) {
	t.Helper()
	for _, err := range []error{
		mustNil(s.MouseInput(t.Context(), api.BrowserMouseOptions{ID: "s", Type: "press", X: 5, Y: 6, Button: "right", Clicks: 2, Modifiers: 8 | 4})),
		mustNil(s.KeyInput(t.Context(), api.BrowserKeyOptions{ID: "s", Type: "down", Key: "a", Code: "KeyA", Modifiers: 2})),
		mustNil(s.TextInput(t.Context(), api.BrowserTextOptions{ID: "s", Text: "x"})),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	m := decodeParams[browserwire.MouseParams](t, f.called(browserwire.Mouse)[0])
	if m.Kind != browserwire.MouseDown || m.X != 5 || m.Y != 6 || m.Button != 2 || m.Clicks != 2 || m.Mods != browserwire.ModShift|browserwire.ModMeta {
		t.Fatalf("mouse %+v, want the pane's right double press at 5,6 with Shift and Meta", m)
	}
	k := decodeParams[browserwire.KeyParams](t, f.called(browserwire.Key)[0])
	if k.Kind != browserwire.KeyDown || k.Key != "a" || k.Code != "KeyA" || k.Mods != browserwire.ModCtrl {
		t.Fatalf("key %+v, want the pane's Ctrl+a down", k)
	}
	if x := decodeParams[browserwire.TextParams](t, f.called(browserwire.Text)[0]); x.Text != "x" {
		t.Fatalf("text %+v, want the pane's x", x)
	}
}
