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
	"strings"
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
	flagTab      = flag.String("tab", "terminal", "what the stage shows at start: terminal or diff (Review changes pressed)")
)

func main() {
	flag.Parse()
	rt, err := backendRuntime()
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		w := new(app.Window)
		w.Option(app.Title("lich (native slice)"), app.Size(unit.Dp(1400), unit.Dp(900)))
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
	diff    *diffView
	sidebar *sidebar
	footer  *footer
	topbar  *topbar
	th      *material.Theme
	theme   *ui.Theme
	root    ui.Root
	// homeOpen is Home's Open project button.
	homeOpen widget.Clickable
	light    bool // the light palette is on screen
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
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(append(gofont.Collection(), faces...)))
	th.Fg, th.Bg = colForeground, colBackground

	u := &window{
		ctx: ctx, client: client, th: th,
		theme: &ui.Theme{Palette: ui.Dark, Shaper: th.Shaper, Mono: faces[0].Font.Typeface, Surface: ui.Dark.Canvas},
		model: newModel(client, w.Invalidate),
		terms: &terminals{client: client, invalidate: w.Invalidate, renderer: *flagRenderer, family: *flagFont,
			size: unit.Sp(*flagSize), byID: map[string]*termView{}},
	}
	u.sidebar = newSidebar(&u.root)
	u.topbar = newTopbar(&u.root)
	u.footer = newFooter(ctx, client, &u.root, u.model.usage, w.Invalidate)
	if u.diff, err = newDiffView(client, w.Invalidate, *flagFont, unit.Sp(*flagSize), th); err != nil {
		return err
	}
	u.footer.reviewing = *flagTab == "diff"
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
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
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
	if err := u.loadTheme(); err != nil {
		return err
	}
	if err := u.loadSidebar(); err != nil {
		return err
	}
	if err := u.footer.start(); err != nil {
		return err
	}
	go u.model.follow(u.ctx, events)
	go u.model.pollGit(u.ctx)
	return nil
}

func (u *window) layout(gtx C) error {
	paint.Fill(gtx.Ops, u.theme.Canvas)
	if u.model.connectionLost() {
		u.lostNotice(gtx)
		return nil
	}
	u.sidebar.morph.advance(gtx)
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
		u.footer.editorLayout(gtx, u.theme, v.in)
		var edge int
		edge, act, contentErr = u.body(gtx, v)
		u.sidebar.flight(gtx, u.theme, u.model)
		top = u.topbar.layout(gtx, u.theme, u.topbarFrame(gtx, v, edge))
		top.close = closed
		return D{Size: gtx.Constraints.Max}
	})
	u.apply(act)
	u.applyTopbar(top)
	return contentErr
}

// stageView is what the frame shows under the strip, read once before
// layout so the strip and the stage agree.
type stageView struct {
	active    sessionView
	hasActive bool
	home      bool
	reviewing bool // the checkout's diff in place of the terminal
	in        footerInput
	tabs      []projectTab
	project   string // the active tab's project
}

func (u *window) stageView(now time.Time) stageView {
	v := stageView{in: u.footer.input(u.model, now), home: u.model.onHome()}
	v.active, v.hasActive = u.model.activeSession()
	v.reviewing = u.footer.reviewing && v.in.isGit
	v.tabs, v.project = u.model.projectTabs()
	u.topbar.open.refresh(u.ctx, u.client, v.tabs, u.model.invalidate)
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
	u.stage(gtx, edge, func(gtx C) D {
		return layout.Inset{Right: gutter}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					err = u.island(gtx, v)
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
	fill := u.islandColor(v)
	paint.Fill(gtx.Ops, fill)
	if u.homeOpen.Clicked(gtx) {
		go u.pickProject()
	}
	switch {
	case v.home:
		homeView(gtx, u.theme.On(fill), &u.homeOpen)
		return nil
	case !v.hasActive:
		layout.Center.Layout(gtx, func(gtx C) D {
			return label(gtx, u.th, "Open a session from the sidebar.", 14, colMuted, font.Normal)
		})
		return nil
	}
	return u.content(gtx, v.active, v.reviewing)
}

// islandColor is the terminal's own background while it is on the island,
// else the theme's.
func (u *window) islandColor(v stageView) color.NRGBA {
	if v.home || !v.hasActive || v.reviewing || u.terms.background.A == 0 {
		return u.theme.Background
	}
	return u.terms.background
}

func (u *window) topbarFrame(gtx C, v stageView, edge int) topbarFrame {
	width, name := u.sidebar.column(gtx)
	return topbarFrame{
		brandWidth: width, brandName: name, stripX: edge, island: u.islandColor(v),
		tabs: v.tabs, active: v.project, pending: u.model.notifications(), light: u.light,
	}
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
	go func() { u.logErr("load providers", u.model.loadProviders(u.ctx)) }()
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

func (u *window) content(gtx C, active sessionView, reviewing bool) error {
	if reviewing {
		u.diff.watch(active.path)
		return u.diff.layout(gtx, u.th)
	}
	u.diff.close()
	t, err := u.terms.show(u.ctx, gtx, active)
	if err != nil {
		return err
	}
	return u.terms.layout(gtx, t, !u.sidebar.holdsKeys() && !u.footer.holdsKeys())
}

func (u *window) apply(act sidebarAction) {
	if act.activate != "" {
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
	if act.close != "" {
		u.closeSession(act.close)
	}
	if act.openURL != "" {
		url := act.openURL
		go func() { u.logErr("open pull request", u.client.OpenExternal(u.ctx, url)) }()
	}
	if act.pin != "" && u.model.setPinned(act.pin, act.pinTo) {
		u.model.invalidate()
		id, pinned := act.pin, act.pinTo
		go func() { u.logErr("pin session", u.client.SetSessionPinned(u.ctx, id, pinned)) }()
	}
	u.applyOpenIn(act)
	u.applySidebarWrites(act.sidebarWrites)
	if act.color != "" && u.model.setColor(act.color, act.colorTo) {
		u.model.invalidate()
		id, tint := act.color, act.colorTo
		go func() { u.logErr("color session", u.client.SetSessionColor(u.ctx, id, tint)) }()
	}
	if r := act.rename; r.id != "" {
		u.model.rename(r.id, r.label)
		u.model.invalidate()
		go func() { u.logErr("rename session", u.client.RenameSession(u.ctx, r.id, r.label)) }()
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
				u.logErr("open in editor", err)
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
		go func() { u.logErr("open folder", u.client.OpenFolder(u.ctx, dir)) }()
	}
}

// closeSession parks the session in the backend and stops its process, the
// two halves of the web card's ×.
func (u *window) closeSession(id string) {
	projectID, active, ok := u.model.closeSession(id)
	if !ok {
		return
	}
	u.terms.drop(id)
	u.model.invalidate()
	go func() {
		u.logErr("close session", u.client.CloseSession(u.ctx, projectID, id, active))
		u.logErr("close terminal", u.client.CloseTerminal(u.ctx, id))
	}()
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
