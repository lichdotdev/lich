package main

// The FilePreview's find bar: Ctrl+F on the code opens it under the header,
// and the view lights every match of what it holds, the current one selected.

import (
	"unicode/utf8"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icFindCase     = icons.Lucide("case-sensitive")
	icFindWord     = icons.Lucide("whole-word")
	icFindRegexp   = icons.Lucide("regex")
	icFindAll      = icons.Lucide("list")
	icFindPrevious = icons.Lucide("chevron-up")
	icFindNext     = icons.Lucide("chevron-down")
	icFindClose    = icons.Lucide("x")
)

const keyFind = "F"

type findBar struct {
	open bool
	// focusField selects the field's text and focuses it on the next frame.
	focusField bool
	query      widget.Editor

	matchCase, wholeWord, regexp widget.Bool
	listing                      widget.Bool
	previous, next, close        widget.Clickable
	caseTip, wordTip, regexpTip  ui.Tooltip
	listTip                      ui.Tooltip
	previousTip, nextTip         ui.Tooltip
	closeTip                     ui.Tooltip
	list                         findList

	// on is the view the search was set on and applied what it was set to: a
	// new view (a font change) gets the search again.
	on      *godemirror.View
	applied godemirror.SearchQuery
	invalid bool
}

func newFindBar() *findBar {
	f := &findBar{list: newFindList()}
	f.query.SingleLine = true
	return f
}

// keys polls the bar's chords; it runs before the view and the field lay out,
// so their own handlers never see them.
func (f *findBar) keys(gtx C, view *godemirror.View) {
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: view, Name: keyFind, Required: key.ModShortcut},
			key.Filter{Focus: &f.query, Name: keyFind, Required: key.ModShortcut},
			key.Filter{Focus: &f.query, Name: key.NameReturn, Optional: key.ModShift},
			key.Filter{Focus: &f.query, Name: key.NameEscape},
		)
		if !ok {
			break
		}
		e, isKey := ev.(key.Event)
		if !isKey || e.State != key.Press {
			continue
		}
		switch {
		case e.Name == keyFind:
			f.show(view)
		case e.Name == key.NameEscape:
			f.hide(gtx, view)
		case e.Modifiers.Contain(key.ModShift):
			view.FindPrevious()
		default:
			view.FindNext()
		}
	}
	f.clicks(gtx, view)
}

// holdsKeys is whether the bar's field or a Find all row has the keyboard.
func (f *findBar) holdsKeys(gtx C) bool {
	if gtx.Focused(&f.query) {
		return true
	}
	for i := range f.list.clicks {
		if gtx.Focused(&f.list.clicks[i]) {
			return true
		}
	}
	return false
}

func (f *findBar) clicks(gtx C, view *godemirror.View) {
	if f.previous.Clicked(gtx) {
		view.FindPrevious()
	}
	if f.next.Clicked(gtx) {
		view.FindNext()
	}
	if f.close.Clicked(gtx) {
		f.hide(gtx, view)
	}
}

// show opens the bar on its field, holding the selection when it is one line
// of text, as an editor's find does; a regexp is kept, since the text it
// found is not what it is.
func (f *findBar) show(view *godemirror.View) {
	f.open, f.focusField = true, true
	if from, to, ok := view.Selection(); ok && from.Line == to.Line && !f.regexp.Value {
		f.query.SetText(view.SelectedText())
	}
}

func (f *findBar) hide(gtx C, view *godemirror.View) {
	f.open = false
	f.applied = godemirror.SearchQuery{}
	_ = view.SetSearch(f.applied) // an empty query is never an error
	gtx.Execute(key.FocusCmd{Tag: view})
}

// apply sets what the bar holds on view once it changed, landing on the first
// match from where the selection starts: typing on refines the match under
// way instead of jumping past it.
func (f *findBar) apply(view *godemirror.View) {
	q := godemirror.SearchQuery{Text: f.query.Text(), CaseSensitive: f.matchCase.Value, WholeWord: f.wholeWord.Value, Regexp: f.regexp.Value}
	if view == f.on && q == f.applied {
		return
	}
	f.on, f.applied = view, q
	f.invalid = view.SetSearch(q) != nil
	if len(view.Matches()) == 0 {
		return
	}
	var at godemirror.Pos
	if from, _, ok := view.Selection(); ok {
		at = from
	}
	findFrom(view, at)
}

// findFrom selects the first match starting at or after at. FindNext reads
// from the end of the selection, so the selection is first made to end at at:
// one rune long, the one before it, or none at the top of the document.
func findFrom(view *godemirror.View, at godemirror.Pos) {
	view.ClearSelection()
	switch {
	case at.Col > 0:
		view.Select(godemirror.Pos{Line: at.Line, Col: at.Col - 1}, at)
	case at.Line > 0:
		above := at.Line - 1
		view.Select(godemirror.Pos{Line: above, Col: utf8.RuneCountInString(view.LineText(above))}, at)
	}
	view.FindNext()
}

// status is what the bar says of the search: where the current match sits
// among them, how many there are, or why there are none.
func (f *findBar) status(view *godemirror.View) (text string, failed bool) {
	switch {
	case f.applied.Text == "":
		return "", false
	case f.invalid:
		return i18n.T("dock.findBar.invalidRegexp"), true
	}
	matches := view.Matches()
	if len(matches) == 0 {
		return i18n.T("dock.findBar.noResults"), true
	}
	if from, to, ok := view.Selection(); ok {
		for i, m := range matches {
			if m.From == from && m.To == to {
				return i18n.T("dock.findBar.position", i18n.Args{"current": i + 1, "total": len(matches)}), false
			}
		}
	}
	return i18n.T("dock.findBar.matches", i18n.Args{"count": len(matches)}), false
}

// layout is the h-9 row: the field, the status, the three toggles, then
// previous, next and close.
func (f *findBar) layout(gtx C, th *ui.Theme, root *ui.Root, view *godemirror.View) D {
	f.apply(view)
	if f.focusField {
		f.focusField = false
		gtx.Execute(key.FocusCmd{Tag: &f.query})
		f.query.SetCaret(0, f.query.Len())
	}
	field := th.Input(&f.query, i18n.T("dock.findBar.placeholder"))
	field.Leading, field.Height, field.TextSize, field.Mono, field.Invalid = icSearch, ui.Space(6), ui.TextXS, true, f.invalid
	status, failed := f.status(view)
	label := th.Text(ui.TextXS, status).Muted()
	if failed {
		label = label.In(th.Destructive)
	}
	return layout.UniformInset(ui.Space(1.5)).Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, field.Layout),
			ui.Gap(1.5),
			layout.Rigid(label.Layout),
			ui.Gap(1),
			layout.Rigid(f.toggle(th, root, &f.matchCase, &f.caseTip, icFindCase, i18n.T("dock.findBar.matchCase"))),
			layout.Rigid(f.toggle(th, root, &f.wholeWord, &f.wordTip, icFindWord, i18n.T("dock.findBar.wholeWord"))),
			layout.Rigid(f.toggle(th, root, &f.regexp, &f.regexpTip, icFindRegexp, i18n.T("dock.findBar.regexp"))),
			layout.Rigid(f.toggle(th, root, &f.listing, &f.listTip, icFindAll, i18n.T("dock.findBar.showAll"))),
			ui.Gap(1),
			layout.Rigid(f.button(th, root, &f.previous, &f.previousTip, icFindPrevious, i18n.T("dock.findBar.previous"))),
			layout.Rigid(f.button(th, root, &f.next, &f.nextTip, icFindNext, i18n.T("dock.findBar.next"))),
			layout.Rigid(f.button(th, root, &f.close, &f.closeTip, icFindClose, i18n.T("dock.findBar.close"))),
		)
	})
}

// layoutList is Find all under the bar, with its own separator, while it is
// on and the search has matches.
func (f *findBar) layoutList(gtx C, th *ui.Theme, view *godemirror.View) D {
	if !f.listing.Value || f.applied.Text == "" || f.invalid {
		return D{}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return f.list.layout(gtx, th, view) }),
		layout.Rigid(func(gtx C) D {
			if len(view.Matches()) == 0 {
				return D{}
			}
			return th.Separator(gtx)
		}),
	)
}

func (f *findBar) toggle(th *ui.Theme, root *ui.Root, pressed *widget.Bool, tip *ui.Tooltip, icon *icons.Icon, label string) layout.Widget {
	return func(gtx C) D {
		t := th.Toggle(pressed, "")
		t.Icon, t.Size, t.Height = icon, ui.ToggleSizeSM, ui.Space(6)
		return th.Tooltip(root, tip, label).Layout(gtx, t.Layout)
	}
}

func (f *findBar) button(th *ui.Theme, root *ui.Root, click *widget.Clickable, tip *ui.Tooltip, icon *icons.Icon, label string) layout.Widget {
	return func(gtx C) D {
		b := th.Button(click, "")
		b.Icon, b.Variant, b.Size = icon, ui.VariantGhost, ui.SizeIconXS
		return th.Tooltip(root, tip, label).Layout(gtx, b.Layout)
	}
}
