package main

import (
	"image"
	"slices"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// focusRig is a window with surface a/main on screen: a search field under
// the key "q" and the shortcut r.
type focusRig struct {
	u      *window
	log    sentLog
	router input.Router
	th     *ui.Theme
}

func newFocusRig() *focusRig {
	r := &focusRig{th: &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Background}}
	r.u = &window{model: newTestModel(nil), extOutbox: newExtensionOutbox()}
	store := &r.u.model.extensions
	store.list = []backend.Extension{{ID: "a", Name: "Board", Status: api.ExtensionRunning}}
	store.setRender("a", extension.RenderData{
		Surface:   "main",
		Root:      extension.Box.New(extension.BoxProps{}, extension.Input.New(extension.InputProps{OnChange: "typed"}).WithKey("q")),
		Shortcuts: []extension.Shortcut{{Keys: "r", Handler: "refresh"}},
	})
	r.u.extension = newExtensionScreen(store, nil, r.log.send)
	r.u.extension.show(extensionKey{"a", "main"})
	r.frame()
	return r
}

func (r *focusRig) frame(events ...event.Event) {
	r.router.Queue(events...)
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(400, 300)), Source: r.router.Source()}
	r.u.extension.layout(gtx, r.th, false)
	r.router.Frame(&ops)
}

func (r *focusRig) focus(ext, surface string) {
	r.u.extensionMessage(ext, extension.Focus.New(extension.FocusData{Surface: surface, Key: "q"}))
	r.frame()
	r.frame(key.EditEvent{Text: "x"})
}

func TestAFocusReachesOnlyTheSurfaceOnScreen(t *testing.T) {
	r := newFocusRig()
	r.focus("a", "board")
	r.focus("b", "main")
	if len(r.log.sent) != 1 {
		t.Fatalf("a focus for a surface off screen moved the keyboard: %v", r.log.sent)
	}
	r.focus("a", "main")
	if want := []string{"a surface.opened main", "a event typed"}; !slices.Equal(r.log.sent, want) {
		t.Errorf("sent %v, want %v", r.log.sent, want)
	}
}

func TestEscapeInAnExtensionsFieldGivesThePlainShortcutsBack(t *testing.T) {
	r := newFocusRig()
	r.focus("a", "main")
	r.frame(chord("R", 0))
	if slices.Contains(r.log.sent, "a event refresh") {
		t.Fatalf("a plain shortcut fired while the field held the keyboard: %v", r.log.sent)
	}
	r.frame(chord(key.NameEscape, 0))
	r.frame()
	r.frame(chord("R", 0))
	if !slices.Contains(r.log.sent, "a event refresh") {
		t.Errorf("after Escape the shortcut still waits: %v", r.log.sent)
	}
}

func TestAFocusSentBeforeASurfaceOpensIsDropped(t *testing.T) {
	r := newFocusRig()
	r.u.extension.hide()
	r.u.extensionMessage("a", extension.Focus.New(extension.FocusData{Surface: "main", Key: "q"}))
	r.u.extension.show(extensionKey{"a", "main"})
	r.frame()
	r.frame(key.EditEvent{Text: "x"})
	if slices.Contains(r.log.sent, "a event typed") {
		t.Errorf("a focus from before the surface opened landed: %v", r.log.sent)
	}
}
