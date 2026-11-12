package main

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	driver "github.com/lichdotdev/gio-driver"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

const brandName = "lich"

// The strip's geometry, from the approved topbar mockup (option E3).
var (
	topbarHeight = ui.Space(11) // h-11
	brandHeight  = ui.Space(8)
	brandIcon    = ui.Space(5)
	// brandPadLeft centres the icon over the rail, the column every tree
	// node sits in, in both shapes, so it never moves.
	brandPadLeft = (railWidth - brandIcon) / 2
	barGap       = ui.Space(1)
	// mainMenuMinWidth is the context menu's min-w-44: two short labels in
	// the dropdown's min-w-32 read cramped.
	mainMenuMinWidth = ui.Space(44)
)

var (
	icLichMark     = icons.LichMark()
	icBell         = icons.Lucide("bell")
	icPullRequests = icons.Lucide("git-pull-request-arrow")
	icSettings     = icons.Lucide("settings")
	icMainMenu     = icons.Lucide("menu")
	icGuides       = icons.Lucide("circle-question-mark")
)

// topbar is the native window's top strip: the brand over the sidebar's
// column, the project tabs from the terminal island's edge, and the bell,
// Pull requests and the menu on the right. The native window has no Home tab,
// and the active tab merges into the island under it.
type topbar struct {
	root  *ui.Root
	brand widget.Clickable
	sort  ui.Sortable
	list  layout.List
	// tabW is the tabs' width as the strip last laid them out, and laid
	// where each tab was drawn, in window pixels.
	tabW int
	laid []laidTab
	// merge eases the active tab between detached and merged into merged,
	// the island it last grew out of; mergedFor is the tab it eases.
	merge     dockSlide
	merged    islandSpan
	mergedFor string
	tabs      map[string]*tabState
	open      openMenu
	bell      bellMenu
	hints     map[string]*ui.Tooltip
	menu      ui.MenuState
	menuOpen  widget.Clickable
	pulls     widget.Clickable
	// tour is the window's guided tour, which points at the menu.
	tour     *driver.Tour
	confirm  closeConfirm
	controls captionControls

	// surfaceClicks and surfaceIcons are the extension surfaces' buttons
	// (topbar_extensions.go).
	surfaceClicks map[extensionKey]*widget.Clickable
	surfaceIcons  map[string]*icons.Icon

	// shift is how far right of the terminal island's edge the strip starts:
	// past the window's own buttons on the left when the sidebar is too
	// narrow to hold them and the brand.
	shift int
}

func newTopbar(root *ui.Root) *topbar {
	return &topbar{root: root, list: layout.List{Axis: layout.Horizontal},
		tabs: map[string]*tabState{}, hints: map[string]*ui.Tooltip{}, tour: new(driver.Tour)}
}

// topbarAction is what a frame's clicks on the strip asked for.
type topbarAction struct {
	home       bool
	show       string // project id
	close      string // project id, past any confirmation
	ask        string // project id whose close needs confirming
	pinProject string
	pin        bool
	drop       ui.SortDrop
	notify     *notification
	dismiss    string // session id
	pick       bool   // the directory picker
	reopen     *backend.RecentProject
	// settings is the menu's Settings; projectSettings a tab's "Project
	// settings…", by project id. tour is the menu's Basics guide.
	settings        bool
	tour            string // a guide's id
	projectSettings string
	pulls           bool // the Pull requests button
	// extension is the extension surface button clicked, zero for none.
	extension extensionKey
	// renamed is a layout tab's new name, committed this frame.
	renamed layoutRename
	// caption is a window button clicked, when captionClicked.
	caption        captionButton
	captionClicked bool
}

// layoutRename is a layout's new name.
type layoutRename struct{ id, name string }

// topbarFrame is what the window measured for the strip this frame.
type topbarFrame struct {
	brandWidth int     // the sidebar's column, rail to panel
	brandName  float32 // the name's opacity, the panel's own mid-morph
	stripX     int     // the terminal island's left edge
	island     color.NRGBA
	// islands are the islands under the strip, in strip pixels, when the
	// screen has more than one (Pulls or the dock); none is one island of
	// island's color under the whole strip.
	islands []islandSpan
	// islandsSliding is whether the islands are mid-slide: the active tab
	// then grows out of the one it overlaps most rather than detaching while
	// it fits wholly over none.
	islandsSliding bool
	tabs           []projectTab
	active         string
	pending        []notification
	settings       bool // the Settings screen is open
	guides         []guideItem
	pulls          bool // the Pulls screen is open
	caption        captionLayout
	chrome         windowChrome
	windowButtons  string // the window buttons setting

	// extensions is every extension, whose surfaces get a button each, and
	// extension the surface on screen, zero for none.
	extensions []backend.Extension
	extension  extensionKey
}

// islandSpan is one island under the tab strip: its left and right edges in
// strip pixels and its color.
type islandSpan struct {
	x0, x1 int
	color  color.NRGBA
}

// layout draws the strip in a topbarHeight band at the top of gtx; th is on
// the window's canvas.
func (t *topbar) layout(gtx C, th *ui.Theme, f topbarFrame) topbarAction {
	var act topbarAction
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(topbarHeight)))
	if t.brand.Clicked(gtx) {
		act.home = true
	}
	if f.chrome.drawn() {
		// The band's bare background drags the window, and a double click on
		// it maximizes, as a title bar does; what is laid over it keeps its
		// own presses.
		area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
		system.ActionInputOp(system.ActionMove).Add(gtx.Ops)
		area.Pop()
	}
	g := gtx.Dp(gutter)
	right := t.rightSide(gtx, th, f, &act)
	left := t.leftSide(gtx, th, f, &act)
	f.brandWidth = max(f.brandWidth-left, gtx.Dp(brandHeight))
	start := stripStart(f.stripX, g+left+f.brandWidth+gtx.Dp(barGap), gtx.Dp(tabRadius))
	t.shift, f.stripX = start-f.stripX, start
	t.place(gtx, image.Pt(g+left, 0), func(gtx C) D { return t.brandButton(gtx, th, f) })
	stripW := gtx.Constraints.Max.X - right - g - gtx.Dp(barGap) - f.stripX
	t.place(gtx, image.Pt(f.stripX, 0), func(gtx C) D {
		gtx.Constraints = layout.Exact(image.Pt(max(stripW, 0), gtx.Constraints.Max.Y))
		return t.strip(gtx, th, f, &act)
	})
	return act
}

// stripStart is where the tab strip starts: at the island's edge, or, when
// the window's own buttons and the brand reach past it, a fillet radius after
// them, the room the active first tab's left fillet draws in.
func stripStart(islandEdge, brandEnd, radius int) int {
	if brandEnd <= islandEdge {
		return islandEdge
	}
	return brandEnd + radius
}

// place lays w out at p.
func (t *topbar) place(gtx C, p image.Point, w layout.Widget) {
	defer op.Offset(p).Push(gtx.Ops).Pop()
	w(gtx)
}

// rightSide lays the bell, the extensions' surfaces, Pull requests and
// the menu out flush right, centered in the band, and answers how wide they
// are.
func (t *topbar) rightSide(gtx C, th *ui.Theme, f topbarFrame, act *topbarAction) int {
	macro := op.Record(gtx.Ops)
	row := gtx
	row.Constraints.Min = image.Point{}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return t.bellButton(gtx, th, f.pending, act) }),
		ui.Gap(1),
	}
	children = append(children, t.extensionButtons(th, f, act)...)
	children = append(children,
		layout.Rigid(func(gtx C) D { return t.pullsButton(gtx, th, f.pulls, act) }),
		ui.Gap(1),
		layout.Rigid(func(gtx C) D { return t.menuButton(gtx, th, f.settings, f.guides, act) }),
	)
	pad := gtx.Dp(gutter)
	if f.chrome.buttonsShown(f.windowButtons) && len(f.caption.right) > 0 {
		children = append(children,
			layout.Rigid(func(gtx C) D { return th.CaptionDivider(gtx, gutter, captionSeparator) }),
			layout.Rigid(func(gtx C) D { return t.captionButtons(gtx, th, f, f.caption.right, act) }))
		// Windows' buttons sit in the window's corner, as its own do.
		if f.caption.style == captionStyleWindows {
			pad = 0
		}
	}
	d := layout.Flex{Alignment: layout.Middle}.Layout(row, children...)
	call := macro.Stop()
	x := gtx.Constraints.Max.X - pad - d.Size.X
	defer op.Offset(image.Pt(x, (gtx.Constraints.Max.Y-d.Size.Y)/2)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return d.Size.X
}

// leftSide draws the window's buttons the desktop puts on the left, or
// leaves room for the system's own, and answers how wide that is.
func (t *topbar) leftSide(gtx C, th *ui.Theme, f topbarFrame, act *topbarAction) int {
	if !f.chrome.buttonsShown(f.windowButtons) {
		return 0
	}
	if len(f.caption.left) == 0 {
		return gtx.Dp(f.caption.leftReserve)
	}
	macro := op.Record(gtx.Ops)
	row := gtx
	row.Constraints.Min = image.Point{}
	d := layout.Flex{Alignment: layout.Middle}.Layout(row,
		layout.Rigid(func(gtx C) D { return t.captionButtons(gtx, th, f, f.caption.left, act) }),
		layout.Rigid(func(gtx C) D { return th.CaptionDivider(gtx, gutter, captionSeparator) }),
	)
	call := macro.Stop()
	defer op.Offset(image.Pt(gtx.Dp(gutter), (gtx.Constraints.Max.Y-d.Size.Y)/2)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return d.Size.X
}

func (t *topbar) captionButtons(gtx C, th *ui.Theme, f topbarFrame, buttons []captionButton, act *topbarAction) D {
	d, b, ok := t.controls.layout(gtx, th, t.root, f.caption.style, buttons, f.chrome)
	if ok {
		act.caption, act.captionClicked = b, true
	}
	return d
}

// brandButton is the mark and the name over the sidebar's column, a
// rounded-md button to Home, clipped to the column as it narrows.
func (t *topbar) brandButton(gtx C, th *ui.Theme, f topbarFrame) D {
	size := image.Pt(f.brandWidth, gtx.Dp(brandHeight))
	defer op.Offset(image.Pt(0, (gtx.Constraints.Max.Y-size.Y)/2)).Push(gtx.Ops).Pop()
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(size)
	return t.brand.Layout(gtx, func(gtx C) D {
		if t.brand.Hovered() {
			ui.Fill(gtx, th.Over(ui.Alpha(th.Accent, ghostHoverFill)), ui.RadiusMD)
		}
		layout.Inset{Left: brandPadLeft}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return icLichMark.Layout(gtx, brandIcon, th.Foreground) }),
				ui.Gap(2),
				layout.Rigid(func(gtx C) D {
					defer paint.PushOpacity(gtx.Ops, f.brandName).Pop()
					return th.Text(ui.TextSM, brandName).Weight(font.SemiBold).Layout(gtx)
				}),
			)
		})
		return D{Size: size}
	})
}

// brandColumn is the brand's width and its name's opacity: the sidebar's
// column, the rail's width with no name up to the panel's with it, eased
// along with the collapse morph at p.
func brandColumn(rail, panel int, open, morphing bool, p float32) (int, float32) {
	openness := float32(0)
	if open {
		openness = 1
	}
	if morphing {
		openness = 1 - p
		if open {
			openness = p
		}
	}
	return lerpInt(rail, panel, openness), openness
}

// menuButton is the strip's menu: Settings, which closes Settings while it
// is open, and the guides in a submenu. The button is lit while Settings is.
func (t *topbar) menuButton(gtx C, th *ui.Theme, settingsOpen bool, guides []guideItem, act *topbarAction) D {
	entries := []ui.MenuEntry{
		{Label: i18n.T("tabs.projectTabs.settings"), Icon: icSettings, OnClick: func() { act.settings = true }},
		{Kind: ui.MenuSeparator},
		{Kind: ui.MenuSubTrigger, Label: i18n.T("tabs.mainMenu.guides"), Icon: icGuides, Sub: guideEntries(guides, act)},
	}
	tip := th.Tooltip(t.root, t.hint("menu"), i18n.T("tabs.mainMenu.open"))
	tip.Side = ui.SideBottom
	button := func(gtx C) D {
		return tip.Layout(gtx, th.BarIconButton(&t.menuOpen, icMainMenu, settingsOpen || t.menu.Opened()).Layout)
	}
	return t.tour.Anchor(gtx, tourMenu, func(gtx C) D {
		menu := th.DropdownMenu(t.root, &t.menu, entries)
		menu.MinWidth = mainMenuMinWidth
		return menu.Layout(gtx, button)
	})
}

// guideEntries is the Guides submenu: a guide that cannot run now is
// disabled, its reason where a shortcut would sit.
func guideEntries(guides []guideItem, act *topbarAction) []ui.MenuEntry {
	entries := make([]ui.MenuEntry, len(guides))
	for i, g := range guides {
		entries[i] = ui.MenuEntry{Label: g.label, Disabled: g.unavailable != "", Shortcut: g.unavailable, OnClick: func() { act.tour = g.id }}
	}
	return entries
}

// pullsButton opens the Pulls screen, and closes it while it is open.
func (t *topbar) pullsButton(gtx C, th *ui.Theme, open bool, act *topbarAction) D {
	if t.pulls.Clicked(gtx) {
		act.pulls = true
	}
	tip := th.Tooltip(t.root, t.hint("pulls"), i18n.T("tabs.projectTabs.pullRequests"))
	tip.Side = ui.SideBottom
	return t.tour.Anchor(gtx, tourPullsButton, func(gtx C) D {
		return tip.Layout(gtx, func(gtx C) D { return th.BarIconButton(&t.pulls, icPullRequests, open).Layout(gtx) })
	})
}

func (t *topbar) hint(key string) *ui.Tooltip {
	h, ok := t.hints[key]
	if !ok {
		h = new(ui.Tooltip)
		t.hints[key] = h
	}
	return h
}
