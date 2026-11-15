package main

// The resolver's panes: what each draws of the chunks, the buttons in the
// gutters at each chunk's line, and the scroll the three share.

import (
	"fmt"
	"image/color"
	"slices"
	"strconv"
	"strings"

	"gioui.org/layout"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/codeview"
)

// resolverTint is how much of its tone a chunk's lines mix in, the tint
// DESIGN.md gives an added or removed row.
const resolverTint = 0.12

func (r *conflictResolver) panes(gtx C, th *ui.Theme, p *reviewPanel) D {
	r.paneHeight = gtx.Constraints.Max.Y
	code := codeview.OnSurface(th, p.code)
	r.ours.Theme, r.theirs.Theme, r.result.Theme = code, code, code
	doc := r.doc()
	return r.row(gtx, layout.Inset{},
		func(gtx C) D { return r.side(gtx, th, r.ours, true, doc) },
		func(gtx C) D { dims, _ := r.result.Layout(gtx); return dims },
		func(gtx C) D { return r.side(gtx, th, r.theirs, false, doc) },
		func(gtx C) D { return r.gutter(gtx, th, p, true, doc) },
		func(gtx C) D { return r.gutter(gtx, th, p, false, doc) })
}

// side draws our version or the incoming one, its chunks tinted: a conflict
// in the waiting tone until the result settles it, a change of its own in
// the passing one. The tints are laid in again when one of them changes.
func (r *conflictResolver) side(gtx C, th *ui.Theme, v *godemirror.View, ours bool, doc string) D {
	var key strings.Builder
	key.WriteString(fmt.Sprint(th.Surface, th.TonePass, th.ToneWait))
	for i := range r.model.chunks {
		key.WriteString(strconv.FormatBool(r.model.resolved(i, doc)))
	}
	if r.sideKey[ours] != key.String() {
		_, shown := r.sideKey[ours]
		r.sideKey[ours] = key.String()
		r.setSide(v, ours, r.sideDecorations(th, ours, doc), shown)
	}
	dims, _ := v.Layout(gtx)
	return dims
}

func (r *conflictResolver) sideDecorations(th *ui.Theme, ours bool, doc string) []godemirror.LineDecoration {
	lines := r.model.theirs
	if ours {
		lines = r.model.ours
	}
	deco := make([]godemirror.LineDecoration, len(lines))
	for i, ch := range r.model.chunks {
		run, mine := ch.Theirs, ch.Kind == api.GitChunkTheirs
		if ours {
			run, mine = ch.Ours, ch.Kind == api.GitChunkOurs
		}
		tone := th.TonePass
		switch {
		case ch.Kind == api.GitChunkConflict && !r.model.resolved(i, doc):
			tone = th.ToneWait
		case ch.Kind != api.GitChunkConflict && !mine && ch.Kind != api.GitChunkBoth:
			continue
		}
		for l := run.From; l < run.To; l++ {
			deco[l] = godemirror.LineDecoration{Background: ui.MixOKLab(th.Surface, tone, resolverTint)}
		}
	}
	return deco
}

// setSide gives a side pane its text and tints at once, and again, keeping
// its scroll, when they change.
func (r *conflictResolver) setSide(v *godemirror.View, ours bool, deco []godemirror.LineDecoration, shown bool) {
	lines := r.model.theirs
	if ours {
		lines = r.model.ours
	}
	content := godemirror.Content{Filename: r.rel, Text: strings.Join(lines, ""), StyleTags: codeview.StyleTags, Decorations: deco}
	if shown {
		v.UpdateContent(content)
		return
	}
	v.SetContent(content)
}

// gutter draws, beside each conflict still open, the side's take and dismiss
// buttons, level with the chunk's first line in that side's pane.
func (r *conflictResolver) gutter(gtx C, th *ui.Theme, p *reviewPanel, ours bool, doc string) D {
	v, btns, branch, take := r.theirs, r.theirsBtn, p.git.Merging, icTakeLeft
	if ours {
		v, btns, branch, take = r.ours, r.oursBtn, branchOrHead(p.git.Branch), icTakeRight
	}
	for i, ch := range r.model.chunks {
		off := ch.theirsOff
		if ours {
			off = ch.oursOff
		}
		if ch.Kind != api.GitChunkConflict || off || r.model.resolved(i, doc) && r.model.applied(i, ours, doc) {
			continue
		}
		line := ch.Theirs.From
		if ours {
			line = ch.Ours.From
		}
		// A side the pane has not drawn yet has no line to stand at.
		if line >= v.LineCount() {
			continue
		}
		top, _ := v.LineBlock(line)
		b := &btns[i]
		buttons := []layout.FlexChild{
			layout.Rigid(func(gtx C) D {
				return th.IconAction(p.root, &b.take, take, i18n.T("git.resolve.take", i18n.Args{"branch": branch}), false).Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				return th.IconAction(p.root, &b.dismiss, icDismiss, i18n.T("git.resolve.dismiss", i18n.Args{"branch": branch}), false).Layout(gtx)
			}),
		}
		if !ours {
			buttons[0], buttons[1] = buttons[1], buttons[0]
		}
		offset(gtx, top-v.ScrollTop(), func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, buttons...)
		})
	}
	return D{Size: gtx.Constraints.Max}
}

// resultDecorations tints each chunk's lines in the result: waiting while a
// conflict is open (its base's lines struck through), passing once it is
// not. An open chunk with no lines marks the line it would go before. The key
// says when they changed.
func (r *conflictResolver) resultDecorations(th *ui.Theme) (godemirror.Decorations, string) {
	state := r.result.State()
	doc := state.Doc()
	var d godemirror.Decorations
	var key strings.Builder
	fmt.Fprintf(&key, "%v%v%v;", th.Surface, th.TonePass, th.ToneWait)
	for i, ch := range r.model.chunks {
		open := !r.model.resolved(i, doc)
		fmt.Fprintf(&key, "%d-%d-%t;", ch.from, ch.to, open)
		deco := godemirror.LineDecoration{Background: ui.MixOKLab(th.Surface, th.TonePass, resolverTint)}
		if open {
			deco = godemirror.LineDecoration{Background: ui.MixOKLab(th.Surface, th.ToneWait, resolverTint), Hatch: tint(th.ToneWait)}
		}
		if ch.from == ch.to {
			if open && ch.from <= len(doc) {
				d.Lines = append(d.Lines, godemirror.LineDecorationAt{Pos: state.LineAt(ch.from).From,
					Decoration: godemirror.LineDecoration{GutterStrip: th.ToneWait}})
			}
			continue
		}
		for pos := ch.from; pos < ch.to; {
			line := state.LineAt(pos)
			d.Lines = append(d.Lines, godemirror.LineDecorationAt{Pos: line.From, Decoration: deco})
			pos = line.To + 1
		}
	}
	return d, key.String()
}

func tint(c color.NRGBA) color.NRGBA { return ui.Alpha(c, 0.35) }

// follow keeps the other two panes level with the one the user scrolled at
// the middle of the panes, where the eye is: the line there, and how far into
// it, mapped through the chunks.
func (r *conflictResolver) follow() {
	src := r.scrolled
	r.scrolled = -1
	if src < 0 {
		return
	}
	y := r.scrollTop(src)
	if r.wanted[src] == y {
		r.wanted[src] = -1
		return
	}
	mid := r.paneHeight / 2
	line := r.lineAtHeight(src, y+mid)
	into := y + mid - r.lineTop(src, line)
	for dst := range 3 {
		if dst == src {
			continue
		}
		target := mapLine(line, r.runs(src), r.runs(dst))
		r.wanted[dst] = max(r.lineTop(dst, target)+into-mid, 0)
		r.setScrollTop(dst, r.wanted[dst])
	}
}

// runs is where each chunk sits in a pane, in lines.
func (r *conflictResolver) runs(pane int) []api.GitLines {
	out := make([]api.GitLines, len(r.model.chunks))
	state := r.result.State()
	for i, ch := range r.model.chunks {
		switch pane {
		case paneOurs:
			out[i] = ch.Ours
		case paneTheirs:
			out[i] = ch.Theirs
		default:
			from := state.LineAt(ch.from).Number - 1
			to := from
			if ch.to > ch.from {
				to = state.LineAt(ch.to - 1).Number
			}
			out[i] = api.GitLines{From: from, To: to}
		}
	}
	return out
}

// mapLine is where line of one pane stands in another: inside a chunk, as
// far in as the other chunk reaches; past one, as far past its end.
func mapLine(line int, src, dst []api.GitLines) int {
	for i, s := range slices.Backward(src) {
		if s.From > line {
			continue
		}
		if line < s.To {
			return dst[i].From + min(line-s.From, max(dst[i].To-dst[i].From-1, 0))
		}
		return dst[i].To + line - s.To
	}
	return line
}

func (r *conflictResolver) scrollTop(pane int) int {
	switch pane {
	case paneOurs:
		return r.ours.ScrollTop()
	case paneTheirs:
		return r.theirs.ScrollTop()
	}
	return r.result.ScrollTop()
}

func (r *conflictResolver) setScrollTop(pane, y int) {
	switch pane {
	case paneOurs:
		r.ours.SetScrollTop(y)
	case paneTheirs:
		r.theirs.SetScrollTop(y)
	default:
		r.result.SetScrollTop(y)
	}
}

func (r *conflictResolver) lineAtHeight(pane, y int) int {
	switch pane {
	case paneOurs:
		return r.ours.LineAtHeight(y)
	case paneTheirs:
		return r.theirs.LineAtHeight(y)
	}
	return r.result.LineBlockAtHeight(y).Number - 1
}

func (r *conflictResolver) lineTop(pane, line int) int {
	var top int
	switch pane {
	case paneOurs:
		top, _ = r.ours.LineBlock(line)
	case paneTheirs:
		top, _ = r.theirs.LineBlock(line)
	default:
		state := r.result.State()
		top, _ = r.result.LineBlockAt(state.Line(min(max(line+1, 1), state.Lines())).From)
	}
	return top
}
