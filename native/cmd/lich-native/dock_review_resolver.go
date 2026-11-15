package main

// The conflict resolver: a conflicted file in three panes, our side and the
// incoming one read-only either side of the result, which the user builds with
// each chunk's buttons in the gutters and edits by hand.

import (
	"context"
	"image"
	"slices"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/codeview"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icTakeRight = icons.Lucide("chevrons-right")
	icTakeLeft  = icons.Lucide("chevrons-left")
	icDismiss   = icons.Lucide("x")
	icPrevChunk = icons.Lucide("chevron-up")
	icNextChunk = icons.Lucide("chevron-down")
)

// resolverGutter is the width between two panes, room for a chunk's two
// buttons side by side; resolverResultShare is how much wider than a side the
// result is.
const (
	resolverGutter      = unit.Dp(56)
	resolverResultShare = 1.15
)

// The panes, by index.
const (
	paneOurs = iota
	paneResult
	paneTheirs
)

type chunkButtons struct{ take, dismiss ui.IconActionState }

type conflictResolver struct {
	rel, sep string
	model    *resolveModel
	ours     *godemirror.View
	theirs   *godemirror.View
	result   *godemirror.Editor
	// oursBtn and theirsBtn are each chunk's buttons in the left and the
	// right gutter.
	oursBtn, theirsBtn []chunkButtons
	all                [2]ui.IconActionState // accept ours, accept theirs
	prev, next, close  ui.IconActionState
	apply              widget.Clickable
	applyTip           ui.Tooltip
	// scrolled is the pane the user scrolled, -1 for none, and wanted the
	// position this window set on each pane, so the echo is not followed.
	scrolled int
	wanted   [3]int
	decoKey  string
	// sideKey is what each side pane was tinted with, and paneHeight the
	// panes' height in the last frame.
	sideKey    map[bool]string
	paneHeight int
}

// newConflictResolver builds the three panes for c, read for rel. Off the UI
// goroutine: it loads fonts.
func newConflictResolver(rel string, c api.GitConflict, family string, size unit.Sp) (*conflictResolver, error) {
	sep := ""
	if strings.Contains(c.Ours, "\r\n") {
		sep = "\r\n"
	}
	for _, t := range []*string{&c.Base, &c.Ours, &c.Theirs} {
		*t = strings.ReplaceAll(*t, "\r\n", "\n")
	}
	model, doc := newResolveModel(c)
	r := &conflictResolver{rel: rel, sep: sep, model: model, scrolled: -1, wanted: [3]int{-1, -1, -1},
		sideKey: map[bool]string{}}
	var err error
	if r.ours, err = codeview.NewEditor(family, size); err != nil {
		return nil, err
	}
	if r.theirs, err = codeview.NewEditor(family, size); err != nil {
		return nil, err
	}
	if r.result, err = codeview.NewTextEditor(family, size); err != nil {
		return nil, err
	}
	r.ours.OnScroll = func() { r.scrolled = paneOurs }
	r.theirs.OnScroll = func() { r.scrolled = paneTheirs }
	r.result.OnScroll = func() { r.scrolled = paneResult }
	r.oursBtn, r.theirsBtn = make([]chunkButtons, len(model.chunks)), make([]chunkButtons, len(model.chunks))
	err = r.result.SetState(godemirror.EditorStateConfig{Doc: doc, Filename: rel, LineSeparator: sep,
		StyleTags: codeview.StyleTags, AllowMultipleSelections: true})
	return r, err
}

func (r *conflictResolver) doc() string { return r.result.State().Doc() }

// layout draws the resolver over the Review's body; it answers whether the
// user asked to go back to the list.
func (r *conflictResolver) layout(gtx C, th *ui.Theme, p *reviewPanel) (closed bool) {
	r.result.OnUpdate = func(u godemirror.Update) { r.updated(u, th) }
	r.redecorate(th)
	closed = r.actions(gtx, p)
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return r.header(gtx, th, p) }),
		layout.Rigid(th.Separator),
		layout.Rigid(func(gtx C) D { return r.titles(gtx, th, p.git) }),
		layout.Flexed(1, func(gtx C) D { return r.panes(gtx, th, p) }),
		layout.Rigid(th.Separator),
		layout.Rigid(func(gtx C) D { return r.footer(gtx, th, p) }),
	)
	r.follow()
	return closed
}

// actions runs what the header's and the gutters' buttons asked for.
func (r *conflictResolver) actions(gtx C, p *reviewPanel) bool {
	doc := r.doc()
	var changes []godemirror.ChangeSpec
	for i := range r.model.chunks {
		ch := &r.model.chunks[i]
		if r.oursBtn[i].take.Click.Clicked(gtx) {
			changes = append(changes, r.model.take(i, true, doc))
		}
		if r.theirsBtn[i].take.Click.Clicked(gtx) {
			changes = append(changes, r.model.take(i, false, doc))
		}
		ch.oursOff = ch.oursOff || r.oursBtn[i].dismiss.Click.Clicked(gtx)
		ch.theirsOff = ch.theirsOff || r.theirsBtn[i].dismiss.Click.Clicked(gtx)
	}
	for side, ours := range []bool{true, false} {
		if r.all[side].Click.Clicked(gtx) {
			changes = r.takeAll(ours, doc)
		}
	}
	if len(changes) > 0 {
		_ = r.result.Dispatch(godemirror.TransactionSpec{Changes: changes, UserEvent: "input.resolve",
			IsolateHistory: godemirror.IsolateFull})
	}
	if r.prev.Click.Clicked(gtx) {
		r.goTo(gtx, -1)
	}
	if r.next.Click.Clicked(gtx) {
		r.goTo(gtx, 1)
	}
	if r.apply.Clicked(gtx) && r.model.unresolved(doc) == 0 {
		p.saveResolution(r)
	}
	return r.close.Click.Clicked(gtx)
}

// takeAll takes a side in every conflict still open.
func (r *conflictResolver) takeAll(ours bool, doc string) []godemirror.ChangeSpec {
	var changes []godemirror.ChangeSpec
	for i := range r.model.chunks {
		if !r.model.resolved(i, doc) {
			changes = append(changes, r.model.take(i, ours, doc))
		}
	}
	return changes
}

// goTo scrolls the result to the next conflict still open after, or before,
// the one at its top.
func (r *conflictResolver) goTo(gtx C, dir int) {
	doc, top := r.doc(), r.result.LineBlockAtHeight(r.result.ScrollTop()).From
	order := r.model.chunks
	for k := range order {
		i := k
		if dir < 0 {
			i = len(order) - 1 - k
		}
		ch := order[i]
		ahead := dir > 0 && ch.from > top || dir < 0 && ch.from < top
		if ahead && !r.model.resolved(i, doc) {
			y, _ := r.result.LineBlockAt(ch.from)
			r.result.SetScrollTop(max(y-gtx.Constraints.Max.Y/3, 0))
			return
		}
	}
}

// updated follows an edit of the result: the chunks move with it, and their
// colors are worked out again.
func (r *conflictResolver) updated(u godemirror.Update, th *ui.Theme) {
	if u.DocChanged() {
		r.model.remap(u.Changes)
	}
	r.redecorate(th)
}

// redecorate tints the result's chunks when what they show changed: an
// edit, a side taken, the theme, or the first frame.
func (r *conflictResolver) redecorate(th *ui.Theme) {
	deco, key := r.resultDecorations(th)
	if key != r.decoKey {
		r.decoKey = key
		_ = r.result.Dispatch(godemirror.TransactionSpec{Decorations: &deco, SkipHistory: true})
	}
}

func (r *conflictResolver) header(gtx C, th *ui.Theme, p *reviewPanel) D {
	left := r.model.unresolved(r.doc())
	title := th.Text(ui.TextSM, i18n.T("git.resolve.left", i18n.Args{"name": r.rel, "count": left})).Weight(font.Medium)
	if left > 0 {
		title = title.In(th.ToneWait)
	}
	icon := func(s *ui.IconActionState, ic *icons.Icon, label string) layout.FlexChild {
		return layout.Rigid(func(gtx C) D { return th.IconAction(p.root, s, ic, label, false).Layout(gtx) })
	}
	accept := func(s *ui.IconActionState, branch string) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			b := th.Button(&s.Click, i18n.T("git.resolve.accept", i18n.Args{"branch": branch}))
			b.Size, b.Variant, b.Disabled = ui.SizeSM, ui.VariantOutline, left == 0
			return b.Layout(gtx)
		})
	}
	return layout.Inset{Left: ui.Space(3), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, para(title).Layout),
			icon(&r.prev, icPrevChunk, i18n.T("git.resolve.prev")), ui.Gap(1),
			icon(&r.next, icNextChunk, i18n.T("git.resolve.next")), ui.Gap(2),
			accept(&r.all[0], branchOrHead(p.git.Branch)), ui.Gap(2),
			accept(&r.all[1], p.git.Merging), ui.Gap(2),
			icon(&r.close, icDockClose, i18n.T("git.resolve.close")),
		)
	})
}

// titles names the three panes over them.
func (r *conflictResolver) titles(gtx C, th *ui.Theme, git backend.DiffStats) D {
	name := func(branch, role string) layout.Widget {
		return func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextXS, branch).Mono().Weight(font.Medium).Layout), ui.Gap(1.5),
				layout.Rigid(th.Text(ui.TextXS, role).Muted().Layout))
		}
	}
	return r.row(gtx, layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5), Left: ui.Space(2)},
		name(branchOrHead(git.Branch), i18n.T("git.resolve.thisCheckout")),
		th.Text(ui.TextXS, i18n.T("git.resolve.result")).Weight(font.Medium).Layout,
		name(git.Merging, i18n.T("git.resolve.incoming")),
		nil, nil)
}

// row lays three columns out at the panes' widths, the two gutters between.
// The gutters come last: their buttons stand at lines of the panes beside
// them, which this frame's layout of those panes places.
func (r *conflictResolver) row(gtx C, in layout.Inset, left, mid, right, gutterL, gutterR layout.Widget) D {
	gw := gtx.Dp(resolverGutter)
	rest := max(gtx.Constraints.Max.X-2*gw, 0)
	wl := int(float32(rest) / (2 + resolverResultShare))
	wm := rest - 2*wl
	xs := [5]int{0, wl, wl + gw, wl + gw + wm, wl + 2*gw + wm}
	ws := [5]int{wl, gw, wm, gw, rest - wl - wm}
	cells := [5]layout.Widget{left, gutterL, mid, gutterR, right}
	height := 0
	for _, i := range []int{0, 2, 4, 1, 3} {
		if cells[i] == nil {
			continue
		}
		cgtx := gtx
		cgtx.Constraints.Min.X, cgtx.Constraints.Max.X = ws[i], ws[i]
		t := op.Offset(image.Pt(xs[i], 0)).Push(gtx.Ops)
		dims := in.Layout(cgtx, cells[i])
		t.Pop()
		height = max(height, dims.Size.Y)
	}
	return D{Size: image.Pt(gtx.Constraints.Max.X, height)}
}

func (r *conflictResolver) footer(gtx C, th *ui.Theme, p *reviewPanel) D {
	left := r.model.unresolved(r.doc())
	why := ""
	if left > 0 {
		why = i18n.T("git.merge.commitBlocked")
	}
	return layout.Inset{Left: ui.Space(3), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
			layout.Rigid(func(gtx C) D {
				b := th.Button(&r.apply, i18n.T("git.resolve.apply"))
				b.Size, b.Disabled = ui.SizeSM, left > 0
				return th.Tooltip(p.root, &r.applyTip, why).Layout(gtx, b.Layout)
			}))
	})
}

// saveResolution writes the result over the file and stages it; the
// resolver closes once that is done.
func (p *reviewPanel) saveResolution(r *conflictResolver) {
	path, rel := p.target, r.rel
	state := r.result.State()
	content := state.SliceDoc(0, len(state.Doc()))
	_, name := splitPath(rel)
	go func() {
		defer p.written(path)
		if err := p.client.GitResolveFile(context.Background(), path, rel, content); err != nil {
			p.notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("git.resolve.applyFailed", i18n.Args{"name": name}), Description: backend.ErrorText(err)})
			return
		}
		p.mu.Lock()
		p.resolvedRel = rel
		p.mu.Unlock()
		p.invalidate()
	}()
}

// offset draws w at y in a gutter, skipping it when it falls off the pane.
func offset(gtx C, y int, w layout.Widget) {
	if y < -gtx.Dp(ui.Space(6)) || y > gtx.Constraints.Max.Y {
		return
	}
	defer op.Offset(image.Pt(0, y)).Push(gtx.Ops).Pop()
	gtx.Constraints.Min = image.Point{}
	w(gtx)
}

// openResolver shows rel in the resolver, reading its conflict the first
// time.
func (p *reviewPanel) openResolver(rel string) {
	if p.resolvers[rel] != nil {
		p.resolving = rel
		return
	}
	path, family, size := p.target, p.family, p.size
	_, name := splitPath(rel)
	go func() {
		c, err := p.client.GitConflict(context.Background(), path, rel)
		var r *conflictResolver
		if err == nil {
			r, err = newConflictResolver(rel, c, family, size)
		}
		if err != nil {
			p.notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("git.resolve.openFailed", i18n.Args{"name": name}), Description: backend.ErrorText(err)})
			return
		}
		p.mu.Lock()
		p.built = r
		p.mu.Unlock()
		p.invalidate()
	}()
}

// takeResolvers lays in a resolver built since the last frame, drops the one
// whose file was saved, and the ones whose file is no longer in conflict.
func (p *reviewPanel) takeResolvers() {
	p.mu.Lock()
	built, saved := p.built, p.resolvedRel
	p.built, p.resolvedRel = nil, ""
	p.mu.Unlock()
	if p.resolvers == nil {
		p.resolvers = map[string]*conflictResolver{}
	}
	if built != nil {
		p.resolvers[built.rel], p.resolving = built, built.rel
	}
	for rel := range p.resolvers {
		if rel == saved || !slices.Contains(p.git.Conflicted, rel) {
			delete(p.resolvers, rel)
		}
	}
	if p.resolvers[p.resolving] == nil {
		p.resolving = ""
	}
}
