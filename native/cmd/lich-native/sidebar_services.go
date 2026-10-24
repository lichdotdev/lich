package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icServices       = icons.Lucide("server")
	icServiceStart   = icons.Lucide("play")
	icServiceStop    = icons.Lucide("square")
	icServiceRestart = icons.Lucide("rotate-cw")
	icServiceOutput  = icons.Lucide("square-terminal")
	icServicesOpen   = icons.Lucide("chevron-down")
	icServicesClosed = icons.Lucide("chevron-up")
	icServiceDelete  = icons.Lucide("trash-2")
)

// serviceRowsShown is how many service rows the open footer shows before it
// scrolls, so a project with many services keeps room for its sessions.
const serviceRowsShown = 8

// serviceOp is what a click asked of a service.
type serviceOp uint8

const (
	serviceNone serviceOp = iota
	serviceStart
	serviceStop
	serviceRestart
	serviceForceStop
	serviceShow
	serviceEdit
	serviceDelete
	serviceStartAll
	serviceStopAll
	serviceAdd
)

// serviceRequest is a service op: id names the service, project the project
// for the ops that act on all of them or add one.
type serviceRequest struct {
	op      serviceOp
	id      string
	project string
}

// servicesFooter is the Services row above the profile: closed, one line that
// counts what runs; open, a short list of the project's services above it.
type servicesFooter struct {
	open     bool
	header   widget.Clickable
	all      widget.Clickable
	allTip   ui.Tooltip
	add      widget.Clickable
	list     widget.List
	rows     map[string]*widget.Clickable
	starts   map[string]*widget.Clickable
	stops    map[string]*widget.Clickable
	restarts map[string]*widget.Clickable
	menus    map[string]*ui.MenuState
}

func newServicesFooter() servicesFooter {
	f := servicesFooter{
		rows: map[string]*widget.Clickable{}, starts: map[string]*widget.Clickable{}, stops: map[string]*widget.Clickable{},
		restarts: map[string]*widget.Clickable{}, menus: map[string]*ui.MenuState{},
	}
	f.list.Axis = layout.Vertical
	return f
}

// holdsKeys is whether a service's menu is open, which takes the keys.
func (f *servicesFooter) holdsKeys() bool {
	for _, m := range f.menus {
		if m.Opened() {
			return true
		}
	}
	return false
}

func (f *servicesFooter) menu(id string) *ui.MenuState {
	m, ok := f.menus[id]
	if !ok {
		m = new(ui.MenuState)
		f.menus[id] = m
	}
	return m
}

// layout is the footer's Services block: the hairline, the open list and its
// New service row, then the row that opens and closes it. console is the
// service whose console is on screen.
func (f *servicesFooter) layout(gtx C, th *ui.Theme, root *ui.Root, projectID string, list []backend.Service, console string, act *sidebarAction) D {
	if f.header.Clicked(gtx) {
		f.open = !f.open
	}
	if f.add.Clicked(gtx) {
		act.service = serviceRequest{op: serviceAdd, project: projectID}
	}
	tally := tallyServices(list)
	if f.all.Clicked(gtx) {
		act.service = serviceRequest{op: allServicesOp(tally), project: projectID}
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(1)))
			return ui.Fill(gtx, th.Over(th.Border), 0)
		}),
		ui.VGap(1.5),
	}
	if f.open {
		children = append(children,
			layout.Rigid(func(gtx C) D { return f.rowList(gtx, th, root, list, console, act) }),
			layout.Rigid(func(gtx C) D { return f.addRow(gtx, th) }),
		)
	}
	children = append(children, layout.Rigid(func(gtx C) D { return f.headerRow(gtx, th, root, tally) }))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// allServicesOp is what the header's all button does: start what is down,
// or stop everything once all of it runs.
func allServicesOp(t servicesTally) serviceOp {
	if t.total > 0 && t.up == t.total {
		return serviceStopAll
	}
	return serviceStartAll
}

func (f *servicesFooter) headerRow(gtx C, th *ui.Theme, root *ui.Root, t servicesTally) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return f.header.Layout(gtx, func(gtx C) D {
		h := gtx.Dp(ui.Space(8))
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
		bg := th.Surface
		if f.header.Hovered() {
			bg = th.Over(ui.Alpha(th.Muted, ghostHoverFill))
			ui.Fill(gtx, bg, ui.RadiusMD)
		}
		on := th.On(bg)
		chevron := icServicesClosed
		if f.open {
			chevron = icServicesOpen
		}
		label := on.Text(ui.TextSM, i18n.T("sidebar.services.title"))
		if t.total == 0 {
			label = label.Muted()
		}
		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return servicesGlyph(gtx, on, t) }),
			ui.Gap(2),
			layout.Rigid(label.Layout),
			ui.Gap(2),
			layout.Rigid(func(gtx C) D { return servicesCount(gtx, on, t) }),
			layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} }),
		}
		if f.open && t.total > 0 {
			children = append(children, layout.Rigid(func(gtx C) D { return f.allButton(gtx, on, root, t) }), ui.Gap(1.5))
		}
		children = append(children, layout.Rigid(func(gtx C) D { return chevron.Layout(gtx, 14, on.MutedForeground) }))
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.Y = h
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}

// allButton starts every service that is down, or stops them all once all
// of them run: the icon of what it does, the words in its tooltip.
func (f *servicesFooter) allButton(gtx C, th *ui.Theme, root *ui.Root, t servicesTally) D {
	ic, hint := icServiceStart, i18n.T("sidebar.services.startAll")
	if allServicesOp(t) == serviceStopAll {
		ic, hint = icServiceStop, i18n.T("sidebar.services.stopAll")
	}
	tip := th.Tooltip(root, &f.allTip, hint)
	return tip.Layout(gtx, func(gtx C) D { return cornerButton(gtx, th, &f.all, ic, false) })
}

// servicesCount is the closed row's readout: up/total, in the running tone
// while something runs, and Add for a project with none.
func servicesCount(gtx C, th *ui.Theme, t servicesTally) D {
	if t.total == 0 {
		return th.Text(ui.TextXS, i18n.T("sidebar.services.add")).Muted().Layout(gtx)
	}
	l := th.Text(ui.TextXS, strconv.Itoa(t.up)+"/"+strconv.Itoa(t.total)).Mono()
	if t.up > 0 {
		l.Color = th.TonePass
	} else {
		l = l.Muted()
	}
	return l.Layout(gtx)
}

// servicesGlyph is the dot that says how a project's services stand: red when
// one failed, pulsing while one runs, hollow otherwise.
func servicesGlyph(gtx C, th *ui.Theme, t servicesTally) D {
	switch {
	case t.failed:
		return stateDot(gtx, th, api.ServiceFailed)
	case t.up > 0:
		return beatingDot(gtx, th)
	}
	return stateDot(gtx, th, api.ServiceStopped)
}

// beatingDot is the running dot with its beat, for the places that say a
// service is up rather than list it: the footer's row and the console's bar.
func beatingDot(gtx C, th *ui.Theme) D {
	size := gtx.Dp(14)
	center := f32.Pt(float32(size)/2, float32(size)/2)
	dot := float32(gtx.Dp(3.5))
	pulseRing(gtx, center, dot, float32(gtx.Dp(3.5)), th.TonePass)
	ui.StrokeArc(gtx, center, dot/2, dot, 0, 2*math.Pi, th.TonePass)
	return D{Size: image.Pt(size, size)}
}

// The running dot beats: a ring grows out of it for pulseLength, drawn at the
// busy spinner's rate, then rests until pulseEvery comes round. It beats for
// as long as a service runs, hours, so the rest is what keeps it cheap: no
// frame is asked for in it. Measured with one service up, a continuous pulse
// at a tenth of that rate already held the window at 6% of a core.
const (
	pulseEvery  = 2400 * time.Millisecond
	pulseLength = 800 * time.Millisecond
)

// pulseRing is that beat around center: a ring from radius to radius+reach
// that fades as it grows, and the next frame it needs.
func pulseRing(gtx C, center f32.Point, radius, reach float32, c color.NRGBA) {
	at := time.Duration(gtx.Now.UnixMilli()%pulseEvery.Milliseconds()) * time.Millisecond
	if at >= pulseLength {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(pulseEvery - at)})
		return
	}
	phase := float32(at) / float32(pulseLength)
	eased := 1 - (1-phase)*(1-phase)
	ui.StrokeArc(gtx, center, radius+reach*eased, float32(gtx.Dp(1.5)), 0, 2*math.Pi, ui.Alpha(c, 0.7*(1-phase)))
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(spinnerFrame)})
}

// stateDot is a service state as a 14dp glyph.
func stateDot(gtx C, th *ui.Theme, state string) D {
	size := gtx.Dp(14)
	center := f32.Pt(float32(size)/2, float32(size)/2)
	dot := float32(gtx.Dp(3.5))
	switch state {
	case api.ServiceRunning:
		ui.StrokeArc(gtx, center, dot/2, dot, 0, 2*math.Pi, th.TonePass)
	case api.ServiceStopping:
		ui.StrokeArc(gtx, center, dot/2, dot, 0, 2*math.Pi, th.ToneWait)
	case api.ServiceFailed:
		ui.StrokeArc(gtx, center, dot/2, dot, 0, 2*math.Pi, th.Destructive)
	default:
		ui.StrokeArc(gtx, center, dot-float32(gtx.Dp(0.75)), float32(gtx.Dp(1.5)), 0, 2*math.Pi, th.MutedForeground)
	}
	return D{Size: image.Pt(size, size)}
}

func (f *servicesFooter) rowList(gtx C, th *ui.Theme, root *ui.Root, list []backend.Service, console string, act *sidebarAction) D {
	rowH := gtx.Dp(ui.Space(6.5))
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, rowH*serviceRowsShown)
	// The thumb floats over the rows, as the rail's does: a reserved gutter
	// would end every row short of the cards above it.
	rows := th.ScrollArea(&f.list)
	rows.AnchorStrategy = material.Overlay
	return rows.Layout(gtx, len(list), func(gtx C, i int) D {
		return f.row(gtx, th, root, list[i], list[i].ID == console, act)
	})
}

// row is one service: its dot, its name, and how long it has run or how it
// ended, which hover trades for the buttons its state offers.
func (f *servicesFooter) row(gtx C, th *ui.Theme, root *ui.Root, v backend.Service, shown bool, act *sidebarAction) D {
	click := clickable(f.rows, v.ID)
	start, stop, restart := clickable(f.starts, v.ID), clickable(f.stops, v.ID), clickable(f.restarts, v.ID)
	switch {
	case start.Clicked(gtx):
		act.service = serviceRequest{op: serviceStart, id: v.ID}
	case stop.Clicked(gtx):
		act.service = serviceRequest{op: serviceStop, id: v.ID}
	case restart.Clicked(gtx):
		act.service = serviceRequest{op: serviceRestart, id: v.ID}
	case click.Clicked(gtx):
		act.service = serviceRequest{op: serviceShow, id: v.ID}
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return th.ContextMenuLazy(root, f.menu(v.ID), func() []ui.MenuEntry { return serviceMenu(v, act) }).Layout(gtx, func(gtx C) D {
		return click.Layout(gtx, func(gtx C) D {
			h := gtx.Dp(ui.Space(6.5))
			gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
			bg := th.Surface
			switch {
			case shown:
				bg = th.Accent
			case click.Hovered():
				bg = th.Over(ui.Alpha(th.Muted, ghostHoverFill))
			}
			if bg != th.Surface {
				ui.Fill(gtx, bg, ui.RadiusMD)
			}
			on := th.On(bg)
			name := on.Text(ui.TextSM, v.Name)
			if !serviceUp(v) && v.State != api.ServiceFailed {
				name = name.Muted()
			}
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.Y = h
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return stateDot(gtx, on, v.State) }),
					ui.Gap(2),
					layout.Flexed(1, name.Layout),
					layout.Rigid(func(gtx C) D {
						if click.Hovered() {
							return serviceRowButtons(gtx, on, v, start, stop, restart)
						}
						return serviceReadout(gtx, on, v)
					}),
				)
			})
		})
	})
}

// serviceRowButtons are the hover actions of a row: Restart and Stop while it
// runs, Start otherwise; none while it is on its way out.
func serviceRowButtons(gtx C, th *ui.Theme, v backend.Service, start, stop, restart *widget.Clickable) D {
	switch v.State {
	case api.ServiceRunning:
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return cornerButton(gtx, th, restart, icServiceRestart, false) }),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D { return cornerButton(gtx, th, stop, icServiceStop, false) }),
		)
	case api.ServiceStopping:
		return D{}
	}
	return cornerButton(gtx, th, start, icServiceStart, false)
}

// serviceReadout is a row's right side at rest: how long it has run, or how
// it ended.
func serviceReadout(gtx C, th *ui.Theme, v backend.Service) D {
	l := th.Text(ui.TextXS, serviceStateText(v, gtx.Now)).Mono().Muted()
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
	return l.Layout(gtx)
}

// serviceStateText is a service's state in a few words: its uptime while it
// runs, its exit code once it failed.
func serviceStateText(v backend.Service, now time.Time) string {
	switch v.State {
	case api.ServiceRunning:
		return formatAge(now.Sub(time.UnixMilli(v.StartedAt)))
	case api.ServiceStopping:
		return i18n.T("sidebar.services.stopping")
	case api.ServiceFailed:
		if v.ExitCode != nil {
			return i18n.T("sidebar.services.exitCode", i18n.Args{"code": fmt.Sprint(*v.ExitCode)})
		}
		return i18n.T("sidebar.services.failed")
	}
	return i18n.T("sidebar.services.stopped")
}

// serviceMenu is a service's right-click menu.
func serviceMenu(v backend.Service, act *sidebarAction) []ui.MenuEntry {
	do := func(op serviceOp) func() { return func() { act.service = serviceRequest{op: op, id: v.ID} } }
	var entries []ui.MenuEntry
	switch v.State {
	case api.ServiceRunning:
		entries = append(entries,
			ui.MenuEntry{Label: i18n.T("sidebar.services.stop"), Icon: icServiceStop, OnClick: do(serviceStop)},
			ui.MenuEntry{Label: i18n.T("sidebar.services.restart"), Icon: icServiceRestart, OnClick: do(serviceRestart)},
		)
	case api.ServiceStopping:
		entries = append(entries, ui.MenuEntry{Label: i18n.T("sidebar.services.forceStop"), Icon: icServiceStop, Variant: ui.VariantDestructive, OnClick: do(serviceForceStop)})
	default:
		entries = append(entries, ui.MenuEntry{Label: i18n.T("sidebar.services.start"), Icon: icServiceStart, OnClick: do(serviceStart)})
	}
	entries = append(entries,
		ui.MenuEntry{Label: i18n.T("sidebar.services.showOutput"), Icon: icServiceOutput, OnClick: do(serviceShow)},
		ui.MenuEntry{Kind: ui.MenuSeparator},
		ui.MenuEntry{Label: i18n.T("sidebar.services.edit"), Icon: icMenuRename, OnClick: do(serviceEdit)},
		ui.MenuEntry{Label: i18n.T("sidebar.services.delete"), Icon: icServiceDelete, Variant: ui.VariantDestructive, Disabled: serviceUp(v), OnClick: do(serviceDelete)},
	)
	return entries
}

// addRow is the open list's last row: New service, into Settings.
func (f *servicesFooter) addRow(gtx C, th *ui.Theme) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return f.add.Layout(gtx, func(gtx C) D {
		h := gtx.Dp(ui.Space(6.5))
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
		bg := th.Surface
		if f.add.Hovered() {
			bg = th.Over(ui.Alpha(th.Muted, ghostHoverFill))
			ui.Fill(gtx, bg, ui.RadiusMD)
		}
		on := th.On(bg)
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.Y = h
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return icPlus.Layout(gtx, 14, on.MutedForeground) }),
				ui.Gap(2),
				layout.Rigid(on.Text(ui.TextSM, i18n.T("sidebar.services.newService")).Muted().Layout),
			)
		})
	})
}
