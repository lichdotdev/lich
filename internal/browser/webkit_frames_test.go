package browser

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/browserwire"
)

// A lich-browser that draws its pages off screen (Wayland without X) is
// shown by the frames it sends, as Chrome's screencast was; one whose
// surfaces the window embeds gets only the size.
func TestWebkitPaneAsksFramesOnlyOfPagesDrawnOffScreen(t *testing.T) {
	for _, tc := range []struct {
		name   string
		frames bool
	}{{"off screen", true}, {"embedded", false}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeLichBrowser(t)
			f.frames = tc.frames
			s, _ := wkService(t, f)
			if _, err := s.Attach("s", Viewport{W: 900, H: 700, Scale: 2}); err != nil {
				t.Fatal(err)
			}
			if err := s.Resize("s", Viewport{W: 800, H: 600, Scale: 2}); err != nil {
				t.Fatal(err)
			}
			if err := s.Detach("s"); err != nil {
				t.Fatal(err)
			}
			casts, stops := f.called(browserwire.Screencast), f.called(browserwire.StopScreencast)
			if !tc.frames {
				if len(casts)+len(stops) != 0 {
					t.Fatalf("%d screencasts and %d stops asked of embedded pages", len(casts), len(stops))
				}
				return
			}
			if len(casts) != 2 || len(stops) != 1 {
				t.Fatalf("%d screencasts and %d stops, want one at attach, one at resize, a stop at detach", len(casts), len(stops))
			}
			for i, want := range []browserwire.ScreencastParams{{W: 900, H: 700, Scale: 2}, {W: 800, H: 600, Scale: 2}} {
				got := decodeParams[browserwire.ScreencastParams](t, casts[i])
				if got.W != want.W || got.H != want.H || got.Scale != want.Scale {
					t.Fatalf("screencast %d at %+v, want %+v", i, got, want)
				}
			}
		})
	}
}

func TestWebkitFramesReachTheWindow(t *testing.T) {
	f := newFakeLichBrowser(t)
	f.frames = true
	s, out := wkService(t, f)
	if _, err := s.Attach("s", Viewport{W: 900, H: 700, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	f.send(browserwire.EventFrame, browserwire.FrameEvent{Page: f.onlyPage(), Tab: "t1", Data: "anVzdCBhIGZyYW1l"})
	name := api.BrowserFrameEvents.Of("s").Name
	waitFor(t, "the frame passed on", func() bool { return len(out.named(name)) > 0 })
	if got := out.named(name)[0].(api.BrowserFrame); got.Tab != "t1" || got.Data != "anVzdCBhIGZyYW1l" {
		t.Fatalf("the window got %+v, want lich-browser's frame as it was", got)
	}
}

func TestWebkitStillIsTheFrontTabsPNG(t *testing.T) {
	for _, tc := range []struct {
		name    string
		frames  bool
		dialog  bool
		wantErr error
	}{
		{"off screen", true, false, nil},
		{"off screen, a dialog open", true, true, errDialogOpen},
		{"embedded", false, false, errNoFrames},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeLichBrowser(t)
			f.frames = tc.frames
			s, _ := wkService(t, f)
			if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
				t.Fatal(err)
			}
			sess := s.current("s")
			if tc.dialog {
				f.send(browserwire.EventDialog, browserwire.DialogEvent{Page: f.onlyPage(), Dialog: &browserwire.JSDialog{Type: browserwire.DialogAlert}})
				waitFor(t, "the dialog held", func() bool {
					select {
					case <-sess.page.(*webkitPage).gate.held():
						return true
					default:
						return false
					}
				})
			}
			tab, data, err := sess.page.Still(t.Context())
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Still = %v, want %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			png, _ := base64.StdEncoding.DecodeString(data)
			if tabs, _ := s.Tabs("s"); len(tabs) == 0 || tab != tabs[0].ID || len(png) == 0 {
				t.Fatalf("Still = tab %q, %d bytes; want the front tab's PNG", tab, len(png))
			}
		})
	}
}
