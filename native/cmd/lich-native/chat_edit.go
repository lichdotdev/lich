package main

import (
	"image"
	"path"
	"path/filepath"
	"strings"

	"gioui.org/layout"

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
	head := func(gtx C) D {
		return f.click.Layout(gtx, func(gtx C) D {
			return layout.Background{}.Layout(gtx,
				func(gtx C) D {
					if !f.click.Hovered() {
						return D{Size: gtx.Constraints.Min}
					}
					return ui.Fill(gtx, th.Over(ui.Alpha(th.Accent, chatEditHover)), ui.RadiusMD)
				},
				func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
						return chatEditLine(gtx, th, f, t, file, running, added, deleted, len(body) > 0)
					})
				})
		})
	}
	d := layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, th.Card, ui.RadiusMD) },
		func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
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
		})
	ui.InsetBorder(gtx, d.Size, gtx.Dp(ui.RadiusMD), th.Over(th.Border))
	return d
}

// chatEditHover is how much of Accent the line takes under the pointer, the
// kit's hover fill.
const chatEditHover = 0.6

// chatEditLine is the edit's line: the file's type, its folder muted and its
// name, the counts once done or why it failed, and the chevron while there is
// a diff to show and the pointer is on it or it is open.
func chatEditLine(gtx C, th *ui.Theme, f *chatFold, t *backend.ChatToolCall, file string, running bool, added, deleted int, diff bool) D {
	dir, name := path.Split(file)
	folder, base := th.Text(ui.TextSM, dir).Mono().Muted(), th.Text(ui.TextSM, name).Mono()
	if running {
		folder, base = folder.Shimmer(), base.Muted().Shimmer()
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(th.FileIcon(file, ui.Space(4)).Layout),
		ui.Gap(2),
		layout.Rigid(folder.Layout),
		layout.Rigid(base.Layout),
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
		layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
		layout.Rigid(func(gtx C) D {
			if !diff || !f.open && !f.click.Hovered() {
				return D{}
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
