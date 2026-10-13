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
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n (palette.commandPalette,
// common.pickerDialog).
const (
	palettePlaceholder      = "Jump to a session, project or something said, or type > for actions…"
	paletteNothingClosed    = "Nothing closed yet"
	paletteNothingClosedHow = "Close a session and it waits here — its branch, its agent and its conversation — until its worktree is removed."
	paletteNoMatchesFor     = "No matches for "
	paletteNoMatchesIn      = " in "
	paletteCheckoutGone     = "checkout gone"
	paletteRelocate         = "relocate"
	paletteReopen           = "reopen"
	paletteShownOfTotal     = "%d of %d"
	paletteMatchesFormat    = "%d matches"
	paletteHintNavigate     = "navigate"
	paletteHintFilter       = "filter"
	paletteHintClose        = "close"
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
	paletteEmptyWidth     unit.Dp = 330
	paletteInputSize      unit.Sp = 15   // text-[0.9375rem]
	paletteTabSize        unit.Sp = 13   // text-[0.8125rem]
	paletteTinySize       unit.Sp = 10   // text-[0.625rem]
	paletteInputRow               = 11.5 // py-3 around a 22.5px line
	paletteGlyphSize              = 16   // size-4
	paletteEnterSize              = 14   // size-3.5
	paletteDimOpacity             = 0.4  // opacity-40, a tab with nothing in it
	paletteCountOpacity           = 0.7  // the count's opacity-70
	paletteSubOpacity             = 0.7  // text-foreground/70
	paletteDotOpacity             = 0.45 // opacity-45 around the dots
	paletteSnippetOpacity         = 0.8  // text-muted-foreground/80
	paletteNoteOpacity            = 0.7  // text-muted-foreground/70
	paletteQueryOpacity           = 0.8  // the query's text-foreground/80
)

var (
	icPaletteSearch     = icons.Lucide("search")
	icPaletteFolder     = icons.Lucide("folder")
	icPaletteFolderGone = icons.Lucide("folder-x")
	icPaletteBranch     = icons.Lucide("git-branch")
	icPaletteMessage    = icons.Lucide("message-square-text")
	icPaletteWarn       = icons.Lucide("triangle-alert")
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
				in := th.Input(&p.query, palettePlaceholder)
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
			children = append(children, ui.Gap(1))
		}
		children = append(children, layout.Rigid(func(gtx C) D { return p.pill(gtx, th, v, tab) }))
	}
	return layout.Inset{Left: ui.Space(2.5), Right: ui.Space(2.5), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

func (p *commandPalette) pill(gtx C, th *ui.Theme, v *paletteView, tab paletteTab) D {
	click := p.tabClick(tab)
	count, counted := v.counts[tab]
	current := tab == p.tab
	return click.Layout(gtx, func(gtx C) D {
		bg, fg := th.Surface, th.MutedForeground
		switch {
		case current:
			bg, fg = th.Accent, th.AccentForeground
		case click.Hovered():
			bg = th.Over(ui.Alpha(th.Accent, ghostHoverFill))
		}
		on := th.On(bg)
		if !current && counted && count == 0 {
			fg = on.Over(ui.Alpha(fg, paletteDimOpacity))
		}
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusMD) },
			func(gtx C) D {
				return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
					children := []layout.FlexChild{layout.Rigid(on.Text(paletteTabSize, string(tab)).In(fg).Leading(20).Layout)}
					if counted {
						num := on.Text(paletteTinySize, fmt.Sprint(count)).Mono().In(on.Over(ui.Alpha(fg, paletteCountOpacity)))
						children = append(children, ui.Gap(1.5), layout.Rigid(num.Layout))
					}
					return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, children...)
				})
			},
		)
	})
}

// footer is the hint bar: what the arrows, Enter, Tab and Escape do, on
// bg-black/10 under its own hairline.
func (p *commandPalette) footer(gtx C, th *ui.Theme, v *paletteView) D {
	action := "open"
	if at := p.active(len(v.rows)); at < len(v.rows) {
		action = rowVerb(v.rows[at])
	}
	fill := th.Over(ui.Alpha(color.NRGBA{A: 0xff}, paletteHintBarFill))
	on := th.On(fill)
	hints := []layout.Widget{
		paletteHint(on, paletteHintNavigate, "↑", "↓"),
		paletteHint(on, action, "↵"),
	}
	if !v.actionMode {
		hints = append(hints, paletteHint(on, paletteHintFilter, "⇥"))
	}
	hints = append(hints, paletteHint(on, paletteHintClose, "esc"))
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
		trailing = fmt.Sprintf(paletteShownOfTotal, len(g.rows), g.total)
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
				layout.Rigid(th.Text(ui.TextSM, paletteNothingClosed).Centered().Layout),
				ui.VGap(2),
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(paletteEmptyWidth))
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					l := th.Text(ui.TextSM, paletteNothingClosedHow).Muted().Leading(22.75).Centered()
					l.MaxLines = 0
					return l.Layout(gtx)
				}),
			)
		}
		parts := []layout.FlexChild{
			layout.Rigid(th.Text(ui.TextSM, paletteNoMatchesFor).Muted().Layout),
			layout.Rigid(th.Text(ui.TextSM, v.query).Mono().In(th.Over(ui.Alpha(th.Foreground, paletteQueryOpacity))).Layout),
		}
		if !v.actionMode && p.tab != tabAll {
			parts = append(parts, layout.Rigid(th.Text(ui.TextSM, paletteNoMatchesIn+strings.ToLower(string(p.tab))).Muted().Layout))
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
				return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
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
		return paletteFace{
			lead: paletteRingOf(on, ring, s.kind, s.status, s.unread, s.agent),
			text: paletteLines(on, fg, s.label, paletteMeta(on, paletteFore(on, s.projectName), paletteTail(on, s.path))),
		}
	case rowMessage:
		m := r.message
		face := paletteFace{
			lead: paletteGlyph(on, icPaletteMessage),
			text: paletteLines(on, fg, m.snippet, paletteMeta(on, paletteFore(on, m.label), paletteTail(on, m.projectName))),
		}
		if m.count > 1 {
			face.trailing = paletteSmall(on, fmt.Sprintf(paletteMatchesFormat, m.count))
		}
		return face
	case rowProject:
		return paletteFace{
			lead:     paletteGlyph(on, icPaletteFolder),
			text:     paletteLines(on, fg, r.project.Name, paletteTail(on, r.project.Path)),
			trailing: paletteSmall(on, plural(v.sessionCount[r.project.ID], "session", "sessions")),
		}
	case rowClosed:
		icon, word := icPaletteFolder, paletteReopen
		if v.missing[r.project.Path] {
			icon, word = icPaletteFolderGone, paletteRelocate
		}
		return paletteFace{
			lead:     paletteGlyph(on, icon),
			text:     paletteLines(on, fg, r.project.Name, paletteTail(on, r.project.Path)),
			trailing: paletteSmall(on, word),
		}
	case rowHistory:
		return paletteHistoryFace(on, r.history, fg, ring, now)
	case rowExternal:
		e := r.external
		dir := e.Path
		if dir == "" {
			dir = e.ProjectPath
		}
		return paletteFace{
			lead:     paletteRingOf(on, ring, e.Kind, "", false, ""),
			text:     paletteLines(on, fg, externalLabel(e), paletteMeta(on, paletteFore(on, e.ProjectName), paletteTail(on, displayPath(dir)))),
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
func paletteHistoryFace(on *ui.Theme, h paletteHistory, fg color.NRGBA, ring paletteRing, now time.Time) paletteFace {
	var meta []layout.FlexChild
	meta = append(meta, layout.Rigid(paletteFore(on, h.ProjectName)))
	switch {
	case h.gone:
		meta = append(meta, paletteDot(on), layout.Rigid(paletteIconText(on, icPaletteWarn, paletteCheckoutGone, on.ToneWait)))
	case h.branch != "":
		meta = append(meta, paletteDot(on), layout.Rigid(paletteIconText(on, icPaletteBranch, h.branch, on.MutedForeground)))
	}
	meta = append(meta, paletteDot(on), layout.Flexed(1, paletteTail(on, displayPath(h.Path))))
	lines := []layout.Widget{
		on.Text(ui.TextSM, h.Label).In(fg).Layout,
		func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, meta...) },
	}
	if h.Snippet != "" {
		lines = append(lines, on.Text(ui.TextXS, h.Snippet).In(on.Over(ui.Alpha(on.MutedForeground, paletteSnippetOpacity))).Layout)
	}
	if note := historyIndexNote(h); note != "" {
		lines = append(lines, on.Text(paletteTinySize, note).Mono().In(on.Over(ui.Alpha(on.MutedForeground, paletteNoteOpacity))).Layout)
	}
	return paletteFace{
		lead: paletteRingOf(on, ring, h.Kind, "", false, ""),
		text: func(gtx C) D {
			children := make([]layout.FlexChild, len(lines))
			for i, l := range lines {
				children[i] = layout.Rigid(l)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		},
		trailing: paletteSmall(on, agoLabel(h.ClosedAt, now)),
	}
}

// paletteLines is a title over one mono line.
func paletteLines(on *ui.Theme, fg color.NRGBA, title string, meta layout.Widget) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(on.Text(ui.TextSM, title).In(fg).Layout),
			layout.Rigid(meta),
		)
	}
}

// paletteMeta is a mono line of a shrink-0 head and a tail that truncates,
// the dot between them.
func paletteMeta(on *ui.Theme, head, tail layout.Widget) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(head), paletteDot(on), layout.Flexed(1, tail))
	}
}

func paletteDot(on *ui.Theme) layout.FlexChild {
	dot := on.Text(ui.TextXS, "·").Mono().In(on.Over(ui.Alpha(on.MutedForeground, paletteDotOpacity)))
	return layout.Rigid(func(gtx C) D {
		return layout.Inset{Left: ui.Space(1.5), Right: ui.Space(1.5)}.Layout(gtx, dot.Layout)
	})
}

// paletteFore is a mono name in text-foreground/70, which leads a meta line.
func paletteFore(on *ui.Theme, s string) layout.Widget {
	return on.Text(ui.TextXS, s).Mono().In(on.Over(ui.Alpha(on.Foreground, paletteSubOpacity))).Layout
}

// paletteTail is a mono text-muted-foreground run.
func paletteTail(on *ui.Theme, s string) layout.Widget {
	return on.Text(ui.TextXS, s).Mono().Muted().Layout
}

// paletteSmall is the trailing note: font-mono text-[0.625rem] muted.
func paletteSmall(on *ui.Theme, s string) layout.Widget {
	return on.Text(paletteTinySize, s).Mono().Muted().Layout
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
