// lich-native is lich's window, a Gio app driven by the lich backend over its
// loopback contract (docs/native-window.md).
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
	"sync"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/gioterm"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/internal/framelog"
	"github.com/lichdotdev/lich/native/internal/shot"
	"github.com/lichdotdev/lich/native/internal/webview"
	"github.com/lichdotdev/lich/native/ui"
)

var (
	flagRuntime  = flag.String("runtime", "", "the backend's runtime file (runtime.json / runtime-dev.json); without it, LICH_PORT and LICH_TOKEN as `lich native` sets them")
	flagProject  = flag.String("project", "", "checkout to open when the backend has no project yet")
	flagRenderer = flag.String("renderer", gioterm.RenderRows, "terminal renderer: rows, atlas or paths")
	flagFont     = flag.String("font", fonts.SystemMono, "terminal and diff font family (fontconfig name) until Settings applies its own")
	flagSize     = flag.Float64("size", 13, "terminal font size in sp")
	flagShot     = flag.String("shot", "", "write a PNG of the frame drawn -shot-after into the run")
	flagAfter    = flag.Duration("shot-after", 3*time.Second, "when to take -shot")
	flagTab      = flag.String("tab", "terminal", "what opens at start beside the terminal: terminal (the dock on its stored tab), the dock on code, review or artifacts, git for the Git panel, or ext:<extension>/<surface> for an extension's dock or panel surface")
	flagWindow   = flag.String("window", "1400x900", "the window's size in dp, WxH, for -shot")
	flagDropHint = flag.Bool("drop-hint", false, "draw the file-drop hint over the terminal as if a drag were over it, for -shot")
)

func main() {
	flag.Parse()
	tuneGC(os.Getenv)
	app.ID = windowClass(os.Getenv)
	if err := claimTaskbarIdentity(app.ID); err != nil {
		log.Printf("taskbar: the window groups apart from lich's shortcut: %v", err)
	}
	rt, err := backendRuntime()
	if err != nil {
		log.Fatal(err)
	}
	if addr := os.Getenv(pprofEnv); addr != "" {
		ln, err := pprofListener(addr)
		if err != nil {
			log.Fatal(err)
		}
		go servePprof(ln)
	}
	webview.PrepareWindow()
	go func() {
		width, height := windowSize(*flagWindow)
		w := new(app.Window)
		w.Option(app.Title("lich"), app.Size(width, height), app.Decorated(false), app.TrafficLights(topbarHeight))
		code := 0
		if err := run(w, rt); err != nil {
			log.Print(err)
			code = 1
		}
		os.Exit(code)
	}()
	app.Main()
}

// windowClass is the Wayland app_id and X11 WM_CLASS. Gio's default, the
// binary's name, matches no .desktop entry, so the dock shows the window as an
// unknown app beside lich's own icon. build/linux/lich.desktop claims "lich",
// and the dev window keeps its own so compositor rules for the daily driver
// never capture it.
func windowClass(getenv func(string) string) string {
	if getenv("LICH_DEV") != "" {
		return "lichdev"
	}
	return "lich"
}

type window struct {
	ctx       context.Context
	client    *backend.Client
	backdrops backdrops
	// covers are where the window's floating layers draw over everything in
	// the frame being laid out, in the window's pixels (coverPages).
	covers []image.Rectangle
	// uiFonts is what the interface's shaper was last built from.
	uiFonts *interfaceFonts
	model   *model
	terms   *terminals
	review  *reviewPanel
	// comments is the review batch every surface shares.
	comments    commentStore
	editors     stageEditors
	commentEdit commentEditDialog
	dock        *dock
	git         *gitPanel
	sidebar     *sidebar
	// console is what a service's console keeps between frames.
	console serviceConsole
	footer  *footer
	topbar  *topbar
	// settings is the Settings screen, drawn over the stage while open.
	settings *settingsScreen
	gate     *spawnGate
	resume   resumeDialog
	chats    chatScreens
	th       *material.Theme
	theme    *ui.Theme
	root     ui.Root
	toaster  ui.Toaster
	// toastFloor is how high off the window's bottom the toasts stand a
	// gutter over this frame: the stage's foot, or the balloons' top;
	// toastRight is the right balloon's right edge they end at, 0 for the
	// window's.
	toastFloor, toastRight int
	// caption is where the topbar draws the window's buttons, swapped whole
	// when the desktop's layout changes; chrome what the window manager last
	// said about the window.
	caption atomic.Pointer[captionLayout]
	chrome  windowChrome
	// shown is what the backend was told is on screen.
	shown shownReport
	// shortcuts is the keyboard shortcuts overlay.
	shortcuts shortcutsOverlay
	// palette is the command palette.
	palette *commandPalette
	// worktree is the new-worktree-session dialog.
	worktree *worktreeDialog
	// pulls is the Pulls screen, drawn over the stage while open.
	pulls *pullsScreen
	// extension is an extension's surface, drawn over the stage while open,
	// and extOutbox what the window sends extensions, in order.
	extension *extensionScreen
	// extPanel is an extension's panel surface at the stage's foot, in the Git
	// panel's place; gitWasOpen is the Git panel's open on the frame before,
	// which onePanel reads to tell which of the two a click just opened.
	extPanel   *extensionPanel
	gitWasOpen bool
	extOutbox  *extensionOutbox
	// extLanguage tells extensions the interface language.
	extLanguage extensionLanguage
	// extUpdates offers the extensions' newer releases.
	extUpdates extensionUpdateGate
	// closeWorktree is the closed toast's question before removing a worktree.
	closeWorktree closeWorktreeConfirm
	// closeAllConfirm is Close all sessions' question when a turn is in flight.
	closeAllConfirm closeAllConfirm
	// homeOpen is Home's Open project button.
	homeOpen ui.Clickable
	// hideSettings is a request, from another goroutine, to take Settings down
	// at the next frame.
	hideSettings atomic.Bool
	// codeAsk is a request, from another goroutine, the frame carries out; a
	// later one replaces it.
	codeAsk  atomic.Pointer[func()]
	codeMenu codeMenu
	// emptyNew is the empty stage's New session button.
	emptyNew ui.Clickable
	// exitFocus is the card that had focus last frame, and exitRaised the
	// cards whose exit toast is up, under exitMu: syncExitToasts reads both.
	exitFocus  string
	exitMu     sync.Mutex
	exitRaised map[string]bool
	light      bool // the theme on screen is a light one
	code       godemirror.Theme
	// appearance is the theme state the frame repaints from.
	appearance *appearance
	// zoom scales everything but the terminals, which keep their own size.
	zoom float64
	// browserZoom scales the pages of the page panes on top of zoom: a small
	// screen fits more of a page without shrinking the rest of the window.
	browserZoom float64
	win         *app.Window
	closer      closePrompt
	// notifier is the desktop channel; optIn its one-time question.
	notifier    *notifier
	optIn       notifyOptIn
	gitMissing  gitMissing
	welcome     welcome
	tour        windowTour
	plugin      pluginGate
	appUpdate   appUpdateGate
	themeUpdate themeUpdateGate
	drop        terminalDrop
	// islandAt is the island's top-left in window pixels, where body put it.
	islandAt image.Point
	// termWidth is the width the session's terminal was last laid out at
	// while no slide moved the stage's edge.
	termWidth int
	// layoutStage is the panes' state while a layout is on screen.
	layoutStage layoutStage
	// notesBar is the notes bar's state every page pane shares
	// (browser_notes_dock.go).
	notesBar notesBarPrefs
	// browserPanes are the page panes' widgets, by session (browser_pane.go).
	browserPanes map[string]*browserPaneUI
	// browserView is the window's size as the backend last heard it, the size
	// a page no pane shows lays out at, sent in order through browserViews.
	browserView api.BrowserDefaultViewOptions
	// newTabLook is what syncNewTabPage last sent.
	newTabLook   api.BrowserNewTabPageOptions
	browserViews *browserQueue
	// hotkeyLanguage is the language the hotkeys' labels were last put in.
	hotkeyLanguage i18n.Locale
	// fontsLanguage is the language the interface's CJK faces were last
	// picked for.
	fontsLanguage i18n.Locale
	// cjkSeen is set once the interface has had CJK to draw (noteOutsideFaces).
	cjkSeen atomic.Bool
	// trayLanguage is the language the tray's words were last sent in, unset
	// until the tray is up.
	trayLanguage atomic.Value
}

// backendRuntime is the backend to talk to: the -runtime file when given,
// else the one a launching `lich native` put in the environment.
func backendRuntime() (backend.Runtime, error) {
	if *flagRuntime != "" {
		return backend.ReadRuntime(*flagRuntime)
	}
	return backend.RuntimeFromEnv(os.Getenv)
}

func run(w *app.Window, rt backend.Runtime) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := backend.New(rt)
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

	beat := &ui.LoopBeat{}
	frameLog, err := openFrameLog(beat)
	if err != nil {
		return err
	}
	defer frameLog.Close()
	invalidate := frameLog.Invalidator(w.Invalidate)
	u := &window{
		ctx: ctx, client: client, th: th, win: w, zoom: defaultZoom, browserZoom: defaultZoom, browserViews: newBrowserQueue(),
		theme: &ui.Theme{Palette: ui.Dark, Shaper: th.Shaper, Sans: fonts.Sans, Mono: faces[0].Font.Typeface, Surface: ui.Dark.Canvas, Beat: beat, Overlays: &ui.OverlayCount{}},
		model: newModel(client, invalidate),
		terms: &terminals{client: client, invalidate: invalidate, beat: beat, renderer: *flagRenderer, family: *flagFont,
			size: unit.Sp(*flagSize), byID: map[string]*termView{}, invalidateEcho: frameLog.Tagged(w.Invalidate, causeEcho)},
	}
	u.appearance = &appearance{ctx: ctx, client: client, invalidate: invalidate}
	caption := platformCaption()
	u.caption.Store(&caption)
	go followCaption(ctx, func(l captionLayout) {
		u.caption.Store(&l)
		invalidate()
	})
	u.toaster.Invalidate = invalidate
	u.terms.toaster = &u.toaster
	u.terms.profileGone = u.profileGone
	u.sidebar = newSidebar(&u.root)
	u.sidebar.tour = &u.tour.Tour
	u.console = newServiceConsole()
	u.palette = newCommandPalette(invalidate)
	u.worktree = newWorktreeDialog(client, &u.root, invalidate)
	u.pulls = newPullsScreen(newPullsStore(ctx, client, invalidate), &u.root)
	u.pulls.sessionsIn = func(projectID, path string) []string {
		ids, _, _ := u.model.sessionsAt(projectID, path)
		return ids
	}
	u.pulls.sessionLabel = u.model.sessionLabel
	u.pulls.threadState = u.model.threadFixes.state
	u.pulls.onAccount = u.model.accountChanged
	u.pulls.tour = &u.tour.Tour
	u.extOutbox = newExtensionOutbox()
	u.extension = newExtensionScreen(&u.model.extensions, &u.root, u.extOutbox.send)
	u.extPanel = newExtensionPanel(&u.root, u.extension)
	u.model.extensionMessage = u.extensionMessage
	u.extLanguage.send = u.extOutbox.send
	u.model.extensionStarted = func(id string) {
		u.extLanguage.started(id)
		u.extension.started(id)
	}
	u.model.extensionUpdate = func(string) { go u.offerExtensionUpdates() }
	u.model.prChanged = u.prChanged
	u.model.themeSelection = u.appearance.applySelection
	u.model.themeUpdates = u.themeUpdatesChanged
	u.model.exited = u.sessionExited
	u.model.toTerminal, u.model.toChat = u.leaveChat, u.enterChat
	u.model.serviceReveal = u.revealService
	u.model.duplicate = u.warnDuplicate
	u.model.profileOffer = u.offerProfile
	u.model.browserOpened = u.offerBrowser
	u.model.browserIdle = u.askIdleBrowser
	u.model.browserDownloaded = u.tellDownload
	u.model.showCode = u.tellShowCode
	u.terms.followCode = u.followCodeRef
	u.model.pluginIncompatible = func() { go u.checkPlugin() }
	u.model.appUpdateProgress = u.updateProgressed
	u.palette.closed = u.model.closed
	u.topbar = newTopbar(&u.root)
	u.topbar.tour = &u.tour.Tour
	u.topbar.open.closed = u.model.closed
	notes := newDesktopNotes(ctx, backendNotes{ctx: ctx, client: client}, func(projectID, id, token string) {
		u.focusSession(projectID, id)
		w.Activate(token)
	}, func(id string) { u.notifier.closed(id) })
	sink := recapping{next: notes, said: func(id string) (string, error) { return client.LastTurnSaid(ctx, id) }}
	u.notifier = newNotifier(ctx, client, sink, invalidate)
	u.model.seen = u.notifier.seen
	attention := attentionToast{show: func(t ui.Toast) { u.toaster.Show(t) }, open: func(projectID, id string) {
		u.settings.hide()
		u.openSession(projectID, id)
	}}
	u.model.notify = func(r statusReport) {
		u.notifier.report(r)
		attention.report(r)
	}
	u.model.withdrawStale = u.notifier.withdrawStale
	u.settings = newSettingsScreen(ctx, client, &u.root, invalidate)
	u.settings.notify = u.notifier
	u.settings.services.store = &u.model.services
	u.settings.services.home = u.model.homeProject
	u.settings.extensions.store = &u.model.extensions
	u.settings.extensions.catalog = u.model.catalog
	u.settings.promptUpdate = u.promptAppUpdate
	u.footer = newFooter(ctx, client, &u.root, u.model.usage, u.model.quota, invalidate)
	u.footer.tour = &u.tour.Tour
	u.gate = &spawnGate{probe: client, invalidate: invalidate, park: u.parkSession, notify: func(t ui.Toast) { u.toaster.Show(t) }}
	u.footer.toaster = &u.toaster
	comments, notify := &u.comments, func(t ui.Toast) { u.toaster.Show(t) }
	if u.review, err = newReviewPanel(client, &u.root, comments, *flagFont, unit.Sp(*flagSize), invalidate, notify); err != nil {
		return err
	}
	if u.pulls.view.files, err = newReviewPanel(client, &u.root, comments, *flagFont, unit.Sp(*flagSize), invalidate, notify); err != nil {
		return err
	}
	u.pulls.view.files.pr = &prFiles{ticks: map[string]*ui.Bool{}}
	u.editors = stageEditors{client: client, invalidate: invalidate, family: *flagFont, size: unit.Sp(*flagSize),
		dirty: func(checkout string, dirty bool) { u.model.markEditorDirty(checkout, dirty); u.model.invalidate() },
		saved: func(checkout string) { go u.model.checkoutWritten(u.ctx, checkout) }}
	code := newCodeTab(client, invalidate, comments, notify, func(checkout, rel string, line int) {
		u.openInEditor(checkout, rel, line, 0, agentNote{})
	})
	u.dock = newDock(code, u.review, u.newArtifactsTab(), u.extension)
	u.dock.tour = &u.tour.Tour
	u.review.tour = &u.tour.Tour
	u.model.diffTouched = u.review.feed.touched
	u.model.editorTouched = u.editors.touched
	u.model.focusSession = u.focusSession
	u.model.showToast = u.showToast
	u.model.raise = func() { w.Activate("") }
	u.model.leave = u.leave
	u.review.wrote = func(checkout string) { u.model.checkoutWritten(u.ctx, checkout) }
	u.settings.applyEditor = u.takeEditorPref
	u.settings.vcs.pathOf, u.settings.vcs.onAccount = u.model.projectPath, u.model.accountChanged
	u.settings.providers.claude.bind(u.model.profiles, u.openLoginTerminal)
	u.settings.providers.codex.bind(u.model.codexProfiles, u.openLoginTerminal)
	u.settings.providers.omp.bind(u.model.ompProfiles, u.openLoginTerminal)
	u.settings.providers.bindXDG(u.model.xdgProfiles, u.openLoginTerminal)
	u.model.loginProgress = u.settings.loginProgress
	u.bindWelcome()
	u.settings.providers.book = u.model.providers
	u.model.projectClosed = u.projectClosed
	u.model.pluginProfiles = u.pluginProfilesFailed
	u.settings.theme = newThemePicker(ctx, client, u.appearance, &u.toaster, &u.root)
	u.footer.dock = u.dock
	u.git = newGitPanel(u.ctx, client, &u.root, invalidate, notify, *flagFont, unit.Sp(*flagSize))
	u.git.openSession = u.focusSession
	u.git.wrote = u.review.wrote
	u.git.showReview = func() { u.dock.show(dockReview) }
	u.footer.git = u.git
	u.footer.panel = u.extPanel
	u.footer.extensions = &u.model.extensions
	u.git.open = *flagTab == "git"
	u.footer.openPullRequest = u.openPR
	u.footer.writePaste = u.writeAtPrompt
	u.footer.showDelegates = func(id string) { u.layoutStage.delegates = id }
	u.settings.footer = u.footer
	u.footer.customize = u.customizeFooter
	if err := u.connect(); err != nil {
		return err
	}
	if *flagToasts != "" {
		if err := u.raiseDemoToasts(*flagToasts); err != nil {
			return err
		}
	}
	started := time.Now()
	stalls := newStallWatch()
	go stalls.watch(ctx)
	var ops op.Ops
	for {
		stalls.waiting()
		event := w.Event()
		stalls.handling()
		switch e := event.(type) {
		case app.DestroyEvent:
			return e.Err
		case app.ConfigEvent:
			frameLog.Note(framelog.CauseConfig)
			u.chrome = chromeOf(e.Config)
			switch was := u.model.isFocused(); {
			case e.Config.Focused && !was:
				u.editors.recheckAll()
			case !e.Config.Focused && was:
				u.editors.flushAll()
			}
			u.model.setFocused(u.ctx, e.Config.Focused)
			u.pulls.setFocused(e.Config.Focused)
			u.toaster.SetHidden(!e.Config.Focused)
			if still := !e.Config.Focused; still != u.theme.Still {
				u.theme.Still = still
				invalidate()
			}
		case app.ViewEvent:
			webview.SetParent(e)
		case *app.ClosingEvent:
			u.onClosing(e)
		case app.DropEvent:
			frameLog.Note(framelog.CauseDrop)
			u.onDrag(e, invalidate)
		case app.FrameEvent:
			frame := frameLog.Begin(e.Size)
			gtx := app.NewContext(&ops, e)
			u.covers = u.covers[:0]
			u.terms.unzoomed = gtx.Metric
			gtx.Metric.PxPerDp *= float32(u.zoom)
			gtx.Metric.PxPerSp *= float32(u.zoom)
			u.syncBrowserView(gtx)
			u.syncNewTabPage()
			if err := u.layout(gtx); err != nil {
				return err
			}
			keyboardFromPages(gtx)
			u.reportShown()
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
			frame.HandOff()
			u.cutNativePages()
			e.Frame(gtx.Ops)
			u.placeNativePages(e.Source)
			frame.End()
			frames.Add(1)
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
	stream, _, err := u.client.Terminal(u.ctx, u.terms.onOutput, u.terms.onReplay)
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
	// After /events attached, like the turn states: a change from here on
	// lands on top of what this read.
	if err := u.model.services.load(u.ctx, u.client); err != nil {
		return err
	}
	if err := u.model.threadFixes.load(u.ctx, u.client); err != nil {
		return err
	}
	if err := u.model.artifacts.load(u.ctx, u.client); err != nil {
		u.logErr("read the session artifacts", err)
	}
	if err := u.model.chat.loadKinds(u.ctx, u.client); err != nil {
		u.logErr("read the chat providers", err)
	}
	u.model.loadChats(u.ctx)
	if err := u.model.extensions.load(u.ctx, u.client); err != nil {
		u.logErr("read the extensions", err)
	}
	u.openTabSurface()
	// The backend's first look for updates may have ended before this window
	// listened for its events.
	go u.offerExtensionUpdates()
	// A stored value this build cannot read starts the window with no
	// layouts and says so; the next split stores over it.
	if err := u.model.loadLayouts(u.ctx); err != nil {
		u.logErr("load layouts", err)
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("layouts.load.failed", i18n.Args{"error": backend.ErrorText(err)})})
	}
	if err := u.model.loadPages(u.ctx); err != nil {
		u.logErr("load the shown pages", err)
	}
	if err := u.model.hydrateBrowsers(u.ctx); err != nil {
		u.logErr("read the session browsers", err)
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
	if u.git.heightRem, err = loadGitHeight(u.ctx, u.client); err != nil {
		return err
	}
	if u.extPanel.heights, err = loadPanelHeights(u.ctx, u.client); err != nil {
		return err
	}
	if err = u.git.loadColumns(u.ctx, u.client); err != nil {
		return err
	}
	if u.dock.widthRem, err = loadDockWidth(u.ctx, u.client); err != nil {
		return err
	}
	if u.dock.collapsed, err = loadDockCollapsed(u.ctx, u.client); err != nil {
		return err
	}
	tab, err := loadDockTab(u.ctx, u.client)
	if err != nil {
		return err
	}
	if t, ok := map[string]dockTab{"code": dockCode, "review": dockReview, "artifacts": dockArtifacts}[*flagTab]; ok {
		tab = t
	}
	u.dock.restore(tab)
	if u.pulls.view.tree.widthRem, err = loadTreeWidth(u.ctx, u.client); err != nil {
		return err
	}
	if u.notesBar.folded, err = loadNotesFolded(u.ctx, u.client); err != nil {
		return err
	}
	if err := u.loadSettings(); err != nil {
		return err
	}
	if err := u.loadAppearance(); err != nil {
		return err
	}
	if err := u.loadLanguages(); err != nil {
		return err
	}
	if u.notifier.prefs, err = loadNotifyPrefs(u.ctx, u.client); err != nil {
		return err
	}
	u.notifier.asking = *flagNotifyOptIn
	if *flagSettings != "" {
		u.settings.openPage(parseSettingsSection(*flagSettings), settingsPageOf(*flagSettings))
		tabs, _ := u.model.projectTabs()
		u.settings.show(*flagSettingsScope, settingsProjectsOf(tabs))
		u.settings.providers.open = *flagSettingsProvider
		u.settings.providers.adding, u.settings.providers.addingSet = *flagSettingsAdd, *flagSettingsAdd
		u.settings.theme.open = *flagSettingsThemes
		u.settings.updates.patch.open = *flagPatchNotes
		u.settings.nav.query.SetText(*flagSettingsQuery)
		u.catalogShot()
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
	u.servicesShot()
	if err := u.welcomeShot(*flagWelcomeShot); err != nil {
		return err
	}
	u.paletteShot()
	u.worktreeShot()
	u.pullsShot()
	u.extensionShot()
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
	go u.browserViews.run(u.ctx, func(err error) { log.Printf("browser default view: %v", err) })
	go u.extOutbox.run(u.ctx, u.client.SendExtension, func(err error) { log.Printf("send to an extension: %v", err) })
	go u.loadProfiles()
	go u.model.pollGit(u.ctx)
	go u.warnUncleanExit()
	go u.attachTray()
	u.gitMissing.missing.Store(*flagGitMissing)
	if !*flagGitMissing {
		go u.checkGit(false)
	}
	go u.settings.startupPatchNotes()
	go u.pollAppUpdate()
	go u.promptThemeUpdates()
	return nil
}

func (u *window) layout(gtx C) error {
	u.followLanguage()
	u.extLanguage.follow(u.model.extensions.all())
	u.takeTheme()
	if u.hideSettings.CompareAndSwap(true, false) {
		u.settings.hide()
	}
	u.takeCodeAsk()
	u.extension.writeClipboard(gtx)
	paint.Fill(gtx.Ops, u.theme.Canvas)
	u.backdrops.sweep(gtx.Now)
	if u.model.connectionLost() {
		u.lostNotice(gtx)
		return nil
	}
	u.sidebar.morph.advance(gtx)
	if *flagCardMenu != "" {
		u.openShotMenu()
	}
	fired := u.pollHotkeys(gtx)
	u.readDragPointer(gtx)
	pointerArea := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, &u.layoutStage.pointer.tag)
	var act, peekAct sidebarAction
	var top topbarAction
	var contentErr error
	v := u.stageView(gtx.Now)
	u.extension.follow(activeContext(v))
	u.frameTour(gtx)
	content := func(gtx C) D {
		u.layoutCodeMenu(gtx)
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
		u.sidebar.schedule.layout(u.ctx, gtx, u.theme, u.client, u.showToast)
		u.layoutRemoveConfirm(gtx, u.theme)
		u.layoutCloseWorktreeConfirm(gtx, u.theme)
		u.layoutCloseAllConfirm(gtx, u.theme)
		u.layoutSubmitDialog(gtx, u.theme)
		u.layoutMergeDialog(gtx, u.theme)
		u.layoutDeleteCommentDialog(gtx, u.theme)
		u.editors.follow(u.model.editorShown())
		u.editors.layoutAsk(gtx, u.theme, &u.root)
		u.review.layoutDialogs(gtx, u.theme)
		u.git.layoutDialogs(gtx, u.theme)
		u.commentEdit.layout(gtx, u.theme, &u.comments)
		u.settings.theme.layoutDialogs(gtx, u.theme)
		u.settings.layoutExtensionDialogs(gtx, u.theme)
		u.settings.layoutFontReplication(gtx, u.theme)
		u.layoutWelcome(gtx, u.theme)
		u.layoutPluginGate(gtx, u.theme)
		u.settings.layoutSandboxDialog(gtx, u.theme)
		u.settings.layoutProviderDialog(gtx, u.theme)
		u.settings.layoutProfileDialogs(gtx, u.theme)
		u.settings.layoutPatchNotesDialog(gtx, u.theme)
		u.settings.layoutHanFontDialog(gtx, u.theme)
		u.layoutClosePrompt(gtx, u.theme)
		u.layoutNotifyOptIn(gtx, u.theme)
		u.layoutGitMissing(gtx, u.theme)
		u.layoutMoveConfirm(gtx, u.theme)
		var edge int
		edge, act, contentErr = u.body(gtx, v)
		peekAct = u.sidebar.peekLayout(gtx, u.theme, u.model, sidebarActive(v), image.Pt(gtx.Dp(gutter), gtx.Dp(topbarHeight)))
		u.sidebar.flight(gtx, u.theme, u.model)
		top = u.topbar.layout(gtx, u.theme, u.topbarFrame(gtx, v, edge))
		u.drawCarried(gtx)
		top.close = closed
		u.layoutToasts(gtx)
		return D{Size: gtx.Constraints.Max}
	}
	u.root.Layout(gtx, func(gtx C) D { return u.tour.Layout(gtx, content) })
	u.extension.settle()
	pointerArea.Pop()
	u.endStrandedDrag(gtx)
	u.apply(act)
	u.apply(peekAct)
	u.applyTopbar(top)
	u.applyLayoutStage()
	u.settleBrowserPanes()
	u.runHotkeys(gtx, fired)
	return contentErr
}

// stageView is what the frame shows under the strip, read once before
// layout so the strip and the stage agree.
type stageView struct {
	active    sessionView
	hasActive bool
	// home is no project on screen at all: the stage offers to open one.
	home    bool
	in      footerInput
	tabs    []projectTab
	project string // the active tab's project
	// strip is the topbar's tabs, the projects then the layouts, and
	// stripActive the one on screen.
	strip       []projectTab
	stripActive string
	// layout is the layout on screen when hasLayout, with each pane's
	// session.
	layout    layoutTab
	hasLayout bool
	// console is the service whose console is on screen in place of the
	// active session, when hasConsole.
	console    backend.Service
	hasConsole bool
	// editor is the checkout whose stage editor is on screen in place of the
	// active session, when hasEditor.
	editor    string
	hasEditor bool
	panes     map[string]sessionView
	// projectNames are the projects' names by id, set only when the layout's
	// panes come from more than one: their headers then name the project.
	projectNames map[string]string
}

func (u *window) stageView(now time.Time) stageView {
	_, shown := u.model.project()
	v := stageView{in: u.footer.input(u.model, now), home: !shown}
	v.active, v.hasActive = u.model.activeSession()
	v.tabs, v.project = u.model.projectTabs()
	v.strip, v.stripActive = u.model.stripTabs()
	v.layout, v.hasLayout = u.model.shownLayoutTab()
	if id := u.model.consoleShown(); id != "" && shown {
		v.console, v.hasConsole = u.model.services.byID(id)
		v.hasLayout = v.hasLayout && !v.hasConsole
	}
	if c := u.model.editorShown(); c != "" && shown && !v.hasConsole {
		v.editor, v.hasEditor = c, true
		v.hasLayout = false
	}
	if v.hasLayout {
		v.panes = u.model.viewsOf(v.layout.sessions(), now)
		if mixedProjects(v.panes) {
			v.projectNames, _ = u.model.projectNamesAndPaths()
		}
	}
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
		d, a := u.sidebar.layout(gtx, u.theme, u.model, sidebarActive(v))
		act = a
		return d
	}).Size.X
	var err error
	u.toastFloor, u.toastRight = gtx.Dp(gutter), 0
	u.footer.frame(gtx, v.in)
	u.actGit(gtx)
	onePanel(u.git, u.extPanel, u.gitWasOpen)
	u.gitWasOpen = u.git.open
	u.islandAt = image.Pt(edge, bar)
	u.stage(gtx, edge, func(gtx C) D {
		return layout.Inset{Right: gutter, Bottom: gutter}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					if u.settings.open {
						u.pulls.hide()
						u.extension.hide()
						u.settings.layout(gtx, u.theme, settingsProjectsOf(v.tabs))
						return D{Size: gtx.Constraints.Max}
					}
					if u.git.covers {
						u.layoutGit(gtx, v)
						return D{Size: gtx.Constraints.Max}
					}
					if u.extPanel != nil && u.extPanel.covers {
						u.layoutExtPanelCovering(gtx, v)
						u.applyExtension(u.extPanel.takeAction())
						return D{Size: gtx.Constraints.Max}
					}
					if u.extension.open {
						u.applyExtension(u.extension.layout(gtx, u.theme, !u.terminalTakesKeys(gtx)))
						return D{Size: gtx.Constraints.Max}
					}
					if u.pulls.open && !v.home {
						u.pulls.files = filesInput{activeID: v.in.sessionID, injector: u.injector, submitter: u.terms.submitter, light: u.light, code: u.code}
						u.applyPulls(u.pulls.layout(gtx, u.theme))
						return D{Size: gtx.Constraints.Max}
					}
					err = u.stageRow(gtx, v)
					return D{Size: gtx.Constraints.Max}
				}),
				layout.Rigid(func(gtx C) D { return u.gitPanelRow(gtx, v) }),
				layout.Rigid(func(gtx C) D {
					d := u.extPanelRow(gtx, v)
					u.applyExtension(u.extPanel.takeAction())
					return d
				}),
			)
		})
	})
	return edge, act, err
}

// mixedProjects reports whether panes come from more than one project.
func mixedProjects(panes map[string]sessionView) bool {
	first := ""
	for _, sv := range panes {
		if first == "" {
			first = sv.projectID
		} else if sv.projectID != first {
			return true
		}
	}
	return false
}

// sidebarActive is the session the sidebar lights: none while a service's
// console or the stage editor is on screen.
func sidebarActive(v stageView) string {
	if v.hasConsole || v.hasEditor {
		return ""
	}
	return v.active.ID
}

// stageBackdropID names the stage island's backdrop: the session alone on
// it, or the stage itself while it holds none.
func stageBackdropID(v stageView) string {
	if v.home || !v.hasActive {
		return "stage"
	}
	return v.active.ID
}

// island is the stage's rounded box: Home, the empty notice, the diff or
// the terminal, under the footer's balloons. A layout has no box: its panes
// are islands of their own on the canvas (layoutPanes).
func (u *window) island(gtx C, v stageView, termWidth, termHeight int) error {
	size := gtx.Constraints.Max
	gtx.Constraints.Min = size
	u.drop.reset()
	u.syncExitToasts(focusedCard(v))
	if v.hasLayout && !v.home && !v.hasConsole && !v.hasEditor {
		defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
		defer u.drawBalloons(gtx, v, 0)
		return u.layoutPanes(gtx, v)
	}
	r := gtx.Dp(ui.RadiusLG)
	defer clip.UniformRRect(image.Rectangle{Max: size}, r).Push(gtx.Ops).Pop()
	defer u.drawBalloons(gtx, v, gtx.Dp(balloonInset))
	fill := u.islandColor(v)
	paint.Fill(gtx.Ops, fill)
	on := u.theme.On(fill)
	defer ui.IslandBorder(gtx, size, r, on.Over(on.Border))
	var under backdropLayer
	if !v.hasConsole && !v.hasEditor {
		under = u.backdrops.layout(gtx, u.theme, stageBackdropID(v), size, fill)
	}
	if u.homeOpen.Clicked(gtx) {
		go u.pickProject()
	}
	if u.emptyNew.Clicked(gtx) {
		go u.newSession(launchRequest{kind: u.model.newSessionKind()}, "")
	}
	switch {
	case v.home:
		homeView(gtx, on, &u.homeOpen)
		return nil
	case v.hasConsole:
		return u.consoleIsland(gtx, v.console, termWidth)
	case v.hasEditor:
		return u.editorIsland(gtx, v)
	case !v.hasActive && u.model.everySessionInALayout():
		allInLayoutView(gtx, on, &u.emptyNew, u.spot)
		return nil
	case !v.hasActive:
		emptySessionsView(gtx, on, &u.emptyNew, u.model.newSessionKind() == "shell", u.spot)
		return nil
	}
	return u.loneSession(gtx, v, fill, termWidth, termHeight, under)
}

// loneSession is the active session alone on the island, laid out termWidth
// wide, which a sidebar morph sets to where the box is going and the dock's
// slide, for a terminal, to where the box came from: wider, the box clips it;
// narrower, its fill shows past it. The Git panel's slide sets termHeight the
// same way.
func (u *window) loneSession(gtx C, v stageView, fill color.NRGBA, termWidth, termHeight int, under backdropLayer) error {
	size := gtx.Constraints.Max
	u.spot(gtx, tourStage, func(C) D { return D{Size: size} })
	u.layoutStage.metric = gtx.Metric
	u.terms.onScreen(v.active.ID)
	term, band := gtx, u.sessionBand(gtx, v)
	term.Constraints = layout.Exact(image.Pt(termWidth, termHeight+band))
	// A session alone has no pane, but the panes it came from or goes to
	// move against its place, so it stands in the run as one (paneMotion).
	motion, alone := &u.layoutStage.motion, image.Rectangle{Max: size}
	motion.begin(gtx.Now, false, []string{v.active.ID})
	reveal := clip.Rect{Max: size}
	if frame := motion.frame(v.active.ID, alone); frame != alone {
		reveal = clip.Rect(frame)
	}
	area := reveal.Push(gtx.Ops)
	err := u.bandedSession(term, v.active, band, fill, u.terminalTakesKeys(gtx), under)
	area.Pop()
	u.endPaneMotion(gtx)
	if err != nil {
		return err
	}
	u.layoutStage.pointer.stage = size
	u.paneDropZones(gtx, v.active.ID, image.Rectangle{Max: size}, image.Rectangle{Max: size}.Add(u.islandAt), true)
	if u.terms.has(v.active.ID) || v.active.View == api.ViewChat {
		u.drop.add(v.active.ID, image.Rectangle{Min: u.islandAt, Max: u.islandAt.Add(size)})
		if u.drop.hinting(v.active.ID) || *flagDropHint {
			dropHintFor(u.theme.On(fill), v.active.Label, v.active.sandbox.Confined).Layout(gtx)
		}
	}
	return nil
}

// focusedCard is the session whose terminal takes the keys, "" for none.
func focusedCard(v stageView) string {
	switch {
	case v.home, v.hasConsole, v.hasEditor:
		return ""
	case v.hasLayout:
		return v.layout.Focus
	case v.hasActive:
		return v.active.ID
	}
	return ""
}

// islandColor is the terminal's own background while it is on the island,
// else the theme's.
func (u *window) islandColor(v stageView) color.NRGBA {
	if v.hasConsole && !v.home {
		if bg := u.terms.backgroundOf(v.console.Terminal); bg.A != 0 {
			return bg
		}
		return u.theme.Background
	}
	if v.home || !v.hasActive || v.hasLayout {
		return u.theme.Background
	}
	if bg := u.terms.backgroundOf(v.active.ID); bg.A != 0 {
		return bg
	}
	return u.theme.Background
}

func (u *window) topbarFrame(gtx C, v stageView, edge int) topbarFrame {
	width, name := u.sidebar.column(gtx)
	f := topbarFrame{
		brandWidth: width, brandName: name, stripX: edge,
		tabs: v.strip, active: v.stripActive, pending: u.model.notifications(),
		caption: u.caption.Load().under(u.settings.look.prefs.windowButtons), chrome: u.chrome,
		windowButtons: u.settings.look.prefs.windowButtons, guides: u.guideItems(),
		pulls: u.pulls.open && !v.home,
	}
	f.extensions, f.surfaceImages = u.model.extensions.all(), u.model.extensions.surfaceImages()
	// Settings and an extension's screen are no project's, so no tab is lit
	// over them.
	switch {
	case u.settings.open:
		f.active, f.settings = "", true
	case u.extension.open:
		f.active, f.extension = "", u.extension.shown
	}
	return f
}

// stageRow is the island and, to its right, the dock, which covers the whole
// row in full screen. Home has no dock: a project-less screen has no checkout
// to show.
func (u *window) stageRow(gtx C, v stageView) error {
	d := u.dock
	d.act(gtx)
	if d.saveCollapsed {
		d.saveCollapsed = false
		collapsed := d.collapsed
		go func() { u.logErr("dock collapsed", saveDockCollapsed(u.ctx, u.client, collapsed)) }()
	}
	if t, ok := d.tabToSave(); ok {
		go func() { u.logErr("dock tab", saveDockTab(u.ctx, u.client, t)) }()
	}
	if d.tab != dockClosed && !v.home {
		d.shown = d.tab
	}
	open := d.docked(v.home)
	if d.fullscreen {
		d.slide.snap(open)
	}
	share := d.slide.at(gtx, open)
	u.terms.refitNow = d.slide.landed || u.sidebar.morph.holdsStage() || u.git.moved() || u.extPanel.moved()
	defer func() { u.terms.refitNow = false }()
	size := gtx.Constraints.Max
	slot := 0
	if !d.fullscreen {
		slot = int(share * float32(d.width(gtx)+gtx.Dp(gutter)))
	}
	// The session takes the width the sidebar's morph ends at from its
	// first frame. While the dock's slide moves the stage's edge, the chat
	// follows the island on every frame, and a terminal keeps its grid,
	// clipped by the island or short of its edge, until the slide lands: a
	// resize reaches the agent, which redraws everything it printed, too slow
	// for the slide's frames.
	termWidth := size.X - u.sidebar.stageOverhang(gtx)
	if open && !d.fullscreen {
		termWidth -= d.width(gtx) + gtx.Dp(gutter)
	}
	termWidth = max(termWidth, 0)
	switch {
	case !d.slide.moving():
		u.termWidth = termWidth
	case v.active.View == api.ViewChat || u.termWidth == 0:
		termWidth = max(size.X-slot, 0)
	default:
		termWidth = u.termWidth
	}
	termHeight := max(size.Y-u.git.pending()-u.extPanel.pending()-u.ptyBand(gtx, v)-u.sessionBand(gtx, v), 0)
	if share == 0 {
		if err := u.island(gtx, v, termWidth, termHeight); err != nil {
			return err
		}
		return u.dockPeek(gtx, v)
	}
	at, w := 0, size.X
	if !d.fullscreen {
		w = d.width(gtx)
		at = size.X - slot + gtx.Dp(gutter)
	}
	island := gtx
	island.Constraints = layout.Exact(image.Pt(max(size.X-slot, 0), size.Y))
	if err := u.island(island, v, termWidth, termHeight); err != nil {
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
	return u.layoutDock(gtx, v)
}

// gitPanelRow is the Git panel at the stage's foot, when it is open and there
// is a repository for it to read.
//
// It slides up from the foot over the balloons' band, as the dock slides in
// from the side, and grows
// over the stage to expand. pendingPx is what the stage still has to give up
// once the slide lands: the terminal is laid out at that size from the first
// frame, so the agent redraws once rather than on every frame of the slide.
func (u *window) gitPanelRow(gtx C, v stageView) D {
	g := u.git
	shown := u.gitShown(v)
	open := g.slide.at(gtx, shown)
	grow := g.grow.at(gtx, shown && g.expanded)
	g.covers, g.pendingPx = false, 0
	if open == 0 {
		g.hidden()
		return D{}
	}
	maxY := gtx.Constraints.Max.Y
	h, released := g.height(gtx, maxY)
	if released {
		rem := g.heightRem
		go func() { u.logErr("git panel height", saveGitHeight(u.ctx, u.client, rem)) }()
	}
	if open == 1 && grow == 1 {
		g.covers = true
		return D{}
	}
	full := max(maxY-gtx.Dp(gutter), h)
	slot := int(open * float32(h+gtx.Dp(gutter)+int(grow*float32(full-h))))
	if shown {
		g.pendingPx = h + gtx.Dp(gutter) - slot
	} else {
		g.pendingPx = -slot
	}
	// Laid out at the height it is heading for and slid up from under the
	// stage's foot, so its rows move rather than squeeze.
	size := image.Pt(gtx.Constraints.Max.X, h+int(grow*float32(full-h)))
	defer clip.Rect{Max: image.Pt(size.X, slot)}.Push(gtx.Ops).Pop()
	defer op.Offset(image.Pt(0, slot-size.Y)).Push(gtx.Ops).Pop()
	panel := gtx
	panel.Constraints = layout.Exact(size)
	u.layoutGit(panel, v)
	return D{Size: image.Pt(size.X, slot)}
}

// actGit runs every click that opens, closes or expands the Git panel, before
// the frame reads any of it.
func (u *window) actGit(gtx C) {
	u.footer.actGit(gtx)
	u.git.act(gtx)
	u.extPanel.actHeader(gtx)
}

// gitShown is whether the Git panel has a repository to read and room to draw.
func (u *window) gitShown(v stageView) bool {
	return u.git.open && !v.home && !u.settings.open && v.in.inRepository()
}

func (u *window) layoutGit(gtx C, v stageView) {
	in := gitPanelInput{path: v.in.path, sessions: u.model.branchSessions(gtx.Now), code: u.code, status: u.model.gitStatus}
	in.cur, _ = u.model.gitStatus(v.in.path)
	if sv, ok := u.model.sessionIn(v.in.path, gtx.Now); ok {
		in.here = &sv
	}
	u.git.layout(gtx, u.theme, in)
}

// gutter is the canvas around the sidebar.
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
			layout.Rigid(u.theme.Text(ui.TextSM, i18n.T("shell.main.lostTitle")).Weight(font.Medium).Layout),
			ui.VGap(1.5),
			layout.Rigid(u.theme.Text(ui.TextSM, i18n.T("shell.main.lostHint")).Muted().Layout),
		)
	})
}

// terminalTakesKeys is whether the terminal on screen takes the keyboard back
// this frame: not while a field or a dialog over it is typing.
func (u *window) terminalTakesKeys(gtx C) bool {
	typing := u.sidebar.holdsKeys() || u.footer.holdsKeys() || u.dock.holdsKeys(gtx) || u.git.holdsKeys(gtx) || u.extPanel.holdsKeys(gtx) || u.topbar.holdsKeys()
	return !typing && !u.palette.open && !u.worktree.open && !u.comments.isEditing()
}

func (u *window) apply(act sidebarAction) {
	u.applyService(act.service)
	if act.activate != "" {
		u.settings.hide()
		u.pulls.hide()
		u.extension.hide()
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
	if act.forkHere != "" {
		go u.forkHere(act.forkHere)
	}
	if act.schedule != "" {
		if sv, ok := u.model.sessionViewOf(act.schedule); ok {
			u.sidebar.schedule.show(sv)
		}
	}
	if act.forkWorktree != "" {
		u.showForkDialog(act.forkWorktree)
	}
	if act.close != "" {
		u.closeSession(act.close)
	}
	if len(act.closeAll) > 0 {
		u.closeAll(act.closeAll)
	}
	u.openPR(act.openPR, act.openPRURL)
	if act.pin != "" && u.model.setPinned(act.pin, act.pinTo) {
		u.model.invalidate()
		id, pinned := act.pin, act.pinTo
		go func() { u.logErr("pin session", u.client.SetSessionPinned(u.ctx, id, pinned)) }()
	}
	u.applyOpenIn(act)
	u.applyBrowser(act)
	u.applyEditor(act)
	if act.unsplit != "" {
		u.model.unsplit(u.ctx, act.unsplit)
		u.model.invalidate()
	}
	u.applyContinueIn(act)
	if pick := act.profilePick; pick.key != "" {
		u.settings.writeProviderKey(pick.key, pick.projectID, pick.value, nil)
	}
	u.applyUseReset(act.useReset)
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
func renameSession(ctx context.Context, c *backend.Client, show func(ui.Toast) string, id, label string) {
	if err := c.RenameSession(ctx, id, label); err != nil {
		show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("shell.main.renameFailed", i18n.Args{"error": backend.ErrorText(err)})})
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
				u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("sidebar.sessionCard.openCheckoutFailed", i18n.Args{"error": backend.ErrorText(err)})})
				return
			}
			// A terminal editor runs in a shell at the directory.
			if command != "" {
				u.newSession(launchRequest{kind: "shell", dir: dir}, command+"\n")
			}
		}()
	}
	if dir := act.folderAt; dir != "" {
		go func() {
			if err := u.client.OpenFolder(u.ctx, dir); err != nil {
				u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("sidebar.sessionCard.openFolderFailed", i18n.Args{"error": backend.ErrorText(err)})})
			}
		}()
	}
}

// closeSession parks the session in the backend and stops its process, and
// offers the park's undo and, for the last session of a worktree, the
// checkout's removal.
func (u *window) closeSession(id string) {
	closed, index, _ := u.model.slotOf(id)
	project, worktree := u.model.worktreeLeftBy(id)
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
	toast := u.closedToast(projectID, closed, index, parked)
	if worktree != "" {
		toast.Secondary = &ui.ToastButton{Label: i18n.T("pulls.pulls.removeWorktree"), OnClick: func() {
			go u.removeClosedWorktree(project, worktree, parked)
		}}
	}
	u.toaster.Show(toast)
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
// typeahead is typed into it once it starts. It reports whether the session
// was added: one the backend refused is said in a toast.
func (u *window) newSession(l launchRequest, typeahead string) bool {
	kind := l.kind
	p, ok := u.model.project()
	if !ok {
		log.Print("new session: the backend has no project open")
		return false
	}
	id := strings.ToLower(rand.Text())
	label := fmt.Sprintf("%s %d", strings.ToUpper(kind[:1])+kind[1:], p.NextSeq+1)
	if err := u.client.AddSessionInFolder(u.ctx, p.ID, id, label, kind, checkoutPath(l.dir, p.Path), p.NextSeq+1, l.folder); err != nil {
		u.logErr("new session", err)
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: backend.ErrorText(err)})
		return false
	}
	if err := u.loadSessions(); err != nil {
		u.logErr("reload", err)
		return true
	}
	if typeahead != "" {
		u.terms.queue(id, typeahead)
	}
	u.model.setActive(u.ctx, id)
	u.logErr("set active session", u.client.SetActiveSession(u.ctx, p.ID, id))
	u.model.invalidate()
	return true
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
