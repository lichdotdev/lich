package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/gioterm"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// A page pane: the browser of a session drawn in its tab beside it, from the
// frames the backend streams, with the person's pointer and keys passed on.

var (
	icBrowserBack    = icons.Lucide("arrow-left")
	icBrowserForward = icons.Lucide("arrow-right")
	icBrowserReload  = icons.Lucide("rotate-cw")
	icBrowserWindow  = icons.Lucide("external-link")
)

// agentHold is how long the agent's cursor stays up after the agent's last
// action on the page or the end of its turn, before it fades. While its turn
// runs the cursor stays: between two tool calls the agent is thinking, not
// gone, and a cursor that left in between made it look stuttering.
const agentHold = 5 * time.Second

// doubleClick is the window two presses of a button count as one double
// click in, as most desktops set it.
const doubleClick = 400 * time.Millisecond

// browserPaneUI is one page pane's widgets and what it last told the
// backend, kept across frames by session.
type browserPaneUI struct {
	back, forward, reload, window iconActionState
	zoom, close                   widget.Clickable
	zoomTip, closeTip             ui.Tooltip
	url                           widget.Editor
	// shownURL is the page's URL the field last showed: a navigation
	// replaces the field's text unless the person is editing it.
	shownURL    string
	tag         int
	composition gioterm.Composition
	// view is the size the backend lays the page out at; zero until the
	// pane is attached.
	view api.BrowserViewOptions
	// held is the button the pointer holds down, for a drag's moves.
	held      string
	lastPress time.Time
	presses   int
	queue     *browserQueue
	// copied is the page selection a copy read, for the next frame to write
	// to the system clipboard.
	copied                chan string
	cursor                agentCursor
	onScreen, wasOnScreen bool
	// flash is the toolbar or dialog button the agent's cursor pressed, and
	// when; typing is the address it types into the field, from when.
	flash  agentPress
	typing agentPress
	notes  browserNotes
	dialog dialogCard
}

// agentPress is what the agent's cursor does on the toolbar, from at: the
// button it presses (back, forward, reload, a dialog's accept or dismiss) or
// the address it types.
type agentPress struct {
	what string
	at   time.Time
}

func (u *window) browserPane(session string) *browserPaneUI {
	if u.browserPanes == nil {
		u.browserPanes = map[string]*browserPaneUI{}
	}
	p, ok := u.browserPanes[session]
	if !ok {
		p = &browserPaneUI{queue: newBrowserQueue(), copied: make(chan string, 1), notes: newBrowserNotes()}
		go p.queue.run(u.ctx, func(err error) {
			log.Printf("browser of %s: %v", session, err)
		})
		u.browserPanes[session] = p
	}
	return p
}

// sendBrowser queues one call to the backend for session's page; a failure
// is logged, since the page answers on screen.
func (u *window) sendBrowser(session, what string, call func(context.Context) error) {
	u.browserPane(session).queue.push(browserCall{send: func(ctx context.Context) error {
		if err := call(ctx); err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
		return nil
	}})
}

// browserPaneAt draws session's page pane in r, the way paneAt draws a
// session's: a header, the toolbar, and the page.
func (u *window) browserPaneAt(gtx C, v stageView, session string, r image.Rectangle, several bool) {
	pane := browserPaneOf(session)
	p := u.browserPane(session)
	p.onScreen = true
	st, frame, marks := u.model.browserView(session)
	defer op.Offset(r.Min).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(r.Size())
	size := r.Size()
	focused := pane == v.layout.Focus
	th := u.theme
	shape := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(paneRadius))
	paint.FillShape(gtx.Ops, th.Background, shape.Op(gtx.Ops))
	area := shape.Push(gtx.Ops)
	head, bar := gtx.Dp(paneHeader), gtx.Dp(ui.Space(9))
	u.playAgentCursor(gtx, p, marks, head, bar)
	p.dialog.sync(st.Dialog, p.answering(gtx.Now))
	header := gtx
	header.Constraints = layout.Exact(image.Pt(size.X, head))
	u.browserHeader(header, p, pane, focused)
	line := th.Over(th.Border)
	paint.FillShape(gtx.Ops, line, clip.Rect{Min: image.Pt(0, head-gtx.Dp(1)), Max: image.Pt(size.X, head)}.Op())
	tool := op.Offset(image.Pt(0, head)).Push(gtx.Ops)
	toolbar := gtx
	toolbar.Constraints = layout.Exact(image.Pt(size.X, bar))
	u.browserToolbar(toolbar, p, session, st)
	tool.Pop()
	paint.FillShape(gtx.Ops, line, clip.Rect{Min: image.Pt(0, head+bar-gtx.Dp(1)), Max: image.Pt(size.X, head+bar)}.Op())
	page := gtx
	page.Constraints = layout.Exact(image.Pt(size.X, max(size.Y-head-bar, 0)))
	off := op.Offset(image.Pt(0, head+bar)).Push(gtx.Ops)
	// Unlike a terminal, the page is not dimmed out of focus: the person keeps
	// the session's prompt in focus nearly always, and watches the page.
	u.browserPage(page, p, session, frame, focused)
	u.notesOverlay(page, p, session, st.URL)
	u.notesTray(page, p, session)
	u.dialogCardAt(page, p, session, head+bar)
	off.Pop()
	u.paintAgentCursor(gtx, p, session)
	u.paneFocusTarget(gtx, pane)
	area.Pop()
	u.lockedDropRefusal(gtx, session, image.Rectangle{Max: size}, r.Add(u.islandAt))
	border, width := line, float32(gtx.Dp(1))
	if several && focused {
		border, width = th.Ring, float32(gtx.Dp(1.5))
	}
	paint.FillShape(gtx.Ops, border, clip.Stroke{Path: shape.Path(gtx.Ops), Width: width}.Op())
}

// browserHeader is the page pane's top row: its name, zoom and close.
func (u *window) browserHeader(gtx C, p *browserPaneUI, pane string, focused bool) {
	th := u.theme
	if p.zoom.Clicked(gtx) {
		u.layoutStage.focus, u.layoutStage.zoom = pane, true
	}
	if p.close.Clicked(gtx) {
		session := paneSession(pane)
		u.sendBrowser(session, "close", func(ctx context.Context) error { return u.client.CloseBrowser(ctx, session) })
	}
	fg, weight := th.MutedForeground, font.Normal
	if focused {
		fg, weight = th.Foreground, font.Medium
	}
	layout.Inset{Left: ui.Space(2.5), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icBrowser.Layout(gtx, ui.Space(3.5), th.MutedForeground) }),
			ui.Gap(2),
			layout.Rigid(th.Text(ui.TextXS, i18n.T("layouts.browser.title")).In(fg).Weight(weight).Layout),
			layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Max.X, 0)} }),
			layout.Rigid(func(gtx C) D {
				zoom := th.Tooltip(&u.root, &p.zoomTip, i18n.T("layouts.pane.zoom"))
				return zoom.Layout(gtx, func(gtx C) D { return cornerButton(gtx, th, &p.zoom, icPaneZoom, false) })
			}),
			ui.Gap(1.5),
			layout.Rigid(func(gtx C) D {
				closer := th.Tooltip(&u.root, &p.closeTip, i18n.T("layouts.browser.close"))
				return closer.Layout(gtx, func(gtx C) D { return cornerButton(gtx, th, &p.close, icPaneUnsplit, false) })
			}),
		)
	})
}

// browserToolbar is back, forward, reload, the address and Open in window.
func (u *window) browserToolbar(gtx C, p *browserPaneUI, session string, st api.BrowserState) {
	th := u.theme
	nav := func(action string) {
		u.sendBrowser(session, action, func(ctx context.Context) error { return u.client.BrowserNav(ctx, session, action, "") })
	}
	if p.back.click.Clicked(gtx) {
		nav("back")
	}
	if p.forward.click.Clicked(gtx) {
		nav("forward")
	}
	if p.reload.click.Clicked(gtx) {
		nav("reload")
	}
	if p.notes.pick.click.Clicked(gtx) {
		p.notes.setPicking(!p.notes.picking)
	}
	if p.window.click.Clicked(gtx) {
		u.sendBrowser(session, "show", func(ctx context.Context) error { return u.client.ShowBrowser(ctx, session) })
	}
	for {
		ev, ok := p.url.Update(gtx)
		if !ok {
			break
		}
		if submit, ok := ev.(widget.SubmitEvent); ok {
			target := typedURL(submit.Text)
			u.sendBrowser(session, "go", func(ctx context.Context) error { return u.client.BrowserNav(ctx, session, "go", target) })
			gtx.Execute(key.FocusCmd{Tag: &p.tag})
		}
	}
	if text, typing := p.typed(gtx); typing && !gtx.Focused(&p.url) {
		p.url.SetText(text)
	} else if st.URL != p.shownURL && !gtx.Focused(&p.url) {
		p.shownURL = st.URL
		p.url.SetText(displayURL(st.URL))
	}
	p.url.Submit = true
	field := th.Input(&p.url, i18n.T("layouts.browser.address"))
	field.Height, field.TextSize, field.Mono = ui.Space(6), ui.TextXS, true
	layout.Inset{Left: ui.Space(1.5), Right: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return iconAction(gtx, th, &u.root, &p.back, icBrowserBack, i18n.T("layouts.browser.back"), p.flashing(gtx, "back"))
			}),
			layout.Rigid(func(gtx C) D {
				return iconAction(gtx, th, &u.root, &p.forward, icBrowserForward, i18n.T("layouts.browser.forward"), p.flashing(gtx, "forward"))
			}),
			layout.Rigid(func(gtx C) D {
				return iconAction(gtx, th, &u.root, &p.reload, icBrowserReload, i18n.T("layouts.browser.reload"), p.flashing(gtx, "reload"))
			}),
			ui.Gap(1.5),
			layout.Flexed(1, field.Layout),
			ui.Gap(1.5),
			layout.Rigid(func(gtx C) D {
				if u.browserZoom == defaultZoom {
					return D{}
				}
				label := fmt.Sprintf("%d%%", int(math.Round(u.browserZoom*100)))
				return layout.Inset{Right: ui.Space(1.5)}.Layout(gtx, th.Text(ui.TextXS, label).In(th.MutedForeground).Layout)
			}),
			layout.Rigid(func(gtx C) D {
				if !p.notes.picking {
					return D{}
				}
				return layout.Inset{Right: ui.Space(1.5)}.Layout(gtx, th.Text(ui.TextXS, i18n.T("layouts.browser.pickHint")).In(notePick).Layout)
			}),
			layout.Rigid(func(gtx C) D {
				return iconAction(gtx, th, &u.root, &p.notes.pick, icBrowserPick, i18n.T("layouts.browser.pick"), p.notes.picking)
			}),
			layout.Rigid(func(gtx C) D {
				return iconAction(gtx, th, &u.root, &p.window, icBrowserWindow, i18n.T("layouts.browser.openInWindow"), false)
			}),
		)
	})
}

// browserPage draws the page's last frame over gtx's area, keeps the
// backend's page the area's size, and passes the pointer and keys on.
func (u *window) browserPage(gtx C, p *browserPaneUI, session string, frame browserFrame, focused bool) {
	size := gtx.Constraints.Max
	scale := u.pageScale(gtx)
	want := api.BrowserViewOptions{ID: session, W: int(float32(size.X) / scale), H: int(float32(size.Y) / scale), Scale: float64(scale)}
	if want.W > 0 && want.H > 0 && want != p.view {
		attach := p.view.ID == ""
		p.view = want
		if attach {
			u.sendBrowser(session, "attach", func(ctx context.Context) error { return u.client.AttachBrowser(ctx, want) })
		} else {
			u.browserPane(session).queue.push(browserCall{resize: true, send: func(ctx context.Context) error { return u.client.ResizeBrowser(ctx, want) }})
		}
	}
	paint.FillShape(gtx.Ops, u.theme.Background, clip.Rect{Max: size}.Op())
	if frame.size.X > 0 {
		// A frame is the page at the size it was last laid out at, which
		// lags a resize by a frame or two: it is drawn scaled to the area.
		sx, sy := float32(size.X)/float32(frame.size.X), float32(size.Y)/float32(frame.size.Y)
		s := min(sx, sy)
		t := op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(s, s))).Push(gtx.Ops)
		frame.img.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		t.Pop()
	}
	u.browserInput(gtx, p, session, scale, focused)
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, &p.tag)
}

// pageScale is the pane pixels a page's CSS pixel takes: the window's scale
// times the page zoom.
func (u *window) pageScale(gtx C) float32 { return gtx.Metric.PxPerDp * float32(u.browserZoom) }

// setBrowserZoom zooms every page pane to zoom and keeps it for the next
// start. The Appearance prefs hold it too: Settings applies its whole set on
// every change it makes.
func (u *window) setBrowserZoom(zoom float64) {
	if zoom == u.browserZoom {
		return
	}
	u.browserZoom, u.settings.look.prefs.browserZoom = zoom, zoom
	value := strconv.FormatFloat(zoom, 'f', -1, 64)
	go func() {
		if err := u.client.SetSetting(u.ctx, browserZoomKey, "", value); err != nil {
			log.Printf("browser: save the page zoom: %v", err)
		}
	}()
	u.win.Invalidate()
}

// syncBrowserView tells the backend the window's size, at the page zoom, when
// it changed: a page no pane shows lays out there instead of at Chrome's
// cramped 800x600.
func (u *window) syncBrowserView(gtx C) {
	scale := u.pageScale(gtx)
	size := gtx.Constraints.Max
	want := api.BrowserDefaultViewOptions{W: int(float32(size.X) / scale), H: int(float32(size.Y) / scale), Scale: float64(scale)}
	if want.W <= 0 || want.H <= 0 || want == u.browserView {
		return
	}
	u.browserView = want
	u.browserViews.push(browserCall{resize: true, send: func(ctx context.Context) error { return u.client.SetBrowserDefaultView(ctx, want) }})
}

// browserInput passes the pane's pointer and keys to the page.
func (u *window) browserInput(gtx C, p *browserPaneUI, session string, scale float32, focused bool) {
	filters := append(gioterm.KeyFilters(&p.tag),
		pointer.Filter{Target: &p.tag, Kinds: pointer.Scroll | pointer.Press | pointer.Release | pointer.Drag | pointer.Move | pointer.Leave,
			ScrollX: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}, ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}},
		key.Filter{Focus: &p.tag, Name: "V", Required: key.ModCommand},
		key.Filter{Focus: &p.tag, Name: "C", Required: key.ModCommand},
		key.Filter{Focus: &p.tag, Name: "X", Required: key.ModCommand},
		key.Filter{Focus: &p.tag, Name: "-", Required: key.ModShortcut, Optional: key.ModShift},
		key.Filter{Focus: &p.tag, Name: "=", Required: key.ModShortcut, Optional: key.ModShift},
		key.Filter{Focus: &p.tag, Name: "+", Required: key.ModShortcut, Optional: key.ModShift},
		key.Filter{Focus: &p.tag, Name: "0", Required: key.ModShortcut},
		key.Filter{Focus: &p.tag, Name: "E", Required: key.ModShortcut | key.ModShift},
		key.Filter{Focus: &p.tag, Name: key.NameReturn, Required: key.ModShortcut},
		key.Filter{Focus: &p.tag, Name: key.NameEnter, Required: key.ModShortcut},
		transfer.TargetFilter{Target: &p.tag, Type: "application/text"})
	if focused && !gtx.Focused(&p.url) && !gtx.Focused(&p.tag) && !gtx.Focused(&p.notes.editor) && !gtx.Focused(&p.dialog.field) {
		gtx.Execute(key.FocusCmd{Tag: &p.tag})
	}
	writeCopied(gtx, p)
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		switch e := ev.(type) {
		case key.EditEvent, key.CompositionEvent, key.SelectionEvent, key.FocusEvent:
			if text := p.composition.Update(e, &p.tag, gtx.Execute); text != "" {
				u.sendBrowser(session, "text", func(ctx context.Context) error { return u.client.BrowserText(ctx, session, text) })
			}
		case key.Event:
			if u.dialogPageKey(p, session, e) || u.notesKey(p, session, e) {
				continue
			}
			if zoom, ok := pageZoomChord(e, runtime.GOOS, u.browserZoom); ok {
				u.setBrowserZoom(zoom)
				continue
			}
			switch browserClipboardChord(e, runtime.GOOS) {
			case "paste":
				gtx.Execute(clipboard.ReadCmd{Tag: &p.tag})
				continue
			case "copy", "cut":
				// Read before the key reaches the page: a cut takes the
				// selection away. The queue keeps the two in order.
				u.copyPageSelection(p, session)
			}
			if k, ok := browserKey(session, e); ok {
				u.sendBrowser(session, "key", func(ctx context.Context) error { return u.client.BrowserKey(ctx, k) })
			}
		case transfer.DataEvent:
			text, err := readTransfer(e)
			if err != nil {
				log.Printf("paste into the browser of %s: %v", session, err)
				break
			}
			u.sendBrowser(session, "paste", func(ctx context.Context) error { return u.client.BrowserText(ctx, session, text) })
		case pointer.Event:
			u.browserPointer(gtx, p, session, scale, e)
		}
	}
}

func (u *window) browserPointer(gtx C, p *browserPaneUI, session string, scale float32, e pointer.Event) {
	m := api.BrowserMouseOptions{ID: session, X: float64(e.Position.X / scale), Y: float64(e.Position.Y / scale), Modifiers: cdpModifiers(e.Modifiers)}
	if p.notes.picking && e.Kind != pointer.Scroll {
		u.notesPointer(gtx, p, session, m.X, m.Y, e)
		return
	}
	if e.Kind == pointer.Scroll && (len(p.notes.list) > 0 || p.notes.open) {
		p.notes.nextLocate, p.notes.followUntil = time.Time{}, gtx.Now.Add(noteFollowFor)
	}
	if e.Kind == pointer.Scroll && p.notes.picking {
		// What the pointer outlined moved with the page: look again under it.
		p.notes.hover = api.BrowserElement{}
		defer u.probeElement(p, session, m.X, m.Y, false)
	}
	switch e.Kind {
	case pointer.Press:
		gtx.Execute(key.FocusCmd{Tag: &p.tag})
		u.layoutStage.focus = browserPaneOf(session)
		if gtx.Now.Sub(p.lastPress) < doubleClick {
			p.presses++
		} else {
			p.presses = 1
		}
		p.lastPress = gtx.Now
		p.held = browserButton(e.Buttons)
		m.Type, m.Button, m.Clicks = "press", p.held, p.presses
	case pointer.Release:
		m.Type, m.Button, m.Clicks = "release", p.held, p.presses
		p.held = ""
	case pointer.Move, pointer.Drag:
		m.Type, m.Button = "move", p.held
	case pointer.Scroll:
		m.Type, m.DX, m.DY = "wheel", float64(e.Scroll.X/scale), float64(e.Scroll.Y/scale)
	default:
		return
	}
	u.browserPane(session).queue.push(browserCall{mouse: &m, send: func(ctx context.Context) error { return u.client.BrowserMouse(ctx, m) }})
}

// settleBrowserPanes stops the frames of every page pane that was on screen
// last frame and is not on this one, after the frame laid everything out.
func (u *window) settleBrowserPanes() {
	for session, p := range u.browserPanes {
		gone := p.wasOnScreen && !p.onScreen
		p.wasOnScreen, p.onScreen = p.onScreen, false
		if !gone || p.view.ID == "" {
			continue
		}
		p.view = api.BrowserViewOptions{}
		u.sendBrowser(session, "detach", func(ctx context.Context) error { return u.client.DetachBrowser(ctx, session) })
	}
}

// lockedDropRefusal tells a drag carried over a browser tab's pane that the
// tab takes no one else.
func (u *window) lockedDropRefusal(gtx C, session string, local, at image.Rectangle) {
	carried := u.sidebar.carry.Carrying()
	if carried == "" || carried == session || !u.layoutStage.pointer.at.In(at) {
		return
	}
	u.dropHighlight(gtx, local, i18n.T("layouts.browser.locked", i18n.Args{"label": u.model.sessionLabel(session)}))
}

// displayURL is the page's URL as the address field shows it: about:blank
// as an empty field.
func displayURL(raw string) string {
	if raw == "about:blank" {
		return ""
	}
	return raw
}

// typedURL is what the address field's text loads: a URL or an absolute path
// as typed, and a bare host (localhost:3000, example.com) over http for a
// local one and https for any other. The backend serves a path from the
// session's checkout (internal/preview).
func typedURL(text string) string {
	switch {
	case text == "":
		return "about:blank"
	case hasScheme(text), filepath.IsAbs(text):
		return text
	case isLocalHost(text):
		return "http://" + text
	}
	return "https://" + text
}

func hasScheme(text string) bool {
	for i, c := range text {
		switch {
		case c == ':':
			return i > 0 && len(text) > i+2 && text[i+1:i+3] == "//"
		case !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || i > 0 && ('0' <= c && c <= '9' || c == '+' || c == '-' || c == '.')):
			return false
		}
	}
	return false
}

func isLocalHost(text string) bool {
	host := text
	for i, c := range text {
		if c == ':' || c == '/' {
			host = text[:i]
			break
		}
	}
	return host == "localhost" || host == "127.0.0.1" || host == "[::1]" || strings.HasSuffix(host, ".localhost")
}

// agentLook is how session's agent cursor is drawn: in its card color, or the
// theme's primary for a card without one, naming the provider that runs in it;
// busy is whether the agent's turn is running.
func (u *window) agentLook(session string) (tint color.NRGBA, provider string, icon *icons.Icon, busy bool) {
	tint = u.theme.Primary
	sv, ok := u.model.sessionViewOf(session)
	if !ok {
		return tint, "", providerIcon(""), false
	}
	if c, tinted := cardColors[sv.Color]; tinted {
		tint = c
	}
	kind := sv.Kind
	if sv.agent != "" {
		kind = sv.agent
	}
	provider = kind
	if info, found := u.model.providers.find(kind); found && info.name != "" {
		provider = info.name
	}
	return tint, provider, providerIcon(kind), sv.status == statusBusy
}

// toolbarFlash is how long a toolbar button the agent's cursor pressed stays
// lit, as a click's would.
const toolbarFlash = 220 * time.Millisecond

// typingStep and typingMost pace the address the agent's cursor types into
// the field: a key every typingStep, the whole address within typingMost.
const (
	typingStep = 28 * time.Millisecond
	typingMost = 700 * time.Millisecond
)

// playAgentCursor takes the agent's new actions and starts the next one, at
// its point of the page under the header and toolbar, on the toolbar, or on
// the dialog's card.
func (u *window) playAgentCursor(gtx C, p *browserPaneUI, marks []browserMark, head, bar int) {
	p.cursor.take(marks)
	scale := u.pageScale(gtx)
	place := func(m browserMark) f32.Point {
		if toolbarActions[m.action] {
			return toolbarPoint(gtx, m.action, head, bar)
		}
		if dialogActions[m.action] {
			return p.dialog.point(m.action, gtx.Constraints.Max.X, head+bar)
		}
		return pagePoint(*m.x, *m.y, scale, image.Rect(0, head+bar, gtx.Constraints.Max.X, gtx.Constraints.Max.Y))
	}
	m, started := p.cursor.advance(gtx.Now, place)
	if !started {
		return
	}
	landed := p.cursor.landing()
	switch m.action {
	case "navigate":
		p.typing = agentPress{what: displayURL(m.url), at: landed}
	case "back", "forward", "reload", "accept", "dismiss":
		p.flash = agentPress{what: m.action, at: landed}
	}
}

// pagePoint is the pane point of the page's CSS point x,y, the page drawn in
// page, kept inside it: an action reported outside the viewport would carry
// the cursor off the pane.
func pagePoint(x, y float64, scale float32, page image.Rectangle) f32.Point {
	px := min(max(float32(x)*scale, float32(page.Min.X)), float32(page.Max.X-1))
	py := min(max(float32(y)*scale+float32(page.Min.Y), float32(page.Min.Y)), float32(page.Max.Y-1))
	return f32.Pt(px, py)
}

// toolbarPoint is where on the toolbar the cursor plays action: the middle of
// its button, or into the address field. It follows browserToolbar's layout:
// an inset, then back, forward and reload of one iconAction side each, a gap,
// and the field.
func toolbarPoint(gtx C, action string, head, bar int) f32.Point {
	inset, side, gap := gtx.Dp(ui.Space(1.5)), gtx.Dp(ui.Space(6)), gtx.Dp(ui.Space(1.5))
	y := float32(head + bar/2)
	slot := map[string]int{"back": 0, "forward": 1, "reload": 2}
	if i, ok := slot[action]; ok {
		return f32.Pt(float32(inset+side*i+side/2), y)
	}
	return f32.Pt(float32(inset+3*side+gap+gtx.Dp(ui.Space(10))), y)
}

// paintAgentCursor draws the agent's cursor over the whole pane.
func (u *window) paintAgentCursor(gtx C, p *browserPaneUI, session string) {
	if !p.cursor.shown {
		return
	}
	tint, provider, icon, busy := u.agentLook(session)
	p.cursor.observe(gtx.Now, busy)
	u.drawAgentCursor(gtx, &p.cursor, tint, u.theme.PrimaryForeground, provider, icon)
}

// flashing reports whether the agent's cursor lit the toolbar or dialog
// button for action at now.
func (p *browserPaneUI) flashing(gtx C, action string) bool {
	if p.flash.what != action || gtx.Now.Before(p.flash.at) {
		return false
	}
	end := p.flash.at.Add(toolbarFlash)
	if gtx.Now.Before(end) {
		gtx.Execute(op.InvalidateCmd{At: end})
		return true
	}
	return false
}

// typed is the part of the agent's address shown in the field at now, and
// whether the typing still runs; it waits for the cursor to land.
func (p *browserPaneUI) typed(gtx C) (string, bool) {
	t := p.typing
	if t.what == "" {
		return "", false
	}
	runes := []rune(t.what)
	step := min(typingStep, typingMost/time.Duration(max(len(runes), 1)))
	if gtx.Now.Before(t.at) {
		gtx.Execute(op.InvalidateCmd{})
		return "", true
	}
	n := int(gtx.Now.Sub(t.at)/step) + 1
	if n >= len(runes) {
		p.typing = agentPress{}
		return t.what, false
	}
	gtx.Execute(op.InvalidateCmd{})
	return string(runes[:n]), true
}
