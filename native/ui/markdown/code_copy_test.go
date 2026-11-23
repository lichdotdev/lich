package markdown

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// copyRig lays a body out, as a window does each frame, on a router the
// pointer is fed to.
type copyRig struct {
	t      *testing.T
	cache  Cache
	src    string
	router input.Router
	ops    op.Ops
	now    time.Time
}

func (r *copyRig) frame() {
	r.ops.Reset()
	gtx := layout.Context{Ops: &r.ops, Now: r.now, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(400, 300)}, Source: r.router.Source()}
	r.cache.Layout(gtx, markdownTheme(), "k", r.src, nil)
	r.router.Frame(&r.ops)
}

func (r *copyRig) pointer(kind pointer.Kind, at f32.Point) {
	ev := pointer.Event{Kind: kind, Source: pointer.Mouse, Position: at}
	if kind == pointer.Press {
		ev.Buttons = pointer.ButtonPrimary
	}
	r.router.Queue(ev)
	r.frame()
}

func (r *copyRig) button() *codeCopy {
	var found *codeCopy
	eachPanel(r.cache["k"].blocks, func(b *markdownBlock) { found = b.copy })
	return found
}

// buttonAt is the middle of the copy button of a panel 400px wide.
func buttonAt() f32.Point {
	side, pad := float32(codeCopySide), float32(codePanelPad)
	return f32.Pt(400-pad-side/2, pad+side/2)
}

func TestACodeBlocksCopyButtonCopiesItsCodeAsWrittenAndShowsACheckForASecondAndAHalf(t *testing.T) {
	r := &copyRig{t: t, cache: Cache{}, src: "```go\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n```\n", now: time.Unix(1, 0)}
	r.frame()
	r.pointer(pointer.Move, buttonAt())
	if !r.button().hovered {
		t.Fatal("the pointer over the code block does not show its copy button")
	}
	r.pointer(pointer.Press, buttonAt())
	r.pointer(pointer.Release, buttonAt())
	_, data, ok := r.router.WriteClipboard()
	if !ok {
		t.Fatal("nothing was put on the clipboard")
	}
	if got, want := string(data), "func main() {\n\tfmt.Println(\"hi\")\n}"; got != want {
		t.Errorf("copied %q, want the block's code as written, %q", got, want)
	}
	c := r.button()
	if !c.done(r.now) {
		t.Fatal("no check right after copying")
	}
	r.now = r.now.Add(time.Second)
	r.frame()
	if !c.done(r.now) {
		t.Error("the check is gone before a second and a half")
	}
	r.now = r.now.Add(time.Second)
	r.frame()
	if c.done(r.now) {
		t.Error("the check stayed past a second and a half")
	}
}

func TestACodeBlocksCopyStateSurvivesTheBodyGrowing(t *testing.T) {
	r := &copyRig{t: t, cache: Cache{}, src: "```go\nx := 1\n```\n\nstreaming", now: time.Unix(1, 0)}
	r.frame()
	r.pointer(pointer.Move, buttonAt())
	before := r.button()
	r.src += " more text"
	r.frame()
	if after := r.button(); after != before || !after.hovered {
		t.Error("a new render of the body lost the code block's hover")
	}
}

func TestTheCopyButtonsRoomHoldsNoTextOfALongFirstLine(t *testing.T) {
	th := markdownTheme()
	img, gtx := nestedPixels(t, "```\n"+strings.Repeat("abcdefghij ", 40)+"\n```")
	side, pad := gtx.Dp(codeCopySide), gtx.Dp(codePanelPad)
	room := image.Rect(img.Bounds().Dx()-pad-side, pad, img.Bounds().Dx()-pad, pad+side)
	fill := codeFill(th)
	for y := room.Min.Y; y < room.Max.Y; y++ {
		for x := room.Min.X; x < room.Max.X; x++ {
			if p := img.RGBAAt(x, y); p.R != fill.R || p.G != fill.G || p.B != fill.B {
				t.Fatalf("pixel (%d,%d) in the copy button's room is %v, want the panel's fill %v: text runs under the button", x, y, p, fill)
			}
		}
	}
}
