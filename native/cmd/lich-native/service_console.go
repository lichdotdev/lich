package main

import (
	"context"
	"errors"
	"image"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/gioterm"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// serviceConsole is the state behind a service's console: its bar's buttons,
// which run each service's terminal shows, and the output fetched for one
// that stopped before this window saw it run.
type serviceConsole struct {
	start, stop, restart, force, edit widget.Clickable

	mu sync.Mutex
	// runs is the StartedAt of the run each service's terminal holds.
	runs map[string]int64
	// tails is the output of each stopped service's last run, keyed by the
	// StoppedAt it was read for.
	tails map[string]consoleTail
}

type consoleTail struct {
	stoppedAt int64
	data      []byte
	loaded    bool
}

func newServiceConsole() serviceConsole {
	return serviceConsole{runs: map[string]int64{}, tails: map[string]consoleTail{}}
}

// hideCursor is DECTCEM off: a console nothing reads input from draws no
// cursor.
const hideCursor = "\x1b[?25l"

// serviceTermKind marks a terminal that shows a service's PTY, not a session's.
const serviceTermKind = "service"

// serviceView sets up a terminal for a service's PTY: the renderer and the
// grid that fits gtx.
func (ts *terminals) serviceView(gtx layout.Context, id string) (*termView, int, int, error) {
	ts.mu.Lock()
	family, size := ts.family, ts.size
	ts.mu.Unlock()
	r, err := gioterm.NewRenderer(ts.renderer, family, size)
	if err != nil {
		return nil, 0, 0, err
	}
	gtx.Metric = ts.unzoomed
	if err := r.Measure(gtx); err != nil {
		return nil, 0, 0, err
	}
	cols, rows := gioterm.Fit(gridArea(gtx), r)
	return &termView{id: id, kind: serviceTermKind, r: r, tag: new(int)}, cols, rows, nil
}

// showLiveService attaches a terminal to a service's running PTY, the backend
// having started it: its replayed tail, then its size. Never Start, which is
// a session's spawn.
func (ts *terminals) showLiveService(ctx context.Context, gtx layout.Context, id string) (*termView, error) {
	ts.mu.Lock()
	t := ts.byID[id]
	ts.mu.Unlock()
	if t != nil {
		return t, nil
	}
	t, cols, rows, err := ts.serviceView(gtx, id)
	if err != nil {
		return nil, err
	}
	if err := ts.attach(ctx, t, cols, rows); err != nil {
		return nil, err
	}
	return t, ts.client.Resize(ctx, id, cols, rows)
}

// showStoppedService draws what a stopped service printed in a terminal of
// its own that takes no input and answers nothing.
func (ts *terminals) showStoppedService(gtx layout.Context, id string, tail []byte) (*termView, error) {
	t, cols, rows, err := ts.serviceView(gtx, id)
	if err != nil {
		return nil, err
	}
	cellW, cellH := t.r.Cell()
	vt, err := gioterm.New(cols, rows, cellW, cellH, func([]byte) {})
	if err != nil {
		return nil, err
	}
	ts.mu.Lock()
	colors := ts.colors
	ts.mu.Unlock()
	if err := vt.SetColors(colors); err != nil {
		vt.Close()
		return nil, err
	}
	vt.Write(tail)
	vt.Write([]byte(hideCursor))
	t.vt = vt
	t.seeded.Store(true)
	t.readOnly.Store(true)
	ts.mu.Lock()
	ts.byID[id] = t
	ts.mu.Unlock()
	return t, nil
}

// consoleIsland is the stage with a service's console on it: its bar, then
// its terminal, live while it runs and what it printed last once it stopped.
func (u *window) consoleIsland(gtx C, v backend.Service, termWidth int) error {
	u.consoleClicks(gtx, v)
	size := gtx.Constraints.Max
	var err error
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return u.consoleBar(gtx, u.theme.On(u.theme.Background), v) }),
		layout.Flexed(1, func(gtx C) D {
			term := gtx
			term.Constraints = layout.Exact(image.Pt(min(termWidth, size.X), gtx.Constraints.Max.Y))
			err = u.consoleTerminal(term, v)
			return D{Size: gtx.Constraints.Max}
		}),
	)
	return err
}

func (u *window) consoleClicks(gtx C, v backend.Service) {
	c := &u.console
	op := serviceNone
	switch {
	case c.start.Clicked(gtx):
		op = serviceStart
	case c.stop.Clicked(gtx):
		op = serviceStop
	case c.restart.Clicked(gtx):
		op = serviceRestart
	case c.force.Clicked(gtx):
		op = serviceForceStop
	case c.edit.Clicked(gtx):
		op = serviceEdit
	}
	u.applyService(serviceRequest{op: op, id: v.ID})
}

func (u *window) consoleTerminal(gtx C, v backend.Service) error {
	id := v.Terminal
	c := &u.console
	if serviceUp(v) {
		if c.run(v.ID) != v.StartedAt {
			u.terms.drop(id)
			c.setRun(v.ID, v.StartedAt)
		}
		u.terms.onScreen(id)
		t, err := u.terms.showLiveService(u.ctx, gtx, id)
		if err != nil {
			return err
		}
		t.readOnly.Store(false)
		return u.layoutConsole(gtx, t)
	}
	u.terms.onScreen()
	if v.StoppedAt == 0 {
		return nil
	}
	if !u.terms.has(id) || c.run(v.ID) != v.StartedAt {
		tail, ok := c.tail(v)
		if !ok {
			go u.fetchTail(v)
			return nil
		}
		u.terms.drop(id)
		if _, err := u.terms.showStoppedService(gtx, id, tail); err != nil {
			return err
		}
		c.setRun(v.ID, v.StartedAt)
	}
	u.terms.mu.Lock()
	t := u.terms.byID[id]
	u.terms.mu.Unlock()
	if t == nil {
		return nil
	}
	t.readOnly.Store(true)
	return u.layoutConsole(gtx, t)
}

func (u *window) layoutConsole(gtx C, t *termView) error {
	err := u.terms.layout(gtx, t, u.terminalTakesKeys(gtx))
	if errors.Is(err, gioterm.ErrClosed) {
		gtx.Execute(op.InvalidateCmd{})
		return nil
	}
	return err
}

func (c *serviceConsole) run(id string) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.runs[id]
}

func (c *serviceConsole) setRun(id string, startedAt int64) {
	c.mu.Lock()
	c.runs[id] = startedAt
	c.mu.Unlock()
}

// tail is the output fetched for v's last run, once it arrived.
func (c *serviceConsole) tail(v backend.Service) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.tails[v.ID]
	return t.data, t.loaded && t.stoppedAt == v.StoppedAt
}

// fetchTail reads what v's last run printed, once per run.
func (u *window) fetchTail(v backend.Service) {
	c := &u.console
	c.mu.Lock()
	if t, ok := c.tails[v.ID]; ok && t.stoppedAt == v.StoppedAt {
		c.mu.Unlock()
		return
	}
	c.tails[v.ID] = consoleTail{stoppedAt: v.StoppedAt}
	c.mu.Unlock()
	data, err := u.client.ServiceOutput(u.ctx, v.ID)
	if err != nil {
		u.logErr("service output", err)
	}
	c.mu.Lock()
	c.tails[v.ID] = consoleTail{stoppedAt: v.StoppedAt, data: data, loaded: true}
	c.mu.Unlock()
	u.model.invalidate()
}

// consoleBar is the line over a service's console: its name, its state, the
// way to stop it from the keyboard while it runs, and the buttons its state
// offers.
func (u *window) consoleBar(gtx C, th *ui.Theme, v backend.Service) D {
	c := &u.console
	button := func(click *widget.Clickable, text string, variant ui.Variant) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			b := th.Button(click, text)
			b.Variant, b.Size = variant, ui.SizeSM
			return b.Layout(gtx)
		})
	}
	children := []layout.FlexChild{
		layout.Rigid(th.Text(ui.TextSM, v.Name).Weight(font.Medium).Layout),
		ui.Gap(3),
		layout.Rigid(func(gtx C) D { return consolePill(gtx, th, v) }),
		layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} }),
	}
	switch v.State {
	case api.ServiceRunning:
		children = append(children,
			layout.Rigid(th.Text(ui.TextXS, i18n.T("sidebar.services.console.ctrlC")).Muted().Layout), ui.Gap(3),
			button(&c.restart, i18n.T("sidebar.services.restart"), ui.VariantOutline), ui.Gap(2),
			button(&c.stop, i18n.T("sidebar.services.stop"), ui.VariantOutline))
	case api.ServiceStopping:
		children = append(children, button(&c.force, i18n.T("sidebar.services.forceStop"), ui.VariantDestructive))
	default:
		children = append(children,
			button(&c.edit, i18n.T("sidebar.services.editShort"), ui.VariantGhost), ui.Gap(2),
			button(&c.start, i18n.T("sidebar.services.start"), ui.VariantDefault))
	}
	return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

// consolePill is a service's state on its bar: the dot and a few words.
func consolePill(gtx C, th *ui.Theme, v backend.Service) D {
	text := consoleStateText(v, gtx.Now)
	l := th.Text(ui.TextXS, text).Muted()
	switch v.State {
	case api.ServiceRunning:
		l.Color = th.TonePass
		elapsed := gtx.Now.Sub(time.UnixMilli(v.StartedAt))
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(nextAgeChange(elapsed))})
	case api.ServiceStopping:
		l.Color = th.ToneWait
	case api.ServiceFailed:
		l.Color = th.Destructive
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if v.State == api.ServiceRunning {
				return beatingDot(gtx, th)
			}
			return stateDot(gtx, th, v.State)
		}),
		ui.Gap(1),
		layout.Rigid(l.Layout),
	)
}

// consoleStateText is the console bar's state: how long it has run, when it
// stopped, or the code it failed with.
func consoleStateText(v backend.Service, now time.Time) string {
	switch v.State {
	case api.ServiceRunning:
		return i18n.T("sidebar.services.console.running", i18n.Args{"age": formatAge(now.Sub(time.UnixMilli(v.StartedAt)))})
	case api.ServiceStopping:
		return i18n.T("sidebar.services.stopping")
	case api.ServiceFailed:
		if v.ExitCode != nil {
			return i18n.T("sidebar.services.console.failedCode", i18n.Args{"code": *v.ExitCode})
		}
		return i18n.T("sidebar.services.failed")
	}
	if v.StoppedAt == 0 {
		return i18n.T("sidebar.services.console.never")
	}
	return i18n.T("sidebar.services.console.stoppedAt", i18n.Args{"time": time.UnixMilli(v.StoppedAt).Format("15:04")})
}
