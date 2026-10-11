package main

import (
	"fmt"
	"image"
	"log"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror"
	"github.com/omartelo/lich/native/lichdiff"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n (diff.fileDiff,
// diff.injectMenu, diff.binaryPreview).
const (
	fileWhitespaceOnly = "Whitespace only"
	fileOnlyWhitespace = "Only whitespace changed"
	fileBinary         = "Binary file"
	fileAddAsContext   = "Add file as context"
	fileDiscard        = "Discard Changes"
	menuRevertSelected = "Revert selected lines"
	menuRevertOne      = "Revert %d changed line"
	menuRevertOther    = "Revert %d changed lines"
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
	viewFail     bool
	afford       *affordances
	blockAt      int // the line the composer is anchored under, 0 for none
	composerH    int
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
	return !c.git.binary && diffRows(c.git, false, nil) <= largeFileRows
}

func (c *fileCard) onlyWhitespace() bool { return whitespaceOnly(c.shown, c.git) }

// layout is the card: its header, then, open, its body.
func (c *fileCard) layout(gtx C, p *reviewPanel) D {
	if c.toggle.Clicked(gtx) {
		c.open = !c.open
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
				return th.Text(ui.TextXS, fileWhitespaceOnly).Muted().Layout(gtx)
			}
			return diffStat(gtx, th, c.patch.Added, c.patch.Deleted)
		}))
	}
	children = append(children,
		ui.Gap(2), layout.Rigid(func(gtx C) D {
			return iconAction(gtx, th, p.root, &c.attach, icAttach, fileAddAsContext, false)
		}),
		ui.Gap(1), layout.Rigid(func(gtx C) D {
			return iconAction(gtx, th, p.root, &c.discard, icDiscard, fileDiscard, false)
		}))
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
		return bodyLine(gtx, th, fileBinary)
	case c.onlyWhitespace():
		return bodyLine(gtx, th, fileOnlyWhitespace)
	}
	view := c.ensureView(p)
	if view == nil {
		return bodyLine(gtx, th, fmt.Sprintf(toastReadFailed, c.git.path))
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
	view.Split = p.splitShown(gtx)
	view.SetPatch(c.patch)
	c.placeComposer(view)
	rows := diffRows(c.shown, view.Split, c.filled)
	height := rows*p.cellHeight(gtx) + c.composerH
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, height))
	dims, err := view.Layout(gtx)
	if err != nil && !c.viewFail {
		c.viewFail = true
		log.Printf("review %s: %v", c.git.path, err)
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
	opts := godemirror.DiffOptions{HighlightChanges: true, ExpandWidget: c.afford.expandButton, RevertWidget: c.afford.revertButton}
	opts.Expand = func(g godemirror.Gap) { p.expand(c, c.gen, g) }
	if c.git.status != statusRenamed {
		opts.Revert = func(ch godemirror.Chunk) { p.revert(c.git.path, lichdiff.RevertLines(ch)) }
	}
	view, err := godemirror.NewDiffView(p.family, p.size, opts)
	if err != nil {
		c.viewFail = true
		log.Printf("review %s: %v", c.git.path, err)
		return nil
	}
	c.view = view
	return view
}

// placeComposer anchors the composer under its line, telling the view only
// when that changed: every SetBlocks highlights the content again.
func (c *fileCard) placeComposer(view *godemirror.DiffView) {
	at := 0
	if c.composer.isOpen() {
		at = c.composer.at()
	} else {
		c.composerH = 0
	}
	if at == c.blockAt {
		return
	}
	c.blockAt = at
	if at == 0 {
		view.SetBlocks(nil)
		return
	}
	view.SetBlocks([]godemirror.DiffBlock{{At: godemirror.LineRef{Side: godemirror.NewSide, Line: at}, Widget: c.composerBlock}})
}

// composerSlot is the padding of the gap the composer opens in.
var composerSlot = layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}

// composerBlock is the composer as the view lays it out, measured for the
// card's height.
func (c *fileCard) composerBlock(gtx C) D {
	p := c.panel
	dims := composerSlot.Layout(gtx, func(gtx C) D { return c.composer.layout(gtx, p.th, p.comments, p.target) })
	if dims.Size.Y != c.composerH {
		c.composerH = dims.Size.Y
		p.invalidate()
	}
	return dims
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
	if c.git.status == statusRenamed {
		return entries
	}
	revert := menuRevertSelected
	switch n := len(changes); {
	case n == 1:
		revert = fmt.Sprintf(menuRevertOne, n)
	case n > 1:
		revert = fmt.Sprintf(menuRevertOther, n)
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
