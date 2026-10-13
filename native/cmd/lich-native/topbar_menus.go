package main

import (
	"image"
	"image/color"
	"strconv"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n.
const (
	openProjectHint     = "Open project"
	recentProjectsLabel = "Recent projects"
	relocateMark        = "relocate"
	openFolderLabel     = "Open folder…"
	notificationsTitle  = "Notifications"
	caughtUpLabel       = "You're all caught up"
)

// recentLimit is OpenProjectMenu's MENU_LIMIT: what fits above the picker
// entry without turning the menu into a second project list.
const recentLimit = 5

// menuWidth is both menus' w-72.
var menuWidth = ui.Space(72)

var (
	icFolderPlain = icons.Lucide("folder")
	icFolderGone  = icons.Lucide("folder-x")
	icFolderOpen  = icons.Lucide("folder-open")
)

// openMenu is OpenProjectMenu: the projects closed earlier, newest first,
// above the directory picker. With none to offer the "+" is the picker.
type openMenu struct {
	popover ui.DropdownPanelState
	plus    widget.Clickable
	folder  widget.Clickable
	rows    []widget.Clickable

	// closed is the window's store of closed projects, shared with the
	// command palette.
	closed *closedProjects
}

// snapshot is the menu's page of the closed projects and which of their
// directories are gone.
func (o *openMenu) snapshot() ([]lichclient.RecentProject, map[string]bool) {
	if o.closed == nil {
		return nil, nil
	}
	answer := o.closed.read("")
	return answer.rows[:min(len(answer.rows), recentLimit)], answer.missing
}

// openButton is the "+" right after the last tab.
func (t *topbar) openButton(gtx C, th *ui.Theme, act *topbarAction) D {
	o := &t.open
	recents, missing := o.snapshot()
	tip := th.Tooltip(t.root, t.hint("open"), openProjectHint)
	tip.Side = ui.SideBottom
	if len(recents) == 0 {
		if o.plus.Clicked(gtx) {
			act.pick = true
		}
		return tip.Layout(gtx, func(gtx C) D { return barIconButton(gtx, th, &o.plus, icPlus, false) })
	}
	if len(o.rows) < len(recents) {
		o.rows = make([]widget.Clickable, len(recents))
	}
	for i := range recents {
		if o.rows[i].Clicked(gtx) {
			act.reopen = &recents[i]
			o.popover.Close()
		}
	}
	if o.folder.Clicked(gtx) {
		act.pick = true
		o.popover.Close()
	}
	content := func(gtx C, th *ui.Theme) D { return o.content(gtx, th, recents, missing) }
	pop := th.DropdownPanel(t.root, &o.popover, menuWidth, content)
	return tip.Layout(gtx, func(gtx C) D {
		return pop.Layout(gtx, func(gtx C) D { return barIconButton(gtx, th, &o.plus, icPlus, o.popover.Opened()) })
	})
}

func (o *openMenu) content(gtx C, th *ui.Theme, recents []lichclient.RecentProject, missing map[string]bool) D {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return menuHeader(gtx, th, recentProjectsLabel, "") }),
		layout.Rigid(menuSeparator(th)),
	}
	for i, r := range recents {
		children = append(children, layout.Rigid(func(gtx C) D { return recentRow(gtx, th, &o.rows[i], r, missing[r.Path]) }))
	}
	children = append(children,
		layout.Rigid(menuSeparator(th)),
		layout.Rigid(func(gtx C) D {
			return popoverRow(gtx, th, &o.folder, func(gtx C, th *ui.Theme) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return icFolderOpen.Layout(gtx, ui.Space(4), th.MutedForeground) }),
					ui.Gap(2),
					layout.Rigid(th.Text(ui.TextSM, openFolderLabel).Layout),
				)
			})
		}),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func recentRow(gtx C, th *ui.Theme, click *widget.Clickable, r lichclient.RecentProject, gone bool) D {
	return popoverRow(gtx, th, click, func(gtx C, th *ui.Theme) D {
		ic := icFolderPlain
		if gone {
			ic = icFolderGone
		}
		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return ic.Layout(gtx, ui.Space(4), th.MutedForeground) }),
			ui.Gap(2),
			layout.Flexed(1, twoLines(th, r.Name, displayPath(r.Path))),
		}
		if gone {
			children = append(children, ui.Gap(2), layout.Rigid(th.Text(10, relocateMark).Muted().Mono().Layout))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

// twoLines is a row's `flex min-w-0 flex-col`: the label, and a muted
// text-xs line under it, each truncated.
func twoLines(th *ui.Theme, top, under string) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextSM, top).Layout),
			layout.Rigid(th.Text(ui.TextXS, under).Muted().Layout),
		)
	}
}

// menuHeader is the menus' plain-div title: `px-2 py-1.5 text-xs
// font-semibold`, and a count on the right when there is one.
func menuHeader(gtx C, th *ui.Theme, title, count string) D {
	return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, th.Text(ui.TextXS, title).Weight(font.SemiBold).Layout),
			layout.Rigid(th.Text(ui.TextXS, count).Muted().Layout),
		)
	})
}

// menuSeparator is DropdownMenuSeparator: -mx-1 my-1 h-px.
func menuSeparator(th *ui.Theme) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, th.Separator)
	}
}

// popoverRow is a DropdownMenuItem drawn by hand: px-2 py-1.5, rounded-sm,
// bg-accent while the pointer is on it.
func popoverRow(gtx C, th *ui.Theme, click *widget.Clickable, content func(gtx C, th *ui.Theme) D) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, func(gtx C) D {
		bg := th.Surface
		if click.Hovered() {
			bg = th.Accent
		}
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusSM) },
			func(gtx C) D {
				return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
					return content(gtx, th.On(bg))
				})
			},
		)
	})
}

// bellMenu is NotificationsButton: the cross-project queue of sessions
// waiting on the user or holding an unread finished turn.
type bellMenu struct {
	popover ui.DropdownPanelState
	trigger widget.Clickable
	rows    map[string]*widget.Clickable
	dismiss map[string]*widget.Clickable
}

func (t *topbar) bellButton(gtx C, th *ui.Theme, items []notification, act *topbarAction) D {
	b := &t.bell
	if b.rows == nil {
		b.rows, b.dismiss = map[string]*widget.Clickable{}, map[string]*widget.Clickable{}
	}
	for _, item := range items {
		dismissed := clickableOf(b.dismiss, item.ID).Clicked(gtx)
		chosen := clickableOf(b.rows, item.ID).Clicked(gtx)
		if dismissed {
			act.dismiss = item.ID
		} else if chosen {
			act.notify = &item
			b.popover.Close()
		}
	}
	tip := th.Tooltip(t.root, t.hint("bell"), notificationsTitle)
	tip.Side = ui.SideBottom
	pop := th.DropdownPanel(t.root, &b.popover, menuWidth, func(gtx C, th *ui.Theme) D { return b.content(gtx, th, items) })
	pop.AlignEnd = true
	return tip.Layout(gtx, func(gtx C) D {
		return pop.Layout(gtx, func(gtx C) D {
			d := barIconButton(gtx, th, &b.trigger, icBell, b.popover.Opened())
			if len(items) > 0 {
				pendingDot(gtx, th, d.Size)
			}
			return d
		})
	})
}

// pendingDot is the red dot on the bell, ringed in the canvas it sits on.
func pendingDot(gtx C, th *ui.Theme, button image.Point) {
	inset, dot, ring := gtx.Dp(5), gtx.Dp(8), gtx.Dp(2)
	center := f32.Pt(float32(button.X-inset-dot/2), float32(inset+dot/2))
	circle := func(radius int, c color.NRGBA) {
		r := float32(radius)
		box := image.Rect(int(center.X-r), int(center.Y-r), int(center.X+r), int(center.Y+r))
		paint.FillShape(gtx.Ops, c, clip.Ellipse(box).Op(gtx.Ops))
	}
	circle(dot/2+ring, th.Surface)
	circle(dot/2, th.Destructive)
}

func (b *bellMenu) content(gtx C, th *ui.Theme, items []notification) D {
	count := ""
	if len(items) > 0 {
		count = strconv.Itoa(len(items))
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return menuHeader(gtx, th, notificationsTitle, count) }),
		layout.Rigid(menuSeparator(th)),
	}
	if len(items) == 0 {
		children = append(children, layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: ui.Space(6), Bottom: ui.Space(6)}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Center.Layout(gtx, th.Text(ui.TextXS, caughtUpLabel).Muted().Layout)
			})
		}))
	}
	for _, item := range items {
		children = append(children, layout.Rigid(func(gtx C) D { return b.row(gtx, th, item) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// row is a queue row: the status glyph, the session over its project, and
// the × that dismisses it, shown while the row is highlighted.
func (b *bellMenu) row(gtx C, th *ui.Theme, item notification) D {
	click, x := b.rows[item.ID], b.dismiss[item.ID]
	return popoverRow(gtx, th, click, func(gtx C, th *ui.Theme) D {
		ic, tone := icTabDone, th.TonePass
		if item.Status == statusWaiting {
			ic, tone = icTabWaiting, th.ToneWait
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return ic.Layout(gtx, ui.Space(4), tone) }),
			ui.Gap(2),
			layout.Flexed(1, twoLines(th, item.sessionLabel, item.projectName)),
			ui.Gap(2),
			layout.Rigid(func(gtx C) D {
				if !click.Hovered() && !x.Hovered() {
					return D{Size: image.Pt(gtx.Dp(16), gtx.Dp(16))}
				}
				return cornerButton(gtx, th, x, icTabClose, false)
			}),
		)
	})
}

func clickableOf(m map[string]*widget.Clickable, id string) *widget.Clickable {
	c, ok := m[id]
	if !ok {
		c = new(widget.Clickable)
		m[id] = c
	}
	return c
}
