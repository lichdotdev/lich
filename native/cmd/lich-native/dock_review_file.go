package main

import (
	"image"
	"log"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichdiff"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// largeFileRows is FileDiff's LARGE_FILE_LINES: a file whose diff draws more
// rows starts collapsed, so one lockfile does not swamp the panel.
const largeFileRows = 500

var (
	icChevronDown  = icons.Lucide("chevron-down")
	icChevronRight = icons.Lucide("chevron-right")
	icDiscard      = icons.Lucide("undo-2")
)

// fileCard is FileDiff: one file of the diff, collapsible, its editor built
// the first time it is laid out open and kept until the card goes.
type fileCard struct {
	git, shown patchFile // as git printed it, and as drawn (whitespace hidden)
	gitText    string    // what tells a new version of the file from a refetch
	shownText  string
	hiding     bool
	patch      godemirror.FilePatch
	open       bool
	bulkSeen   int
	// gen counts the versions shown, so a gap answered for an older one is
	// dropped; filled is what was pulled into each gap, keyed by its last line.
	gen    int
	filled map[int]int

	view         *godemirror.DiffView
	bin          *binaryPreview
	viewFail     bool
	afford       *affordances
	blocks       cardBlocks
	headerHeight int

	toggle          widget.Clickable
	attach, discard iconActionState
	menu            ui.MenuState
	composer        commentComposer
	panel           *reviewPanel
}

func newFileCard(p *reviewPanel, f patchFile, hiding bool, bulkSeen int) *fileCard {
	c := &fileCard{panel: p, bulkSeen: bulkSeen, afford: newAffordances()}
	c.sync(f, hiding)
	return c
}

// sync takes a fresh read of the file. A new text is a new file: it opens
// or folds by its size again and drops what was pulled into its gaps, which
// are numbered against a file that no longer exists; hiding whitespace moves
// the gaps too.
func (c *fileCard) sync(f patchFile, hiding bool) {
	text := f.text()
	changed := text != c.gitText
	if !changed && hiding == c.hiding {
		return
	}
	c.git, c.gitText, c.hiding = f, text, hiding
	shown := f
	if hiding {
		shown = hideWhitespace(f)
	}
	c.shown = shown
	// godemirror keeps what was filled while the drawn diff stays the same,
	// so the count of filled lines starts over only when it changes.
	if shownText := shown.text(); shownText != c.shownText {
		patch, err := shown.filePatch()
		if err != nil {
			log.Printf("review %s: %v", f.path, err)
		}
		c.shownText, c.patch, c.gen, c.filled = shownText, patch, c.gen+1, map[int]int{}
	}
	if changed {
		c.open = c.openByDefault()
	}
}

func (c *fileCard) openByDefault() bool {
	if c.git.binary {
		return c.previewsImage()
	}
	return diffRows(c.git, false, nil) <= largeFileRows
}

func (c *fileCard) onlyWhitespace() bool { return whitespaceOnly(c.shown, c.git) }

// layout is the card: its header, then, open, its body.
func (c *fileCard) layout(gtx C, p *reviewPanel) D {
	if c.toggle.Clicked(gtx) {
		if c.open {
			p.fold(c)
		} else {
			c.open = true
		}
	}
	if c.attach.click.Clicked(gtx) {
		p.inject(fileReference(c.git.path))
	}
	if c.discard.click.Clicked(gtx) {
		p.pendingDiscard = &c.git
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return c.header(gtx, p) }),
		layout.Rigid(func(gtx C) D {
			if !c.open {
				return D{}
			}
			return c.body(gtx, p)
		}),
	)
}

// header is the sticky row (bg-sidebar px-2 py-1 text-xs gap-2), ruled
// underneath while the file is open.
func (c *fileCard) header(gtx C, p *reviewPanel) D {
	th := p.th
	on := th.On(th.Sidebar)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	dims := layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, th.Sidebar, 0) },
		func(gtx C) D {
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
				return c.headerRow(gtx, on, p)
			})
		},
	)
	c.headerHeight = dims.Size.Y
	if c.open {
		defer op.Offset(image.Pt(0, dims.Size.Y-gtx.Dp(1))).Push(gtx.Ops).Pop()
		on.Separator(gtx)
	}
	return dims
}

func (c *fileCard) headerRow(gtx C, th *ui.Theme, p *reviewPanel) D {
	children := []layout.FlexChild{
		layout.Flexed(1, func(gtx C) D { return c.toggle.Layout(gtx, func(gtx C) D { return c.title(gtx, th, p.light) }) }),
	}
	if !c.git.binary {
		children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D {
			if c.onlyWhitespace() {
				return th.Text(ui.TextXS, i18n.T("diff.fileDiff.whitespaceOnly")).Muted().Layout(gtx)
			}
			return diffStat(gtx, th, c.patch.Added, c.patch.Deleted)
		}))
	}
	children = append(children,
		ui.Gap(2), layout.Rigid(func(gtx C) D {
			return iconAction(gtx, th, p.root, &c.attach, icAttach, i18n.T("diff.fileDiff.addAsContext"), false)
		}))
	if p.pr != nil {
		children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D { return c.viewedTick(gtx, th, p) }))
	} else {
		children = append(children, ui.Gap(1), layout.Rigid(func(gtx C) D {
			return iconAction(gtx, th, p.root, &c.discard, icDiscard, i18n.T("diff.fileDiff.discard"), false)
		}))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// title is the toggle's face: chevron, language badge, name, directory.
func (c *fileCard) title(gtx C, th *ui.Theme, light bool) D {
	chevron := icChevronRight
	if c.open {
		chevron = icChevronDown
	}
	dir, base := splitPath(c.git.path)
	if c.git.status == statusRenamed {
		base = c.git.oldPath + " → " + base
	}
	return layout.Inset{Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return chevron.Layout(gtx, ui.Space(3.5), th.MutedForeground) }),
			ui.Gap(2),
			layout.Rigid(func(gtx C) D { return badge(gtx, th, languageAbbr(c.git.path), light) }),
			ui.Gap(2),
			layout.Rigid(func(gtx C) D { return truncated(gtx, th.Text(ui.TextXS, base).Weight(font.Medium)) }),
			ui.Gap(2),
			layout.Flexed(1, func(gtx C) D { return truncated(gtx, th.Text(ui.TextXS, dir).Muted()) }),
		)
	})
}

func truncated(gtx C, l ui.LabelStyle) D {
	l.MaxLines = 1
	return l.Layout(gtx)
}

// badgeSize and badgeText are the badge's size-5 and text-[0.5625rem].
const (
	badgeSize = unit.Dp(20)
	badgeText = unit.Sp(9)
)

func badge(gtx C, th *ui.Theme, b langBadge, light bool) D {
	bg, fg := b.colors(th, light)
	size := gtx.Dp(badgeSize)
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	ui.Fill(gtx, bg, ui.RadiusSM)
	return layout.Center.Layout(gtx, th.On(bg).Text(badgeText, b.abbr).In(fg).Weight(font.Bold).Layout)
}

// body is what opening the file reveals: the binary line, the whitespace
// line, or the diff with its right-click menu.
func (c *fileCard) body(gtx C, p *reviewPanel) D {
	th := p.th
	switch {
	case c.git.binary:
		return c.binaryBody(gtx, p)
	case c.onlyWhitespace():
		return bodyLine(gtx, th, i18n.T("diff.fileDiff.onlyWhitespaceChanged"))
	}
	view := c.ensureView(p)
	if view == nil {
		return bodyLine(gtx, th, i18n.T("diff.fileDiff.readFailed", i18n.Args{"path": c.git.path}))
	}
	return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return th.ContextMenu(p.root, &c.menu, c.menuEntries(p)).Layout(gtx, func(gtx C) D {
			return c.diff(gtx, p, view)
		})
	})
}

// bodyLine is the px-9 py-2 text-xs muted line a body without a diff shows.
func bodyLine(gtx C, th *ui.Theme, text string) D {
	return layout.Inset{Left: ui.Space(9), Right: ui.Space(9), Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx,
		th.Text(ui.TextXS, text).Muted().Layout)
}

// diff lays the editor out exactly as tall as its rows and the composer
// open under them, so the panel scrolls and the editor never does.
func (c *fileCard) diff(gtx C, p *reviewPanel, view *godemirror.DiffView) D {
	c.afford.theme = p.th
	view.SetTheme(p.diffTheme())
	view.Split, view.Nested = p.splitShown(gtx), true
	view.SetPatch(c.patch)
	c.blocks.place(view, c.cardBlocks(p))
	rows := diffRows(c.shown, view.Split, c.filled)
	height := rows*p.cellHeight(gtx) + c.blocks.height()
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, height))
	c.blocks.startFrame()
	dims, err := view.Layout(gtx)
	if err != nil && !c.viewFail {
		c.viewFail = true
		log.Printf("review %s: %v", c.git.path, err)
	}
	if c.blocks.endFrame() {
		p.invalidate()
	}
	return dims
}

// ensureView builds the card's editor on first need. A rename gets no revert:
// its old side lives under another path, which the one-file revert cannot
// reach; Discard still reverts the pair.
func (c *fileCard) ensureView(p *reviewPanel) *godemirror.DiffView {
	if c.view != nil || c.viewFail {
		return c.view
	}
	opts := godemirror.DiffOptions{HighlightChanges: true, StyleTags: codeStyleTags, ExpandWidget: c.afford.expandButton, RevertWidget: c.afford.revertButton}
	opts.Expand = func(g godemirror.Gap) { p.expand(c, c.gen, g) }
	if c.git.status != statusRenamed && p.pr == nil {
		opts.Revert = func(ch godemirror.Chunk) { p.revert(c.git.path, lichdiff.RevertLines(ch)) }
	}
	view, err := newDiffView(p.family, p.size, opts)
	if err != nil {
		c.viewFail = true
		log.Printf("review %s: %v", c.git.path, err)
		return nil
	}
	c.view = view
	return view
}

// composerSlot is the padding of the gap the composer opens in.
var composerSlot = layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}

// composerBlock is the composer as the view lays it out.
func (c *fileCard) composerBlock(gtx C) D {
	p := c.panel
	if p.pr != nil {
		c.composer.toReview = p.pr.toReview
	}
	return composerSlot.Layout(gtx, func(gtx C) D { return c.composer.layout(gtx, p.th, p.comments, p.target) })
}

// cardBlocks are what hangs under the card's lines: the composer while it is
// open, and on the Pulls screen the review threads and pending comments.
func (c *fileCard) cardBlocks(p *reviewPanel) []cardBlock {
	var out []cardBlock
	if c.composer.isOpen() {
		at := godemirror.LineRef{Side: godemirror.NewSide, Line: c.composer.at()}
		out = append(out, cardBlock{key: "composer", at: at, widget: c.composerBlock})
	}
	if p.pr != nil && p.pr.blocks != nil {
		out = append(out, p.pr.blocks(c.git.path)...)
	}
	return out
}

// menuEntries is InjectMenu over the selection as it stands.
func (c *fileCard) menuEntries(p *reviewPanel) []ui.MenuEntry {
	path, lines, changes := c.git.path, "", []godemirror.ChangedLine(nil)
	if r, ok := c.view.SelectedRange(); ok {
		if r.NewFrom > 0 {
			lines = formatLineRef(r.NewFrom, r.NewTo)
		}
		changes = c.view.SelectedChanges()
	}
	entries := injectEntries(path, lines, p.inject, func() { c.composer.open(path, lines) })
	if p.pr != nil {
		return append(entries, reviewCommentEntry(lines, func() { c.composer.openReview(path, lines) }))
	}
	if c.git.status == statusRenamed {
		return entries
	}
	revert := i18n.T("diff.injectMenu.revertSelected")
	switch n := len(changes); {
	case n > 0:
		revert = i18n.T("diff.injectMenu.revert", i18n.Args{"count": n})
	}
	reverted := lichdiff.RevertLines(godemirror.Chunk{Lines: changes})
	return append(entries, ui.MenuEntry{Kind: ui.MenuSeparator},
		ui.MenuEntry{Label: revert, Variant: ui.VariantDestructive, Disabled: len(changes) == 0,
			OnClick: func() { p.revert(path, reverted) }})
}

// fill lays a gap's answer in, unless the card has moved on since it asked.
func (c *fileCard) fill(gen int, g godemirror.Gap, lines []string) {
	if gen != c.gen || c.view == nil {
		return
	}
	c.view.FillGap(g, lines)
	c.filled[g.To] += min(len(lines), g.To-g.From+1)
}
