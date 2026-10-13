// gio-lich is a vertical slice of lich as a native Gio app: the session
// sidebar, the active session's terminal and its checkout's diff, all driven
// by a real (isolated) lich backend. It exists to judge whether a native
// lich feels native, not to replace the web window.
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/omartelo/lich/native/fonts"
	"github.com/omartelo/lich/native/gioterm"
	"github.com/omartelo/lich/native/internal/shot"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

var (
	flagRuntime  = flag.String("runtime", "", "the backend's runtime file (runtime.json / runtime-dev.json); without it, LICH_PORT and LICH_TOKEN as `lich native` sets them")
	flagProject  = flag.String("project", "", "checkout to open when the backend has no project yet")
	flagRenderer = flag.String("renderer", gioterm.RenderRows, "terminal renderer: rows, atlas or paths")
	flagFont     = flag.String("font", "JetBrainsMono Nerd Font", "terminal and diff font family (fontconfig name)")
	flagSize     = flag.Float64("size", 13, "terminal font size in sp")
	flagShot     = flag.String("shot", "", "write a PNG of the frame drawn -shot-after into the run")
	flagAfter    = flag.Duration("shot-after", 3*time.Second, "when to take -shot")
	flagTab      = flag.String("tab", "terminal", "what opens at start beside the terminal: terminal (nothing), or the dock on code or review")
	flagWindow   = flag.String("window", "1400x900", "the window's size in dp, WxH, for -shot")
	flagDropHint = flag.Bool("drop-hint", false, "draw the file-drop hint over the terminal as if a drag were over it, for -shot")
)

func main() {
	flag.Parse()
	rt, err := backendRuntime()
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		width, height := windowSize(*flagWindow)
		w := new(app.Window)
		w.Option(app.Title("lich (native slice)"), app.Size(width, height))
		code := 0
		if err := run(w, rt); err != nil {
			log.Print(err)
			code = 1
		}
		os.Exit(code)
	}()
	app.Main()
}

type window struct {
	ctx     context.Context
	client  *lichclient.Client
	model   *model
	terms   *terminals
	review  *reviewPanel
	dock    *dock
	sidebar *sidebar
	footer  *footer
	topbar  *topbar
	// settings is the Settings screen, drawn over the stage while open.
	settings *settingsScreen
	gate     *spawnGate
	resume   resumeDialog
	th       *material.Theme
	theme    *ui.Theme
	root     ui.Root
	toaster  ui.Toaster
	// shortcuts is the keyboard shortcuts overlay.
	shortcuts shortcutsOverlay
	// palette is the command palette.
	palette *commandPalette
	// worktree is the new-worktree-session dialog.
	worktree *worktreeDialog
	// pulls is the Pulls screen, drawn over the stage while open.
	pulls *pullsScreen
	// homeOpen is Home's Open project button.
	homeOpen widget.Clickable
	// hideSettings is a request, from another goroutine, to take Settings down
	// at the next frame.
	hideSettings atomic.Bool
	// emptyNew is the empty stage's New session button.
	emptyNew widget.Clickable
	light    bool // the theme on screen is a light one
	// appearance is the theme state the frame repaints from.
	appearance *appearance
	// zoom scales everything but the terminals, which keep their own size.
	zoom   float64
	win    *app.Window
	closer closePrompt
	// notifier is the desktop channel; optIn its one-time question.
	notifier *notifier
	optIn    notifyOptIn
	drop     terminalDrop
	// islandAt is the island's top-left in window pixels, where body put it.
	islandAt image.Point
}

// backendRuntime is the backend to talk to: the -runtime file when given,
// else the one a launching `lich native` put in the environment.
func backendRuntime() (lichclient.Runtime, error) {
	if *flagRuntime != "" {
		return lichclient.ReadRuntime(*flagRuntime)
	}
	return lichclient.RuntimeFromEnv(os.Getenv)
}

func run(w *app.Window, rt lichclient.Runtime) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := lichclient.New(rt)
	faces, err := gioterm.Faces(*flagFont)
	if err != nil {
		return err
	}
	sans, err := fonts.Faces()
	if err != nil {
		return err
	}
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(append(append(gofont.Collection(), sans...), faces...)))
	th.Fg, th.Bg = ui.Dark.Foreground, ui.Dark.Background

	u := &window{
		ctx: ctx, client: client, th: th, win: w, zoom: defaultZoom,
		theme: &ui.Theme{Palette: ui.Dark, Shaper: th.Shaper, Sans: fonts.Sans, Mono: faces[0].Font.Typeface, Surface: ui.Dark.Canvas},
		model: newModel(client, w.Invalidate),
		terms: &terminals{client: client, invalidate: w.Invalidate, renderer: *flagRenderer, family: *flagFont,
			size: unit.Sp(*flagSize), byID: map[string]*termView{}},
	}
	u.appearance = &appearance{ctx: ctx, client: client, invalidate: w.Invalidate}
	u.toaster.Invalidate = w.Invalidate
	u.terms.toaster = &u.toaster
	u.terms.profileGone = u.profileGone
	u.sidebar = newSidebar(&u.root)
	u.palette = newCommandPalette(w.Invalidate)
	u.worktree = newWorktreeDialog(client, &u.root, w.Invalidate)
	u.pulls = newPullsScreen(newPullsStore(ctx, client, w.Invalidate), &u.root)
	u.pulls.sessionAt = func(projectID, path string) bool {
		ids, _, _ := u.model.sessionsAt(projectID, path)
		return len(ids) > 0
	}
	u.model.prChanged = u.prChanged
	u.palette.closed = u.model.closed
	u.topbar = newTopbar(&u.root)
	u.topbar.open.closed = u.model.closed
	u.notifier = newNotifier(ctx, client, w.Invalidate)
	attention := attentionToast{show: func(t ui.Toast) { u.toaster.Show(t) }, open: func(projectID, id string) {
		u.settings.hide()
		u.openSession(projectID, id)
	}}
	u.model.notify = func(r statusReport) {
		u.notifier.report(r)
		attention.report(r)
	}
	u.settings = newSettingsScreen(ctx, client, &u.root, w.Invalidate)
	u.settings.notify = u.notifier
	u.footer = newFooter(ctx, client, &u.root, u.model.usage, u.model.quota, w.Invalidate)
	u.gate = &spawnGate{probe: client, invalidate: w.Invalidate, park: u.parkSession, notify: func(t ui.Toast) { u.toaster.Show(t) }}
	u.footer.toaster = &u.toaster
	comments, notify := new(commentStore), func(t ui.Toast) { u.toaster.Show(t) }
	if u.review, err = newReviewPanel(client, &u.root, comments, *flagFont, unit.Sp(*flagSize), w.Invalidate, notify); err != nil {
		return err
	}
	if u.pulls.view.files, err = newReviewPanel(client, &u.root, comments, *flagFont, unit.Sp(*flagSize), w.Invalidate, notify); err != nil {
		return err
	}
	u.pulls.view.files.pr = &prFiles{ticks: map[string]*widget.Bool{}}
	code, err := newCodeTab(client, w.Invalidate, *flagFont, comments, notify)
	if err != nil {
		return err
	}
	u.dock = newDock(code, u.review)
	u.model.diffTouched = u.review.feed.touched
	u.model.focusSession = u.focusSession
	u.review.wrote = func(checkout string) { u.model.checkoutWritten(u.ctx, checkout) }
	u.settings.applyEditor = u.takeEditorPref
	u.settings.vcs.pathOf, u.settings.vcs.onAccount = u.model.projectPath, u.model.accountChanged
	u.settings.providers.claude.bind(u.model.profiles, u.openLoginTerminal)
	u.settings.providers.codex.bind(u.model.codexProfiles, u.openLoginTerminal)
	u.settings.providers.omp.bind(u.model.ompProfiles, u.openLoginTerminal)
	u.model.loginProgress = u.settings.loginProgress
	u.settings.providers.book = u.model.providers
	u.model.projectClosed = u.projectClosed
	u.model.pluginProfiles = u.pluginProfilesFailed
	u.settings.theme = newThemePicker(ctx, client, u.appearance, &u.toaster, &u.root)
	u.dock.tab = map[string]dockTab{"code": dockCode, "review": dockReview}[*flagTab]
	u.footer.dock = u.dock
	u.footer.openPullRequest = u.showPulls
	u.settings.footer = u.footer
	u.footer.customize = u.customizeFooter
	if *flagToasts != "" {
		if err := u.raiseDemoToasts(*flagToasts); err != nil {
			return err
		}
	}
	if err := u.connect(); err != nil {
		return err
	}
	started := time.Now()
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.ConfigEvent:
			u.model.setFocused(u.ctx, e.Config.Focused)
			u.toaster.SetHidden(!e.Config.Focused)
		case *app.ClosingEvent:
			u.onClosing(e)
		case app.DropEvent:
			u.onDrag(e, w.Invalidate)
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			u.terms.unzoomed = gtx.Metric
			gtx.Metric.PxPerDp *= float32(u.zoom)
			gtx.Metric.PxPerSp *= float32(u.zoom)
			if err := u.layout(gtx); err != nil {
				return err
			}
			u.sidebar.morph.capture(gtx.Ops)
			if *flagShot != "" {
				gtx.Execute(op.InvalidateCmd{At: started.Add(*flagAfter)})
				if time.Since(started) > *flagAfter {
					if err := shot.Write(*flagShot, e.Size, &ops); err != nil {
						return err
					}
					*flagShot = ""
				}
			}
			e.Frame(gtx.Ops)
		}
	}
}

// connect opens the event and terminal sockets before loading state, so no
// change made in between is missed.
func (u *window) connect() error {
	events, err := u.client.Events(u.ctx)
	if err != nil {
		return err
	}
	stream, _, err := u.client.Terminal(u.ctx, func(id string, data []byte) { u.terms.onOutput(id, data) })
	if err != nil {
		return err
	}
	u.terms.stream = stream
	if err := u.model.reload(u.ctx); err != nil {
		return err
	}
	if err := u.model.hydrateTurnStates(u.ctx); err != nil {
		return err
	}
	if _, ok := u.model.project(); !ok && *flagProject != "" {
		path, err := filepath.Abs(*flagProject)
		if err != nil {
			return err
		}
		if err := u.client.AddProject(u.ctx, "native", filepath.Base(path), path); err != nil {
			return err
		}
		if err := u.model.reload(u.ctx); err != nil {
			return err
		}
		u.model.setActiveProject("native")
	}
	if err := u.appearance.load(); err != nil {
		return err
	}
	u.takeTheme()
	if err := u.loadSidebar(); err != nil {
		return err
	}
	if u.dock.widthRem, err = loadDockWidth(u.ctx, u.client); err != nil {
		return err
	}
	if err := u.loadSettings(); err != nil {
		return err
	}
	if err := u.loadAppearance(); err != nil {
		return err
	}
	if u.notifier.prefs, err = loadNotifyPrefs(u.ctx, u.client); err != nil {
		return err
	}
	u.notifier.asking = *flagNotifyOptIn
	if *flagSettings != "" {
		u.settings.openSection(parseSettingsSection(*flagSettings))
		tabs, _ := u.model.projectTabs()
		u.settings.show(*flagSettingsScope, settingsProjectsOf(tabs))
		u.settings.providers.open = *flagSettingsProvider
		u.settings.providers.adding, u.settings.providers.addingSet = *flagSettingsAdd, *flagSettingsAdd
		u.settings.updates.patch.open = *flagPatchNotes
		u.settings.nav.query.SetText(*flagSettingsQuery)
		if *flagProfileDialog != "" {
			if err := u.settings.showProfileDialog(*flagProfileDialog); err != nil {
				return err
			}
		}
	}
	if *flagProfileUsage != "" {
		if err := u.model.profiles.seedProfilePlans(*flagProfileUsage); err != nil {
			return err
		}
	}
	u.shortcuts.open = *flagShortcuts
	u.paletteShot()
	u.worktreeShot()
	u.pullsShot()
	if *flagCustomizeFooter {
		u.customizeFooter()
	}
	if *flagThemePicker != "" {
		if err := u.showThemePicker(*flagThemePicker); err != nil {
			return err
		}
	}
	if err := u.footer.start(); err != nil {
		return err
	}
	go u.model.follow(u.ctx, events)
	go u.loadProfiles()
	go u.model.pollGit(u.ctx)
	return nil
}

func (u *window) layout(gtx C) error {
	u.takeTheme()
	if u.hideSettings.CompareAndSwap(true, false) {
		u.settings.hide()
	}
	paint.Fill(gtx.Ops, u.theme.Canvas)
	if u.model.connectionLost() {
		u.lostNotice(gtx)
		return nil
	}
	u.sidebar.morph.advance(gtx)
	if *flagCardMenu != "" {
		u.openShotMenu()
	}
	fired := u.pollHotkeys(gtx)
	var act sidebarAction
	var top topbarAction
	var contentErr error
	v := u.stageView(gtx.Now)
	u.root.Layout(gtx, func(gtx C) D {
		filed := u.sidebar.layoutFolderDialog(gtx, u.theme, u.model.projectFolders())
		defer func() {
			if filed.ids != nil {
				act.file = filed
			}
		}()
		closed := u.topbar.layoutConfirm(gtx, u.theme)
		u.resume.layout(u.ctx, gtx, u.theme, u.gate, u.model)
		u.shortcuts.layout(gtx, u.theme, u.settings.hotkeys.current())
		u.layoutPalette(gtx, u.theme)
		u.layoutWorktreeDialog(gtx, u.theme)
		u.layoutRemoveConfirm(gtx, u.theme)
		u.layoutSubmitDialog(gtx, u.theme)
		u.review.layoutDialogs(gtx, u.theme)
		u.settings.theme.layoutDialogs(gtx, u.theme)
		u.settings.layoutSandboxDialog(gtx, u.theme)
		u.settings.layoutProviderDialog(gtx, u.theme)
		u.settings.layoutProfileDialogs(gtx, u.theme)
		u.settings.layoutPatchNotesDialog(gtx, u.theme)
		u.layoutClosePrompt(gtx, u.theme)
		u.layoutNotifyOptIn(gtx, u.theme)
		var edge int
		edge, act, contentErr = u.body(gtx, v)
		u.sidebar.flight(gtx, u.theme, u.model)
		top = u.topbar.layout(gtx, u.theme, u.topbarFrame(gtx, v, edge))
		top.close = closed
		toasts := u.theme.Toaster(&u.toaster, u.light)
		toasts.Expand = *flagToastsExpanded
		toasts.Layout(gtx)
		return D{Size: gtx.Constraints.Max}
	})
	u.apply(act)
	u.applyTopbar(top)
	u.runHotkeys(gtx, fired)
	return contentErr
}

// stageView is what the frame shows under the strip, read once before
// layout so the strip and the stage agree.
type stageView struct {
	active    sessionView
	hasActive bool
	home      bool
	in        footerInput
	tabs      []projectTab
	project   string // the active tab's project
}

func (u *window) stageView(now time.Time) stageView {
	v := stageView{in: u.footer.input(u.model, now), home: u.model.onHome()}
	v.active, v.hasActive = u.model.activeSession()
	v.tabs, v.project = u.model.projectTabs()
	return v
}

// body lays the sidebar and the stage out under the strip, and answers the
// stage's left edge.
func (u *window) body(gtx C, v stageView) (int, sidebarAction, error) {
	bar := gtx.Dp(topbarHeight)
	defer op.Offset(image.Pt(0, bar)).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, max(gtx.Constraints.Max.Y-bar, 0)))
	var act sidebarAction
	column := gtx
	column.Constraints.Min = image.Point{}
	edge := layout.Inset{Left: gutter, Right: gutter, Bottom: gutter}.Layout(column, func(gtx C) D {
		d, a := u.sidebar.layout(gtx, u.theme, u.model, v.active.ID)
		act = a
		return d
	}).Size.X
	var err error
	u.islandAt = image.Pt(edge, bar)
	u.stage(gtx, edge, func(gtx C) D {
		return layout.Inset{Right: gutter}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					if u.settings.open {
						u.pulls.hide()
						u.settings.layout(gtx, u.theme, settingsProjectsOf(v.tabs))
						return D{Size: gtx.Constraints.Max}
					}
					if u.pulls.open && !v.home {
						u.pulls.files = filesInput{sessionID: v.in.sessionID, checkout: v.in.path, light: u.light}
						u.applyPulls(u.pulls.layout(gtx, u.theme))
						return D{Size: gtx.Constraints.Max}
					}
					err = u.stageRow(gtx, v)
					return D{Size: gtx.Constraints.Max}
				}),
				layout.Rigid(func(gtx C) D { return u.footer.layoutBar(gtx, u.theme, v.in) }),
			)
		})
	})
	return edge, act, err
}

// island is the stage's rounded-lg box: Home, the empty notice, the diff or
// the terminal, in the color the active tab merges with. Its top-left corner
// is square while that tab sits on it.
func (u *window) island(gtx C, v stageView) error {
	size := gtx.Constraints.Max
	gtx.Constraints.Min = size
	r := gtx.Dp(ui.RadiusLG)
	nw := r
	if u.topbar.islandSquare(topbarFrame{tabs: v.tabs, active: v.project}) {
		nw = 0
	}
	defer clip.RRect{Rect: image.Rectangle{Max: size}, NW: nw, NE: r, SE: r, SW: r}.Push(gtx.Ops).Pop()
	u.drop.setRect(image.Rectangle{})
	fill := u.islandColor(v)
	paint.Fill(gtx.Ops, fill)
	if u.homeOpen.Clicked(gtx) {
		go u.pickProject()
	}
	if u.emptyNew.Clicked(gtx) {
		go u.newSession(launchRequest{kind: u.model.newSessionKind()}, "")
	}
	switch {
	case v.home:
		homeView(gtx, u.theme.On(fill), &u.homeOpen)
		return nil
	case !v.hasActive:
		emptySessionsView(gtx, u.theme.On(fill), &u.emptyNew, u.model.newSessionKind() == "shell")
		return nil
	}
	if err := u.content(gtx, v.active); err != nil {
		return err
	}
	if u.terms.has(v.active.ID) {
		u.drop.setRect(image.Rectangle{Min: u.islandAt, Max: u.islandAt.Add(size)})
		if u.drop.hinting() || *flagDropHint {
			dropHint(gtx, u.theme.On(fill), v.active.Label, v.active.sandbox.Confined)
		}
	}
	return nil
}

// islandColor is the terminal's own background while it is on the island,
// else the theme's.
func (u *window) islandColor(v stageView) color.NRGBA {
	if v.home || !v.hasActive || u.terms.background.A == 0 {
		return u.theme.Background
	}
	return u.terms.background
}

func (u *window) topbarFrame(gtx C, v stageView, edge int) topbarFrame {
	width, name := u.sidebar.column(gtx)
	f := topbarFrame{
		brandWidth: width, brandName: name, stripX: edge, island: u.islandColor(v),
		tabs: v.tabs, active: v.project, pending: u.model.notifications(),
	}
	// Settings is no project's screen, so no tab merges into it.
	if u.settings.open {
		f.active, f.settings = "", true
	}
	f.pulls = u.pulls.open && !v.home
	return f
}

// stageRow is App.tsx's row above the footer: the island and, gap-1.5 to its
// right, the dock, which covers the whole row in full screen. Home has no
// dock, as the web's project-less screens have no checkout to show.
func (u *window) stageRow(gtx C, v stageView) error {
	d := u.dock
	open := d.tab != dockClosed && !v.home
	if open {
		d.shown = d.tab
	}
	if d.fullscreen {
		d.slide.snap(open)
	}
	share := d.slide.at(gtx, open)
	if share == 0 {
		return u.island(gtx, v)
	}
	size := gtx.Constraints.Max
	at, w, slot := 0, size.X, 0
	if !d.fullscreen {
		w = d.width(gtx)
		slot = int(share * float32(w+gtx.Dp(gutter)))
		at = size.X - slot + gtx.Dp(gutter)
	}
	island := gtx
	island.Constraints = layout.Exact(image.Pt(max(size.X-slot, 0), size.Y))
	if err := u.island(island, v); err != nil {
		return err
	}
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	if !open {
		// Sliding out: draw the tab it closed from, then hand the state back.
		defer func(tab dockTab) { d.tab = tab }(d.tab)
		d.tab = d.shown
	}
	defer op.Offset(image.Pt(at, 0)).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(image.Pt(w, size.Y))
	in := dockInput{sessionID: v.in.sessionID, checkout: v.in.path, git: v.in.git, isGit: v.in.isGit, light: u.light}
	save, err := d.layout(gtx, u.theme, &u.root, in)
	if save {
		rem := u.dock.widthRem
		go func() { u.logErr("dock width", saveDockWidth(u.ctx, u.client, rem)) }()
	}
	return err
}

// gutter is the canvas around the sidebar: App.tsx's pl-1.5 and gap-1.5.
const gutter = unit.Dp(6)

// stage lays the main column out from x, the sidebar's edge, at the width the
// sidebar leaves it (sidebar.stageReserve), clipped to the window.
func (u *window) stage(gtx C, x int, w layout.Widget) {
	width := gtx.Constraints.Max.X - u.sidebar.stageReserve(gtx) - 2*gtx.Dp(gutter)
	defer clip.Rect{Min: image.Pt(x, 0), Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	defer op.Offset(image.Pt(x, 0)).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(image.Pt(max(width, 0), gtx.Constraints.Max.Y))
	w(gtx)
}

// loadSidebar reads the sidebar's stored width, open state and folds before
// the first frame, so nothing is drawn one way and then jumps; the launch
// menus' providers come in after, being slower to detect.
func (u *window) loadSidebar() error {
	l, err := loadSidebarLayout(u.ctx, u.client)
	if err != nil {
		return err
	}
	u.sidebar.widthRem, u.sidebar.open = l.widthRem, l.open
	if err := u.model.loadFolds(u.ctx); err != nil {
		return err
	}
	go func() { u.logErr("load providers", u.model.providers.reload(u.ctx)) }()
	return nil
}

// lostNotice replaces the whole window once the backend dropped it: what it
// would draw is stale, and a terminal there would type into nothing.
func (u *window) lostNotice(gtx C) D {
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(u.theme.Text(ui.TextSM, "Another lich window took over, or lich quit.").Weight(font.Medium).Layout),
			ui.VGap(1.5),
			layout.Rigid(u.theme.Text(ui.TextSM, "Run `lich native` to reopen here.").Muted().Layout),
		)
	})
}

func (u *window) content(gtx C, active sessionView) error {
	resume, ok := "", u.terms.has(active.ID)
	if !ok {
		resume, ok = u.gate.release(u.ctx, active)
	}
	if !ok {
		return nil
	}
	t, err := u.terms.show(u.ctx, gtx, active, resume)
	if err != nil {
		return err
	}
	return u.terms.layout(gtx, t, u.terminalTakesKeys(gtx))
}

// terminalTakesKeys is whether the terminal on screen takes the keyboard back
// this frame: not while a field or a dialog over it is typing.
func (u *window) terminalTakesKeys(gtx C) bool {
	typing := u.sidebar.holdsKeys() || u.footer.holdsKeys() || u.dock.holdsKeys(gtx)
	return !typing && !u.palette.open && !u.worktree.open
}

func (u *window) apply(act sidebarAction) {
	if act.activate != "" {
		u.settings.hide()
		u.pulls.hide()
		u.model.setActive(u.ctx, act.activate)
		if p, ok := u.model.project(); ok {
			id := act.activate
			go func() { u.logErr("set active session", u.client.SetActiveSession(u.ctx, p.ID, id)) }()
		}
		u.model.invalidate()
	}
	if l := act.launch; l.kind != "" {
		go u.newSession(l, "")
	}
	if act.worktree {
		u.showWorktreeDialog()
	}
	if act.close != "" {
		u.closeSession(act.close)
	}
	if act.openPR != 0 {
		u.showPulls(act.openPR)
	}
	if act.pin != "" && u.model.setPinned(act.pin, act.pinTo) {
		u.model.invalidate()
		id, pinned := act.pin, act.pinTo
		go func() { u.logErr("pin session", u.client.SetSessionPinned(u.ctx, id, pinned)) }()
	}
	u.applyOpenIn(act)
	u.applyContinueIn(act)
	u.applySidebarWrites(act.sidebarWrites)
	if act.color != "" && u.model.setColor(act.color, act.colorTo) {
		u.model.invalidate()
		id, tint := act.color, act.colorTo
		go func() { u.logErr("color session", u.client.SetSessionColor(u.ctx, id, tint)) }()
	}
	if r := act.rename; r.id != "" {
		go renameSession(u.ctx, u.client, u.toaster.Show, r.id, r.label)
	}
}

// renameSession asks the backend for a new label. The card takes it from the
// session-title event the backend emits once the row is written, so a rename
// that fails changes nothing on screen and says why.
func renameSession(ctx context.Context, c *lichclient.Client, show func(ui.Toast) string, id, label string) {
	if err := c.RenameSession(ctx, id, label); err != nil {
		show(failure(toastRenameFailed, err))
	}
}

// applyOpenIn carries out the card menu's Open in: a shell session at the
// card's directory, the directory in the editor, or in the file manager.
func (u *window) applyOpenIn(act sidebarAction) {
	if act.terminalAt != "" {
		go u.newSession(launchRequest{kind: "shell", dir: act.terminalAt}, "")
	}
	if dir := act.editorAt; dir != "" {
		go func() {
			command, err := u.client.OpenFolderInEditor(u.ctx, dir)
			if err != nil {
				u.toaster.Show(failure(toastOpenCheckoutFailed, err))
				return
			}
			// A terminal editor runs in a shell at the directory, as the
			// web card does.
			if command != "" {
				u.newSession(launchRequest{kind: "shell", dir: dir}, command+"\n")
			}
		}()
	}
	if dir := act.folderAt; dir != "" {
		go func() {
			if err := u.client.OpenFolder(u.ctx, dir); err != nil {
				u.toaster.Show(failure(toastOpenFolderFailed, err))
			}
		}()
	}
}

// closeSession parks the session in the backend and stops its process, the
// two halves of the web card's ×, and offers the park's undo.
func (u *window) closeSession(id string) {
	closed, index, _ := u.model.slotOf(id)
	projectID, active, ok := u.model.closeSession(id)
	if !ok {
		return
	}
	u.terms.drop(id)
	u.gate.forget(id)
	u.model.invalidate()
	parked := make(chan error, 1)
	go func() {
		err := u.client.CloseSession(u.ctx, projectID, id, active)
		parked <- err
		u.logErr("close session", err)
		u.logErr("close terminal", u.client.CloseTerminal(u.ctx, id))
	}()
	u.toaster.Show(u.closedToast(projectID, closed, index, parked))
}

// parkSession is keepSession: the row leaves the open list and stays for a
// later reopen. There is no PTY to close; the gate held its spawn.
func (u *window) parkSession(id string) {
	projectID, active, ok := u.model.closeSession(id)
	if !ok {
		return
	}
	u.logErr("park session", u.client.CloseSession(u.ctx, projectID, id, active))
}

func (u *window) logErr(what string, err error) {
	if err != nil {
		log.Printf("%s: %v", what, err)
	}
}

// applySidebarWrites runs the frame's block-level writes off the frame.
func (u *window) applySidebarWrites(w sidebarWrites) {
	if w.fold == "" && w.file.ids == nil && w.paint.ids == nil && w.folder.from == "" &&
		w.blockOrder == nil && w.cardOrder.key == "" && !w.saveWidth && !w.saveOpen {
		return
	}
	l := sidebarLayout{widthRem: u.sidebar.widthRem, open: u.sidebar.open}
	go func() { u.logErr("sidebar", writeSidebar(u.ctx, u.model, w, l)) }()
}

// newSession adds a session of the launch's kind at its dir ("" for the
// project's own checkout), filed under its folder, and puts it on screen;
// typeahead is typed into it once it starts.
func (u *window) newSession(l launchRequest, typeahead string) {
	kind := l.kind
	p, ok := u.model.project()
	if !ok {
		log.Print("new session: the backend has no project open")
		return
	}
	id := strings.ToLower(rand.Text())
	label := fmt.Sprintf("%s %d", strings.ToUpper(kind[:1])+kind[1:], p.NextSeq+1)
	if err := u.client.AddSessionInFolder(u.ctx, p.ID, id, label, kind, checkoutPath(l.dir, p.Path), p.NextSeq+1, l.folder); err != nil {
		u.logErr("new session", err)
		return
	}
	if err := u.model.reload(u.ctx); err != nil {
		u.logErr("reload", err)
		return
	}
	u.logErr("load folds", u.model.loadFolds(u.ctx))
	if typeahead != "" {
		u.terms.queue(id, typeahead)
	}
	u.model.setActive(u.ctx, id)
	u.logErr("set active session", u.client.SetActiveSession(u.ctx, p.ID, id))
	u.model.invalidate()
}

// windowSize reads -window; one that is not WxH is the default size.
func windowSize(spec string) (width, height unit.Dp) {
	w, h, ok := strings.Cut(spec, "x")
	wi, errW := strconv.Atoi(w)
	hi, errH := strconv.Atoi(h)
	if !ok || errW != nil || errH != nil || wi <= 0 || hi <= 0 {
		return 1400, 900
	}
	return unit.Dp(wi), unit.Dp(hi)
}
