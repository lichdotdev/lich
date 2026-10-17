package main

import (
	"image"
	"image/color"
	"maps"
	"math"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// text2XS is index.css --text-2xs (0.65625rem), the uppercase label that
// heads a group of rows; it sets no line height, so the 16px of text-xs's is
// taken for the row.
const (
	text2XS        unit.Sp = 10.5
	text2XSLeading unit.Sp = 16
)

// SessionGroupHeader's tints: the title's text-muted-foreground/70, the
// count's /60, the title button's hover:bg-accent/50, the drop target's
// bg-tone-pass/15 and a refusing header's opacity-45.
const (
	headerTitleOpacity  = 0.7
	headerCountOpacity  = 0.6
	headerHoverFill     = 0.5
	headerDropFill      = 0.15
	headerRefuseOpacity = 0.45
)

// valueMarker stands for a value the message renders in a weight of its own
// (a folder's typed name, a peer): the message splits around it.
const valueMarker = "\x00"

// aroundValue is the text before and after the value in the message under key
// at {placeholder}; rest fills the message's other placeholders.
func aroundValue(key, placeholder string, rest ...i18n.Args) (before, after string) {
	args := i18n.Args{placeholder: valueMarker}
	for _, r := range rest {
		maps.Copy(args, r)
	}
	before, after, _ = strings.Cut(i18n.T(key, args), valueMarker)
	return before, after
}

var (
	icGroupChevron = icons.Lucide("chevron-right")
	icGroupFolder  = icons.Lucide("folder")
	icGroupMore    = icons.Lucide("ellipsis")
)

// groupHeader is SessionGroupHeader: `flex flex-wrap items-center gap-1 px-1
// pb-0.5 pt-1.5`, the title (or the folder's rename field), the + and the …,
// and under them the note a rename onto another folder earns. While a card is
// carried, a header that takes it is a drop target; every other one fades.
func (s *sidebar) groupHeader(gtx C, th *ui.Theme, b blockView, st *blockState, f blockFrame) D {
	takes, refuses := s.headerDrop(b, f.view)
	if st.toggle.Clicked(gtx) && !f.headerDragEnded {
		f.act.fold = b.key
	}
	row := func(gtx C) D {
		return layout.Inset{Left: ui.Space(1), Right: ui.Space(1), Top: ui.Space(1.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return s.headerLine(gtx, th, b, st, f, refuses) }),
				layout.Rigid(func(gtx C) D { return s.mergeNote(gtx, th, b, st, f.view.folders) }),
			)
		})
	}
	if !takes {
		return row(gtx)
	}
	return s.carry.Target(gtx, b.key, row)
}

// headerDrop is how the header answers the card in the air: whether it takes
// it, and whether it refuses it (file-drag-store.ts dropState).
func (s *sidebar) headerDrop(b blockView, view sidebarView) (takes, refuses bool) {
	carried := s.carry.Carrying()
	if carried == "" {
		return false, false
	}
	card, ok := carriedCard(view.blocks, carried)
	takes = ok && takesCard(b, card)
	return takes, !takes
}

func (s *sidebar) headerLine(gtx C, th *ui.Theme, b blockView, st *blockState, f blockFrame, refuses bool) D {
	title := func(gtx C) D {
		if st.rename.Active() {
			return s.folderRenameField(gtx, th, b, st, f.act)
		}
		over := s.carry.Over() == b.key
		w := func(gtx C) D { return s.titleButton(gtx, th, b, st, over, refuses) }
		if b.kind == blockPinned || f.filtering {
			return w(gtx)
		}
		return s.sort.Item(gtx, b.key, w, nil)
	}
	children := []layout.FlexChild{layout.Flexed(1, title)}
	act := f.act
	if entries := blockLaunch(b, s.launchProviders, act); entries != nil {
		children = append(children, ui.Gap(1), layout.Rigid(func(gtx C) D {
			return s.headerMenu(gtx, th, &st.launch, &st.plus, icPlus, i18n.T("sidebar.sessionGroupHeader.newSessionIn", i18n.Args{"name": b.title}), entries)
		}))
	}
	hooks := blockMenuHooks{rename: func() { st.rename.Start(b.folder) }, newFolder: s.askFolderName}
	if entries := blockOptions(b, f.view.folders, f.filtering, act, hooks); entries != nil {
		children = append(children, ui.Gap(1), layout.Rigid(func(gtx C) D {
			return s.headerMenu(gtx, th, &st.options, &st.more, icGroupMore, i18n.T("sidebar.sessionGroupHeader.optionsFor", i18n.Args{"name": b.title}), entries)
		}))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// headerMenu is a ghost icon-xs trigger under its Hint, opening entries.
func (s *sidebar) headerMenu(gtx C, th *ui.Theme, state *ui.MenuState, click *widget.Clickable, ic *icons.Icon, hint string, entries []ui.MenuEntry) D {
	return th.DropdownMenu(s.root, state, entries).Layout(gtx, func(gtx C) D {
		return th.Tooltip(s.root, s.hint(hint), hint).Layout(gtx, func(gtx C) D {
			btn := th.Button(click, "")
			btn.Icon, btn.Variant, btn.Size = ic, ui.VariantGhost, ui.SizeIconXS
			return btn.Layout(gtx)
		})
	})
}

// titleButton is SessionGroupTitleButton: `-ml-1 flex flex-1 items-center
// gap-1.5 rounded-sm px-1 py-0.5`, the chevron, a folder's glyph and count,
// the folded block's mark, then the rule to the row's end.
func (s *sidebar) titleButton(gtx C, th *ui.Theme, b blockView, st *blockState, over, refuses bool) D {
	hint := i18n.T("sidebar.sessionGroupHeader.collapse", i18n.Args{"name": b.title})
	if b.collapsed {
		hint = i18n.T("sidebar.sessionGroupHeader.expand", i18n.Args{"name": b.title})
	}
	return layout.Inset{Left: -ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return th.Tooltip(s.root, s.hint("fold:"+b.key), hint).Layout(gtx, func(gtx C) D {
			return st.toggle.Layout(gtx, func(gtx C) D {
				var bg color.NRGBA
				switch {
				case over:
					bg = th.Over(ui.Alpha(th.TonePass, headerDropFill))
				case st.toggle.Hovered():
					bg = th.Over(ui.Alpha(th.Accent, headerHoverFill))
				}
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Background{}.Layout(gtx,
					func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusSM) },
					func(gtx C) D { return s.titleContent(gtx, th, b, st.toggle.Hovered(), refuses) },
				)
			})
		})
	})
}

func (s *sidebar) titleContent(gtx C, th *ui.Theme, b blockView, hovered, refuses bool) D {
	fade := func(c color.NRGBA, opacity float32) color.NRGBA {
		if refuses {
			opacity *= headerRefuseOpacity
		}
		return th.Over(ui.Alpha(c, opacity))
	}
	fg := fade(th.MutedForeground, headerTitleOpacity)
	if hovered {
		fg = fade(th.MutedForeground, 1)
	}
	kids := []layout.FlexChild{layout.Rigid(func(gtx C) D { return chevron(gtx, fg, !b.collapsed) }), ui.Gap(1.5)}
	if b.kind == blockFolder {
		glyph := fade(th.MutedForeground, headerTitleOpacity)
		if c, ok := cardColors[b.color]; ok {
			glyph = fade(c, 1)
		}
		kids = append(kids, layout.Rigid(func(gtx C) D { return icGroupFolder.Layout(gtx, ui.Space(3), glyph) }), ui.Gap(1.5))
	}
	kids = append(kids, layout.Rigid(th.Text(text2XS, strings.ToUpper(b.title)).Leading(text2XSLeading).In(fg).Weight(font.SemiBold).Layout))
	if b.kind == blockFolder {
		count := th.Text(text2XS, strconv.Itoa(len(b.sessions))).Leading(text2XSLeading).In(fade(th.MutedForeground, headerCountOpacity)).Weight(font.Medium)
		kids = append(kids, ui.Gap(1.5), layout.Rigid(count.Layout))
	}
	if dot, ok := markColor(th, b); ok {
		kids = append(kids, ui.Gap(1.5), layout.Rigid(func(gtx C) D { return markDot(gtx, fade(dot, 1)) }))
	}
	kids = append(kids, ui.Gap(1.5), layout.Flexed(1, th.Separator))
	return layout.Inset{Left: ui.Space(1), Right: ui.Space(1), Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
	})
}

// markColor is the folded block's dot: amber for a session waiting on the
// user, emerald for a turn nobody read.
func markColor(th *ui.Theme, b blockView) (color.NRGBA, bool) {
	if !b.collapsed {
		return color.NRGBA{}, false
	}
	switch b.mark {
	case markWait:
		return th.ToneWait, true
	case markDone:
		return th.TonePass, true
	}
	return color.NRGBA{}, false
}

// markDot is `size-1.5 rounded-full`.
func markDot(gtx C, c color.NRGBA) D {
	size := gtx.Dp(ui.Space(1.5))
	paint.FillShape(gtx.Ops, c, clip.Ellipse{Max: image.Pt(size, size)}.Op(gtx.Ops))
	return D{Size: image.Pt(size, size)}
}

// chevron is the size-3 ChevronRight, turned down (rotate-90) while open.
// ponytail: turns at once, where the web eases it over 150ms.
func chevron(gtx C, c color.NRGBA, open bool) D {
	size := gtx.Dp(ui.Space(3))
	if open {
		center := f32.Pt(float32(size)/2, float32(size)/2)
		defer op.Affine(f32.Affine2D{}.Rotate(center, math.Pi/2)).Push(gtx.Ops).Pop()
	}
	return icGroupChevron.Layout(gtx, ui.Space(3), c)
}

// folderRenameField is the header's input while a folder is renamed: the
// title alone swaps for it, so the menu it was chosen from stays mounted.
func (s *sidebar) folderRenameField(gtx C, th *ui.Theme, b blockView, st *blockState, act *sidebarAction) D {
	return layout.Inset{Left: -ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Inset{Left: ui.Space(1), Right: ui.Space(1), Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
			d, res := th.InlineEdit(&st.rename, text2XS).Weight(font.SemiBold).Layout(gtx)
			if res.Outcome == ui.InlineEditCommitted {
				act.folder = folderRename{from: b.folder, to: res.Value}
			}
			return d
		})
	})
}

// mergeNote is `basis-full px-1 text-xs text-muted-foreground`, the typed
// name in font-medium foreground.
func (s *sidebar) mergeNote(gtx C, th *ui.Theme, b blockView, st *blockState, folders []string) D {
	typed := strings.TrimSpace(st.rename.Text())
	if b.kind != blockFolder || !st.rename.Active() || !mergesInto(b.folder, typed, folders) {
		return D{}
	}
	before, after := aroundValue("sidebar.sessionGroupHeader.mergesInto", "name")
	rest := th.Text(ui.TextXS, after).Muted()
	rest.MaxLines = 0
	return layout.Inset{Left: ui.Space(1), Right: ui.Space(1), Top: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextXS, before).Muted().Layout),
			layout.Rigid(th.Text(ui.TextXS, typed).Weight(font.Medium).Layout),
			layout.Flexed(1, rest.Layout),
		)
	})
}
