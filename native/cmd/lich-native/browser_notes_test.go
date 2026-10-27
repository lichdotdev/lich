package main

import (
	"math"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/api"
)

func TestNotesMessageNamesEachElementItsSelectorAndTheNote(t *testing.T) {
	notes := []*browserNote{
		{el: api.BrowserElement{Label: `h1 "Simple pricing"`, Selector: "section.hero > h1", Component: "Hero", Source: "src/Hero.tsx:14", HTML: "<h1>Simple pricing</h1>", URL: "http://localhost:5173/pricing"}, text: "tighter,\nit wraps on mobile"},
		{el: api.BrowserElement{Label: `button "Buy"`, Selector: "button.buy", HTML: "<button class=\"buy\">Buy</button>", URL: "http://localhost:5173/pricing"}},
	}
	got := notesMessage(notes)
	want := `[lich] The person left 2 notes on the page in your browser tab:
1. h1 "Simple pricing" on http://localhost:5173/pricing
   section.hero > h1
   React: Hero (src/Hero.tsx:14)
   <h1>Simple pricing</h1>
   Note: tighter, it wraps on mobile
2. button "Buy" on http://localhost:5173/pricing
   button.buy
   <button class="buy">Buy</button>
Each selector above works as the selector of browser_click and browser_type.`
	if got != want {
		t.Fatalf("message:\n%s\nwant:\n%s", got, want)
	}
}

func TestPinsHideOnAnotherPageOrOnceTheElementLeft(t *testing.T) {
	note := &browserNote{el: api.BrowserElement{URL: "http://localhost:5173/pricing"}, box: api.BrowserRect{X: 1, Y: 1, W: 10, H: 10}}
	if !noteVisible(note, "http://localhost:5173/pricing?plan=pro#top") {
		t.Fatal("pin hidden on its own page with a query")
	}
	if noteVisible(note, "http://localhost:5173/docs") {
		t.Fatal("pin shown on another page")
	}
	note.box = api.BrowserRect{}
	if noteVisible(note, "http://localhost:5173/pricing") {
		t.Fatal("pin shown for an element gone from the page")
	}
}

func TestNotesKeepEditsAndDrops(t *testing.T) {
	n := newBrowserNotes()
	n.setPicking(true)
	n.openNote(api.BrowserElement{Label: "a", Selector: "a"}, -1)
	n.editor.SetText("first")
	n.commit()
	n.openNote(api.BrowserElement{Label: "b", Selector: "b"}, -1)
	n.commit()
	n.openNote(n.list[0].el, 0)
	if n.editor.Text() != "first" {
		t.Fatalf("editing note 1 opened with %q", n.editor.Text())
	}
	n.editor.SetText("changed")
	n.commit()
	if len(n.list) != 2 || n.list[0].text != "changed" {
		t.Fatalf("notes after an edit: %+v", n.list)
	}
	n.drop(0)
	if len(n.list) != 1 || n.list[0].el.Selector != "b" || n.open {
		t.Fatalf("notes after dropping the first: %d, open %v", len(n.list), n.open)
	}
	n.setPicking(false)
	if len(n.list) != 1 {
		t.Fatal("stopping the pick dropped the notes")
	}
}

func TestBoxesFromAStaleLookupAreIgnored(t *testing.T) {
	n := newBrowserNotes()
	n.list = []*browserNote{{el: api.BrowserElement{Selector: "a"}}, {el: api.BrowserElement{Selector: "b"}}}
	n.applyBoxes(locatedNotes{selectors: []string{"a"}, boxes: []api.BrowserRect{{W: 5, H: 5}}})
	if n.list[0].box.W != 0 {
		t.Fatal("a lookup made before a note was added moved the pins")
	}
	n.applyBoxes(locatedNotes{selectors: []string{"a", "b"}, boxes: []api.BrowserRect{{W: 5, H: 5}, {W: 7, H: 7}}})
	if n.list[1].box.W != 7 {
		t.Fatalf("pins not moved: %+v", n.list[1].box)
	}
}

func TestNotesStayUntilTheirMessageWasSent(t *testing.T) {
	n := newBrowserNotes()
	a, b := &browserNote{el: api.BrowserElement{Selector: "a"}}, &browserNote{el: api.BrowserElement{Selector: "b"}}
	n.list = []*browserNote{a, b}
	sending := n.sending()
	if len(n.list) != 2 {
		t.Fatal("notes left the tray before their message was sent")
	}
	c := &browserNote{el: api.BrowserElement{Selector: "c"}}
	n.list = append(n.list, c)
	n.delivered(sending)
	if len(n.list) != 1 || n.list[0] != c {
		t.Fatalf("after the send the tray holds %d notes, want only the one added meanwhile", len(n.list))
	}
}

func TestAnOpenNoteFollowsItsElementAsThePageScrolls(t *testing.T) {
	n := newBrowserNotes()
	n.openNote(api.BrowserElement{Selector: "section.chart", Box: api.BrowserRect{Y: 400, W: 600, H: 300}}, -1)
	n.applyBoxes(locatedNotes{selectors: []string{"section.chart"}, boxes: []api.BrowserRect{{Y: 250, W: 600, H: 300}}, draft: true})
	if got := n.draftBox(); got.Y != 250 {
		t.Fatalf("the open note stayed at y %d after the page scrolled its element to 250", got.Y)
	}
}

func TestANoteOpenedFromTheTrayIsRevealedByTheNextLookup(t *testing.T) {
	n := newBrowserNotes()
	n.list = []*browserNote{{el: api.BrowserElement{Selector: "li.far"}, box: api.BrowserRect{Y: 2100, W: 150, H: 40}}}
	n.nextLocate = time.Now().Add(time.Hour)
	n.openListed(layout.Context{Ops: new(op.Ops)}, 0)
	if n.reveal != "li.far" || !n.nextLocate.IsZero() {
		t.Fatalf("reveal %q, next lookup %v: want li.far looked up and scrolled to right away", n.reveal, n.nextLocate)
	}
	if got := n.draftBox(); got.Y != 2100 {
		t.Fatalf("the popover opened at y %d, want the box the tray knew until the lookup answers", got.Y)
	}
	n.applyBoxes(locatedNotes{selectors: []string{"li.far"}, boxes: []api.BrowserRect{{Y: 280, W: 150, H: 40}}})
	if got := n.draftBox(); got.Y != 280 {
		t.Fatalf("the popover stayed at y %d after the reveal scrolled its element to 280", got.Y)
	}
}

func TestANotePickedOnThePageIsNotScrolledTo(t *testing.T) {
	n := newBrowserNotes()
	n.openNote(api.BrowserElement{Label: "a", Selector: "a"}, -1)
	if n.reveal != "" {
		t.Fatalf("a note picked under the pointer asks to reveal %q", n.reveal)
	}
}

func TestTheHoverOutlineGlidesToTheNextElement(t *testing.T) {
	n := newBrowserNotes()
	t0 := time.Now()
	a := api.BrowserRect{X: 0, Y: 0, W: 100, H: 40}
	b := api.BrowserRect{X: 200, Y: 100, W: 300, H: 80}
	n.setHover(api.BrowserElement{Label: "a", Selector: "a", Box: a}, t0)
	if got := n.hoverBox(t0); got != a {
		t.Fatalf("the first element under the pointer is outlined at %+v, want its own box %+v at once", got, a)
	}
	t1 := t0.Add(time.Second)
	n.setHover(api.BrowserElement{Label: "b", Selector: "b", Box: b}, t1)
	if got := n.hoverBox(t1); got != a {
		t.Fatalf("on the move to b the outline jumped to %+v, want it to leave from a %+v", got, a)
	}
	if got := n.hoverBox(t1.Add(noteHoverGlide / 2)); got != (api.BrowserRect{X: 100, Y: 50, W: 200, H: 60}) {
		t.Fatalf("half way the outline stands at %+v, want half way between a and b", got)
	}
	if got := n.hoverBox(t1.Add(noteHoverGlide)); got != b {
		t.Fatalf("after the glide the outline stands at %+v, want b %+v", got, b)
	}
	c := api.BrowserRect{X: 400, Y: 300, W: 10, H: 10}
	n.setHover(api.BrowserElement{Label: "c", Selector: "c", Box: c}, t1.Add(noteHoverGlide/2))
	if got := n.hoverBox(t1.Add(noteHoverGlide / 2)); got != (api.BrowserRect{X: 100, Y: 50, W: 200, H: 60}) {
		t.Fatalf("a move mid glide restarted from %+v, want from where the outline stood", got)
	}
	n.setHover(api.BrowserElement{Label: "c", Selector: "c", Box: c}, t1.Add(noteHoverGlide))
	if got := n.hoverBox(t1.Add(noteHoverGlide * 3 / 2)); got != c {
		t.Fatalf("the probe answering c again restarted the glide: outline at %+v, want c %+v", got, c)
	}
}

func TestTheRingsOutlineWalksItsSidesAndCorners(t *testing.T) {
	r := roundedRect{max: f32.Pt(100, 50), radius: 10}
	if got, want := r.perimeter(), float32(2*80+2*30+2*math.Pi*10); math.Abs(float64(got-want)) > 1e-3 {
		t.Fatalf("perimeter %v, want %v", got, want)
	}
	near := func(p, q f32.Point) bool { return math.Hypot(float64(p.X-q.X), float64(p.Y-q.Y)) < 1e-2 }
	arc := float32(math.Pi / 2 * 10)
	cases := []struct {
		t    float32
		want f32.Point
	}{
		{0, f32.Pt(10, 0)},
		{80, f32.Pt(90, 0)},
		{80 + arc, f32.Pt(100, 10)},
		{80 + arc + 30, f32.Pt(100, 40)},
		{80 + 2*arc + 30, f32.Pt(90, 50)},
		{2*80 + 4*arc + 2*30, f32.Pt(10, 0)},
	}
	for _, c := range cases {
		if got := r.at(c.t); !near(got, c.want) {
			t.Errorf("at(%v) = %v, want %v", c.t, got, c.want)
		}
	}
}
