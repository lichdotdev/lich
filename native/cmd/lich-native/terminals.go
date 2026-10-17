package main

import (
	"context"
	"image"
	"image/color"
	"log"
	"maps"
	"runtime"
	"sync"
	"time"

	"gioui.org/f32"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/gioterm"
	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// termView is one session's terminal: its emulator, a renderer of its own
// (renderers cache per-row state, so they cannot be shared between grids) and
// the input tag that focuses it.
type termView struct {
	id  string
	vt  *gioterm.Terminal
	r   gioterm.Renderer
	tag *int
	// The area size waiting to be fitted, and since when it has held.
	fitFor   image.Point
	fitSince time.Time
	// selecting is a drag that selects, held until its button is let go.
	selecting bool
	// held is the button a tracking program saw pressed.
	held gioterm.MouseButton
	// linkHeld is a press that opened a link, whose release is kept too.
	linkHeld bool
	// pointerAt is where the pointer is over the grid, pointerIn false once
	// it left: the link under it is underlined and takes a hand cursor.
	pointerAt f32.Point
	pointerIn bool
	// exit is set once the process in the PTY is gone, until a restart.
	exit *sessionExit
}

// terminalPadding is term-view.ts TERMINAL_PADDING, in the terminal's own
// background: the grid sits inside it.
var terminalPadding = layout.Inset{Top: 6, Right: 6, Bottom: 6, Left: 10}

// gridArea is the part of gtx's area the grid fits in, inside the padding.
func gridArea(gtx layout.Context) image.Point {
	p := terminalPadding
	return image.Pt(
		max(gtx.Constraints.Max.X-gtx.Dp(p.Left)-gtx.Dp(p.Right), 0),
		max(gtx.Constraints.Max.Y-gtx.Dp(p.Top)-gtx.Dp(p.Bottom), 0))
}

// refitSettle is TerminalView.tsx REFIT_DEBOUNCE_MS: a resize reaches the
// PTY, and every one makes the agent redraw its screen, so a dragged edge
// refits once it stops rather than on every frame of the drag.
const refitSettle = 100 * time.Millisecond

// refitDue reports whether the grid should follow size now, and otherwise
// when to look again: once size has held still for refitSettle.
func (t *termView) refitDue(size image.Point, now time.Time) (bool, time.Time) {
	if size != t.fitFor {
		t.fitFor, t.fitSince = size, now
	}
	at := t.fitSince.Add(refitSettle)
	return !now.Before(at), at
}

// terminals keeps a termView per session opened in this window. The /ws
// stream carries every session's output; a session gets a termView, and from
// then on all its output, the first time it is shown.
type terminals struct {
	client     *lichclient.Client
	stream     *lichclient.Stream
	invalidate func()
	renderer   string
	family     string
	size       unit.Sp
	// unzoomed is the window's own scale, which terminals draw at whatever
	// the interface zoom: a zoom step never changes a grid's cols×rows.
	unzoomed unit.Metric

	mu    sync.Mutex
	byID  map[string]*termView
	shown string
	// colors are the theme's terminal defaults, given to every terminal.
	colors gioterm.Colors
	// background is the shown terminal's default background as last drawn,
	// the color of the island it fills and of the tab merged into it.
	background color.NRGBA
	// typeahead is input for a session whose terminal has not started yet,
	// sent right after Start; the PTY holds it until the shell reads it.
	typeahead map[string]string
	toaster   *ui.Toaster

	// forks are the sessions whose first start branches a conversation, by
	// the conversation: the provider assigns the copy its own id, reported by
	// the session-start hook, so nothing here outlives that start.
	forks map[string]string
	// profileGone takes a session whose Claude Code profile was deleted: its
	// spawn is refused, so a toast in place of the generic failure offers a
	// way on.
	profileGone func(s sessionView)
}

// queue sends text to session id's terminal once it starts.
func (ts *terminals) queue(id, text string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.typeahead == nil {
		ts.typeahead = map[string]string{}
	}
	ts.typeahead[id] += text
}

// queueFork makes session id's first start branch the provider conversation
// parent rather than open a fresh one.
func (ts *terminals) queueFork(id, parent string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.forks == nil {
		ts.forks = map[string]string{}
	}
	ts.forks[id] = parent
}

// takeFork is the conversation session id's start branches, "" for none,
// and forgets it.
func (ts *terminals) takeFork(id string) string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	parent := ts.forks[id]
	delete(ts.forks, id)
	return parent
}

// start spawns s's process: a fork of the conversation queued for it, else
// resume (or a fresh one for "").
func (ts *terminals) start(ctx context.Context, s sessionView, resume string, cols, rows int) error {
	if parent := ts.takeFork(s.ID); parent != "" {
		return ts.client.Fork(ctx, s.ID, s.projectID, s.path, s.Kind, parent, cols, rows)
	}
	return ts.client.Start(ctx, s.ID, s.projectID, s.path, s.Kind, resume, cols, rows)
}

func (ts *terminals) onOutput(id string, data []byte) {
	ts.mu.Lock()
	t, shown := ts.byID[id], ts.shown
	ts.mu.Unlock()
	if t == nil {
		return
	}
	t.vt.Write(data)
	if id == shown {
		ts.invalidate()
	}
}

// openLink opens url in the user's browser, off the frame.
func (ts *terminals) openLink(url string) {
	go func() {
		if err := ts.client.OpenExternal(context.Background(), url); err != nil {
			log.Printf("open link %s: %v", url, err)
		}
	}()
}

func (ts *terminals) send(id string, b []byte) {
	if len(b) == 0 {
		return
	}
	ts.mu.Lock()
	stream := ts.stream
	ts.mu.Unlock()
	if err := stream.Send(context.Background(), id, b); err != nil {
		log.Printf("send to %s: %v", id, err)
	}
}

// has reports whether session id's terminal was opened in this window.
func (ts *terminals) has(id string) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.byID[id] != nil
}

// show returns s's terminal, attaching it on first use the way the web page
// does: the replayed tail first, then Start (a no-op for a running session)
// resuming the conversation resume names, "" for none, then the size. ponytail: output that lands between Replay and registering
// the view is dropped; the page subscribes before replaying instead.
func (ts *terminals) show(ctx context.Context, gtx layout.Context, s sessionView, resume string) (*termView, error) {
	ts.mu.Lock()
	t, previous := ts.byID[s.ID], ts.shown
	ts.shown = s.ID
	ts.mu.Unlock()
	if previous != s.ID {
		go ts.setVisible(previous, s.ID)
	}
	if t != nil {
		return t, nil
	}
	ts.mu.Lock()
	family, size := ts.family, ts.size
	ts.mu.Unlock()
	r, err := gioterm.NewRenderer(ts.renderer, family, size)
	if err != nil {
		return nil, err
	}
	gtx.Metric = ts.unzoomed
	if err := r.Measure(gtx); err != nil {
		return nil, err
	}
	cols, rows := gioterm.Fit(gridArea(gtx), r)
	cellW, cellH := r.Cell()
	vt, err := gioterm.New(cols, rows, cellW, cellH, func(b []byte) { ts.send(s.ID, b) })
	if err != nil {
		return nil, err
	}
	ts.mu.Lock()
	colors := ts.colors
	ts.mu.Unlock()
	if err := vt.SetColors(colors); err != nil {
		return nil, err
	}
	replay, err := ts.client.Replay(ctx, s.ID)
	if err != nil {
		return nil, err
	}
	if len(replay) > 0 {
		vt.Write(replay)
	}
	t = &termView{id: s.ID, vt: vt, r: r, tag: new(int)}
	ts.mu.Lock()
	ts.byID[s.ID] = t
	ts.mu.Unlock()
	// A failed start leaves the terminal in place, empty, as the web's does.
	if err := ts.start(ctx, s, resume, cols, rows); err != nil {
		if lichclient.ProfileGone(err) && ts.profileGone != nil {
			ts.profileGone(s)
			return t, nil
		}
		ts.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("terminal.view.startFailed", i18n.Args{"error": lichclient.ErrorText(err)})})
		return t, nil
	}
	ts.mu.Lock()
	pending := ts.typeahead[s.ID]
	delete(ts.typeahead, s.ID)
	ts.mu.Unlock()
	ts.send(s.ID, []byte(pending))
	return t, ts.client.Resize(ctx, s.ID, cols, rows)
}

// setColors gives every terminal, and every one opened after, a theme's
// default colors. The lock is released before painting: a terminal whose
// scheme flips under mode 2031 reports it through send, which takes the lock.
func (ts *terminals) setColors(c gioterm.Colors) {
	ts.mu.Lock()
	ts.colors = c
	views := maps.Clone(ts.byID)
	ts.mu.Unlock()
	for id, t := range views {
		if err := t.vt.SetColors(c); err != nil {
			log.Printf("terminal colors %s: %v", id, err)
		}
	}
}

// setFont draws every terminal, and every one opened after, in family at
// size. Each gets a renderer of the new face; its next frame measures the
// cells and refits the grid and the PTY to them.
func (ts *terminals) setFont(family string, size unit.Sp) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if family == ts.family && size == ts.size {
		return nil
	}
	for _, t := range ts.byID {
		r, err := gioterm.NewRenderer(ts.renderer, family, size)
		if err != nil {
			return err
		}
		t.r = r
	}
	ts.family, ts.size = family, size
	ts.invalidate()
	return nil
}

// reattach types into stream from now on and frees every terminal: the
// sessions they showed belong to a backend that is gone, and each attaches
// again, replay first, when it is next shown.
func (ts *terminals) reattach(stream *lichclient.Stream) {
	ts.mu.Lock()
	views := ts.byID
	ts.stream, ts.byID, ts.shown = stream, map[string]*termView{}, ""
	ts.mu.Unlock()
	for _, t := range views {
		t.vt.Close()
	}
}

// drop frees a closed session's terminal. Output still in flight for it finds
// no view and is discarded.
func (ts *terminals) drop(id string) {
	ts.mu.Lock()
	t := ts.byID[id]
	delete(ts.byID, id)
	if ts.shown == id {
		ts.shown = ""
	}
	ts.mu.Unlock()
	if t != nil {
		t.vt.Close()
	}
}

func (ts *terminals) setVisible(hidden, shown string) {
	ctx := context.Background()
	if hidden != "" {
		if err := ts.client.SetVisible(ctx, hidden, false); err != nil {
			log.Printf("hide %s: %v", hidden, err)
		}
	}
	if err := ts.client.SetVisible(ctx, shown, true); err != nil {
		log.Printf("show %s: %v", shown, err)
	}
}

// layout draws t into the whole of gtx.Constraints.Max, following the area's
// size with the grid and the backend PTY, and routes keys typed into it.
// layout draws t and types into it. focus says nothing else in the window
// wants the keyboard (a rename, an open menu), so the terminal takes it.
func (ts *terminals) layout(gtx layout.Context, t *termView, focus bool) error {
	gtx.Metric = ts.unzoomed
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	// xterm.js keeps the arrow while the program takes the mouse (tmux, vim).
	if !t.vt.MouseTracking() {
		pointer.CursorText.Add(gtx.Ops)
	}
	defer op.Offset(image.Pt(gtx.Dp(terminalPadding.Left), gtx.Dp(terminalPadding.Top))).Push(gtx.Ops).Pop()
	size := gridArea(gtx)
	gtx.Constraints = layout.Exact(size)
	if err := t.r.Measure(gtx); err != nil {
		return err
	}
	cols, rows := gioterm.Fit(size, t.r)
	// A font change can resize the cells and keep the grid: pointer
	// positions are read in the new cells straight away.
	if cellW, cellH := t.r.Cell(); !sameCell(t.vt, cellW, cellH) {
		c, r := t.vt.Size()
		if err := t.vt.Resize(c, r, cellW, cellH); err != nil {
			return err
		}
	}
	if c, r := t.vt.Size(); c != cols || r != rows {
		if err := ts.refit(gtx, t, size, cols, rows); err != nil {
			return err
		}
	}
	ts.input(gtx, t, focus)
	snap, err := t.vt.Update()
	if err != nil {
		return err
	}
	ts.background = snap.Background()
	if _, err := t.r.Draw(gtx, snap); err != nil {
		return err
	}
	t.drawHoveredLink(gtx, snap.Foreground())
	// Gio reports a pointer in the space of the clip it hit, not of the
	// event.Op: this one, inside the padding, is the grid's own.
	defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, t.tag)
	return nil
}

func sameCell(vt *gioterm.Terminal, w, h int) bool {
	cw, ch := vt.CellSize()
	return cw == w && ch == h
}

// refit follows the area with the grid and the PTY once size has settled,
// and until then asks for the frame that will.
func (ts *terminals) refit(gtx layout.Context, t *termView, size image.Point, cols, rows int) error {
	if due, at := t.refitDue(size, gtx.Now); !due {
		gtx.Execute(op.InvalidateCmd{At: at})
		return nil
	}
	cellW, cellH := t.r.Cell()
	if err := t.vt.Resize(cols, rows, cellW, cellH); err != nil {
		return err
	}
	go func() {
		if err := ts.client.Resize(context.Background(), t.id, cols, rows); err != nil {
			log.Printf("resize %s: %v", t.id, err)
		}
	}()
	return nil
}

func (ts *terminals) input(gtx layout.Context, t *termView, focus bool) {
	filters := append(gioterm.KeyFilters(t.tag),
		pointer.Filter{Target: t.tag, Kinds: pointer.Scroll | pointer.Press | pointer.Release | pointer.Drag | pointer.Move | pointer.Leave,
			ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}},
		key.Filter{Focus: t.tag, Name: "V", Required: key.ModCommand},
		transfer.TargetFilter{Target: t.tag, Type: "application/text"})
	if focus {
		gtx.Execute(key.FocusCmd{Tag: t.tag})
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		switch e := ev.(type) {
		case key.EditEvent:
			ts.send(t.id, []byte(e.Text))
		case key.Event:
			if e.State != key.Press {
				break
			}
			if isPasteChord(e, runtime.GOOS) {
				gtx.Execute(clipboard.ReadCmd{Tag: t.tag})
				break
			}
			ts.send(t.id, terminalKeyBytes(t.vt, e, runtime.GOOS))
		case transfer.DataEvent:
			text, err := readTransfer(e)
			if err != nil {
				log.Printf("paste into %s: %v", t.id, err)
				break
			}
			ts.send(t.id, pastedClipboard(t.vt, text, runtime.GOOS))
		case pointer.Event:
			t.track(e)
			if url, took := t.linkClick(e); took {
				if url != "" {
					ts.openLink(url)
				}
				break
			}
			send, copied := t.pointer(e)
			ts.send(t.id, send)
			if copied != "" {
				ts.copySelection(gtx, copied)
			}
		}
	}
}
