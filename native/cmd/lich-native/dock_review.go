package main

// The dock's Review tab: the active session's working-tree diff, one
// collapsible card per file, with the comment batch at its foot. It is the
// dock's reviewTab; the dock owns the chrome around it.
//
// The window lays the discard confirmation out at its own level, before the
// content, as it does every dialog: review.layoutDialogs(gtx, th).

import (
	"context"
	"image"
	"log"
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/glyphs"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
)

// revertUndoFor is REVERT_UNDO_MS: a revert's Undo stays longer than a plain
// toast, since the reader looks at the diff that just changed first.
const revertUndoFor = 10 * time.Second

// splitMinWidth is SPLIT_MIN_WIDTH_PX: narrower, each side of a split holds
// under ~35 columns of code.
const splitMinWidth = unit.Dp(560)

// reviewPanel is the Review tab. Every field is the UI goroutine's but the
// gap answers, which arrive from the backend.
type reviewPanel struct {
	client     *lichclient.Client
	root       *ui.Root
	comments   *commentStore
	notify     func(ui.Toast)
	invalidate func()
	family     string
	size       unit.Sp
	cells      *glyphs.Rasterizer

	// wrote hears the checkout an edit of the panel's own changed, for the git
	// store the card, footer and badge read.
	wrote func(checkout string)

	// The frame's: theme, palette, checkout and its latest read.
	th      *ui.Theme
	light   bool
	code    godemirror.Theme
	target  string
	inject  func(string) bool
	submit  func(string) bool
	shown   *diffRead
	splitOK bool

	feed     diffFeed
	split    bool
	hiding   bool
	bulkOpen bool
	bulk     int
	// cardsFor is the checkout the cards were built from: another starts
	// them over.
	cardsFor string
	cards    map[string]*fileCard
	order    []string
	list     widget.List
	heights  itemHeights
	// viewport is the list's height and laying the item it is laying out,
	// so a Markdown preview draws only the blocks on screen.
	viewport, laying int
	batch            commentBatch
	actions          reviewActions

	pendingDiscard *patchFile
	discardShown   patchFile
	dialog         ui.DialogState
	confirm, back  widget.Clickable

	mu      sync.Mutex
	answers []gapAnswer

	// pr makes this the Pulls screen's Files tab rather than the dock's
	// Review tab; nil for the dock.
	pr *prFiles
}

// prFiles is a reviewPanel showing a pull request's diff instead of a
// checkout's working tree: nothing to revert or discard, its gaps read at the
// PR's head, and each file tickable as viewed.
type prFiles struct {
	number int
	head   string
	// mergeBase is the old side every image card reads, asked once per head.
	mergeBase mergeBaseMemo
	url       string // the pull request's page
	// viewed is each ticked file's diff text when it was ticked: a commit
	// that changes the file unticks it, one that does not leaves it.
	viewed map[string]string
	ticks  map[string]*widget.Bool
	// toReview files a line comment into the PR's pending review.
	toReview func(path, lines, body string)
	// blocks are the review threads and pending comments under a file's
	// lines.
	blocks func(path string) []cardBlock
}

func (f *prFiles) isViewed(c *fileCard) bool { return f.viewed[c.git.path] == c.gitText }

type gapAnswer struct {
	path  string
	gen   int
	gap   godemirror.Gap
	lines []string
}

func newReviewPanel(client *lichclient.Client, root *ui.Root, comments *commentStore,
	family string, size unit.Sp, invalidate func(), notify func(ui.Toast)) (*reviewPanel, error) {
	cells, err := glyphs.New(family)
	if err != nil {
		return nil, err
	}
	p := &reviewPanel{client: client, root: root, comments: comments, notify: notify,
		invalidate: invalidate, family: family, size: size, cells: cells, bulkOpen: true, cards: map[string]*fileCard{}}
	p.feed = diffFeed{client: client, invalidate: invalidate}
	p.list.Axis = layout.Vertical
	return p, nil
}

// hidden stops the polling while the tab is off screen.
func (p *reviewPanel) hidden() { p.feed.close() }

// holdsKeys keeps the terminal off the note field and any open composer. The
// terminal asks before the dock is laid out, so it is read off the focus as
// it stands, never a flag the last layout left.
func (p *reviewPanel) holdsKeys(gtx C) bool {
	if gtx.Focused(&p.batch.note) {
		return true
	}
	for _, c := range p.cards {
		if c.composer.holdsKeys(gtx) {
			return true
		}
	}
	return false
}

// layoutBody is the panel under the dock's header: the checkout's
// working-tree diff, and the comment batch at its foot.
func (p *reviewPanel) layoutBody(gtx C, th *ui.Theme, in dockInput) error {
	p.th, p.light, p.code, p.target = th, in.light, in.code, in.checkout
	p.inject, p.submit = in.inject, in.submit
	p.feed.follow(in.checkout)
	p.takeRead(in.checkout, p.feed.latest())
	p.takeAnswers()
	return p.bodyWithBatch(gtx, th)
}

// bodyWithBatch is the files over the comment batch at their foot.
func (p *reviewPanel) bodyWithBatch(gtx C, th *ui.Theme) error {
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			p.panelBody(gtx, th)
			return D{Size: gtx.Constraints.Max}
		}),
		layout.Rigid(func(gtx C) D { return p.batch.layout(gtx, th, p.comments, p.target, p.submit, p.notify) }),
	)
	return nil
}

// takeRead lays the newest read into the cards. Only a changed text touches
// them, so a poll that read the same diff leaves every editor, its selection
// and its scroll as they were.
func (p *reviewPanel) takeRead(checkout string, read *diffRead) {
	if checkout != p.cardsFor {
		p.cardsFor, p.cards, p.order = checkout, map[string]*fileCard{}, nil
	}
	p.shown = read
	if read == nil || read.failed {
		return
	}
	files, err := parsePatchFiles(read.text)
	if err != nil {
		log.Printf("review: %v", err)
	}
	p.order = p.order[:0]
	seen := map[string]bool{}
	for _, f := range files {
		p.order, seen[f.path] = append(p.order, f.path), true
		if c, ok := p.cards[f.path]; ok {
			c.sync(f, p.hiding)
			continue
		}
		p.cards[f.path] = newFileCard(p, f, p.hiding, p.bulk)
	}
	for path := range p.cards {
		if !seen[path] {
			delete(p.cards, path)
		}
	}
}

// cellHeight is one diff row in pixels, as godemirror lays it out: its
// rasterizer's line height for the same family and size.
func (p *reviewPanel) cellHeight(gtx C) int {
	if _, err := p.cells.SetSize(float64(gtx.Sp(p.size))); err != nil {
		log.Printf("review: %v", err)
	}
	return p.cells.CellH()
}

// scrollToFile brings path's card to the top of the list, past the squeezed
// note when it is drawn.
func (p *reviewPanel) scrollToFile(path string) {
	if i, ok := p.itemOf(path); ok {
		p.list.Position.First, p.list.Position.Offset = i, 0
	}
}

// itemOf is the list item path's card is drawn in.
func (p *reviewPanel) itemOf(path string) (int, bool) {
	for i, f := range p.order {
		if f != path {
			continue
		}
		if p.split && !p.splitOK {
			i++
		}
		return i, true
	}
	return 0, false
}

// markViewed ticks c and folds it.
func (p *reviewPanel) markViewed(c *fileCard) {
	p.pr.viewed[c.git.path] = c.gitText
	p.fold(c)
}

// fold closes c. When its top had already scrolled away, the list moves back
// to it so the next file sits right under the fold instead of several files
// further down.
func (p *reviewPanel) fold(c *fileCard) {
	c.open = false
	if i, ok := p.itemOf(c.git.path); ok && p.list.Position.First == i && p.list.Position.Offset > 0 {
		p.scrollToFile(c.git.path)
	}
}

// splitShown is shownLayout: side by side only once the cards are wide
// enough.
func (p *reviewPanel) splitShown(gtx C) bool { return p.split && p.splitOK }

// diffTheme is the window theme's code palette on the panel's own ground.
func (p *reviewPanel) diffTheme() godemirror.Theme {
	t := p.code
	t.Background, t.GutterBackground = p.th.Surface, p.th.Surface
	return t
}

// expand fetches a gap's lines off the UI goroutine: from the working tree,
// or from a pull request's head.
func (p *reviewPanel) expand(c *fileCard, gen int, g godemirror.Gap) {
	path, rel, ref := p.target, c.git.path, ""
	if p.pr != nil {
		ref = p.pr.head
	}
	go func() {
		lines, err := p.client.FileLines(context.Background(), path, rel, ref, g.From, g.To)
		if err != nil {
			p.notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("diff.fileDiff.readFailed", i18n.Args{"path": rel})})
			return
		}
		p.mu.Lock()
		p.answers = append(p.answers, gapAnswer{path: rel, gen: gen, gap: g, lines: lines})
		p.mu.Unlock()
		p.invalidate()
	}()
}

func (p *reviewPanel) takeAnswers() {
	p.mu.Lock()
	answers := p.answers
	p.answers = nil
	p.mu.Unlock()
	for _, a := range answers {
		if c, ok := p.cards[a.path]; ok {
			c.fill(a.gen, a.gap, a.lines)
		}
	}
}

// written reads the checkout again after an edit the panel made itself.
func (p *reviewPanel) written(checkout string) {
	p.feed.refresh()
	if p.wrote != nil {
		p.wrote(checkout)
	}
}

// revert puts changed lines of rel back to HEAD, with no confirmation: the
// toast carries the undo for as long as it stays up.
func (p *reviewPanel) revert(rel string, lines []lichclient.RevertLine) {
	path := p.target
	_, name := splitPath(rel)
	go func() {
		defer p.written(path)
		undo, err := p.client.RevertLines(context.Background(), path, rel, lines)
		if err != nil {
			p.notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("diff.reviewPanel.revertFailed", i18n.Args{"name": name}), Description: lichclient.ErrorText(err)})
			return
		}
		added, deleted := revertStat(lines)
		p.notify(ui.Toast{Title: i18n.T("diff.reviewPanel.reverted", i18n.Args{"added": added, "deleted": deleted, "name": name}), Duration: revertUndoFor,
			Action: &ui.ToastButton{Label: i18n.T("diff.reviewPanel.undo"), OnClick: func() { go p.restore(path, rel, name, undo) }}})
	}()
}

func (p *reviewPanel) restore(path, rel, name string, undo lichclient.RevertResult) {
	defer p.written(path)
	if err := p.client.Call(context.Background(), "project.RestoreLines", nil, path, rel, undo); err != nil {
		p.notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("diff.reviewPanel.undoFailed", i18n.Args{"name": name}), Description: lichclient.ErrorText(err)})
	}
}

// revertStat counts a revert the way DiffStat draws a diff.
func revertStat(lines []lichclient.RevertLine) (added, deleted int) {
	for _, l := range lines {
		if l.Side == "new" {
			added++
		}
	}
	return added, len(lines) - added
}

// layoutDialogs is the discard confirmation, laid out by the window before
// its content so it takes Escape first.
func (p *reviewPanel) layoutDialogs(gtx C, th *ui.Theme) {
	if p.pendingDiscard != nil {
		p.discardShown = *p.pendingDiscard
	}
	f := p.discardShown
	switch {
	case p.confirm.Clicked(gtx):
		p.discard(f)
	case p.back.Clicked(gtx):
		p.pendingDiscard = nil
	}
	description := i18n.T("diff.discardDialog.description", i18n.Args{"path": f.path})
	if f.status == statusAdded {
		description = i18n.T("diff.discardDialog.descriptionAdded", i18n.Args{"path": f.path})
	}
	pop := th.On(th.Popover)
	d := th.Dialog(&p.dialog, p.pendingDiscard != nil, i18n.T("diff.discardDialog.title"), description)
	d.MaxWidth = ui.DialogWidthXL
	confirm := pop.Button(&p.confirm, i18n.T("diff.discardDialog.confirm"))
	confirm.Variant = ui.VariantDestructive
	back := pop.Button(&p.back, i18n.T("common.action.cancel"))
	back.Variant = ui.VariantOutline
	d.Actions = []layout.Widget{back.Layout, confirm.Layout}
	if d.Layout(gtx) {
		p.pendingDiscard = nil
	}
}

// discard reverts the file and, for a rename, its old path too.
func (p *reviewPanel) discard(f patchFile) {
	p.pendingDiscard = nil
	path := p.target
	go func() {
		defer p.written(path)
		for _, rel := range discardTargets(f) {
			if err := p.client.Call(context.Background(), "project.DiscardFile", nil, path, rel); err != nil {
				p.notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("diff.reviewPanel.discardFailed", i18n.Args{"error": lichclient.ErrorText(err)})})
				return
			}
		}
	}()
}

// stickyAt is where the header of the card scrolled into sits: pinned to
// the top, then pushed up by the next card. ok is false when no header needs
// drawing over the list.
func stickyAt(offset, cardHeight, headerHeight int) (y int, ok bool) {
	if offset <= 0 || offset >= cardHeight {
		return 0, false
	}
	return min(0, cardHeight-offset-headerHeight), true
}

// cardSpan is where a card sat in its list item this frame: its top, its
// height and its header's.
type cardSpan struct{ top, height, header int }

// layoutSticky draws the first visible card's header again over the list.
func (p *reviewPanel) layoutSticky(gtx C, spans map[string]cardSpan) {
	pos := p.list.Position
	if pos.First >= len(p.order) {
		return
	}
	path := p.order[pos.First]
	s, laid := spans[path]
	y, ok := stickyAt(pos.Offset-s.top, s.height, s.header)
	if !laid || !ok {
		return
	}
	pad := gtx.Dp(ui.Space(2))
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	defer op.Offset(image.Pt(pad, y)).Push(gtx.Ops).Pop()
	gtx.Constraints.Min.Y = 0
	gtx.Constraints.Max.X -= 2 * pad
	p.cards[path].header(gtx, p)
}

// setFont redraws the cards in family at size. Each card builds its editor
// again on its next frame, with what it had filled in forgotten.
func (p *reviewPanel) setFont(family string, size unit.Sp) error {
	if family == p.family && size == p.size {
		return nil
	}
	cells, err := glyphs.New(family)
	if err != nil {
		return err
	}
	p.cells, p.family, p.size = cells, family, size
	for _, c := range p.cards {
		c.view, c.viewFail, c.gen, c.filled = nil, false, c.gen+1, map[int]int{}
	}
	p.invalidate()
	return nil
}
