package main

import (
	"image"
	"path"
	"path/filepath"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// diffNoFile is the side of a unified diff a file created or deleted lacks.
const diffNoFile = "/dev/null"

// chatEdit is a call that changed a file: one line with the file's type, its
// path and its counts, the diff under it once clicked; a Ctrl+click opens the
// file instead. While the call runs its path shimmers; failed, it says so.
// Edits one after another share one ground, the first of them heading it.
func (u *window) chatEdit(gtx C, th *ui.Theme, p *chatPane, sv sessionView, r chatRow) D {
	t := r.item.Tool
	if t == nil {
		return D{}
	}
	f := p.fold("edit:" + r.item.ID)
	file := editPath(t, sv.path)
	_, added, deleted, body := splitDiff(t.Diff)
	if f.click.Clicked(gtx) {
		if p.paths != nil && p.paths.Held && p.paths.Link(file) {
			p.paths.Open(file)
		} else if len(body) > 0 {
			f.open, f.turned = !f.open, gtx.Now
		}
	}
	running := r.live && callRunning(r.item)
	pad := layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2), Bottom: ui.Space(2)}
	if r.editAbove || len(r.edits) > 0 {
		pad.Top = ui.Space(1.5)
	}
	if r.editBelow {
		pad.Bottom = ui.Space(1.5)
	}
	head := func(gtx C) D {
		return f.click.Layout(gtx, func(gtx C) D {
			return layout.Background{}.Layout(gtx, chatEditHoverFill(th, &f.click), func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return pad.Layout(gtx, func(gtx C) D {
					return chatEditLine(gtx, th, f, t, file, running, added, deleted, len(body) > 0)
				})
			})
		})
	}
	m := op.Record(gtx.Ops)
	d := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if len(r.edits) == 0 {
				return D{}
			}
			return u.chatEditsHead(gtx, th, p, sv, r)
		}),
		layout.Rigid(head),
		layout.Rigid(func(gtx C) D {
			if f.shut(gtx.Now) {
				return f.body.Layout(gtx, false, chatNothing)
			}
			return f.body.Layout(gtx, f.open, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))
						return ui.Fill(gtx, th.Over(th.Border), 0)
					}),
					layout.Rigid(func(gtx C) D { return u.chatEditDiff(gtx, th.On(th.Background), p, f, t.Diff, body) }),
				)
			})
		}),
	)
	call := m.Stop()
	defer chatEditGround(gtx, r, d.Size).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, ui.MixOKLab(th.Card, th.Background, chatEditTint))
	call.Add(gtx.Ops)
	return d
}

// chatEditTint is how far an edit's ground goes from Card toward the
// conversation's Background: a tint, not a box.
const chatEditTint = 0.3

// chatEditGround is the shape of edit row r's ground: RadiusLG at the corners
// of its group's outer edge, square where another edit row joins it.
func chatEditGround(gtx C, r chatRow, size image.Point) clip.RRect {
	top, bottom := gtx.Dp(ui.RadiusLG), gtx.Dp(ui.RadiusLG)
	if r.editAbove {
		top = 0
	}
	if r.editBelow {
		bottom = 0
	}
	return clip.RRect{Rect: image.Rectangle{Max: size}, NW: top, NE: top, SE: bottom, SW: bottom}
}

// chatEditsHead heads a group of edits: how many files they changed and all
// their lines; a click opens Review, as the footer's changes item does.
func (u *window) chatEditsHead(gtx C, th *ui.Theme, p *chatPane, sv sessionView, r chatRow) D {
	f := p.fold("edits:" + r.item.ID)
	if f.click.Clicked(gtx) {
		u.dock.show(dockReview)
	}
	files := map[string]bool{}
	added, deleted := 0, 0
	for _, it := range r.edits {
		files[editPath(it.Tool, sv.path)] = true
		if it.Tool.State == api.ToolFailed || callRunning(it) {
			continue
		}
		_, a, d, _ := splitDiff(it.Tool.Diff)
		added, deleted = added+a, deleted+d
	}
	label := chatEditsLabel(len(files))
	return f.click.Layout(gtx, func(gtx C) D {
		return layout.Background{}.Layout(gtx, chatEditHoverFill(th, &f.click), func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, th.Text(ui.TextSM, label).Weight(font.Medium).Layout),
					ui.Gap(2),
					layout.Rigid(th.DiffStat(added, deleted).Layout),
					ui.Gap(2),
					layout.Rigid(layout.Spacer{Width: ui.Space(4)}.Layout),
				)
			})
		})
	})
}

// chatEditsLabel is the head of a group of edits, "Edited 3 files": the fold's
// own clause for its edits, read on its own.
func chatEditsLabel(files int) string {
	return raiseFirst(i18n.T("chat.view.calls.edited", i18n.Args{"count": files}))
}

// chatEditHover is how much of Accent the line takes under the pointer, the
// kit's hover fill.
const chatEditHover = 0.6

func chatEditHoverFill(th *ui.Theme, click *ui.Clickable) layout.Widget {
	return func(gtx C) D {
		if !click.Hovered() {
			return D{Size: gtx.Constraints.Min}
		}
		return ui.Fill(gtx, th.Over(ui.Alpha(th.Accent, chatEditHover)), ui.RadiusMD)
	}
}

// chatEditLine is the edit's line: the file's type, its name, its folder
// muted and cut first when the line is short, the counts once done or why it
// failed, and the chevron while there is a diff to show and the pointer is on
// it or it is open.
func chatEditLine(gtx C, th *ui.Theme, f *chatFold, t *backend.ChatToolCall, file string, running bool, added, deleted int, diff bool) D {
	dir, name := path.Split(file)
	base, folder := th.Text(ui.TextSM, name).Mono().Weight(font.Medium), th.Text(ui.TextXS, strings.TrimSuffix(dir, "/")).Mono().Muted()
	if running {
		folder, base = folder.Shimmer(), base.Muted().Shimmer()
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(th.FileIcon(file, ui.Space(4)).Layout),
		ui.Gap(2),
		layout.Rigid(base.Layout),
		ui.Gap(2),
		layout.Flexed(1, func(gtx C) D {
			d := folder.Layout(gtx)
			d.Size.X = gtx.Constraints.Max.X
			return d
		}),
		ui.Gap(2),
		layout.Rigid(func(gtx C) D {
			switch {
			case running:
				return D{}
			case t.State == api.ToolFailed:
				return th.Text(ui.TextSM, i18n.T("chat.view.toolFailed")).In(th.Destructive).Layout(gtx)
			}
			return th.DiffStat(added, deleted).Layout(gtx)
		}),
		ui.Gap(2),
		layout.Rigid(func(gtx C) D {
			if !diff || !f.open && !f.click.Hovered() {
				return layout.Spacer{Width: ui.Space(4)}.Layout(gtx)
			}
			ic := icChatClosed
			if f.open {
				ic = icChatOpen
			}
			return ic.Layout(gtx, ui.Space(4), th.MutedForeground)
		}),
	)
}

// chatEditDiff is the open edit's diff as the Review panel draws it, in the
// editors' code font; a diff it cannot read is its lines as they came.
func (u *window) chatEditDiff(gtx C, th *ui.Theme, p *chatPane, f *chatFold, diff string, body []string) D {
	if f.diff == nil {
		f.diff = new(chatEditDiff)
	}
	f.diff.build(diff, u.editors.family, u.editors.size)
	cell, err := p.diffCells.height(gtx, u.editors.family, u.editors.size)
	if err != nil || f.diff.fail {
		return chatDiffLines(gtx, th, body)
	}
	return f.diff.layout(gtx, th, th.Code, cell)
}

// editPath is the file call t changed, as the chat names it: the diff's own
// path, or the call's target without one, made relative to the checkout when
// it lies inside it.
func editPath(t *backend.ChatToolCall, checkout string) string {
	file := diffPath(t.Diff)
	if file == "" {
		file = t.Title
	}
	if checkout != "" && filepath.IsAbs(file) {
		if rel, err := filepath.Rel(checkout, file); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return file
}

// diffPath is the file a unified diff changes: its new side, or its old one
// for a file it deletes, without git's a/ and b/; "" without a header.
func diffPath(diff string) string {
	from, to := "", ""
	for line := range strings.SplitSeq(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "--- ") && from == "":
			from = strings.TrimSpace(line[4:])
		case strings.HasPrefix(line, "+++ ") && to == "":
			to = strings.TrimSpace(line[4:])
		case strings.HasPrefix(line, "@@"):
			return pickDiffSide(from, to)
		}
	}
	return pickDiffSide(from, to)
}

func pickDiffSide(from, to string) string {
	if to == "" || to == diffNoFile {
		to = from
	}
	if to == diffNoFile {
		return ""
	}
	if rest, ok := strings.CutPrefix(to, "b/"); ok {
		return rest
	}
	rest, _ := strings.CutPrefix(to, "a/")
	return rest
}
