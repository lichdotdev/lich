package main

import (
	"image"

	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// agentNote is the comment an agent left on the lines it pointed at.
type agentNote struct {
	text, author, kind string
}

// notes is what is said about the file over its code: the agent's note on
// the lines it pointed at, and the comment box a "Comment for the session"
// opens. Both sit over the code rather than under their lines: godemirror's
// Editor, unlike its read-only View, hangs no block under a line.
func (f *fileEditor) notes(gtx C, th *ui.Theme) D {
	if f.kind != blobNone || f.md.mode == mdRendered || (f.note.text == "" && !f.composer.isOpen()) {
		return D{}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if f.note.text == "" {
				return D{}
			}
			return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2)}.Layout(gtx, f.noteBlock)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(1)}.Layout(gtx, func(gtx C) D {
				return f.composer.layout(gtx, th, f.env.comments, f.checkout)
			})
		}),
		ui.VGap(2),
	)
}

// noteBlock is a comment box on Background, which sets it off the stage the
// code is drawn on, the author's mark in the avatar's gutter.
func (f *fileEditor) noteBlock(gtx C) D {
	th := f.th
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, th.Background, ui.RadiusMD) },
		func(gtx C) D { return f.noteBody(gtx, th.On(th.Background)) },
	)
}

func (f *fileEditor) noteBody(gtx C, th *ui.Theme) D {
	author := f.note.author
	if f.line > 0 {
		author = i18n.T("dock.editor.noteOn", i18n.Args{"author": f.note.author, "lines": formatLineRef(f.line, max(f.line, f.endLine))})
	}
	return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2.5), Bottom: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return f.noteMark(gtx, th) }),
			ui.Gap(2.5),
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(th.Text(ui.TextXS, author).Muted().Layout),
					ui.VGap(0.5),
					layout.Rigid(func(gtx C) D { return th.Text(ui.TextSM, f.note.text).Wrapped().Layout(gtx) }),
				)
			}),
		)
	})
}

// noteMark is the provider's glyph on the disc a comment's avatar sits on.
func (f *fileEditor) noteMark(gtx C, th *ui.Theme) D {
	side := gtx.Dp(authorAvatarSize)
	gtx.Constraints = layout.Exact(image.Pt(side, side))
	ui.Fill(gtx, th.Accent, ui.Space(3))
	return layout.Center.Layout(gtx, func(gtx C) D {
		return providerIcon(f.note.kind).Layout(gtx, ui.Space(3.5), th.MutedForeground)
	})
}
