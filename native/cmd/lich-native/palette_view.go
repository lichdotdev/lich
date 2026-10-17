package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// The geometry of PickerDialog's popup and rows.
const (
	paletteWidth unit.Dp = 640 // max-w-[40rem]
	// paletteTop is top-[14vh].
	paletteTop = 0.14
	// paletteScrim is bg-black/50.
	paletteScrim = 0.5
	// paletteMaxHeight is max-h-[70vh], in percent of the window.
	paletteMaxHeight = 70
	// paletteHintBarFill is the hint bar's bg-black/10.
	paletteHintBarFill = 0.1
	// paletteEmptyWidth is the hint's max-w-[44ch], in dp.
	paletteEmptyWidth unit.Dp = 330
	paletteInputSize  unit.Sp = 15 // text-[0.9375rem]
	paletteTabSize    unit.Sp = 13 // text-[0.8125rem]
	paletteTinySize   unit.Sp = 10 // text-[0.625rem]
	// paletteTinyLeading is what the tiny text inherits from its text-sm row
	// on the web: the 20/14 line height, at 10px.
	paletteTinyLeading    unit.Sp = 14.3
	paletteInputRow               = 11.5 // py-3 around a 22.5px line
	paletteGlyphSize              = 16   // size-4
	paletteEnterSize              = 14   // size-3.5
	paletteTabGap                 = 4.5  // between two filter tabs
	paletteTabUnderline   unit.Dp = 1.5  // under the current filter tab
	paletteDimOpacity             = 0.4  // opacity-40, a tab with nothing in it
	paletteCountOpacity           = 0.7  // the count's opacity-70
	paletteSnippetOpacity         = 0.8  // text-muted-foreground/80
	paletteQueryOpacity           = 0.8  // the query's text-foreground/80
	paletteChipOpacity            = 0.4  // a chip's border, its tone at 40%
)

var (
	icPaletteSearch     = icons.Lucide("search")
	icPaletteFolder     = icons.Lucide("folder")
	icPaletteFolderGone = icons.Lucide("folder-x")
	icPaletteBranch     = icons.Lucide("git-branch")
	icPaletteMessage    = icons.Lucide("message-square-text")
	icPaletteAction     = icons.Lucide("chevron-right")
	icPaletteEnter      = icons.Lucide("corner-down-left")
)

// paletteRing draws a session's status mark: the provider's glyph in the ring
// a card wears.
type paletteRing func(gtx C, th *ui.Theme, sv *sessionView) D

// recorded is a widget laid out and held, so the panel can size its parts
// before it places any of them.
type recorded struct {
	call op.CallOp
	size image.Point
}

func record(gtx C, w layout.Widget) recorded {
	macro := op.Record(gtx.Ops)
	d := w(gtx)
	return recorded{call: macro.Stop(), size: d.Size}
}

func (r recorded) place(gtx C, y int) {
	defer op.Offset(image.Pt(0, y)).Push(gtx.Ops).Pop()
	r.call.Add(gtx.Ops)
}

// panel is the popup of PickerDialog: the field and the filters over the
// list, the hint bar under it. The list takes what the field, the filters and
// the bar leave of 70% of the window, and no more than its rows need, so the
// panel is as tall as its content.
func (p *commandPalette) panel(gtx C, th *ui.Theme, v *paletteView, ring paletteRing, now time.Time) D {
	width := gtx.Constraints.Max.X
	gtx.Constraints.Min = image.Pt(width, 0)
	limit := gtx.Constraints.Max.Y * paletteMaxHeight / 100
	head := record(gtx, func(gtx C) D { return p.header(gtx, th, v) })
	foot := record(gtx, func(gtx C) D { return p.footer(gtx, th, v) })
	room := gtx
	room.Constraints.Max.Y = max(limit-head.size.Y-foot.size.Y, 0)
	listed := record(room, func(gtx C) D { return p.body(gtx, th, v, ring, now) })
	head.place(gtx, 0)
	listed.place(gtx, head.size.Y)
	foot.place(gtx, head.size.Y+listed.size.Y)
	return D{Size: image.Pt(width, head.size.Y+listed.size.Y+foot.size.Y)}
}

// header is the query field over its hairline, and the filter tabs over
// theirs: an action search has no kinds to filter, so no tabs.
func (p *commandPalette) header(gtx C, th *ui.Theme, v *paletteView) D {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return p.field(gtx, th) }),
		layout.Rigid(th.Separator),
	}
	if !v.actionMode {
		children = append(children,
			layout.Rigid(func(gtx C) D { return p.filters(gtx, th, v) }),
			layout.Rigid(th.Separator),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// field is px-4 gap-3 over a search glyph and the bare input.
func (p *commandPalette) field(gtx C, th *ui.Theme) D {
	return layout.Inset{Left: ui.Space(4), Right: ui.Space(4)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icPaletteSearch.Layout(gtx, ui.Space(4), th.MutedForeground) }),
			// The input carries its own px-2.5; the gap makes up the rest of gap-3.
			ui.Gap(0.5),
			layout.Flexed(1, func(gtx C) D {
				in := th.Input(&p.query, i18n.T("palette.commandPalette.placeholder"))
				in.Bare, in.TextSize, in.Height = true, paletteInputSize, ui.Space(paletteInputRow)
				return in.Layout(gtx)
			}),
		)
	})
}

// filters is the tab row. A tab with no match is dimmed rather than dropped:
// a row of tabs that reshuffles as the query changes costs more to aim at than
// the space it saves.
func (p *commandPalette) filters(gtx C, th *ui.Theme, v *paletteView) D {
	var children []layout.FlexChild
	for i, tab := range paletteTabs {
		if i > 0 {
			children = append(children, ui.Gap(paletteTabGap))
		}
		children = append(children, layout.Rigid(func(gtx C) D { return p.filterTab(gtx, th, v, tab) }))
	}
	return layout.Inset{Left: ui.Space(4), Right: ui.Space(4)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

// filterTab is one filter: its name and count, the current one in the foreground
// and underlined, a hovered one brought up to the foreground.
func (p *commandPalette) filterTab(gtx C, th *ui.Theme, v *paletteView, tab paletteTab) D {
	click := p.tabClick(tab)
	count, counted := v.counts[tab]
	current := tab == p.tab
	return click.Layout(gtx, func(gtx C) D {
		fg := th.MutedForeground
		switch {
		case current, click.Hovered():
			fg = th.Foreground
		case counted && count == 0:
			fg = th.Over(ui.Alpha(fg, paletteDimOpacity))
		}
		d := layout.Inset{Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			children := []layout.FlexChild{layout.Rigid(th.Text(paletteTabSize, tab.label()).In(fg).Leading(20).Layout)}
			if counted {
				num := th.Text(paletteTinySize, fmt.Sprint(count)).Leading(paletteTinyLeading).Mono().In(th.Over(ui.Alpha(fg, paletteCountOpacity)))
				children = append(children, ui.Gap(1), layout.Rigid(num.Layout))
			}
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, children...)
		})
		if current {
			line := gtx.Dp(paletteTabUnderline)
			paint.FillShape(gtx.Ops, th.Foreground, clip.Rect{Min: image.Pt(0, d.Size.Y-line), Max: d.Size}.Op())
		}
		return d
	})
}

// footer is the hint bar: what the arrows, Enter, Tab and Escape do, on
// bg-black/10 under its own hairline.
func (p *commandPalette) footer(gtx C, th *ui.Theme, v *paletteView) D {
	action := i18n.T("palette.palette.verbOpen")
	if at := p.active(len(v.rows)); at < len(v.rows) {
		action = rowVerb(v.rows[at])
	}
	fill := th.Over(ui.Alpha(color.NRGBA{A: 0xff}, paletteHintBarFill))
	on := th.On(fill)
	hints := []layout.Widget{
		paletteHint(on, i18n.T("common.pickerDialog.navigate"), "↑", "↓"),
		paletteHint(on, action, "↵"),
	}
	if !v.actionMode {
		hints = append(hints, paletteHint(on, i18n.T("common.pickerDialog.filter"), "⇥"))
	}
	hints = append(hints, paletteHint(on, i18n.T("common.pickerDialog.close"), "esc"))
	var children []layout.FlexChild
	for i, h := range hints {
		if i > 0 {
			children = append(children, ui.Gap(4))
		}
		children = append(children, layout.Rigid(h))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Separator),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Background{}.Layout(gtx,
				func(gtx C) D { return ui.Fill(gtx, fill, 0) },
				func(gtx C) D {
					return layout.Inset{Left: ui.Space(4), Right: ui.Space(4), Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
					})
				},
			)
		}),
	)
}

// paletteHint is Hint: the key caps, gap-1, then what they do, gap-1.5.
func paletteHint(th *ui.Theme, label string, keys ...string) layout.Widget {
	return func(gtx C) D {
		var children []layout.FlexChild
		for i, k := range keys {
			if i > 0 {
				children = append(children, ui.Gap(1))
			}
			children = append(children, layout.Rigid(func(gtx C) D { return keyCap(gtx, th, k) }))
		}
		children = append(children, ui.Gap(1.5), layout.Rigid(th.Text(ui.TextXS, label).Muted().Layout))
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	}
}

// paletteItem is one line of the list: a group's heading, or one of its rows.
type paletteItem struct {
	group int
	row   int // flat row index, -1 for a heading
}

func paletteItems(v *paletteView) []paletteItem {
	var items []paletteItem
	for gi, g := range v.groups {
		items = append(items, paletteItem{group: gi, row: -1})
		for i := range g.rows {
			items = append(items, paletteItem{group: gi, row: v.offsets[gi] + i})
		}
	}
	return items
}

// body is the rows under their group headings, or what stands in for them.
func (p *commandPalette) body(gtx C, th *ui.Theme, v *paletteView, ring paletteRing, now time.Time) D {
	if len(v.rows) == 0 {
		return p.empty(gtx, th, v)
	}
	if len(p.rows) < len(v.rows) {
		p.rows = make([]widget.Clickable, len(v.rows))
	}
	items := paletteItems(v)
	p.keepInView(items, v)
	gtx.Constraints.Min.Y = 0
	return layout.UniformInset(ui.Space(1.5)).Layout(gtx, func(gtx C) D {
		return th.ScrollArea(&p.list).Layout(gtx, len(items), func(gtx C, i int) D {
			it := items[i]
			if it.row < 0 {
				return p.heading(gtx, th, v.groups[it.group])
			}
			return p.row(gtx, th, v, it.row, ring, now)
		})
	})
}

// keepInView scrolls the cursor's row into the list when the keys moved it,
// its group's heading with it when it is the first of its group.
func (p *commandPalette) keepInView(items []paletteItem, v *paletteView) {
	if !p.reveal {
		return
	}
	p.reveal = false
	at := p.active(len(v.rows))
	idx := -1
	for i, it := range items {
		if it.row == at {
			idx = i
		}
	}
	if idx < 0 {
		return
	}
	first := idx
	if idx > 0 && items[idx-1].row < 0 {
		first = idx - 1
	}
	pos := p.list.Position
	switch {
	case first < pos.First:
		p.list.ScrollTo(first)
	case pos.Count > 0 && idx >= pos.First+max(pos.Count-1, 1):
		p.list.ScrollTo(max(idx-pos.Count+2, 0))
	}
}

// heading is PickerGroup's label: px-3 pb-1 pt-3, uppercase, and opposite it
// what the group is holding back.
func (p *commandPalette) heading(gtx C, th *ui.Theme, g paletteGroup) D {
	trailing := g.note
	if trailing == "" && g.total > len(g.rows) {
		trailing = i18n.T("palette.commandPalette.shownOfTotal", i18n.Args{"shown": len(g.rows), "total": g.total})
	}
	return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(3), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		children := []layout.FlexChild{
			layout.Flexed(1, th.Text(text2XS, strings.ToUpper(g.label)).Muted().Weight(font.SemiBold).Leading(text2XSLeading).Layout),
		}
		if trailing != "" {
			children = append(children, layout.Rigid(th.Text(text2XS, trailing).Mono().Muted().Leading(text2XSLeading).Layout))
		}
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, children...)
	})
}

// empty is PickerEmpty. Two different empties, deliberately not sharing a
// sentence: one says the query found nothing, the other says nothing has ever
// been closed, and states the retention rule where somebody wondering what
// this remembers will read it.
func (p *commandPalette) empty(gtx C, th *ui.Theme, v *paletteView) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(8), Bottom: ui.Space(8)}.Layout(gtx, func(gtx C) D {
		if v.emptyHistory {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextSM, i18n.T("palette.commandPalette.nothingClosed")).Centered().Layout),
				ui.VGap(2),
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(paletteEmptyWidth))
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					l := th.Text(ui.TextSM, i18n.T("palette.commandPalette.nothingClosedHint")).Muted().Leading(22.75).Centered()
					l.MaxLines = 0
					return l.Layout(gtx)
				}),
			)
		}
		key, rest := "palette.commandPalette.noMatches", i18n.Args{}
		if !v.actionMode && p.tab != tabAll {
			key, rest = "palette.commandPalette.noMatchesInTab", i18n.Args{"tab": strings.ToLower(p.tab.label())}
		}
		before, after := aroundValue(key, "query", rest)
		parts := []layout.FlexChild{
			layout.Rigid(th.Text(ui.TextSM, before).Muted().Layout),
			layout.Rigid(th.Text(ui.TextSM, v.query).Mono().In(th.Over(ui.Alpha(th.Foreground, paletteQueryOpacity))).Layout),
			layout.Rigid(th.Text(ui.TextSM, after).Muted().Layout),
		}
		return layout.Center.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, parts...)
		})
	})
}

// row is PickerRow: rounded-md px-3 py-2 gap-3, accent while the cursor is on
// it, and the corner arrow that says Enter will run it.
func (p *commandPalette) row(gtx C, th *ui.Theme, v *paletteView, i int, ring paletteRing, now time.Time) D {
	row := v.rows[i]
	click := &p.rows[i]
	selected := i == p.active(len(v.rows))
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, func(gtx C) D {
		bg, fg := th.Surface, th.Foreground
		if selected {
			bg, fg = th.Accent, th.AccentForeground
		}
		on := th.On(bg)
		face := paletteFaceOf(on, v, row, fg, ring, now)
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusMD) },
			func(gtx C) D {
				return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
					children := []layout.FlexChild{
						layout.Rigid(face.lead), ui.Gap(3),
						layout.Flexed(1, face.text),
					}
					if face.trailing != nil {
						children = append(children, ui.Gap(3), layout.Rigid(face.trailing))
					}
					children = append(children, ui.Gap(3), layout.Rigid(func(gtx C) D {
						size := gtx.Dp(paletteEnterSize)
						if selected {
							return icPaletteEnter.Layout(gtx, paletteEnterSize, on.MutedForeground)
						}
						return D{Size: image.Pt(size, size)}
					}))
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
				})
			},
		)
	})
}

// paletteFace is what a row says of itself: its leading mark, its lines and
// what trails them.
type paletteFace struct {
	lead     layout.Widget
	text     layout.Widget
	trailing layout.Widget
}

func paletteFaceOf(on *ui.Theme, v *paletteView, r paletteRow, fg color.NRGBA, ring paletteRing, now time.Time) paletteFace {
	switch r.kind {
	case rowSession:
		s := r.session
		face := paletteFace{
			lead: paletteRingOf(on, ring, s.kind, s.status, s.unread, s.agent),
			text: paletteLine(on, fg, s.label, shortPath(s.path)),
		}
		if s.status == statusWaiting {
			face.trailing = paletteChip(on, i18n.T("session.filter.phase.waiting"), on.ToneWait)
		}
		return face
	case rowMessage:
		m := r.message
		face := paletteFace{
			lead: paletteGlyph(on, icPaletteMessage),
			text: paletteLine(on, fg, m.snippet, m.label),
		}
		if m.count > 1 {
			face.trailing = paletteSmall(on, i18n.T("palette.commandPalette.matches", i18n.Args{"count": m.count}))
		}
		return face
	case rowProject:
		return paletteFace{
			lead:     paletteGlyph(on, icPaletteFolder),
			text:     paletteLine(on, fg, r.project.Name, shortPath(r.project.Path)),
			trailing: paletteSmall(on, i18n.T("palette.commandPalette.sessions", i18n.Args{"count": v.sessionCount[r.project.ID]})),
		}
	case rowClosed:
		icon, word := icPaletteFolder, i18n.T("palette.commandPalette.reopen")
		if v.missing[r.project.Path] {
			icon, word = icPaletteFolderGone, i18n.T("palette.commandPalette.relocate")
		}
		return paletteFace{
			lead:     paletteGlyph(on, icon),
			text:     paletteLine(on, fg, r.project.Name, shortPath(r.project.Path)),
			trailing: paletteSmall(on, word),
		}
	case rowHistory:
		return paletteHistoryFace(on, r.history, v.query, fg, ring, now)
	case rowExternal:
		e := r.external
		dir := e.Path
		if dir == "" {
			dir = e.ProjectPath
		}
		return paletteFace{
			lead:     paletteRingOf(on, ring, e.Kind, "", false, ""),
			text:     paletteLine(on, fg, externalLabel(e), shortPath(dir)),
			trailing: paletteSmall(on, agoLabel(e.UpdatedAt, now)),
		}
	}
	return paletteFace{
		lead: paletteGlyph(on, icPaletteAction),
		text: func(gtx C) D { return on.Text(ui.TextSM, hotkeyLabel(r.action)).In(fg).Layout(gtx) },
		trailing: func(gtx C) D {
			if r.keys == hotkeyUnassigned {
				return on.Text(ui.TextXS, r.keys).Muted().Layout(gtx)
			}
			return keyCap(gtx, on, r.keys)
		},
	}
}

// paletteHistoryFace names a closed session the way its card did, plus the two
// facts a card never needed: the branch, because a worktree keeps the name it
// was created with while the branch moves on, and when it was closed. The mark
// carries no ring: nothing runs in a closed session, and its absence is what
// tells a history row from a live one at a glance.
//
// A checkout that is gone replaces the branch rather than sitting beside it:
// there is no branch to read off a directory that is not there, and it is the
// same absence that makes the row unresumable.
//
// Under a query a second line says why the row matched: the conversation's
// snippet with the query's words marked, and the note that only the newest
// part of a large transcript was searched.
func paletteHistoryFace(on *ui.Theme, h paletteHistory, query string, fg color.NRGBA, ring paletteRing, now time.Time) paletteFace {
	dir := h.Path
	if dir == "" {
		dir = h.ProjectPath
	}
	title := paletteLine(on, fg, h.Label, shortPath(dir))
	var why []layout.FlexChild
	if h.Snippet != "" {
		why = append(why, layout.Flexed(1, paletteSnippet(on, h.Snippet, query)))
	}
	if note := historyIndexNote(h); note != "" && query != "" {
		why = append(why, layout.Rigid(paletteSmall(on, note)))
	}
	text := title
	if len(why) > 0 {
		text = func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(title),
				layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, why...) }),
			)
		}
	}
	mark := paletteIconText(on, icPaletteBranch, h.branch, on.MutedForeground)
	if h.gone {
		mark = paletteChip(on, i18n.T("palette.commandPalette.checkoutGone"), on.ToneWait)
	}
	return paletteFace{
		lead: paletteRingOf(on, ring, h.Kind, "", false, ""),
		text: text,
		trailing: func(gtx C) D {
			children := []layout.FlexChild{}
			if h.gone || h.branch != "" {
				children = append(children, layout.Rigid(mark), ui.Gap(2.5))
			}
			children = append(children, layout.Rigid(paletteSmall(on, agoLabel(h.ClosedAt, now))))
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		},
	}
}

// paletteSnippet is one line of what was said, the query's words in the
// foreground over the muted rest.
func paletteSnippet(on *ui.Theme, snippet, query string) layout.Widget {
	return func(gtx C) D {
		segs := querySegments(snippet, query)
		kids := make([]layout.FlexChild, len(segs))
		for i, seg := range segs {
			l := on.Text(ui.TextXS, seg.text).In(on.Over(ui.Alpha(on.MutedForeground, paletteSnippetOpacity)))
			if seg.hit {
				l = on.Text(ui.TextXS, seg.text).In(on.Foreground).Weight(font.Medium)
			}
			l.MaxLines = 1
			kids[i] = layout.Rigid(l.Layout)
		}
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, kids...)
	}
}

// paletteLine is a title and, after it on the same line, a muted mono note
// that gives up its start when the row is too narrow, as a card's path does.
func paletteLine(on *ui.Theme, fg color.NRGBA, title, note string) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(on.Text(ui.TextSM, title).In(fg).Layout),
			ui.Gap(2.5),
			layout.Flexed(1, on.Text(ui.TextXS, note).Mono().Muted().LayoutTail),
		)
	}
}

// paletteChip is an outlined word in a tone: the "waiting" a session waiting
// on the user trails with, the "checkout gone" of a history row.
func paletteChip(on *ui.Theme, word string, tone color.NRGBA) layout.Widget {
	return func(gtx C) D {
		macro := op.Record(gtx.Ops)
		gtx.Constraints.Min = image.Point{}
		d := layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx,
			on.Text(paletteTinySize, word).Leading(paletteTinyLeading).Mono().In(tone).Layout)
		call := macro.Stop()
		insetBorder(gtx, d.Size, d.Size.Y/2, on.Over(ui.Alpha(tone, paletteChipOpacity)))
		call.Add(gtx.Ops)
		return d
	}
}

// paletteSmall is the trailing note: font-mono text-[0.625rem] muted.
func paletteSmall(on *ui.Theme, s string) layout.Widget {
	return on.Text(paletteTinySize, s).Leading(paletteTinyLeading).Mono().Muted().Layout
}

func paletteGlyph(on *ui.Theme, icon *icons.Icon) layout.Widget {
	return func(gtx C) D { return icon.Layout(gtx, paletteGlyphSize, on.MutedForeground) }
}

// paletteIconText is a size-3 glyph and its words, gap-1.
func paletteIconText(on *ui.Theme, icon *icons.Icon, s string, c color.NRGBA) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icon.Layout(gtx, ui.Space(3), c) }),
			ui.Gap(1),
			layout.Rigid(on.Text(ui.TextXS, s).Mono().In(c).Layout),
		)
	}
}

func paletteRingOf(on *ui.Theme, ring paletteRing, kind, status string, unread bool, agent string) layout.Widget {
	sv := &sessionView{Session: lichclient.Session{Kind: kind}, status: status, unread: unread, agent: agent}
	return func(gtx C) D { return ring(gtx, on, sv) }
}
