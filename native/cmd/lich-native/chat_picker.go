package main

import (
	"image"
	"runtime"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

var (
	pickerRowH    = ui.Space(8)
	pickerModelW  = ui.Space(75)
	pickerEffortW = ui.Space(52)
	// pickerChrome is what the panel adds to its columns: the hairline between
	// them and the popover's p-1 either side.
	pickerChrome = unit.Dp(9)
	// pickerMaxH is the tallest the panel grows: past it the models scroll.
	pickerMaxH = unit.Dp(420)
	// pickerBarW, pickerBarGap and pickerBarStep draw the signal glyph of an
	// effort: bars three wide, two apart, each two taller than the one before.
	pickerBarW    = ui.Space(0.75)
	pickerBarGap  = ui.Space(0.5)
	pickerBarBase = ui.Space(1.5)
	pickerBarStep = ui.Space(0.75)
)

const (
	// pickerRows is how many models show before the rest go behind Show more.
	pickerRows = 6
	// pickerKeys is how many models answer to Ctrl+1, Ctrl+2...
	pickerKeys = 4
	// pickerBars is the bars of the signal glyph, however many levels the
	// provider offers.
	pickerBars = 5
)

// modelPicker is one chat's model and effort panel, kept across frames.
type modelPicker struct {
	panel    ui.DropdownPanelState
	query    widget.Editor
	lastTerm string
	expanded bool
	fresh    bool
	cursor   int
	rows     []ui.Clickable
	efforts  []ui.Clickable
	more     ui.Clickable
	// list scrolls the models; moved says the keyboard just moved the cursor,
	// which is to be kept in view.
	list  widget.List
	moved bool
}

// pickerVisible is the models the panel lists, as positions in choices: those
// whose name holds the search, or with none the first pickerRows while the
// list is folded, which it says it is.
func pickerVisible(choices []backend.ChatChoice, term string, expanded bool) (rows []int, folded bool) {
	term = strings.ToLower(strings.TrimSpace(term))
	for i, c := range choices {
		if term == "" || strings.Contains(strings.ToLower(choiceLabel(c)), term) {
			rows = append(rows, i)
		}
	}
	if term == "" && !expanded && len(rows) > pickerRows {
		return rows[:pickerRows], true
	}
	return rows, false
}

// pickerLit is how many of pickerBars the effort at position i of n lights: one
// for the first, all for the last, the rest between.
func pickerLit(i, n int) int {
	if n <= 1 {
		return pickerBars
	}
	return 1 + i*(pickerBars-1)/(n-1)
}

// pickerDefault is whether to mark a model "Default": the one the provider
// names default by id, when its name does not say so already.
func pickerDefault(c backend.ChatChoice) bool {
	return strings.EqualFold(c.ID, "default") && !strings.Contains(strings.ToLower(choiceLabel(c)), "default")
}

// keys carries out the keys the search takes while the panel is open: the
// arrows walk the models, Enter takes the one under the cursor, Ctrl+1..4 the
// n-th shown, Escape closes. It answers the position taken, -1 for none.
func (pk *modelPicker) keys(gtx C, shown int) (take int) {
	take = -1
	var filters []event.Filter
	for _, name := range []key.Name{key.NameUpArrow, key.NameDownArrow, key.NameReturn, key.NameEnter, key.NameEscape} {
		filters = append(filters, key.Filter{Focus: &pk.query, Name: name})
	}
	for n := 1; n <= pickerKeys; n++ {
		filters = append(filters, key.Filter{Focus: &pk.query, Required: key.ModShortcut, Name: key.Name(string(rune('0' + n)))})
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return take
		}
		e, isKey := ev.(key.Event)
		if !isKey || e.State != key.Press {
			continue
		}
		switch {
		case e.Name == key.NameDownArrow:
			pk.cursor, pk.moved = min(pk.cursor+1, max(shown-1, 0)), true
		case e.Name == key.NameUpArrow:
			pk.cursor, pk.moved = max(pk.cursor-1, 0), true
		case e.Name == key.NameReturn || e.Name == key.NameEnter:
			take = pk.cursor
		case e.Name == key.NameEscape:
			pk.panel.Close()
		case e.Modifiers.Contain(key.ModShortcut):
			if n := int(e.Name[0] - '1'); n < shown {
				take = n
			}
		}
	}
}

// modelPanel is what the model chip opens: the models in a column with a
// search over them, and the effort in another, a hairline between. Choosing a
// model leaves the panel open for the effort; choosing an effort closes it.
func (u *window) modelPanel(gtx C, th *ui.Theme, p *chatPane, sv sessionView) D {
	pk := &p.picker
	model, hasModel := u.model.chat.optionOf(sv.ID, api.OptionModel)
	effort, hasEffort := u.model.chat.optionOf(sv.ID, api.OptionEffort)
	if pk.fresh {
		pk.fresh, pk.expanded, pk.lastTerm, pk.cursor = false, false, "", 0
		pk.query.SetText("")
		if i := positionOf(model, model.Current); hasModel && i >= 0 {
			pk.cursor = i
		}
	}
	if hasModel && !gtx.Focused(&pk.query) {
		gtx.Execute(key.FocusCmd{Tag: &pk.query})
	}
	var columns []layout.Widget
	if hasModel {
		columns = append(columns, func(gtx C) D { return u.modelColumn(gtx, th, pk, sv, model) })
	}
	if hasEffort {
		columns = append(columns, func(gtx C) D { return u.effortColumn(gtx, th, pk, sv, effort, model) })
	}
	return pickerColumns(gtx, th, columns, hasModel)
}

// pickerColumns lays the columns side by side, each at its width, with a 1px
// hairline between and as tall as the tallest.
func pickerColumns(gtx C, th *ui.Theme, columns []layout.Widget, hasModel bool) D {
	type placed struct {
		call op.CallOp
		size image.Point
	}
	var cols []placed
	h := 0
	for i, w := range columns {
		width := pickerEffortW
		if hasModel && i == 0 {
			width = pickerModelW
		}
		cg := gtx
		cg.Constraints = layout.Exact(image.Pt(gtx.Dp(width), 0))
		cg.Constraints.Max.Y = gtx.Constraints.Max.Y
		m := op.Record(gtx.Ops)
		d := w(cg)
		cols = append(cols, placed{m.Stop(), d.Size})
		h = max(h, d.Size.Y)
	}
	x := 0
	for i, c := range cols {
		if i > 0 {
			paint.FillShape(gtx.Ops, th.Over(th.Border), clip.Rect{Min: image.Pt(x, 0), Max: image.Pt(x+gtx.Dp(1), h)}.Op())
			x += gtx.Dp(1)
		}
		t := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
		c.call.Add(gtx.Ops)
		t.Pop()
		x += c.size.X
	}
	return D{Size: image.Pt(x, h)}
}

func positionOf(o backend.ChatOption, id string) int {
	for i, c := range o.Choices {
		if c.ID == id {
			return i
		}
	}
	return -1
}

// modelColumn is the search and the models.
func (u *window) modelColumn(gtx C, th *ui.Theme, pk *modelPicker, sv sessionView, model backend.ChatOption) D {
	name := sv.Kind
	if info, ok := u.model.providers.find(sv.Kind); ok && info.name != "" {
		name = info.name
	}
	// Clicks and keys come before the rows are laid out, so a frame that
	// takes one draws what it changed.
	if pk.more.Clicked(gtx) {
		pk.expanded = true
	}
	term := pk.query.Text()
	if term != pk.lastTerm {
		pk.lastTerm, pk.cursor = term, 0
	}
	shown, folded := pickerVisible(model.Choices, term, pk.expanded)
	pk.cursor = min(pk.cursor, max(len(shown)-1, 0))
	for len(pk.rows) < len(model.Choices) {
		pk.rows = append(pk.rows, ui.Clickable{})
	}
	for pos, i := range shown {
		if pk.rows[pos].Clicked(gtx) {
			pk.cursor = pos
			u.setChatOption(sv.ID, model.ID, model.Choices[i].ID)
		}
	}
	if take := pk.keys(gtx, len(shown)); take >= 0 && take < len(shown) {
		pk.cursor = take
		u.setChatOption(sv.ID, model.ID, model.Choices[shown[take]].ID)
	}
	isMac := runtime.GOOS == "darwin"
	var items []layout.Widget
	for pos, i := range shown {
		c := model.Choices[i]
		click := &pk.rows[pos]
		current := c.ID == model.Current
		items = append(items, func(gtx C) D {
			return pickerRow(gtx, th, click, pos == pk.cursor, func(gtx C, th *ui.Theme) D {
				row := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, choiceLabel(c)).Layout)}
				if pickerDefault(c) {
					row = append(row, ui.Gap(2), layout.Rigid(th.Text(ui.TextXS, i18n.T("chat.model.default")).Muted().Layout))
				}
				row = append(row, layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }))
				if pos < pickerKeys {
					row = append(row, layout.Rigid(th.KeyCap(formatCombo(combo{Mod: true, Key: string(rune('1' + pos))}, isMac)).Layout), ui.Gap(2))
				}
				row = append(row, layout.Rigid(pickerCheck(th, current)))
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
			})
		})
	}
	if folded {
		items = append(items, func(gtx C) D {
			return pickerRow(gtx, th, &pk.more, false, func(gtx C, th *ui.Theme) D {
				return th.Text(ui.TextSM, i18n.T("chat.model.showMore")).Muted().Layout(gtx)
			})
		})
	}
	// A cursor the keyboard moved is kept in view.
	pk.list.Axis = layout.Vertical
	if pos := pk.list.Position; pk.moved && pos.Count > 0 {
		switch {
		case pk.cursor < pos.First:
			pk.list.ScrollTo(pk.cursor)
		case pk.cursor >= pos.First+pos.Count-1:
			pk.list.ScrollTo(pk.cursor - pos.Count + 2)
		}
	}
	pk.moved = false
	// The search stays where it is and the models scroll under it, in what the
	// panel's height leaves.
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			// a row of the panel, as tall as the effort's title beside it, so the
			// two hairlines under them meet
			in := th.Input(&pk.query, i18n.T("chat.model.search", i18n.Args{"provider": name}))
			in.Bare, in.Height = true, pickerRowH
			return in.Layout(gtx)
		}),
		layout.Rigid(th.Separator),
		ui.VGap(1),
		layout.Rigid(func(gtx C) D {
			return th.ThinScrollArea(&pk.list).Layout(gtx, len(items), func(gtx C, i int) D { return items[i](gtx) })
		}),
	)
}

// effortColumn is the effort's title, with the model in force beside it, and
// a row per level under a signal glyph that rises with it.
func (u *window) effortColumn(gtx C, th *ui.Theme, pk *modelPicker, sv sessionView, effort, model backend.ChatOption) D {
	for len(pk.efforts) < len(effort.Choices) {
		pk.efforts = append(pk.efforts, ui.Clickable{})
	}
	for i, c := range effort.Choices {
		if pk.efforts[i].Clicked(gtx) {
			u.setChatOption(sv.ID, effort.ID, c.ID)
			pk.panel.Close()
		}
	}
	current, _ := currentChoice(model)
	head := func(gtx C) D {
		gtx.Constraints.Min.Y = gtx.Dp(pickerRowH)
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			row := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, optionTitle(effort)).Weight(500).Layout), layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} })}
			if model.Current != "" {
				row = append(row, layout.Rigid(th.Text(ui.TextXS, choiceLabel(current)).Muted().Layout))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
		})
	}
	children := []layout.FlexChild{layout.Rigid(head), layout.Rigid(th.Separator), ui.VGap(1)}
	for i, c := range effort.Choices {
		click := &pk.efforts[i]
		lit, now := pickerLit(i, len(effort.Choices)), c.ID == effort.Current
		children = append(children, layout.Rigid(func(gtx C) D {
			return pickerRow(gtx, th, click, false, func(gtx C, th *ui.Theme) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return pickerBarsGlyph(gtx, th, lit) }),
					ui.Gap(2),
					layout.Rigid(th.Text(ui.TextSM, choiceLabel(c)).Layout),
					layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
					layout.Rigid(pickerCheck(th, now)),
				)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// pickerRow is a row of the panel: pickerRowH tall, Accent under the pointer
// or the keyboard's cursor.
func pickerRow(gtx C, th *ui.Theme, click *ui.Clickable, cursor bool, content func(gtx C, th *ui.Theme) D) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, func(gtx C) D {
		bg := th.Surface
		if click.Hovered() || cursor {
			bg = th.Accent
		}
		gtx.Constraints.Min.Y = gtx.Dp(pickerRowH)
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusSM) },
			func(gtx C) D {
				return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.Y = gtx.Dp(pickerRowH)
					return layout.W.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return content(gtx, th.On(bg))
					})
				})
			},
		)
	})
}

// pickerCheck is the mark of the choice in force, or the room it takes.
func pickerCheck(th *ui.Theme, on bool) layout.Widget {
	return func(gtx C) D {
		if !on {
			return D{Size: image.Pt(gtx.Dp(footerGlyph), gtx.Dp(footerGlyph))}
		}
		return icCheck.Layout(gtx, footerGlyph, th.Foreground)
	}
}

// pickerBarsGlyph is the signal glyph of a level: pickerBars bars of rising
// height, the first lit of them in Foreground and the rest in MutedForeground.
func pickerBarsGlyph(gtx C, th *ui.Theme, lit int) D {
	w, gap := gtx.Dp(pickerBarW), gtx.Dp(pickerBarGap)
	base, step := gtx.Dp(pickerBarBase), gtx.Dp(pickerBarStep)
	height := base + (pickerBars-1)*step
	muted := th.Over(ui.Alpha(th.MutedForeground, 0.5))
	for i := range pickerBars {
		c := muted
		if i < lit {
			c = th.Foreground
		}
		h := base + i*step
		x := i * (w + gap)
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(x, height-h), Max: image.Pt(x+w, height)}.Op())
	}
	return D{Size: image.Pt(pickerBars*w+(pickerBars-1)*gap, height)}
}
