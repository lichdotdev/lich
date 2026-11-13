package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
)

func TestNotesMessageNamesEachElementItsSelectorAndTheNote(t *testing.T) {
	notes := []*browserNote{
		{el: api.BrowserElement{Label: `h1 "Simple pricing"`, Selector: "section.hero > h1", Component: "Hero", Source: "src/Hero.tsx:14", HTML: "<h1>Simple pricing</h1>", URL: "http://localhost:5173/pricing"}, text: "tighter,\nit wraps on mobile"},
		{el: api.BrowserElement{Label: `button "Buy"`, Selector: "button.buy", HTML: "<button class=\"buy\">Buy</button>", URL: "http://localhost:5173/pricing"}},
	}
	got := notesMessage(notes, "")
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

func TestTheClosingNoteEndsTheMessage(t *testing.T) {
	notes := []*browserNote{{el: api.BrowserElement{Label: "nav", Selector: "nav", HTML: "<nav></nav>", URL: "http://localhost:5173/"}}}
	got := notesMessage(notes, "  keep it under\nthe header height\n")
	want := `[lich] The person left a note on the page in your browser tab:
1. nav on http://localhost:5173/
   nav
   <nav></nav>
Each selector above works as the selector of browser_click and browser_type.

keep it under
the header height`
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

func TestTheClosingNoteKeepsTheKeysOfAFocusedPane(t *testing.T) {
	u := &window{}
	p := &browserPaneUI{copied: make(chan string, 1), notes: newBrowserNotes()}
	var router input.Router
	shaper := text.NewShaper(text.WithCollection(gofont.Collection()))
	frame := func(before func(C)) {
		ops := new(op.Ops)
		gtx := layout.Context{Ops: ops, Source: router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(600, 400))}
		if before != nil {
			before(gtx)
		}
		u.browserInput(gtx, p, "s", 1, true)
		p.notes.closing.Layout(gtx, shaper, font.Font{}, 12, op.CallOp{}, op.CallOp{})
		router.Frame(ops)
	}
	frame(nil)
	frame(func(gtx C) { gtx.Execute(key.FocusCmd{Tag: &p.notes.closing}) })
	frame(nil)
	if !router.Source().Focused(&p.notes.closing) {
		t.Fatal("the pane took the keys from the closing note")
	}
}
