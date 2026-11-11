package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
)

// NoticeStyle is the quiet line a panel shows instead of content.
type NoticeStyle struct {
	Text  string
	theme *Theme
}

func (th *Theme) Notice(text string) NoticeStyle { return NoticeStyle{Text: text, theme: th} }

func (n NoticeStyle) Layout(gtx C) D {
	l := n.theme.Text(TextXS, n.Text).Muted()
	l.MaxLines = 0
	return layout.Inset{Left: Space(3), Right: Space(3), Top: Space(4), Bottom: Space(4)}.Layout(gtx, l.Layout)
}

// FootnoteStyle is a notice under a list: what the listing owes the reader
// about itself.
type FootnoteStyle struct {
	Text  string
	theme *Theme
}

func (th *Theme) Footnote(text string) FootnoteStyle { return FootnoteStyle{Text: text, theme: th} }

func (f FootnoteStyle) Layout(gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(f.theme.Separator),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: Space(3), Right: Space(3), Top: Space(2), Bottom: Space(2)}.Layout(gtx,
				func(gtx C) D {
					l := f.theme.Text(TextXS, f.Text).Muted()
					l.MaxLines = 0
					return l.Layout(gtx)
				})
		}),
	)
}

// PanelNoticeStyle is a notice of a full-size screen's panel, text-sm, set in
// from the sides by the screen's own padding.
type PanelNoticeStyle struct {
	Text  string
	PadX  unit.Dp
	theme *Theme
}

func (th *Theme) PanelNotice(text string, padX unit.Dp) PanelNoticeStyle {
	return PanelNoticeStyle{Text: text, PadX: padX, theme: th}
}

func (n PanelNoticeStyle) Layout(gtx C) D {
	return layout.Inset{Left: n.PadX, Right: n.PadX, Top: Space(5), Bottom: Space(5)}.Layout(gtx,
		n.theme.Text(TextSM, n.Text).Muted().Layout)
}

// BodyLineStyle is the px-9 py-2 text-xs muted line a body without a diff
// shows.
type BodyLineStyle struct {
	Text  string
	theme *Theme
}

func (th *Theme) BodyLine(text string) BodyLineStyle { return BodyLineStyle{Text: text, theme: th} }

func (b BodyLineStyle) Layout(gtx C) D {
	return layout.Inset{Left: Space(9), Right: Space(9), Top: Space(2), Bottom: Space(2)}.Layout(gtx,
		b.theme.Text(TextXS, b.Text).Muted().Layout)
}
