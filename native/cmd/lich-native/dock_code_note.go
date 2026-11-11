package main

import (
	"cmp"
	"image"
	"slices"

	"gioui.org/layout"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/ui"
)

// agentNote is the comment an agent left on the lines it pointed at.
type agentNote struct {
	text, author, kind string
}

// noteAt is the document line the note hangs under: the selection's last.
func (p *preview) noteAt() int { return max(p.line, p.endLine) - 1 }

// blocksWith is the view's blocks with the composer under line at, -1 for no
// composer. A note and a composer on one line are one block, the note first:
// the view promises no order between blocks that share a line.
func (p *preview) blocksWith(at int) []godemirror.BlockWidget {
	hasNote := p.note.text != ""
	switch {
	case hasNote && at == p.noteAt():
		return []godemirror.BlockWidget{{After: at, Widget: p.noteThenComposer}}
	case hasNote && at < 0:
		return []godemirror.BlockWidget{{After: p.noteAt(), Widget: p.noteBlock}}
	case at < 0:
		return nil
	case !hasNote:
		return []godemirror.BlockWidget{{After: at, Widget: p.composerBlock}}
	}
	blocks := []godemirror.BlockWidget{{After: p.noteAt(), Widget: p.noteBlock}, {After: at, Widget: p.composerBlock}}
	slices.SortFunc(blocks, func(a, b godemirror.BlockWidget) int { return cmp.Compare(a.After, b.After) })
	return blocks
}

func (p *preview) noteThenComposer(gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(p.noteBlock),
		layout.Rigid(p.composerBlock),
	)
}

// noteBlock is a comment box on Background, which sets it off the panel's
// surface the code is drawn on, the author's mark in the avatar's gutter.
func (p *preview) noteBlock(gtx C) D {
	th := p.blockTh
	return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, th.Background, ui.RadiusMD) },
			func(gtx C) D { return p.noteBody(gtx, th.On(th.Background)) },
		)
	})
}

func (p *preview) noteBody(gtx C, th *ui.Theme) D {
	return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2.5), Bottom: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return p.noteMark(gtx, th) }),
			ui.Gap(2.5),
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(th.Text(ui.TextXS, p.note.author).Muted().Layout),
					ui.VGap(0.5),
					layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(ui.TextSM, p.note.text)) }),
				)
			}),
		)
	})
}

// noteMark is the provider's glyph on the disc a comment's avatar sits on.
func (p *preview) noteMark(gtx C, th *ui.Theme) D {
	side := gtx.Dp(authorAvatarSize)
	gtx.Constraints = layout.Exact(image.Pt(side, side))
	ui.Fill(gtx, th.Accent, ui.Space(3))
	return layout.Center.Layout(gtx, func(gtx C) D {
		return providerIcon(p.note.kind).Layout(gtx, ui.Space(3.5), th.MutedForeground)
	})
}
