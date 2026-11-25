package main

// The resolver's panes: what each draws of the chunks, the buttons in the
// gutters at each chunk's line, and the scroll the three share.

import (
	"fmt"
	"image/color"
	"slices"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/codeview"
)

// resolverTint is how much of its tone a chunk's lines mix in, the tint
// DESIGN.md gives an added or removed row.
const resolverTint = 0.12

// resolverWordTint is the stronger mix of the words a side changed within a
// line.
const resolverWordTint = 0.3

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

// side draws our version or the incoming one, the chunks it changed tinted as
// chunkTone has them. The tints are laid in again when one of them changes.
func (r *conflictResolver) side(gtx C, th *ui.Theme, v *godemirror.View, ours bool, doc string) D {
	var key strings.Builder
	key.WriteString(fmt.Sprint(th.Surface, th.TonePass, th.ToneWait))
	for i := range r.model.chunks {
		key.WriteString(strconv.Itoa(int(r.model.state(i, doc))))
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
		tone, ok := r.chunkTone(th, i, doc)
		if !ok || !bandOnSide(ch.Kind, ours) {
			continue
		}
		run := ch.Theirs
		if ours {
			run = ch.Ours
		}
		for l := run.From; l < run.To; l++ {
			deco[l] = chunkLine(th, tone)
			if base, paired := pairedLine(r.model.base, ch.Base, run, l); paired {
				deco[l].Spans = wordSpan(lines[l], []string{base}, ui.MixOKLab(th.Surface, tone, resolverWordTint))
			}
		}
	}
	return deco
}

// chunkLine tints a chunk's line, its line number with it: the band in the
// gutter beside it runs into the pane unbroken.
func chunkLine(th *ui.Theme, tone color.NRGBA) godemirror.LineDecoration {
	fill := ui.MixOKLab(th.Surface, tone, resolverTint)
	return godemirror.LineDecoration{Background: fill, GutterBackground: fill}
}

// chunkTone is the tone a chunk shows in, false for none: waiting for a
// conflict still open, neutral for a change not taken yet, passing once its
// lines are in the result, and nothing for one left out.
func (r *conflictResolver) chunkTone(th *ui.Theme, i int, doc string) (color.NRGBA, bool) {
	switch r.model.state(i, doc) {
	case chunkApplied:
		return th.TonePass, true
	case chunkDismissed:
		return color.NRGBA{}, false
	}
	if r.model.chunks[i].Kind == api.GitChunkConflict {
		return th.ToneWait, true
	}
	return th.MutedForeground, true
}

// pairedLine is the line of a base run that stands across from line l of a
// side's run, when the two runs have as many lines.
func pairedLine(base []string, baseRun, run api.GitLines, l int) (string, bool) {
	if baseRun.To-baseRun.From != run.To-run.From {
		return "", false
	}
	return base[baseRun.From+l-run.From], true
}

// wordSpan marks the stretch of line that differs from every one of others,
// what is left between the start and the end they all share with it; nothing
// when the whole line differs, or none of it.
func wordSpan(line string, others []string, fill color.NRGBA) []godemirror.Span {
	a := []rune(strings.TrimRight(line, "\r\n"))
	from, to := len(a), 0
	for _, other := range others {
		b := []rune(strings.TrimRight(other, "\r\n"))
		pre := 0
		for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
			pre++
		}
		suf := 0
		for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
			suf++
		}
		from, to = min(from, pre), max(to, len(a)-suf)
	}
	if from >= to || from == 0 && to == len(a) {
		return nil
	}
	return []godemirror.Span{{From: from, To: to, Color: fill}}
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

// gutter draws each chunk's band from its side's pane to the result and,
// beside each conflict still open, the side's take and dismiss buttons, level
// with the chunk's first line in that side's pane.
func (r *conflictResolver) gutter(gtx C, th *ui.Theme, p *reviewPanel, ours bool, doc string) D {
	v, btns, branch, take := r.theirs, r.theirsBtn, p.git.Merging, icTakeLeft
	if ours {
		v, btns, branch, take = r.ours, r.oursBtn, branchOrHead(p.git.Branch), icTakeRight
	}
	r.bands(gtx, th, ours, doc)
	for i, ch := range r.model.chunks {
		off := ch.theirsOff
		if ours {
			off = ch.oursOff
		}
		if !bandOnSide(ch.Kind, ours) || off || !r.offers(i, ours, doc) {
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
		// The arrow stands next to the result it writes to, and the pair
		// against the side pane it belongs to.
		edge := layout.E
		if ours {
			buttons[0], buttons[1] = buttons[1], buttons[0]
			edge = layout.W
		}
		offset(gtx, top-v.ScrollTop(), func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return edge.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = 0
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, buttons...)
			})
		})
	}
	return D{Size: gtx.Constraints.Max}
}

// resultDecorations tints each chunk's lines in the result as chunkTone has
// them, a conflict still open hatched, and marks in a pending chunk the words
// its sides changed. A pending chunk with no lines marks the line it would go
// before. The key says when they changed.
func (r *conflictResolver) resultDecorations(th *ui.Theme) (godemirror.Decorations, string) {
	state := r.result.State()
	doc := state.Doc()
	var d godemirror.Decorations
	var key strings.Builder
	fmt.Fprintf(&key, "%v%v%v;", th.Surface, th.TonePass, th.ToneWait)
	for i, ch := range r.model.chunks {
		st := r.model.state(i, doc)
		fmt.Fprintf(&key, "%d-%d-%d;", ch.from, ch.to, st)
		tone, ok := r.chunkTone(th, i, doc)
		if !ok {
			continue
		}
		if ch.from == ch.to {
			if st == chunkPending && ch.from <= len(doc) {
				d.Lines = append(d.Lines, godemirror.LineDecorationAt{Pos: state.LineAt(ch.from).From,
					Decoration: godemirror.LineDecoration{GutterStrip: tone}})
			}
			continue
		}
		deco := chunkLine(th, tone)
		if st == chunkPending && ch.Kind == api.GitChunkConflict {
			deco.Hatch = tint(tone)
		}
		n := 0
		for pos := ch.from; pos < ch.to; n++ {
			line := state.LineAt(pos)
			at := deco
			if st == chunkPending {
				at.Spans = wordSpan(line.Text, r.sidesOf(ch, n), ui.MixOKLab(th.Surface, tone, resolverWordTint))
			}
			d.Lines = append(d.Lines, godemirror.LineDecorationAt{Pos: line.From, Decoration: at})
			pos = line.To + 1
		}
	}
	return d, key.String()
}

// sidesOf is line n of a chunk as each side that changed it has it, for the
// sides whose run has as many lines as the base's.
func (r *conflictResolver) sidesOf(ch resolveChunk, n int) []string {
	var out []string
	size := ch.Base.To - ch.Base.From
	if ch.Kind != api.GitChunkTheirs && ch.Ours.To-ch.Ours.From == size {
		out = append(out, r.model.ours[ch.Ours.From+n])
	}
	if ch.Kind != api.GitChunkOurs && ch.Theirs.To-ch.Theirs.From == size {
		out = append(out, r.model.theirs[ch.Theirs.From+n])
	}
	return out
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

// bands fills, for each chunk the side changed, the stretch of the gutter
// between where it stands in the side's pane and where it stands in the result, in the tint its
// lines have, so the eye follows a chunk across panes that are not level.
func (r *conflictResolver) bands(gtx C, th *ui.Theme, ours bool, doc string) {
	side := paneTheirs
	if ours {
		side = paneOurs
	}
	sideRuns, resultRuns := r.runs(side), r.runs(paneResult)
	size := gtx.Constraints.Max
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	for i, ch := range r.model.chunks {
		if !bandOnSide(ch.Kind, ours) {
			continue
		}
		s1, s2 := r.span(side, sideRuns[i])
		m1, m2 := r.span(paneResult, resultRuns[i])
		// The left edge of the gutter is the pane on its left.
		l1, l2, r1, r2 := m1, m2, s1, s2
		if ours {
			l1, l2, r1, r2 = s1, s2, m1, m2
		}
		if max(l2, r2) < 0 || min(l1, r1) > size.Y {
			continue
		}
		tone, ok := r.chunkTone(th, i, doc)
		if !ok {
			continue
		}
		// The band bends past the buttons, on the result's side of them.
		from, to := float32(gtx.Dp(resolverButtons)), float32(size.X)
		if !ours {
			from, to = 0, float32(size.X-gtx.Dp(resolverButtons))
		}
		fillBand(gtx, float32(size.X), from, to, [4]float32{float32(l1), float32(l2), float32(r1), float32(r2)},
			ui.MixOKLab(th.Surface, tone, resolverTint))
	}
}

// offers is whether a side's buttons show on a chunk: while it is pending,
// and on a conflict taken from the other side, whose lines can still be
// stacked.
func (r *conflictResolver) offers(i int, ours bool, doc string) bool {
	switch r.model.state(i, doc) {
	case chunkPending:
		return true
	case chunkApplied:
		return r.model.chunks[i].Kind == api.GitChunkConflict && !r.model.applied(i, ours, doc)
	}
	return false
}

// bandOnSide is whether a chunk has a band to a side's pane: a change only
// the other side made has nothing there to join.
func bandOnSide(kind string, ours bool) bool {
	if ours {
		return kind != api.GitChunkTheirs
	}
	return kind != api.GitChunkOurs
}

// fillBand paints the band from [ys[0], ys[1]] on the gutter's left edge to
// [ys[2], ys[3]] on its right, level outside [from, to] and eased between.
func fillBand(gtx C, w, from, to float32, ys [4]float32, fill color.NRGBA) {
	mid := (from + to) / 2
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(0, ys[0]))
	p.LineTo(f32.Pt(from, ys[0]))
	p.CubeTo(f32.Pt(mid, ys[0]), f32.Pt(mid, ys[2]), f32.Pt(to, ys[2]))
	p.LineTo(f32.Pt(w, ys[2]))
	p.LineTo(f32.Pt(w, ys[3]))
	p.LineTo(f32.Pt(to, ys[3]))
	p.CubeTo(f32.Pt(mid, ys[3]), f32.Pt(mid, ys[1]), f32.Pt(from, ys[1]))
	p.LineTo(f32.Pt(0, ys[1]))
	p.Close()
	paint.FillShape(gtx.Ops, fill, clip.Outline{Path: p.End()}.Op())
}

// span is where a run of a pane's lines shows, top and bottom on screen.
func (r *conflictResolver) span(pane int, run api.GitLines) (top, bottom int) {
	count, block := r.theirs.LineCount(), r.theirs.LineBlock
	switch pane {
	case paneOurs:
		count, block = r.ours.LineCount(), r.ours.LineBlock
	case paneResult:
		state := r.result.State()
		count, block = state.Lines(), func(line int) (int, int) { return r.result.LineBlockAt(state.Line(line + 1).From) }
	}
	top, bottom = runSpan(run, count, block)
	scroll := r.scrollTop(pane)
	return top - scroll, bottom - scroll
}

// runSpan is the top of a run's first line and the bottom of its last, in a
// pane of count lines; an empty run is the seam before its line, or the end
// of the pane past the last one.
func runSpan(run api.GitLines, count int, block func(line int) (top, bottom int)) (top, bottom int) {
	switch {
	case count == 0:
		return 0, 0
	case run.From >= count:
		_, bottom = block(count - 1)
		return bottom, bottom
	case run.From >= run.To:
		top, _ = block(run.From)
		return top, top
	}
	top, _ = block(run.From)
	_, bottom = block(min(run.To, count) - 1)
	return top, bottom
}
