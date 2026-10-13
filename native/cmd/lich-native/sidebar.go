package main

import (
	"image"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// remDp is one rem in dp: the web's 16px root font.
const remDp = 16

type sidebar struct {
	root *ui.Root // where tooltips and menus float
	// The session cards, keyed by session id.
	tips    map[string]*ui.Tooltip
	menus   map[string]*ui.MenuState
	renames map[string]*ui.InlineEdit
	// renamed is the label a rename committed during this frame's layout.
	renamed  sessionRename
	cards    map[string]*widget.Clickable
	closes   map[string]*widget.Clickable
	pins     map[string]*widget.Clickable
	prChips  map[string]*widget.Clickable
	icBranch *icons.Icon
	icClose  *icons.Icon
	icPin    *icons.Icon
	icShield *icons.Icon
	icBehind *icons.Icon
	icClash  *icons.Icon
	icPR     *icons.Icon
	started  time.Time

	// The panel: its persisted layout and the resize strip on its edge.
	widthRem float64
	open     bool
	resize   ui.ResizeHandle
	list     widget.List
	hints    map[string]*ui.Tooltip
	header   sidebarHeader
	sort     ui.Sortable // the blocks, by their headers
	carry    ui.SortCarry
	blocks   map[string]*blockState
	naming   folderNaming
	rail     sidebarRail
	morph    sidebarMorph
	// launchProviders is this frame's m.launchProviders(), what every launch
	// menu lists.
	launchProviders []launchProvider
}

func newSidebar(root *ui.Root) *sidebar {
	s := &sidebar{
		root:     root,
		cards:    map[string]*widget.Clickable{},
		closes:   map[string]*widget.Clickable{},
		pins:     map[string]*widget.Clickable{},
		tips:     map[string]*ui.Tooltip{},
		menus:    map[string]*ui.MenuState{},
		renames:  map[string]*ui.InlineEdit{},
		prChips:  map[string]*widget.Clickable{},
		icBranch: icons.Lucide("git-branch"),
		icClose:  icons.Lucide("x"),
		icPin:    icons.Lucide("pin"),
		icShield: icons.Lucide("shield"),
		icBehind: icons.Lucide("arrow-down"),
		icClash:  icons.Lucide("triangle-alert"),
		icPR:     icons.Lucide("git-pull-request-arrow"),
		started:  time.Now(),
		widthRem: sidebarDefaultRem,
		open:     true,
		hints:    map[string]*ui.Tooltip{},
		blocks:   map[string]*blockState{},
		rail:     sidebarRail{cards: map[string]*widget.Clickable{}, tips: map[string]*ui.Tooltip{}},
	}
	s.header.phases = map[sessionPhase]bool{}
	s.list.Axis = layout.Vertical
	s.rail.list.Axis = layout.Vertical
	return s
}

// holdsKeys reports whether the sidebar wants the keyboard the terminal
// otherwise takes: the filter field holds focus, a card or a folder is being
// renamed, a menu is open, or the new folder dialog is up.
func (s *sidebar) holdsKeys() bool {
	if s.header.searchFocused || s.naming.open {
		return true
	}
	for _, r := range s.renames {
		if r.Active() {
			return true
		}
	}
	for _, m := range s.menus {
		if m.Opened() {
			return true
		}
	}
	if s.header.newMenu.Opened() {
		return true
	}
	for _, b := range s.blocks {
		if b.rename.Active() || b.launch.Opened() || b.options.Opened() {
			return true
		}
	}
	return false
}

// sidebarAction is what a frame's clicks asked for.
type sidebarAction struct {
	activate string // session id
	close    string // session id
	pin      string // session id
	pinTo    bool   // the pin the click asked for
	openPR   int    // a pull request to show on the Pulls screen
	launch   launchRequest
	// Open in, from the card menu: a directory for a shell session, the
	// editor or the file manager.
	terminalAt, editorAt, folderAt string
	rename                         sessionRename
	color, colorTo                 string // session id, and its new tint ("" for the theme)
	continueIn                     continueRequest
	worktree                       bool // the + menu asked for a worktree session
	sidebarWrites
}

// sidebarWrites are the block-level writes, carried out off the frame by
// writeSidebar.
type sidebarWrites struct {
	fold       string // block key to fold or unfold
	file       fileRequest
	paint      paintRequest
	folder     folderRename
	blockOrder []string // every movable block's key, in the dragged order
	cardOrder  blockReorder
	saveWidth  bool // the resize ended: store the panel's width
	saveOpen   bool // the panel opened or closed: store it
}

// launchRequest is a new session: kind "" for none, dir "" for the project's
// own checkout, folder the folder it is filed under.
type launchRequest struct{ kind, dir, folder string }

// fileRequest files ids under folder, "" to take them out; nil ids for none.
type fileRequest struct {
	ids    []string
	folder string
}

// paintRequest tints every card of a folder at once; nil ids for none.
type paintRequest struct {
	ids   []string
	color string
}

// folderRename renames a folder, or ungroups it for an empty to; from "" for
// none.
type folderRename struct{ from, to string }

// blockReorder is one block's cards in their dragged order; key "" for none.
type blockReorder struct {
	key string
	ids []string
}

// sessionRename is a label typed into a card.
type sessionRename struct{ id, label string }

// takeCard keeps what one card's clicks asked for, leaving the block-level
// writes of the frame alone.
func (a *sidebarAction) takeCard(c sidebarAction) {
	pick := func(dst *string, src string) {
		if src != "" {
			*dst = src
		}
	}
	pick(&a.activate, c.activate)
	pick(&a.close, c.close)
	if c.openPR != 0 {
		a.openPR = c.openPR
	}
	pick(&a.terminalAt, c.terminalAt)
	pick(&a.editorAt, c.editorAt)
	pick(&a.folderAt, c.folderAt)
	if c.pin != "" {
		a.pin, a.pinTo = c.pin, c.pinTo
	}
	if c.color != "" {
		a.color, a.colorTo = c.color, c.colorTo
	}
	if c.rename.id != "" {
		a.rename = c.rename
	}
	if c.file.ids != nil {
		a.file = c.file
	}
	if c.continueIn.id != "" {
		a.continueIn = c.continueIn
	}
}

// layout is App.tsx's sidebar mount: the open sidebar, or its rail, or the
// morph between them.
func (s *sidebar) layout(gtx C, th *ui.Theme, m *model, active string) (D, sidebarAction) {
	if s.morph.active() {
		return s.morphing(gtx, th, m, active)
	}
	return s.shape(gtx, th, m, active, s.open)
}

func (s *sidebar) shape(gtx C, th *ui.Theme, m *model, active string, open bool) (D, sidebarAction) {
	if !open {
		return s.rail.layout(gtx, th, s, m, active)
	}
	return s.panel(gtx, th, m, active)
}

// setOpen is App.tsx's toggleSidebar: the switch, stored, morphed to.
func (s *sidebar) setOpen(gtx C, open bool, act *sidebarAction) {
	s.open, act.saveOpen = open, true
	s.morph.begin(gtx)
}

// panel is SessionSidebar's aside: `relative flex flex-col rounded-lg
// bg-sidebar p-2`, the header row, the filter when open, the blocks, and the
// resize strip on its right edge.
func (s *sidebar) panel(gtx C, th *ui.Theme, m *model, active string) (D, sidebarAction) {
	var act sidebarAction
	width, released := s.resize.Update(gtx, unit.Dp(s.widthRem*remDp), sidebarMinRem*remDp, sidebarMaxRem*remDp)
	s.widthRem, act.saveWidth = float64(width)/remDp, released
	size := image.Pt(gtx.Dp(width), gtx.Constraints.Max.Y)
	gtx.Constraints = layout.Exact(size)
	ui.Fill(gtx, th.Sidebar, ui.RadiusLG)
	on := th.On(th.Sidebar)
	view := m.sidebarBlocks(s.header.filter())
	s.launchProviders = m.launchProviders()
	layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return s.headerRow(gtx, on, m, &act) }),
			ui.VGap(2),
		}
		if s.header.open {
			children = append(children, layout.Rigid(func(gtx C) D { return s.filterRow(gtx, on, view) }), ui.VGap(2))
		}
		children = append(children, layout.Flexed(1, func(gtx C) D {
			// -mr-2: the scroll column reaches the panel's edge, so its
			// scrollbar sits in the padding and its pr-2 keeps the cards off it.
			gtx.Constraints.Max.X += gtx.Dp(ui.Space(2))
			return s.blockList(gtx, on, view, active, &act)
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
	gtx.Constraints = layout.Exact(size)
	s.resize.Layout(gtx, on)
	return D{Size: size}, act
}

// hint is the Tooltip state of one Hint, keyed by what it labels.
func (s *sidebar) hint(key string) *ui.Tooltip {
	t, ok := s.hints[key]
	if !ok {
		t = new(ui.Tooltip)
		s.hints[key] = t
	}
	return t
}
