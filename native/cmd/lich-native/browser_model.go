package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"slices"
	"sync"
	"time"

	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// The model's store of session browsers: which sessions have one and what it
// shows, fed by browser.Running once and browser-state after; the page's last
// frame; and when the agent last acted on it.

// browserFrame is a page's last screencast frame, ready to draw.
type browserFrame struct {
	img  paint.ImageOp
	size image.Point
	// seq orders the frames of a page: decoders finish out of order.
	seq uint64
}

// pendingFrame is a page's newest frame not yet decoded.
type pendingFrame struct {
	ev  backend.Event
	seq uint64
}

// browserMark is one agent action on its page: which one, where, and for a
// scroll how far (positive down). seq orders the marks of one store.
type browserMark struct {
	seq    uint64
	action string
	dy     int
	url    string
	tab    string
	at     time.Time
	x, y   *float64
}

// agentMarksKept is how many of a session's latest agent actions the store
// keeps for its pane: an agent that calls tools in parallel sends several
// between two frames, and the cursor plays them in turn.
const agentMarksKept = 8

type browserStore struct {
	states map[string]api.BrowserState
	frames map[string]browserFrame
	// pending is each page's newest frame not yet decoded (decodeFrames),
	// under its own lock: the /events loop never waits on m.mu for a frame.
	pendingMu sync.Mutex
	pending   map[string]pendingFrame
	frameSeq  uint64
	agent     map[string][]browserMark
	seq       uint64
}

func newBrowserStore() browserStore {
	return browserStore{states: map[string]api.BrowserState{}, frames: map[string]browserFrame{}, pending: map[string]pendingFrame{}, agent: map[string][]browserMark{}}
}

// hydrateBrowsers reads the browsers running now, and takes out of the
// layouts every page whose browser did not survive (a backend restarted, or
// a browser moved to its own window while this one was away).
func (m *model) hydrateBrowsers(ctx context.Context) error {
	running, err := m.client.RunningBrowsers(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	for _, st := range running {
		m.browsers.states[st.Owner] = st
	}
	idle := m.browserIdle
	pruned := false
	for _, t := range append([]layoutTab(nil), m.layouts.tabs...) {
		if owner, ok := t.browserOf(); ok && !m.browserShowableLocked(owner) {
			pruned = m.layouts.dropBrowser(owner) || pruned
		}
	}
	m.mu.Unlock()
	if pruned {
		m.saveLayouts()
	}
	for _, st := range running {
		if st.Idle && idle != nil {
			idle(st)
		}
	}
	m.invalidate()
	return nil
}

// browserShowableLocked reports whether session's browser can be drawn in a
// pane: it runs, and not in its own window. Callers hold m.mu.
func (m *model) browserShowableLocked(session string) bool {
	st, ok := m.browsers.states[session]
	return ok && st.Open && !st.Headed
}

// browserState is session's browser, false for none.
func (m *model) browserState(session string) (api.BrowserState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.browsers.states[session]
	return st, ok
}

// applyBrowserState takes a browser-state event: a page that can no longer
// be drawn leaves its tab, a browser the agent just started is offered to
// the person (browserOpened), and the question about an idle one goes up or
// down (browserIdle).
func (m *model) applyBrowserState(ctx context.Context, st api.BrowserState) {
	m.mu.Lock()
	prev, known := m.browsers.states[st.Owner]
	idle := m.browserIdle
	if st.Open {
		m.browsers.states[st.Owner] = st
	} else {
		delete(m.browsers.states, st.Owner)
		delete(m.browsers.frames, st.Owner)
		m.browsers.pendingMu.Lock()
		delete(m.browsers.pending, st.Owner)
		m.browsers.pendingMu.Unlock()
		delete(m.browsers.agent, st.Owner)
	}
	shownTab := ""
	if i := m.layouts.browserTab(st.Owner); i >= 0 && m.shownLayout == m.layouts.tabs[i].ID {
		shownTab = m.shownLayout
	}
	dropped := !m.browserShowableLocked(st.Owner) && m.layouts.dropBrowser(st.Owner)
	m.mu.Unlock()
	if dropped {
		m.saveLayouts()
		if shownTab != "" {
			m.setActive(ctx, st.Owner)
		}
	}
	if st.Open && st.ByAgent && !known && m.browserOpened != nil {
		m.browserOpened(st)
	}
	if prev.Idle != (st.Open && st.Idle) && idle != nil {
		idle(st)
	}
}

// frameDecoders is how many frames the window decodes at once. A busy page's
// 1280x720 frame takes 15 ms to decode (measured 2026-10-29), so one decoder
// only just keeps up with 60 fps; four leave room for a slower CPU.
const frameDecoders = 4

// offerBrowserFrame hands owner's newest frame to decodeFrames, replacing one
// no decoder has taken yet.
func (m *model) offerBrowserFrame(owner string, ev backend.Event, kick chan<- struct{}) {
	m.browsers.pendingMu.Lock()
	m.browsers.frameSeq++
	m.browsers.pending[owner] = pendingFrame{ev: ev, seq: m.browsers.frameSeq}
	m.browsers.pendingMu.Unlock()
	select {
	case kick <- struct{}{}:
	default:
	}
}

// takeBrowserFrame takes one page's pending frame, false for none.
func (m *model) takeBrowserFrame() (string, pendingFrame, bool) {
	m.browsers.pendingMu.Lock()
	defer m.browsers.pendingMu.Unlock()
	for owner, f := range m.browsers.pending {
		delete(m.browsers.pending, owner)
		return owner, f, true
	}
	return "", pendingFrame{}, false
}

// decodeFrames decodes, off the /events loop and frameDecoders at a time,
// only each page's newest frame, until kick closes; then it closes the
// returned channel. A page animating at 60 fps sends frames faster than one
// decode takes, and decoding each in turn queued them for seconds, and every
// other event behind them. kick holds a wake per decoder.
func (m *model) decodeFrames(kick <-chan struct{}) <-chan struct{} {
	var wg sync.WaitGroup
	for range frameDecoders {
		wg.Go(func() {
			for range kick {
				for owner, f, ok := m.takeBrowserFrame(); ok; owner, f, ok = m.takeBrowserFrame() {
					if err := m.applyBrowserFrame(owner, f.ev, f.seq); err != nil {
						log.Print(err)
					}
					m.invalidate()
				}
			}
		})
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	return done
}

// applyBrowserFrame decodes one frame of owner's page off the frame
// goroutine, so the pane only draws it, and keeps it unless a newer one
// landed first.
func (m *model) applyBrowserFrame(owner string, ev backend.Event, seq uint64) error {
	data, err := backend.Decode(ev, api.BrowserFrameEvents.Of(owner))
	if err != nil {
		return err
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return fmt.Errorf("%s: %w", ev.Name, err)
	}
	img, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("%s: %w", ev.Name, err)
	}
	frame := browserFrame{img: paint.NewImageOp(img), size: img.Bounds().Size(), seq: seq}
	m.mu.Lock()
	if frame.seq > m.browsers.frames[owner].seq {
		m.browsers.frames[owner] = frame
	}
	m.mu.Unlock()
	return nil
}

func (m *model) applyBrowserAgent(a api.BrowserAgentAct, now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.browsers.seq++
	mark := browserMark{seq: m.browsers.seq, at: now, x: a.X, y: a.Y, action: a.Action, dy: a.DY, url: a.URL, tab: a.Tab}
	marks := append(m.browsers.agent[a.Owner], mark)
	m.browsers.agent[a.Owner] = marks[max(0, len(marks)-agentMarksKept):]
}

// browserView is what session's page pane draws: its state, its last frame
// and the agent's latest actions, oldest first.
func (m *model) browserView(session string) (api.BrowserState, browserFrame, []browserMark) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.browsers.states[session], m.browsers.frames[session], slices.Clone(m.browsers.agent[session])
}

// openBrowserTab puts session's browser tab on screen, making it when there
// is none. A session in another layout stays there unless move is set; it
// reports false when nothing opened.
func (m *model) openBrowserTab(ctx context.Context, session string, move bool) bool {
	m.mu.Lock()
	if !m.liveLocked(session) {
		m.mu.Unlock()
		return false
	}
	if i := m.layouts.browserTab(session); i >= 0 {
		m.mu.Unlock()
		m.focusPane(ctx, browserPaneOf(session))
		return true
	}
	if m.layouts.of(session) >= 0 {
		if !move {
			m.mu.Unlock()
			return false
		}
		m.layouts.take(session)
	}
	p, i := m.sessionAt(session)
	m.layouts.openBrowser(p.Sessions[i].Label, session)
	m.mu.Unlock()
	m.setActive(ctx, session)
	m.focusPane(ctx, browserPaneOf(session))
	m.saveLayouts()
	return true
}
