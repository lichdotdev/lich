package main

import (
	"image"
	"io"
	"slices"
	"strings"
	"sync/atomic"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/extrender"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var icExtensionFailed = icons.Lucide("circle-alert")

// restartingDim is how far a restarting extension's last tree fades.
const restartingDim = 0.5

// extensionView is what the screen draws for the surface it shows.
type extensionView int

const (
	extensionStarting extensionView = iota
	extensionLive
	extensionRestarting
	extensionFailed
)

// extensionViewOf is the screen's state for an extension in status, with or
// without a render of the surface yet.
func extensionViewOf(status string, hasTree bool) extensionView {
	switch {
	case status == api.ExtensionFailed || status == api.ExtensionInvalid:
		return extensionFailed
	case !hasTree:
		return extensionStarting
	case status == api.ExtensionRestarting:
		return extensionRestarting
	}
	return extensionLive
}

// extensionPlace is a place of the window other than the full screen where a
// surface is drawn, in the dock or the panel over the footer.
type extensionPlace int

const (
	placeDock extensionPlace = iota
	placePanel
	placeCount
)

// extensionScreen draws extension surfaces: one over the whole stage, like
// Pulls, and one in each extensionPlace. The extension draws its own islands,
// so the full screen draws none.
type extensionScreen struct {
	store *extensionStore
	root  *ui.Root
	send  func(id string, msg extension.Message)

	open  bool
	shown extensionKey
	// at is the surface each place drew last frame, and drawn the one it
	// draws this frame; settle tells the extensions what changed.
	at, drawn [placeCount]extensionKey
	// visible is every surface on screen, for the events goroutine.
	visible atomic.Pointer[[]extensionKey]
	// context is what the extensions with a placed surface last heard was
	// active, nil before the first.
	context atomic.Pointer[extension.ContextData]
	trees   map[extensionKey]*extrender.Renderer
	// code is the palette and font the window's own code views draw with.
	code extrender.Code

	failedButtons map[extensionKey]*failedButtons
	// clip is text an extension asked to copy, from the events goroutine,
	// which the next frame writes.
	clip atomic.Pointer[string]
	// focus is a field an extension asked to focus, from the events
	// goroutine, which the next frame hands to the surface it names.
	focus atomic.Pointer[extensionFocus]
}

// failedButtons are a surface's failed state's two ways on.
type failedButtons struct{ restart, folder widget.Clickable }

type extensionFocus struct {
	at  extensionKey
	key string
}

// extensionAction is what the failed state's buttons asked for, by
// extension id.
type extensionAction struct {
	restart, folder string
}

func newExtensionScreen(store *extensionStore, root *ui.Root, send func(string, extension.Message)) *extensionScreen {
	return &extensionScreen{store: store, root: root, send: send, trees: map[extensionKey]*extrender.Renderer{}, failedButtons: map[extensionKey]*failedButtons{}}
}

// setCodeTheme and setCodeFont follow the window's code palette and editor
// font, so an extension's code and diff views read like the window's.
func (s *extensionScreen) setCodeTheme(t godemirror.Theme) {
	s.code.Theme = t
	s.applyCode()
}

func (s *extensionScreen) setCodeFont(family string, size unit.Sp) {
	s.code.Family, s.code.Size = family, size
	s.applyCode()
}

func (s *extensionScreen) applyCode() {
	for _, r := range s.trees {
		r.SetCode(s.code)
	}
}

// show puts surface k on screen, telling the extension it left the one it
// replaces and opened this one.
func (s *extensionScreen) show(k extensionKey) {
	if s.open && s.shown == k {
		return
	}
	s.hide()
	s.open, s.shown = true, k
	s.publishVisible()
	s.focus.Store(nil)
	s.sendOpened(k)
}

func (s *extensionScreen) sendOpened(k extensionKey) {
	s.send(k.ext, extension.SurfaceOpened.New(extension.SurfaceData{Surface: k.surface, Language: string(i18n.Current())}))
}

// started tells extension id, which has just started, what is active and
// which of its surfaces are on screen: the new process never saw what its
// predecessor was told. It runs on the events goroutine.
func (s *extensionScreen) started(id string) {
	if c := s.context.Load(); c != nil && s.store.hasPlaced(id) {
		s.send(id, extension.Context.New(*c))
	}
	if keys := s.visible.Load(); keys != nil {
		for _, k := range *keys {
			if k.ext == id {
				s.sendOpened(k)
			}
		}
	}
}

func (s *extensionScreen) hide() {
	if !s.open {
		return
	}
	s.open = false
	s.publishVisible()
	s.send(s.shown.ext, extension.SurfaceClosed.New(extension.SurfaceData{Surface: s.shown.surface}))
}

// isShown is whether surface k is the one on screen.
func (s *extensionScreen) isShown(k extensionKey) bool { return s.open && s.shown == k }

// settle ends a frame: a place that now draws another surface, or none, tells
// the one it left that it closed and the one it took that it opened.
func (s *extensionScreen) settle() {
	changed := false
	for p := range placeCount {
		was, now := s.at[p], s.drawn[p]
		s.at[p], s.drawn[p] = now, extensionKey{}
		if was == now {
			continue
		}
		changed = true
		if was != (extensionKey{}) {
			s.send(was.ext, extension.SurfaceClosed.New(extension.SurfaceData{Surface: was.surface}))
		}
		if now != (extensionKey{}) {
			s.sendOpened(now)
		}
	}
	if changed {
		s.publishVisible()
	}
}

// publishVisible stores the surfaces on screen for started.
func (s *extensionScreen) publishVisible() {
	var keys []extensionKey
	if s.open {
		keys = append(keys, s.shown)
	}
	for _, k := range s.at {
		if k != (extensionKey{}) {
			keys = append(keys, k)
		}
	}
	s.visible.Store(&keys)
}

// onScreen is whether surface k is drawn anywhere.
func (s *extensionScreen) onScreen(k extensionKey) bool {
	return s.isShown(k) || slices.Contains(s.at[:], k) || slices.Contains(s.drawn[:], k)
}

// follow tells every running extension with a placed surface what is active
// now, when that changed since they last heard.
func (s *extensionScreen) follow(c extension.ContextData) {
	if last := s.context.Load(); last != nil && *last == c || last == nil && c == (extension.ContextData{}) {
		return
	}
	s.context.Store(&c)
	s.store.sawPath(c.Path)
	for _, e := range s.store.all() {
		if e.Status == api.ExtensionRunning && hasPlaced(e) {
			s.send(e.ID, extension.Context.New(c))
		}
	}
}

// activePath is the checkout the placed surfaces follow.
func (s *extensionScreen) activePath() string {
	if c := s.context.Load(); c != nil {
		return c.Path
	}
	return ""
}

// holdsKeys is whether a field of surface k holds the keyboard.
func (s *extensionScreen) holdsKeys(gtx C, k extensionKey) bool {
	r, ok := s.trees[k]
	return ok && r.Typing(gtx)
}

func (s *extensionScreen) copy(text string) { s.clip.Store(&text) }

func (s *extensionScreen) requestFocus(ext string, d extension.FocusData) {
	s.focus.Store(&extensionFocus{at: extensionKey{ext, d.Surface}, key: d.Key})
}

// takeFocus hands a pending focus to surface k, drawn this frame; one for a
// surface on screen nowhere is dropped.
func (s *extensionScreen) takeFocus(k extensionKey) {
	f := s.focus.Load()
	switch {
	case f == nil:
	case f.at == k:
		if s.focus.CompareAndSwap(f, nil) {
			s.renderer(k).Focus(f.key)
		}
	case !s.onScreen(f.at):
		s.focus.CompareAndSwap(f, nil)
	}
}

// giveBackKeys drops the keyboard on an Escape the surface's own widgets left
// unread, as the window's fields do, so its plain shortcuts fire again.
func giveBackKeys(gtx C) {
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			return
		}
		if e, isKey := ev.(key.Event); isKey && e.State == key.Press {
			gtx.Execute(key.FocusCmd{})
		}
	}
}

// writeClipboard hands the frame's pending copy to the system.
func (s *extensionScreen) writeClipboard(gtx C) {
	if text := s.clip.Swap(nil); text != nil {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(*text))})
	}
}

// renderer is surface k's renderer, made on its first frame; its events go to
// the extension as event lines.
func (s *extensionScreen) renderer(k extensionKey) *extrender.Renderer {
	r, ok := s.trees[k]
	if !ok {
		r = extrender.New(s.root, func(e extension.EventData) { s.send(k.ext, extension.FireEvent.New(e)) })
		r.SetCode(s.code)
		s.trees[k] = r
	}
	return r
}

// layout draws the surface across gtx; typing is whether a field or a dialog
// of the window holds the keyboard, which leaves only ctrl chords live.
func (s *extensionScreen) layout(gtx C, th *ui.Theme, typing bool) extensionAction {
	gtx.Constraints.Min = gtx.Constraints.Max
	ext, ok := s.store.byID(s.shown.ext)
	if !ok {
		// Removed: its process is gone, so there is no one to tell the surface closed.
		s.open = false
		s.publishVisible()
		delete(s.trees, s.shown)
		return extensionAction{}
	}
	s.takeFocus(s.shown)
	tree, has := s.store.render(s.shown)
	view := extensionViewOf(ext.Status, has)
	if view == extensionLive {
		for _, h := range pollShortcuts(gtx, tree.Shortcuts, typing || !gtx.Source.Focused(nil)) {
			s.send(ext.ID, extension.FireEvent.New(extension.EventData{Handler: h}))
		}
	}
	act := s.draw(gtx, th, ext, s.shown, view, tree.Root)
	if view == extensionLive && !typing && !gtx.Source.Focused(nil) {
		giveBackKeys(gtx)
	}
	return act
}

// placed draws surface k in place p for the active checkout. A tree drawn for
// another checkout waits as a placeholder for the one drawn for this. Only a
// chord with ctrl is live, and only while a field of the surface types: the
// terminal beside it keeps every other key.
func (s *extensionScreen) placed(gtx C, th *ui.Theme, p extensionPlace, k extensionKey) extensionAction {
	gtx.Constraints.Min = gtx.Constraints.Max
	s.drawn[p] = k
	ext, ok := s.store.byID(k.ext)
	if !ok {
		return extensionAction{}
	}
	s.takeFocus(k)
	tree, has := s.store.render(k)
	if has && tree.Path != "" && tree.Path != s.activePath() {
		if ext.Status == api.ExtensionRunning {
			placedSkeleton(gtx, th)
			return extensionAction{}
		}
		has = false
	}
	view := extensionViewOf(ext.Status, has)
	typing := view == extensionLive && s.holdsKeys(gtx, k)
	if typing {
		for _, h := range pollShortcuts(gtx, tree.Shortcuts, true) {
			s.send(ext.ID, extension.FireEvent.New(extension.EventData{Handler: h}))
		}
	}
	act := s.draw(gtx, th, ext, k, view, tree.Root)
	if typing {
		giveBackKeys(gtx)
	}
	return act
}

// draw lays surface k out in the state view says.
func (s *extensionScreen) draw(gtx C, th *ui.Theme, ext backend.Extension, k extensionKey, view extensionView, root extension.Node) extensionAction {
	switch view {
	case extensionFailed:
		return s.failed(gtx, th, k, extensionName(ext), ext.Error, ext.Dir)
	case extensionStarting:
		layout.Center.Layout(gtx, func(gtx C) D { return th.Spinner(gtx, ui.Space(6), th.MutedForeground) })
	case extensionRestarting:
		s.restarting(gtx, th, k, extensionName(ext), root)
	case extensionLive:
		s.renderer(k).Layout(gtx, th, root)
	}
	return extensionAction{}
}

// placedSkeletonRows are the widths of the placeholder's lines, a heading and
// a few lines of text.
var placedSkeletonRows = []unit.Dp{144, 224, 176, 200, 160}

// placedSkeleton stands in for a tree drawn for another checkout until the
// extension draws one for the active checkout.
func placedSkeleton(gtx C, th *ui.Theme) D {
	rows := make([]layout.FlexChild, 0, 2*len(placedSkeletonRows))
	for i, w := range placedSkeletonRows {
		if i > 0 {
			rows = append(rows, ui.VGap(2.5))
		}
		rows = append(rows, layout.Rigid(th.Skeleton(w, ui.Space(3)).Layout))
	}
	return layout.UniformInset(ui.Space(4)).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
}

// restarting draws the last tree dimmed, taking no input, under a strip that
// says why.
func (s *extensionScreen) restarting(gtx C, th *ui.Theme, k extensionKey, name string, root extension.Node) {
	inert := gtx.Disabled()
	fade := paint.PushOpacity(gtx.Ops, restartingDim)
	s.renderer(k).Layout(inert, th, root)
	fade.Pop()
	restartingStrip(gtx, th, i18n.T("shell.extensionScreen.restarting", i18n.Args{"name": name}))
}

// restartingStrip is a popover pill at the top centre of gtx.
func restartingStrip(gtx C, th *ui.Theme, text string) {
	on := th.On(th.Popover)
	macro := op.Record(gtx.Ops)
	pill := gtx
	pill.Constraints.Min = image.Point{}
	d := layout.UniformInset(ui.Space(2)).Layout(pill, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return on.Spinner(gtx, ui.Space(4), on.MutedForeground) }),
			ui.Gap(2),
			layout.Rigid(on.Text(ui.TextSM, text).Weight(font.Medium).Layout),
		)
	})
	call := macro.Stop()
	at := image.Pt((gtx.Constraints.Max.X-d.Size.X)/2, gtx.Dp(ui.Space(4)))
	defer op.Offset(at).Push(gtx.Ops).Pop()
	box := gtx
	box.Constraints = layout.Exact(d.Size)
	ui.Fill(box, th.Popover, ui.RadiusMD)
	ui.InsetBorder(box, d.Size, gtx.Dp(ui.RadiusMD), th.Border)
	call.Add(gtx.Ops)
}

// failed is the extension having stopped for good: why, and the two ways on.
func (s *extensionScreen) failed(gtx C, th *ui.Theme, k extensionKey, name, reason, dir string) extensionAction {
	b, ok := s.failedButtons[k]
	if !ok {
		b = new(failedButtons)
		s.failedButtons[k] = b
	}
	var act extensionAction
	if b.restart.Clicked(gtx) {
		act.restart = k.ext
	}
	if b.folder.Clicked(gtx) {
		act.folder = dir
	}
	restart := th.Button(&b.restart, i18n.T("shell.extensionScreen.restart"))
	folder := th.Button(&b.folder, i18n.T("shell.extensionScreen.openFolder"))
	folder.Variant = ui.VariantOutline
	actions := func(gtx C) D {
		return layout.Flex{}.Layout(gtx, layout.Rigid(folder.Layout), ui.Gap(2), layout.Rigid(restart.Layout))
	}
	empty := th.EmptyState(icExtensionFailed, i18n.T("shell.extensionScreen.failedTitle", i18n.Args{"name": name}), reason, actions)
	empty.MaxLines = 0
	empty.Layout(gtx)
	return act
}
