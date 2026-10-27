package main

import (
	"context"
	"fmt"
	"image"
	"slices"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// Notes on the page: with the pane's pick on, the pointer stops reaching the
// page; the element under it is outlined and a click opens a note on it.
// Notes collect in a tray under the page, each element pinned where it now
// stands, and reach the session's agent as one message. The page only reports
// what is where: the outline, the pins and the note are the pane's.

var (
	icBrowserPick = icons.Lucide("crosshair")
	icNoteRemove  = icons.Lucide("x")
)

// noteRelocate is how often noted elements are looked up again while their
// pins show: a page scrolls, reflows and animates with no event the pane
// hears. After the person turns the wheel they are looked up every
// noteFollow for noteFollowFor, the length of the page's smooth scroll and
// then some: at the slow pace the outlines trailed the page by a step.
const (
	noteRelocate  = 400 * time.Millisecond
	noteFollow    = 30 * time.Millisecond
	noteFollowFor = 700 * time.Millisecond
)

// browserNote is one note: the element as it was picked, what the person
// wrote, and where the element stands now (empty when it left the page).
type browserNote struct {
	el          api.BrowserElement
	text        string
	box         api.BrowserRect
	row, remove widget.Clickable
	pin         widget.Clickable
}

// browserNotes is one pane's pick and its notes.
type browserNotes struct {
	picking bool
	pick    iconActionState
	// hover is the element under the pointer while picking.
	hover api.BrowserElement
	// draft is the element the note popover is open on; editing is the note
	// it edits, -1 for a new one.
	draft       api.BrowserElement
	editing     int
	open        bool
	focusEditor bool
	// openedAt is when the popover opened, for its animate-in.
	openedAt time.Time
	// trayShown is when the tray appeared, for its animate-in.
	trayShown        time.Time
	editor           widget.Editor
	add, cancel, del widget.Clickable
	clear, send      widget.Clickable
	list             []*browserNote
	rows             layout.List
	found            chan foundElement
	located          chan locatedNotes
	sent             chan []*browserNote
	failed           chan string
	// failure is why the last send failed, shown in the tray until the next.
	failure    string
	nextLocate time.Time
	// reveal is the selector of a note opened from the tray or its pin,
	// which the next lookup scrolls into view first.
	reveal string
	// followUntil is when the fast lookups after a scroll end.
	followUntil time.Time
}

// foundElement is the element a probe found, and whether a click asked for
// it, which opens a note on it.
type foundElement struct {
	el   api.BrowserElement
	open bool
}

// locatedNotes is where the elements selectors find stand; with draft set,
// the last one is the element a new note is open on.
type locatedNotes struct {
	selectors []string
	boxes     []api.BrowserRect
	draft     bool
}

func newBrowserNotes() browserNotes {
	n := browserNotes{found: make(chan foundElement, 4), located: make(chan locatedNotes, 1), sent: make(chan []*browserNote, 4), failed: make(chan string, 1)}
	n.rows.Axis = layout.Vertical
	return n
}

// offer hands v to the frame, dropping the oldest value when the frame has not
// taken the last ones yet: only the newest matters.
func offer[T any](ch chan T, v T) {
	for {
		select {
		case ch <- v:
			return
		default:
		}
		select {
		case <-ch:
		default:
		}
	}
}

// setPicking turns the pick on or off; off drops the outline and an open
// note, keeping the notes.
func (n *browserNotes) setPicking(on bool) {
	n.picking = on
	n.hover = api.BrowserElement{}
	if !on {
		n.open = false
	}
}

// openNote opens the popover on el, for note i or a new one (-1).
func (n *browserNotes) openNote(el api.BrowserElement, i int) {
	if !n.open {
		n.openedAt = time.Now()
	}
	n.draft, n.editing, n.open, n.focusEditor = el, i, true, true
	n.hover = api.BrowserElement{}
	text := ""
	if i >= 0 {
		text = n.list[i].text
	}
	n.editor.SetText(text)
	n.editor.SetCaret(len([]rune(text)), len([]rune(text)))
}

// openListed opens the popover on note i and has the next lookup, right
// away, scroll its element into view: the box the tray knows may stand off
// the page's viewport, where the popover would open far from it.
func (n *browserNotes) openListed(gtx C, i int) {
	el := n.list[i].el
	el.Box = n.list[i].box
	n.openNote(el, i)
	n.reveal, n.nextLocate = el.Selector, time.Time{}
	gtx.Execute(op.InvalidateCmd{})
}

// draftBox is where the element the popover is open on stands now.
func (n *browserNotes) draftBox() api.BrowserRect {
	if n.editing >= 0 && n.editing < len(n.list) {
		return n.list[n.editing].box
	}
	return n.draft.Box
}

// commit keeps the popover's note, replacing the one it edits.
func (n *browserNotes) commit() {
	text := n.editor.Text()
	if n.editing >= 0 && n.editing < len(n.list) {
		n.list[n.editing].text = text
	} else {
		n.list = append(n.list, &browserNote{el: n.draft, text: text, box: n.draft.Box})
	}
	n.open = false
}

// drop removes note i.
func (n *browserNotes) drop(i int) {
	n.list = append(n.list[:i], n.list[i+1:]...)
	n.open = false
}

// probeElement asks for the element under x,y of the page; open asks for the
// note popover on it. Hover probes queued behind one in flight merge.
func (u *window) probeElement(p *browserPaneUI, session string, x, y float64, open bool) {
	merge := "hover"
	if open {
		merge = ""
	}
	p.queue.push(browserCall{merge: merge, send: func(ctx context.Context) error {
		el, err := u.client.BrowserElementAt(ctx, session, x, y)
		if err != nil {
			return fmt.Errorf("element at: %w", err)
		}
		offer(p.notes.found, foundElement{el: el, open: open})
		u.model.invalidate()
		return nil
	}})
}

// relocateNotes looks the noted elements up again, and the one a new note
// is open on, first scrolling the one opened from the tray into view.
func (u *window) relocateNotes(p *browserPaneUI, session string) {
	selectors := make([]string, 0, len(p.notes.list)+1)
	for _, n := range p.notes.list {
		selectors = append(selectors, n.el.Selector)
	}
	draft := p.notes.open && p.notes.editing < 0
	if draft {
		selectors = append(selectors, p.notes.draft.Selector)
	}
	reveal := p.notes.reveal
	p.notes.reveal = ""
	merge := "locate"
	if reveal != "" {
		// The lookups queued behind a reveal must not merge it away.
		merge = ""
	}
	p.queue.push(browserCall{merge: merge, send: func(ctx context.Context) error {
		boxes, err := u.client.BrowserLocate(ctx, session, selectors, reveal)
		if err != nil {
			return fmt.Errorf("locate notes: %w", err)
		}
		offer(p.notes.located, locatedNotes{selectors: selectors, boxes: boxes, draft: draft})
		u.model.invalidate()
		return nil
	}})
}

// takeFound applies what the probes and lookups answered since last frame,
// and looks the noted elements up again when it is time.
func (u *window) takeFound(gtx C, p *browserPaneUI, session string) {
	n := &p.notes
	for done := false; !done; {
		select {
		case f := <-n.found:
			switch {
			case !n.picking || n.open:
			case f.open && f.el.Label != "":
				n.openNote(f.el, -1)
			case !f.open:
				n.hover = f.el
			}
		case l := <-n.located:
			n.applyBoxes(l)
		case sent := <-n.sent:
			n.delivered(sent)
		case why := <-n.failed:
			n.failure = why
		default:
			done = true
		}
	}
	if len(n.list) == 0 && !n.open {
		return
	}
	if !gtx.Now.Before(n.nextLocate) {
		every := noteRelocate
		if gtx.Now.Before(n.followUntil) {
			every = noteFollow
		}
		n.nextLocate = gtx.Now.Add(every)
		u.relocateNotes(p, session)
	}
	gtx.Execute(op.InvalidateCmd{At: n.nextLocate})
}

// applyBoxes moves each pin to where its element stands, unless the notes
// changed since the lookup left.
func (n *browserNotes) applyBoxes(l locatedNotes) {
	if l.draft && len(l.selectors) > 0 && len(l.boxes) == len(l.selectors) {
		last := len(l.selectors) - 1
		if n.open && n.editing < 0 && n.draft.Selector == l.selectors[last] {
			n.draft.Box = l.boxes[last]
		}
		l.selectors, l.boxes = l.selectors[:last], l.boxes[:last]
	}
	if len(l.selectors) != len(n.list) || len(l.boxes) != len(n.list) {
		return
	}
	for i, note := range n.list {
		if note.el.Selector != l.selectors[i] {
			return
		}
	}
	for i, note := range n.list {
		note.box = l.boxes[i]
	}
}

// notesPointer handles the pointer while picking: a move outlines the element
// under it, a press opens a note on it. Nothing reaches the page.
func (u *window) notesPointer(gtx C, p *browserPaneUI, session string, x, y float64, e pointer.Event) {
	n := &p.notes
	switch e.Kind {
	case pointer.Press:
		gtx.Execute(key.FocusCmd{Tag: &p.tag})
		u.layoutStage.focus = browserPaneOf(session)
		if !n.open && e.Buttons.Contain(pointer.ButtonPrimary) {
			u.probeElement(p, session, x, y, true)
		}
	case pointer.Move, pointer.Drag:
		if !n.open {
			u.probeElement(p, session, x, y, false)
		}
	case pointer.Leave:
		n.hover = api.BrowserElement{}
	}
}

// notesKey takes the keys of the pick from the page: Ctrl+Shift+E (Cmd on
// macOS) turns it on and off, Esc closes the note or stops the pick, and
// Ctrl+Enter sends the notes. It reports whether it took e.
func (u *window) notesKey(p *browserPaneUI, session string, e key.Event) bool {
	if e.State != key.Press {
		return false
	}
	n := &p.notes
	switch {
	case e.Name == "E" && e.Modifiers == key.ModShortcut|key.ModShift:
		n.setPicking(!n.picking)
	case e.Name == key.NameEscape && n.picking:
		if n.open {
			n.open = false
		} else {
			n.setPicking(false)
		}
	case (e.Name == key.NameReturn || e.Name == key.NameEnter) && e.Modifiers == key.ModShortcut && len(n.list) > 0:
		u.sendNotes(p, session)
	default:
		return false
	}
	return true
}

// sendNotes types the notes into the session's prompt as one message. They
// leave the tray only once it is there: a send that fails keeps them, to send
// again, and says why in the tray rather than in a toast over the page.
func (u *window) sendNotes(p *browserPaneUI, session string) {
	n := &p.notes
	notes := n.sending()
	message := notesMessage(notes)
	n.setPicking(false)
	label := u.model.sessionLabel(session)
	n.failure = ""
	go func() {
		if err := u.client.Submit(u.ctx, session, message); err != nil {
			offer(n.failed, i18n.T("layouts.browser.notesFailed", i18n.Args{"label": label, "error": backend.ErrorText(err)}))
		} else {
			offer(n.sent, notes)
		}
		u.model.invalidate()
	}()
}

// sending is the notes a send takes: those in the tray now.
func (n *browserNotes) sending() []*browserNote {
	return append([]*browserNote(nil), n.list...)
}

// delivered takes the notes of a sent message out of the tray, keeping any
// added while it was sent.
func (n *browserNotes) delivered(sent []*browserNote) {
	n.list = slices.DeleteFunc(n.list, func(note *browserNote) bool { return slices.Contains(sent, note) })
	if n.open && n.editing >= len(n.list) {
		n.open = false
	}
}

// noteVisible is whether note's element stands on the page the pane shows:
// a pin of a page navigated away from is not drawn.
func noteVisible(note *browserNote, pageURL string) bool {
	return note.box.W > 0 && note.box.H > 0 && note.el.URL == pageURLOf(pageURL)
}

// pageURLOf is url without its query and fragment, as a note records it.
func pageURLOf(url string) string {
	for i, c := range url {
		if c == '?' || c == '#' {
			return url[:i]
		}
	}
	return url
}

// cssRect is box, in CSS pixels, as pane pixels at scale.
func cssRect(box api.BrowserRect, scale float32) image.Rectangle {
	px := func(v int) int { return int(float32(v) * scale) }
	return image.Rect(px(box.X), px(box.Y), px(box.X+box.W), px(box.Y+box.H))
}
