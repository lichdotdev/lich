package main

import (
	"path"
	"path/filepath"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"

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
// Edits one after another are one group under a label, the first of them
// heading it; the group stands on the conversation, with no ground.
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
					ui.VGap(1),
					layout.Rigid(func(gtx C) D { return u.chatEditDiff(gtx, th.On(th.Background), p, f, t.Diff, body) }),
				)
			})
		}),
	)
	call := m.Stop()
	call.Add(gtx.Ops)
	return d
}

// chatEditsHead heads a group of edits, a group's label: EDITED, how many
// files muted and all their lines in the tones; a click opens Review, as the
// footer's changes item does.
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
	return f.click.Layout(gtx, func(gtx C) D {
		return layout.Background{}.Layout(gtx, chatEditHoverFill(th, &f.click), func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(th.Text(ui.Text2XS, strings.ToUpper(i18n.T("chat.view.edited"))).Muted().Weight(font.SemiBold).Leading(text2XSLeading).Layout),
					ui.Gap(1.5),
					layout.Flexed(1, th.Text(ui.TextXS, chatEditsCount(len(files))).Muted().Layout),
					ui.Gap(2),
					layout.Rigid(th.DiffStat(added, deleted).Layout),
					ui.Gap(2),
					layout.Rigid(layout.Spacer{Width: ui.Space(4)}.Layout),
				)
			})
		})
	})
}

// chatEditsCount is how many files a group of edits changed, beside its label.
func chatEditsCount(files int) string {
	return i18n.T("common.count.file", i18n.Args{"count": files})
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

// chatEditFailure is a failed edit's end: the failed glyph and the first line
// of what the tool answered, which says why, cut to the room left.
func chatEditFailure(gtx C, th *ui.Theme, t *backend.ChatToolCall) D {
	why := cleanLine(t.Output)
	if why == "" {
		why = i18n.T("chat.view.toolFailed")
	}
	gtx.Constraints.Max.X = int(float32(gtx.Constraints.Max.X) * chatEditWhyShare)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return icToolsFailed.Layout(gtx, ui.Space(3.5), th.Destructive) }),
		ui.Gap(1.5),
		layout.Flexed(1, th.Text(ui.TextXS, why).Mono().In(th.Destructive).Truncated().Layout),
	)
}

// chatEditWhyShare is how much of the line's room a failure's reason may take,
// leaving the file's name in view.
const chatEditWhyShare = 0.5

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
				return chatEditFailure(gtx, th, t)
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
