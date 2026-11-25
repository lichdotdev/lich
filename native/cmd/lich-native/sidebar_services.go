package main

import (
	"fmt"
	"image"
	"strconv"
	"strings"
	"time"

	"gioui.org/font"

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

// servicesFooter is the Services block above the profile: a group header
// that opens and closes it, and the project's services under it.
type servicesFooter struct {
	open       bool
	header     ui.Clickable
	more       ui.Clickable
	headerMenu ui.MenuState // the header's …
	headerCtx  ui.MenuState // the header's right click
	all        ui.Clickable
	allTip     ui.Tooltip
	add        ui.Clickable
	list       widget.List
	rows       map[string]*ui.Clickable
	mores      map[string]*ui.Clickable
	menus      map[string]*ui.MenuState // a row's right click
	moreMenus  map[string]*ui.MenuState // a row's …
}

func newServicesFooter() servicesFooter {
	f := servicesFooter{
		rows: map[string]*ui.Clickable{}, mores: map[string]*ui.Clickable{},
		menus: map[string]*ui.MenuState{}, moreMenus: map[string]*ui.MenuState{},
	}
	f.list.Axis = layout.Vertical
	return f
}

// holdsKeys is whether one of the block's menus is open, which takes the keys.
func (f *servicesFooter) holdsKeys() bool {
	if f.headerMenu.Opened() || f.headerCtx.Opened() {
		return true
	}
	for _, states := range []map[string]*ui.MenuState{f.menus, f.moreMenus} {
		for _, m := range states {
			if m.Opened() {
				return true
			}
		}
	}
	return false
}

func menuOf(states map[string]*ui.MenuState, id string) *ui.MenuState {
	m, ok := states[id]
	if !ok {
		m = new(ui.MenuState)
		states[id] = m
	}
	return m
}

// layout is the footer's Services block: the hairline that parts it from
// the sessions, the header that opens and closes it, then the open list, or
// New service while there is none. console is the service whose console is
// on screen.
func (f *servicesFooter) layout(gtx C, th *ui.Theme, root *ui.Root, projectID string, list []backend.Service, console string, act *sidebarAction) D {
	if f.header.Clicked(gtx) {
		f.open = !f.open
	}
	if f.add.Clicked(gtx) {
		act.service = serviceRequest{op: serviceAdd, project: projectID}
	}
	tally := tallyServices(list)
	children := []layout.FlexChild{
		ui.VGap(2),
		layout.Rigid(func(gtx C) D { return f.headerRow(gtx, th, root, projectID, tally, act) }),
	}
	switch {
	case f.open && len(list) == 0:
		children = append(children, layout.Rigid(func(gtx C) D { return f.addRow(gtx, th) }))
	case f.open:
		children = append(children, layout.Rigid(func(gtx C) D { return f.rowList(gtx, th, root, list, console, act) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// rowHeight is a sidebar row's height: the node's box and its inset above and
// below.
func rowHeight(gtx C) int { return gtx.Dp(2*sessionTreeRingR) + 2*gtx.Dp(ui.Space(1.5)) }

// headerRow is the block's header as a folder's is drawn: the glyph in the
// node's column, turning to the fold's chevron under the pointer, SERVICES,
// how many run of how many, a rule to the …, whose entries the right click
// opens too.
func (f *servicesFooter) headerRow(gtx C, th *ui.Theme, root *ui.Root, projectID string, t servicesTally, act *sidebarAction) D {
	entries := func() []ui.MenuEntry { return servicesEntries(t, projectID, act) }
	h := rowHeight(gtx)
	lit := f.header.Hovered() || f.more.Hovered() || f.headerMenu.Opened() || f.headerCtx.Opened()
	fill := cardLayer(th, "", false, lit)
	return th.ContextMenuLazy(root, &f.headerCtx, entries).Layout(gtx, func(gtx C) D {
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, fill, ui.RadiusMD) },
			func(gtx C) D {
				return layout.Inset{Right: ui.Space(1)}.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.Y = h
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D {
							gtx.Constraints.Min.Y = h
							return f.header.Layout(gtx, func(gtx C) D {
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								return servicesTitle(gtx, th, t, f.open, f.header.Hovered())
							})
						}),
						ui.Gap(1),
						layout.Rigid(func(gtx C) D {
							return th.DropdownMenu(root, &f.headerMenu, entries()).Layout(gtx, func(gtx C) D {
								btn := th.Button(&f.more, "")
								btn.Icon, btn.Variant, btn.Size = icGroupMore, ui.VariantGhost, ui.SizeIconXS
								return btn.Layout(gtx)
							})
						}),
					)
				})
			},
		)
	})
}

// servicesTitle is the header's line: the glyph, SERVICES and how many run
// and failed, in their tones.
func servicesTitle(gtx C, th *ui.Theme, t servicesTally, open, hovered bool) D {
	fg := th.Over(ui.Alpha(th.MutedForeground, headerTitleOpacity))
	glyph := icServices
	if hovered {
		fg, glyph = th.MutedForeground, foldIcon(open)
	}

	return layout.Inset{Left: rowInset, Right: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(sessionTreeRingR)*2, gtx.Dp(ui.Space(3))))
				return ui.CenteredIcon(gtx, glyph, ui.Space(3), fg)
			}),
			ui.Gap(1.5),
			layout.Rigid(th.Text(ui.Text2XS, strings.ToUpper(i18n.T("sidebar.services.title"))).Leading(text2XSLeading).In(fg).Weight(font.SemiBold).Layout),
			ui.Gap(1.5),
			layout.Flexed(1, servicesTallyLine(th, t)),
		)
	})
}

// servicesTallyLine is how many services run, in TonePass, and failed, in
// Destructive; "stopped" muted when none does either, Add when there are none.
func servicesTallyLine(th *ui.Theme, t servicesTally) layout.Widget {
	muted := th.Over(ui.Alpha(th.MutedForeground, headerCountOpacity))
	var parts []toned
	if t.up > 0 {
		parts = append(parts, toned{text: i18n.T("sidebar.services.runningCount", i18n.Args{"count": t.up}), ink: th.TonePass})
	}
	if t.failed > 0 {
		parts = append(parts, toned{text: i18n.T("sidebar.services.failedCount", i18n.Args{"count": t.failed}), ink: th.Destructive})
	}
	switch {
	case t.total == 0:
		parts = []toned{{text: i18n.T("sidebar.services.add"), ink: muted}}
	case len(parts) == 0:
		parts = []toned{{text: i18n.T("sidebar.services.stoppedCount", i18n.Args{"count": t.total}), ink: muted}}
	}
	return func(gtx C) D {
		row := make([]layout.FlexChild, 0, 2*len(parts))
		for i, p := range parts {
			text := p.text
			if i > 0 {
				text = " · " + text
			}
			row = append(row, layout.Rigid(th.Text(ui.Text2XS, text).Leading(text2XSLeading).In(p.ink).Weight(font.Medium).Layout))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
	}
}

// servicesEntries is the header's menu: start what is down, stop what runs,
// and add a service.
func servicesEntries(t servicesTally, projectID string, act *sidebarAction) []ui.MenuEntry {
	do := func(op serviceOp) func() { return func() { act.service = serviceRequest{op: op, project: projectID} } }
	var entries []ui.MenuEntry
	if t.up < t.total {
		entries = append(entries, ui.MenuEntry{Label: i18n.T("sidebar.services.startAll"), Icon: icServiceStart, OnClick: do(serviceStartAll)})
	}
	if t.up > 0 {
		entries = append(entries, ui.MenuEntry{Label: i18n.T("sidebar.services.stopAll"), Icon: icServiceStop, OnClick: do(serviceStopAll)})
	}
	if len(entries) > 0 {
		entries = append(entries, ui.MenuEntry{Kind: ui.MenuSeparator})
	}
	return append(entries, ui.MenuEntry{Label: i18n.T("sidebar.services.newService"), Icon: icPlus, OnClick: do(serviceAdd)})
}

func (f *servicesFooter) rowList(gtx C, th *ui.Theme, root *ui.Root, list []backend.Service, console string, act *sidebarAction) D {
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, rowHeight(gtx)*serviceRowsShown)
	// The thumb floats over the rows, as the rail's does: a reserved gutter
	// would end every row short of the cards above it.
	rows := th.ScrollArea(&f.list)
	rows.AnchorStrategy = material.Overlay
	return rows.Layout(gtx, len(list), func(gtx C, i int) D {
		return f.row(gtx, th, root, list[i], list[i].ID == console, act)
	})
}

// row is one service as a session's row is drawn: its glyph where a
// session's mark sits, the name, muted while it is down, how long it has run
// or how it failed, and the corner: the … under the pointer, the spinner while
// it stops. Its actions are its menu, from the … or the right click; a click
// shows its console.
func (f *servicesFooter) row(gtx C, th *ui.Theme, root *ui.Root, v backend.Service, shown bool, act *sidebarAction) D {
	click, more := clickable(f.rows, v.ID), clickable(f.mores, v.ID)
	if click.Clicked(gtx) && !more.Hovered() {
		act.service = serviceRequest{op: serviceShow, id: v.ID}
	}
	moreMenu := menuOf(f.moreMenus, v.ID)
	entries := func() []ui.MenuEntry { return serviceMenu(v, act) }
	lit := click.Hovered() || moreMenu.Opened()
	fill := cardLayer(th, "", shown, lit)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return th.ContextMenuLazy(root, menuOf(f.menus, v.ID), entries).Layout(gtx, func(gtx C) D {
		return click.Layout(gtx, func(gtx C) D {
			return layout.Background{}.Layout(gtx,
				func(gtx C) D { return ui.Fill(gtx, fill, ui.RadiusMD) },
				func(gtx C) D {
					on := th.On(th.Over(fill))
					name, glyph := on.Text(ui.TextSM, v.Name), on.Foreground
					if !serviceUp(v) && v.State != api.ServiceFailed {
						name, glyph = name.Muted(), on.MutedForeground
					}
					return layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5), Left: rowInset, Right: ui.Space(1)}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								side := gtx.Dp(2 * sessionTreeRingR)
								gtx.Constraints = layout.Exact(image.Pt(side, side))
								return ui.CenteredIcon(gtx, icServiceOutput, 13, glyph)
							}),
							ui.Gap(1.5),
							layout.Flexed(1, name.Layout),
							layout.Rigid(func(gtx C) D { return serviceReadout(gtx, on, v) }),
							ui.Gap(1),
							layout.Rigid(func(gtx C) D { return serviceCorner(gtx, on, root, v, more, moreMenu, lit, entries) }),
						)
					})
				},
			)
		})
	})
}

// serviceCorner is a row's corner slot: the … under the pointer or while its
// menu is open, the spinner while the service stops, empty otherwise.
func serviceCorner(gtx C, th *ui.Theme, root *ui.Root, v backend.Service, more *ui.Clickable, menu *ui.MenuState, lit bool, entries func() []ui.MenuEntry) D {
	size := gtx.Dp(cardSlotSize)
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	switch {
	case lit:
		return layout.Center.Layout(gtx, func(gtx C) D {
			return th.DropdownMenu(root, menu, entries()).Layout(gtx, func(gtx C) D {
				return th.CornerButton(more, icGroupMore, menu.Opened()).Layout(gtx)
			})
		})
	case v.State == api.ServiceStopping:
		return pixelSpinner(gtx, th)
	}
	return D{Size: gtx.Constraints.Min}
}

// serviceReadout is a row's end: how long it has run, or how it failed; a
// stopped service says nothing, its muted name already does.
func serviceReadout(gtx C, th *ui.Theme, v backend.Service) D {
	l := th.Text(ui.TextXS, serviceStateText(v, gtx.Now)).Mono().Muted()
	switch v.State {
	case api.ServiceRunning:
		elapsed := gtx.Now.Sub(time.UnixMilli(v.StartedAt))
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(nextAgeChange(elapsed))})
	case api.ServiceFailed:
		l.Color = th.Destructive
	case api.ServiceStopped:
		return D{}
	}
	return l.Layout(gtx)
}

// allServicesOp is what the header's all button does: start what is down,
// or stop everything once all of it runs.
func allServicesOp(t servicesTally) serviceOp {
	if t.total > 0 && t.up == t.total {
		return serviceStopAll
	}
	return serviceStartAll
}

// allButton starts every service that is down, or stops them all once all
// of them run: the icon of what it does, the words in its tooltip.
func (f *servicesFooter) allButton(gtx C, th *ui.Theme, root *ui.Root, t servicesTally) D {
	ic, hint := icServiceStart, i18n.T("sidebar.services.startAll")
	if allServicesOp(t) == serviceStopAll {
		ic, hint = icServiceStop, i18n.T("sidebar.services.stopAll")
	}
	tip := th.Tooltip(root, &f.allTip, hint)
	return tip.Layout(gtx, func(gtx C) D { return th.CornerButton(&f.all, ic, false).Layout(gtx) })
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

// stateDot is a service state as a 14dp glyph.
func stateDot(gtx C, th *ui.Theme, state string) D {
	switch state {
	case api.ServiceRunning:
		return ui.StatusDot(gtx, th.TonePass)
	case api.ServiceStopping:
		return ui.StatusDot(gtx, th.ToneWait)
	case api.ServiceFailed:
		return ui.StatusDot(gtx, th.Destructive)
	}
	return ui.HollowDot(gtx, th.MutedForeground)
}

// serviceStateText is a service's state in a few words: its uptime while it
// runs, its exit code once it failed.
func serviceStateText(v backend.Service, now time.Time) string {
	switch v.State {
	case api.ServiceRunning:
		return formatAge(now.Sub(time.UnixMilli(v.StartedAt)))
	case api.ServiceStopping:
		// the spinner says it is stopping
		return ""
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
