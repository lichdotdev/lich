package extrender

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/ui"
)

// rig draws a tree inside a window's overlay layer, as the window does, so
// menus, tooltips and dialogs open for real; it records every event sent.
type rig struct {
	t      *testing.T
	r      *Renderer
	root   ui.Root
	router input.Router
	ops    op.Ops
	now    time.Time
	tree   extension.Node
	got    []extension.EventData
}

var rigWindow = image.Pt(400, 300)

func newRig(t *testing.T, tree extension.Node) *rig {
	g := &rig{t: t, tree: tree, now: time.Unix(0, 0)}
	g.r = New(&g.root, func(e extension.EventData) { g.got = append(g.got, e) })
	g.frame()
	return g
}

func (g *rig) frame() {
	g.ops.Reset()
	gtx := layout.Context{Ops: &g.ops, Now: g.now, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(rigWindow), Source: g.router.Source()}
	g.root.Layout(gtx, func(gtx C) D { return g.r.Layout(gtx, testTheme(), g.tree) })
	g.router.Frame(&g.ops)
}

// shot is the frame just drawn, as the window paints it.
func (g *rig) shot() *image.RGBA {
	g.t.Helper()
	win, err := headless.NewWindow(rigWindow.X, rigWindow.Y)
	if err != nil {
		g.t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	if err := win.Frame(&g.ops); err != nil {
		g.t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: rigWindow})
	if err := win.Screenshot(img); err != nil {
		g.t.Fatal(err)
	}
	return img
}

// wait moves the clock on by d and draws, for a motion or a tooltip delay.
func (g *rig) wait(d time.Duration) {
	g.now = g.now.Add(d)
	g.frame()
}

func (g *rig) click(buttons pointer.Buttons, x, y float32) {
	pos := f32.Pt(x, y)
	g.router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: pos},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: buttons, Position: pos},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: pos},
	)
	g.frame()
}

func (g *rig) press(names ...key.Name) {
	for _, n := range names {
		g.router.Queue(key.Event{Name: n, State: key.Press})
	}
	g.frame()
}

func (g *rig) typed(text string) {
	g.router.Queue(key.EditEvent{Text: text})
	g.frame()
}

// fired is every event sent under h.
func (g *rig) fired(h extension.Handler) []extension.EventData {
	var out []extension.EventData
	for _, e := range g.got {
		if e.Handler == h {
			out = append(out, e)
		}
	}
	return out
}

// once is the one event sent under h, failing when there is not exactly one.
func (g *rig) once(h extension.Handler) extension.EventData {
	g.t.Helper()
	got := g.fired(h)
	if len(got) != 1 {
		g.t.Fatalf("events under %q = %+v, want one (all: %+v)", h, got, g.got)
	}
	return got[0]
}

// drawsWithoutPanic lays tree out a few frames, in a rig and in a renderer
// made with no overlay layer, as a test makes one.
func drawsWithoutPanic(t *testing.T, tree extension.Node) *rig {
	t.Helper()
	bare := New(nil, func(extension.EventData) {})
	router := new(input.Router)
	for range 3 {
		frame(bare, router, tree)
	}
	g := newRig(t, tree)
	g.wait(time.Second)
	return g
}

func label(s string) extension.Node { return extension.Label.New(extension.LabelProps{Text: s}) }

func column(children ...extension.Node) extension.Node {
	return extension.Box.New(extension.BoxProps{}, children...)
}
