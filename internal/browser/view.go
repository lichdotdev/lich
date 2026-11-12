package browser

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// inputWait bounds one of the pane's input events. Chrome answers one in
// ~15ms, but an event dispatched while a navigation swaps the document can go
// unanswered (as Puppeteer has seen); the pane's queue waits on it, so it
// must give up long before pageWait.
const inputWait = 2 * time.Second

// errInWindow: the pane draws a headless browser, and this one moved to its
// own window, where the person already sees it.
var errInWindow = errors.New("this browser is open in its own window")

// Attach shows this session's browser in a pane sized v, starting a headless
// one at about:blank when there is none. The page the agent left stays as it
// is: the pane only starts watching it.
func (s *Service) Attach(owner string, v Viewport) (Handle, error) {
	if owner == "" {
		return Handle{}, errNoOwner
	}
	// A browser drawing at another scale than the window's would show the
	// pane blurred frames.
	if _, err := s.rescale(owner); err != nil {
		return Handle{}, err
	}
	lock := s.ownerLock(owner)
	lock.Lock()
	defer lock.Unlock()
	if s.current(owner) == nil {
		if _, err := s.launchFresh(owner, "about:blank", false, false); err != nil {
			return Handle{}, err
		}
	}
	sess := s.current(owner)
	if sess == nil {
		return Handle{}, errNoBrowser
	}
	if sess.headed {
		return Handle{}, errInWindow
	}
	s.mu.Lock()
	sess.attaches++
	s.mu.Unlock()
	if err := runPage(func(ctx context.Context) error { return sess.page.Screencast(ctx, v) }); err != nil {
		return Handle{}, s.failed(sess, err)
	}
	s.mu.Lock()
	sess.watching, sess.view = true, v
	s.mu.Unlock()
	return s.handleOf(sess), nil
}

// Detach stops the frames of a pane that closed or went off screen. The
// browser keeps running for the agent, at the window's size again rather than
// the pane's. A browser in its own window is left alone: an override there
// pins the page and the window's own resizing stops laying it out.
func (s *Service) Detach(owner string) error {
	if s.inWindow(owner) {
		return nil
	}
	view := s.unwatchedView()
	sess, err := s.steer(owner, inputWait, func(p Page, ctx context.Context) error {
		if err := p.StopScreencast(ctx); err != nil || view == (Viewport{}) {
			return err
		}
		return p.Resize(ctx, view)
	})
	if err != nil {
		return err
	}
	s.mu.Lock()
	sess.watching = false
	s.mu.Unlock()
	return nil
}

// Resize lays the page out again at the pane's new size. A pane showing it
// gets its frames at that size too: the screencast keeps the size it started
// at, and the window would stretch the smaller frames over the larger pane.
func (s *Service) Resize(owner string, v Viewport) error {
	sess, err := s.live(owner)
	if err != nil {
		return err
	}
	s.mu.Lock()
	watching, headed := sess.watching, sess.headed
	s.mu.Unlock()
	if headed {
		return nil
	}
	_, err = s.steer(owner, inputWait, func(p Page, ctx context.Context) error {
		if watching {
			return p.Screencast(ctx, v)
		}
		return p.Resize(ctx, v)
	})
	if err == nil && watching {
		s.mu.Lock()
		sess.view = v
		s.mu.Unlock()
	}
	return err
}

// SetDefaultView sets the size a headless page no pane shows lays out at, and
// lays out the running ones there. It is not a use: a window resized does not
// keep an idle browser from the reaper. window is the window's own scale: a
// pane's browser launched under another relaunches once it holds still.
func (s *Service) SetDefaultView(v Viewport, window float64) {
	s.mu.Lock()
	s.defaultView, s.window = v, window
	s.scheduleRescaleLocked()
	var pages []Page
	for _, sess := range s.byID {
		if sess.alive() && !sess.headed && !sess.watching {
			pages = append(pages, sess.page)
		}
	}
	s.mu.Unlock()
	for _, p := range pages {
		ctx, cancel := context.WithTimeout(context.Background(), inputWait)
		if err := p.Resize(ctx, v); err != nil && !errors.Is(err, errDialogOpen) {
			slog.Warn("browser: lay a page out at the window's size", "err", err)
		}
		cancel()
	}
}

func (s *Service) inWindow(owner string) bool {
	s.mu.Lock()
	defer s.unlock()
	sess := s.lookupLocked(owner)
	return sess != nil && sess.headed
}

func (s *Service) unwatchedView() Viewport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.defaultView
}

// Nav runs the pane's toolbar: back, forward, reload, or go to url.
func (s *Service) Nav(owner, action, rawURL string) error {
	var fn func(Page, context.Context) error
	switch action {
	case "back":
		fn = func(p Page, ctx context.Context) error { return p.Back(ctx) }
	case "forward":
		fn = func(p Page, ctx context.Context) error { return p.Forward(ctx) }
	case "reload":
		fn = func(p Page, ctx context.Context) error { return p.Reload(ctx) }
	case "go":
		if err := s.checkURL(rawURL); err != nil {
			return err
		}
		sess, err := s.live(owner)
		if err != nil {
			return err
		}
		u, err := s.resolveURL(sess.owner, rawURL)
		if err != nil {
			return err
		}
		fn = func(_ Page, ctx context.Context) error {
			_, err := s.navigate(ctx, sess, u)
			return err
		}
	default:
		return fmt.Errorf("unknown browser action %q", action)
	}
	_, err := s.steer(owner, pageWait, fn)
	return err
}

// steer runs one of the window's own calls on owner's page, bounded by wait.
// Unlike an agent action, a call that fails leaves the browser running
// unless its process is gone: the person is still looking at the page. A
// call a dialog or the debugger held off does nothing and is no error: the
// pane shows the dialog, DevTools the pause, and answering or resuming is what
// the person can do.
func (s *Service) steer(owner string, wait time.Duration, fn func(Page, context.Context) error) (*session, error) {
	sess, err := s.onPage(owner, wait, fn)
	if errors.Is(err, errDialogOpen) || errors.Is(err, errPaused) {
		return sess, nil
	}
	return sess, err
}

// onPage runs fn on owner's page, bounded by wait, and keeps the browser
// when it fails, unless its process is gone.
func (s *Service) onPage(owner string, wait time.Duration, fn func(Page, context.Context) error) (*session, error) {
	sess, err := s.live(owner)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	if err := fn(sess.page, ctx); err != nil {
		return sess, s.failed(sess, err)
	}
	s.touch(sess)
	return sess, nil
}

// Running is every browser this process runs, for a window catching up.
func (s *Service) Running() []api.BrowserState {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []api.BrowserState{}
	for _, sess := range s.byID {
		if sess.alive() {
			out = append(out, sess.stateLocked(true))
		}
	}
	return out
}

// maxFrameData bounds a frame sent to the window. Its /events socket reads at
// most api.MaxEventBytes a message and one message over it ends that socket
// for every session, so this leaves room for the envelope. A 3440x1440 pane of
// a page drawing noise sent frames of 4.5 MiB (measured 2026-11-11).
const maxFrameData = api.MaxEventBytes - 1<<10

// stillAfter is how long a page sends no frame before its pane gets one PNG
// of it: a JPEG's ringing around text stays on screen for as long as the page
// is still. Past a caret's 500 ms blink, which Still refuses outright too
// (errCaret): under it, each blink was captured, two PNGs a second for 17% of
// a core, and the text flickered between sharp and blurred (Chromium 157,
// measured 2026-11-12).
const stillAfter = 600 * time.Millisecond

// stillWait bounds a still capture: 30 ms for text at 1280x720, 200 ms for a
// photo at 2880x1620 (measured 2026-11-12).
const stillWait = 2 * time.Second

// stillGrowth is how many times the JPEG it replaces a still PNG may weigh. Text
// came to 0.8 to 1.1 times its JPEG and a photo to 9 to 11 times, no sharper
// for it: 0.37% of its colour values moved by more than 16, where text has
// its ringing (Chromium 157, 1280x720 and 2880x1620, measured 2026-11-12).
const stillGrowth = 3

// stillEcho is how long after a capture a frame is still taken for the one
// the capture itself draws, which Chrome on Linux sends 8 to 21 ms into it
// (Chromium 157, measured 2026-11-12). On Apple silicon that frame is not the
// one before it byte for byte, so it went for a change and started the settle
// over, a capture every stillAfter for as long as the pane was open (CI,
// macos-latest, 2026-11-12).
const stillEcho = 250 * time.Millisecond

// stillTries is how many captures one still page gets. The second is for a
// pane drawn under the browser's scale, where the first capture's frame is a
// pixel narrower than the one before it and the ones after it are equal.
const stillTries = 2

// frameSink sends the window sess's frames, except one too large for its
// socket: the pane keeps its last frame until one fits. Once the page is
// still for stillAfter, it sends one PNG of it too.
func (s *Service) frameSink(sess *session) func(tab, data string) {
	f := &frameStream{s: s, sess: sess, frames: api.BrowserFrameEvents.Of(sess.owner)}
	return f.frame
}

// frameStream is the frames of one browser on their way to the window. mu
// orders the sends: a still captured before the page drew again is never sent
// after that frame, which the window keeps as the newest.
type frameStream struct {
	s        *Service
	sess     *session
	frames   api.Event[api.BrowserFrame]
	skipping atomic.Bool
	mu       sync.Mutex
	drawn    uint64
	// last is the last frame sent, of the pane attached attach-th.
	last   api.BrowserFrame
	attach uint64
	settle *time.Timer
	// capturing is set during a capture and captured when the last one
	// ended; tries counts the captures since the page last drew on its own.
	capturing bool
	captured  time.Time
	tries     int
}

func (f *frameStream) frame(tab, data string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.s.mu.Lock()
	attach := f.sess.attaches
	f.s.mu.Unlock()
	// A capture draws a frame byte for byte the last one (Chromium 157,
	// measured 2026-11-12): taken as a change, it would replace the PNG and
	// settle into another capture, for as long as the pane is open. A pane
	// attached since may be on a window holding no frame.
	frame := api.BrowserFrame{Tab: tab, Data: data}
	if frame == f.last && attach == f.attach {
		return
	}
	f.drawn++
	f.arm()
	if len(data) > maxFrameData {
		if !f.skipping.Swap(true) {
			slog.Warn("browser: a frame too large for the window, its pane keeps the last one", "owner", f.sess.owner, "bytes", len(data))
		}
		return
	}
	f.skipping.Store(false)
	f.last, f.attach = frame, attach
	events.Send(f.s.emit, f.frames, frame)
}

// arm starts the settle over, unless the frame is a capture's own and the
// page had its tries: whatever a capture draws, a still page stops there.
func (f *frameStream) arm() {
	if f.capturing || time.Since(f.captured) < stillEcho {
		if f.tries >= stillTries {
			return
		}
	} else {
		f.tries = 0
	}
	if f.settle == nil {
		f.settle = time.AfterFunc(stillAfter, f.still)
	} else {
		f.settle.Reset(stillAfter)
	}
}

// still sends a PNG of the page once it settled, unless it drew again
// meanwhile or the PNG weighs too much for what it gives.
func (f *frameStream) still() {
	f.mu.Lock()
	drawn, jpeg := f.drawn, f.last
	f.capturing = true
	f.tries++
	f.mu.Unlock()
	tab, data, ok := f.s.captureStill(f.sess)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.capturing, f.captured = false, time.Now()
	// A relaunch replaced the browser while it captured: the old page's PNG
	// would land over the new one's frames.
	f.s.mu.Lock()
	current := f.s.byID[f.sess.id] == f.sess
	f.s.mu.Unlock()
	if !ok || !current || f.drawn != drawn || tab != jpeg.Tab || len(data) > maxFrameData || len(data) > stillGrowth*len(jpeg.Data) {
		return
	}
	events.Send(f.s.emit, f.frames, api.BrowserFrame{Tab: tab, Data: data})
}

// captureStill is sess's page as a PNG while a pane draws it. A page holding
// a dialog is not captured: Page.captureScreenshot waits for it to close.
func (s *Service) captureStill(sess *session) (tab, data string, ok bool) {
	s.mu.Lock()
	drawn := sess.watching && !sess.headed && sess.alive()
	s.mu.Unlock()
	if !drawn {
		return "", "", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), stillWait)
	defer cancel()
	tab, data, err := sess.page.Still(ctx)
	if err != nil {
		if sess.alive() && !errors.Is(err, errDialogOpen) && !errors.Is(err, errPaused) && !errors.Is(err, errAnnotating) && !errors.Is(err, errTabChanged) && !errors.Is(err, errCaret) {
			slog.Warn("browser: capture a still page", "owner", sess.owner, "err", err)
		}
		return "", "", false
	}
	return tab, data, true
}

// infoSink records the page's URL and title as it navigates on its own (a
// link the person or the agent clicked), and tells the window once the
// browser is registered.
func (s *Service) infoSink(sess *session) func(url, title string) {
	return func(url, title string) {
		s.mu.Lock()
		if url != "" {
			sess.url = url
		}
		sess.title = title
		registered := s.byID[sess.id] == sess
		s.mu.Unlock()
		if registered {
			s.emitState(sess, true)
		}
	}
}

func (s *Service) emitState(sess *session, open bool) {
	s.mu.Lock()
	state := sess.stateLocked(open)
	s.mu.Unlock()
	events.Send(s.emit, api.BrowserStateEvent, state)
}

// agentActed tells the window the agent acted on sess's page, and keeps the
// point it acted at for its next scroll.
func (s *Service) agentActed(sess *session, act api.BrowserAgentAct) {
	act.Owner = sess.owner
	if act.X != nil && act.Y != nil {
		s.mu.Lock()
		sess.agentAt = &point{X: *act.X, Y: *act.Y}
		s.mu.Unlock()
	}
	events.Send(s.emit, api.BrowserAgentEvent, act)
}

// agentPoint is where the agent last acted on owner's page, nil before it
// acted at a point.
func (s *Service) agentPoint(owner string) *point {
	s.mu.Lock()
	defer s.unlock()
	sess := s.lookupLocked(owner)
	if sess == nil || sess.agentAt == nil {
		return nil
	}
	at := *sess.agentAt
	return &at
}

func (s *session) stateLocked(open bool) api.BrowserState {
	return api.BrowserState{
		Owner: s.owner, Open: open, Headed: s.headed, ByAgent: s.byAgent, URL: s.url, Title: s.title,
		Dialog: s.dialog, Tabs: s.paneTabs, Idle: s.asked,
	}
}
