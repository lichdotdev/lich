package ui

import "gioui.org/layout"

// MarkedSegment is one run of a line: Marked is drawn on the highlight.
type MarkedSegment struct {
	Text   string
	Marked bool
}

// MarkedTextStyle draws a line's segments in mono, the marked ones on a
// bg-tone-wait/30 mark.
type MarkedTextStyle struct {
	Segments []MarkedSegment
	theme    *Theme
}

func (th *Theme) MarkedText(segments []MarkedSegment) MarkedTextStyle {
	return MarkedTextStyle{Segments: segments, theme: th}
}

func (m MarkedTextStyle) Layout(gtx C) D {
	th := m.theme
	kids := make([]layout.FlexChild, len(m.Segments))
	mark := th.Over(Alpha(th.ToneWait, 0.3))
	for i, s := range m.Segments {
		if !s.Marked {
			l := th.Text(TextXS, s.Text).Mono()
			l.MaxLines = 1
			kids[i] = layout.Rigid(l.Layout)
			continue
		}
		kids[i] = layout.Rigid(func(gtx C) D {
			return layout.Background{}.Layout(gtx,
				func(gtx C) D { return Fill(gtx, mark, RadiusSM) },
				th.On(mark).Text(TextXS, s.Text).Mono().Layout)
		})
	}
	return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, kids...)
}
