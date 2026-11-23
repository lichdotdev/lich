package markdown

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/x/styledtext"
	"github.com/lichdotdev/godemirror"

	"github.com/lichdotdev/lich/native/ui"
)

// selectRig lays src out 240px wide through a router, a frame per event, so
// a drag reads as a person's: press, moves, release.
type selectRig struct {
	t      *testing.T
	router input.Router
	cache  Cache
	src    string
	opened []string
	paths  *Paths
	at     time.Duration
	theme  *ui.Theme
}

const selectWidth = 240

func newSelectRig(t *testing.T, src string) *selectRig {
	r := &selectRig{t: t, cache: Cache{}, src: src}
	r.frame()
	return r
}

func (r *selectRig) th() *ui.Theme {
	if r.theme == nil {
		r.theme = markdownTheme()
		r.theme.Code = godemirror.DarkTheme
	}
	return r.theme
}

func (r *selectRig) gtx(ops *op.Ops) C {
	return layout.Context{Ops: ops, Now: time.Unix(1, 0), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(selectWidth, 400)}, Source: r.router.Source()}
}

func (r *selectRig) frame() {
	ops := new(op.Ops)
	th := r.th()
	l := Links{Open: func(url string) { r.opened = append(r.opened, url) }, Paths: r.paths}
	r.cache.LayoutLinks(r.gtx(ops), th, "k", r.src, l)
	r.router.Frame(ops)
}

// cell is the n-th selectable cell of the body.
func (r *selectRig) cell(n int) *markdownCell { return r.cache["k"].cells[n] }

// runeAt is the middle of rune i of cell 0's first block, as laid out.
func (r *selectRig) runeAt(i int) f32.Point {
	b := r.geo().boxes[i]
	return f32.Pt(float32(b.x0+b.x1)/2, float32(b.top+b.height/2))
}

func (r *selectRig) geo() *cellGeometry {
	return r.cell(0).geometry(r.gtx(new(op.Ops)), r.th().Shaper)
}

// edgeOf is the left edge of rune i.
func (r *selectRig) edgeOf(i int) f32.Point {
	b := r.geo().boxes[i]
	return f32.Pt(float32(b.x0)+0.5, float32(b.top+b.height/2))
}

func (r *selectRig) send(kind pointer.Kind, at f32.Point, mods key.Modifiers) {
	r.at += 50 * time.Millisecond
	ev := pointer.Event{Kind: kind, Source: pointer.Mouse, Position: at, Time: r.at, Modifiers: mods}
	if kind == pointer.Press || kind == pointer.Move && r.down() {
		ev.Buttons = pointer.ButtonPrimary
	}
	r.router.Queue(ev)
	r.frame()
}

func (r *selectRig) down() bool {
	for _, c := range r.cache["k"].cells {
		if c.sel.pressed {
			return true
		}
	}
	return false
}

// drag presses at from, moves to to in steps and lets go there.
func (r *selectRig) drag(from, to f32.Point) {
	r.send(pointer.Press, from, 0)
	for k := 1; k <= 4; k++ {
		r.send(pointer.Move, from.Add(to.Sub(from).Mul(float32(k)/4)), 0)
	}
	r.send(pointer.Release, to, 0)
}

func (r *selectRig) click(at f32.Point, mods key.Modifiers) {
	r.send(pointer.Press, at, mods)
	r.send(pointer.Release, at, mods)
}

func (r *selectRig) chord(name key.Name, mods key.Modifiers) {
	r.router.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press})
	r.frame()
}

// copied is what Ctrl+C puts on the clipboard, "" for nothing.
func (r *selectRig) copied() string {
	r.chord("C", key.ModShortcut)
	_, data, ok := r.router.WriteClipboard()
	if !ok {
		return ""
	}
	return string(data)
}

func (r *selectRig) selected(n int) string {
	c := r.cell(n)
	from, to := c.sel.span()
	return c.visibleText(from, to)
}

// runeIndex is the rune offset of sub in s.
func runeIndex(s, sub string) int {
	return utf8.RuneCountInString(s[:strings.Index(s, sub)])
}

// styledFrom is cell drawn as richtext draws it, without the link state.
func styledFrom(th *ui.Theme, cell *markdownCell) styledtext.TextStyle {
	spans := make([]styledtext.SpanStyle, len(cell.spans))
	for i, s := range cell.spans {
		spans[i] = styledtext.SpanStyle{Font: s.Font, Size: s.Size, Color: s.Color, Content: s.Content}
	}
	style := styledtext.Text(th.Shaper, spans...)
	style.LineHeightScale = cell.lineHeightScale()
	return style
}

const wrapped = "Run **the bold part** and `go test ./...` before the long tail of this paragraph wraps"

func TestEveryRuneOfACellHasABox(t *testing.T) {
	for _, src := range []string{
		wrapped,
		"one line",
		"a hard  \nbreak and *italics* then a [link](https://example.com) on it",
		"averyveryverylongwordthatcannotfitonalineofthiswidthatall and more",
		"```\nfunc main() {\n\tprintln(1)\n}\n```",
		"| a | b |\n|---|---|\n| `x` | **y** |",
	} {
		r := newSelectRig(t, src)
		for i, c := range r.cache["k"].cells {
			g := c.geometry(r.gtx(new(op.Ops)), r.th().Shaper)
			if got, want := len(g.boxes), utf8.RuneCountInString(c.cellText()); got != want {
				t.Errorf("%q cell %d: %d boxes for %d runes", src, i, got, want)
			}
		}
	}
}

// The geometry is styledtext's layout again: every rune with ink has its ink
// inside its box, and no ink falls outside the boxes.
func TestTheGeometryLaysOutAsStyledtextDraws(t *testing.T) {
	size := image.Pt(selectWidth, 200)
	win, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	th := markdownTheme()
	cell := renderBlocks(th, wrapped)[0].prose
	for i := range cell.spans {
		cell.spans[i].Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: size}}
	paint.Fill(ops, color.NRGBA{A: 255})
	styledFrom(th, cell).Layout(gtx, nil)
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	g := cell.geometry(gtx, th.Shaper)
	ink := func(r image.Rectangle) bool {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				if img.RGBAAt(x, y).R > 128 {
					return true
				}
			}
		}
		return false
	}
	runes := []rune(cell.cellText())
	lines := map[int]bool{}
	for i, b := range g.boxes {
		lines[b.top] = true
		if unicode.IsSpace(runes[i]) {
			continue
		}
		if !ink(image.Rect(b.x0, b.top, b.x1, b.top+b.height)) {
			t.Errorf("rune %d %q: no ink in its box %v", i, runes[i], b)
		}
	}
	if len(lines) < 3 {
		t.Fatalf("the paragraph laid out on %d lines, want it wrapped", len(lines))
	}
	for y := range size.Y {
		for x := range size.X {
			if img.RGBAAt(x, y).R <= 128 {
				continue
			}
			inside := false
			for _, b := range g.boxes {
				if image.Pt(x, y).In(image.Rect(b.x0-1, b.top, b.x1+1, b.top+b.height)) {
					inside = true
					break
				}
			}
			if !inside {
				t.Fatalf("ink at (%d,%d) outside every rune's box", x, y)
			}
		}
	}
}

func TestAPointHitsTheRuneItIsOver(t *testing.T) {
	r := newSelectRig(t, wrapped)
	r.click(r.runeAt(0), 0)
	g := r.geo()
	runes := []rune(r.cell(0).cellText())
	for i := range g.boxes {
		if runes[i] == '\n' || g.boxes[i].x1-g.boxes[i].x0 < 2 {
			continue
		}
		if got := g.hit(r.runeAt(i), true); got != i {
			t.Errorf("the middle of rune %d %q hits %d", i, runes[i], got)
		}
		if got := g.hit(r.edgeOf(i), false); got != i {
			t.Errorf("the left edge of rune %d %q hits boundary %d", i, runes[i], got)
		}
	}
}

func TestADragSelectsFromThePressAcrossWrappedLinesAndSpans(t *testing.T) {
	r := newSelectRig(t, wrapped)
	r.click(r.runeAt(0), 0)
	text := r.cell(0).cellText()
	from := runeIndex(text, "bold")
	to := runeIndex(text, "tail")
	if r.geo().boxes[from].top == r.geo().boxes[to].top {
		t.Fatal("the drag does not cross a wrapped line")
	}
	r.drag(r.edgeOf(from), r.edgeOf(to))
	if got, want := r.selected(0), "bold part and go test ./... before the long "; got != want {
		t.Errorf("selected %q, want %q", got, want)
	}
	if got := r.copied(); got != "bold part and go test ./... before the long " {
		t.Errorf("Ctrl+C copied %q", got)
	}
	r.drag(r.edgeOf(to), r.edgeOf(from))
	if got, want := r.selected(0), "bold part and go test ./... before the long "; got != want {
		t.Errorf("dragged back, selected %q, want %q", got, want)
	}
}

func TestADoubleClickSelectsAWordAndATripleClickTheCell(t *testing.T) {
	r := newSelectRig(t, wrapped)
	r.click(r.runeAt(0), 0)
	at := r.runeAt(runeIndex(r.cell(0).cellText(), "paragraph") + 2)
	r.click(at, 0)
	r.click(at, 0)
	if got := r.selected(0); got != "paragraph" {
		t.Errorf("a double click selected %q, want paragraph", got)
	}
	r.click(at, 0)
	if got, want := r.selected(0), "Run the bold part and go test ./... before the long tail of this paragraph wraps"; got != want {
		t.Errorf("a triple click selected %q, want the cell's text %q", got, want)
	}
	r.at += time.Second
	r.click(at, 0)
	if got := r.selected(0); got != "" {
		t.Errorf("a click a second later left %q selected", got)
	}
}

// The clipboard gets what is read: no room around inline code nor before a
// link's mark, and a link's words, not its address.
func TestCopyingLeavesTheRendersRoomOut(t *testing.T) {
	r := newSelectRig(t, "Use `x` and see [the repo](https://github.com/a/b) or [docs](https://example.com).")
	at := r.runeAt(0)
	r.click(at, 0)
	r.click(at, 0)
	r.click(at, 0)
	if got, want := r.copied(), "Use x and see the repo or docs."; got != want {
		t.Errorf("copied %q, want %q", got, want)
	}
	if text := r.cell(0).cellText(); !strings.Contains(text, " x ") || !strings.Contains(text, linkIconRoom+"the") {
		t.Fatalf("the cell %q has no room to leave out", text)
	}
}

func TestALinkOpensOnAClickAndADragOverItSelects(t *testing.T) {
	r := newSelectRig(t, "Read [the docs](https://example.com/docs) first")
	r.click(r.runeAt(0), 0)
	text := r.cell(0).cellText()
	on := runeIndex(text, "docs")
	r.click(r.runeAt(on), 0)
	if len(r.opened) != 1 || r.opened[0] != "https://example.com/docs" {
		t.Fatalf("a click on the link opened %q", r.opened)
	}
	r.drag(r.edgeOf(on-4), r.edgeOf(runeIndex(text, "first")))
	if len(r.opened) != 1 {
		t.Errorf("a drag over the link opened it: %q", r.opened)
	}
	if got := r.selected(0); got != "the docs " {
		t.Errorf("the drag selected %q, want %q", got, "the docs ")
	}
	r.send(pointer.Move, r.runeAt(on), 0)
	if got := r.router.Cursor(); got != pointer.CursorPointer {
		t.Errorf("cursor over the link %v, want the hand", got)
	}
	r.send(pointer.Move, r.runeAt(0), 0)
	if got := r.router.Cursor(); got != pointer.CursorText {
		t.Errorf("cursor over prose %v, want the text cursor", got)
	}
}

// A Ctrl+drag that starts on a path selects; only a Ctrl+click opens it.
func TestACtrlDragFromAPathOpensNothing(t *testing.T) {
	r := &selectRig{t: t, cache: Cache{}, src: "Read `app.go` now please"}
	var opened []string
	r.paths = &Paths{Link: func(string) bool { return true }, Open: func(p string) { opened = append(opened, p) }, Held: true}
	r.frame()
	r.click(r.runeAt(0), 0)
	on := r.runeAt(runeIndex(r.cell(0).cellText(), "app") + 1)
	r.send(pointer.Press, on, key.ModShortcut)
	r.send(pointer.Move, on.Add(f32.Pt(40, 0)), key.ModShortcut)
	r.send(pointer.Release, on.Add(f32.Pt(40, 0)), key.ModShortcut)
	if len(opened) != 0 {
		t.Errorf("a Ctrl+drag from the path opened %q", opened)
	}
	if r.selected(0) == "" {
		t.Error("the Ctrl+drag selected nothing")
	}
	r.at += time.Second
	r.click(on, key.ModShortcut)
	if len(opened) != 1 || opened[0] != "app.go" {
		t.Errorf("a Ctrl+click opened %q, want app.go", opened)
	}
}

func TestEscapeAndAPressElsewhereDropTheSelection(t *testing.T) {
	r := newSelectRig(t, wrapped+"\n\nA second paragraph.")
	r.click(r.runeAt(0), 0)
	r.drag(r.edgeOf(0), r.edgeOf(10))
	if r.selected(0) == "" {
		t.Fatal("nothing selected")
	}
	r.chord(key.NameEscape, 0)
	if got := r.selected(0); got != "" {
		t.Errorf("Escape left %q selected", got)
	}
	r.drag(r.edgeOf(0), r.edgeOf(10))
	r.click(f32.Pt(10, 390), 0)
	if got := r.selected(0); got != "" {
		t.Errorf("a press under the text left %q selected", got)
	}
	if got := r.copied(); got != "" {
		t.Errorf("Ctrl+C with nothing selected copied %q", got)
	}
}

func TestASelectionSurvivesARenderUnlessItsTextChanged(t *testing.T) {
	r := newSelectRig(t, "First paragraph stays.\n\nSecond one grows")
	r.click(r.runeAt(0), 0)
	r.drag(r.edgeOf(0), r.edgeOf(5))
	r.src += " as it streams"
	r.frame()
	if got := r.selected(0); got != "First" {
		t.Errorf("after the answer grew, selected %q, want First", got)
	}
	r.src = "First paragraph changed.\n\nSecond one grows"
	r.frame()
	if got := r.selected(0); got != "First" {
		t.Errorf("a change past the selection dropped it: %q", got)
	}
	r.src = "Fist paragraph changed.\n\nSecond one grows"
	r.frame()
	if got := r.selected(0); got != "" {
		t.Errorf("the selected text changed and %q stayed selected", got)
	}
}

// A drag goes on while the answer under it streams.
func TestADragSurvivesTheAnswerGrowing(t *testing.T) {
	r := newSelectRig(t, wrapped)
	r.click(r.runeAt(0), 0)
	r.send(pointer.Press, r.edgeOf(0), 0)
	r.send(pointer.Move, r.edgeOf(3), 0)
	r.src += " further"
	r.frame()
	r.send(pointer.Move, r.edgeOf(8), 0)
	r.send(pointer.Release, r.edgeOf(8), 0)
	if got := r.selected(0); got != "Run the " {
		t.Errorf("selected %q, want %q", got, "Run the ")
	}
}

const acrossCells = "First paragraph here.\n\n- one item\n- two item\n\nLast paragraph."

// bodyHeight is how tall the rig's body stands.
func (r *selectRig) bodyHeight() float32 {
	l := Links{Paths: r.paths}
	return float32(r.cache.LayoutLinks(r.gtx(new(op.Ops)), r.th(), "k", r.src, l).Size.Y)
}

func TestADragRunsOnAcrossParagraphsAndListItems(t *testing.T) {
	r := newSelectRig(t, acrossCells)
	r.drag(r.edgeOf(6), f32.Pt(40, r.bodyHeight()-2))
	want := []string{"paragraph here.", "one item", "two item"}
	for i, w := range want {
		if got := r.selected(i); got != w {
			t.Errorf("cell %d: selected %q, want %q", i, got, w)
		}
	}
	if got := r.selected(3); got == "" || !strings.HasPrefix("Last paragraph.", got) {
		t.Errorf("last cell: selected %q, want the start of Last paragraph.", got)
	}
	if got := r.copied(); !strings.HasPrefix(got, "paragraph here.\none item\ntwo item\nLast") {
		t.Errorf("Ctrl+C copied %q", got)
	}
}

func TestADragUpwardRunsOnAcrossCellsToo(t *testing.T) {
	r := newSelectRig(t, acrossCells)
	r.drag(f32.Pt(40, r.bodyHeight()-2), r.edgeOf(6))
	if got := r.selected(0); got != "paragraph here." {
		t.Errorf("first cell: selected %q", got)
	}
	for i, w := range []string{"one item", "two item"} {
		if got := r.selected(i + 1); got != w {
			t.Errorf("cell %d: selected %q, want %q", i+1, got, w)
		}
	}
	if got := r.selected(3); got == "" || !strings.HasPrefix("Last paragraph.", got) {
		t.Errorf("last cell: selected %q, want the start of Last paragraph., up to the press", got)
	}
}

func TestADragBackIntoTheFirstCellLetsTheOthersGo(t *testing.T) {
	r := newSelectRig(t, acrossCells)
	r.send(pointer.Press, r.edgeOf(0), 0)
	r.send(pointer.Move, f32.Pt(40, r.bodyHeight()-2), 0)
	r.send(pointer.Move, r.edgeOf(5), 0)
	r.send(pointer.Release, r.edgeOf(5), 0)
	if got := r.copied(); got != "First" {
		t.Errorf("Ctrl+C copied %q, want First", got)
	}
}
