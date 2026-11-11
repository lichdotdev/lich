package main

import (
	"image"
	"io"
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

// extensionScreen is one extension surface over the whole stage, like Pulls.
// The extension draws its own islands, so the screen draws none.
type extensionScreen struct {
	store *extensionStore
	root  *ui.Root
	send  func(id string, msg extension.Message)

	open  bool
	shown extensionKey
	trees map[extensionKey]*extrender.Renderer
	// code is the palette and font the window's own code views draw with.
	code extrender.Code

	restart, folder widget.Clickable
	// clip is text an extension asked to copy, from the events goroutine,
	// which the next frame writes.
	clip atomic.Pointer[string]
	// focus is a field an extension asked to focus, from the events
	// goroutine, which the next frame hands to the surface it names.
	focus atomic.Pointer[extensionFocus]
}

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
	return &extensionScreen{store: store, root: root, send: send, trees: map[extensionKey]*extrender.Renderer{}}
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
	s.focus.Store(nil)
	s.send(k.ext, extension.SurfaceOpened.New(extension.SurfaceData{Surface: k.surface, Language: string(i18n.Current())}))
}

func (s *extensionScreen) hide() {
	if !s.open {
		return
	}
	s.open = false
	s.send(s.shown.ext, extension.SurfaceClosed.New(extension.SurfaceData{Surface: s.shown.surface}))
}

// isShown is whether surface k is the one on screen.
func (s *extensionScreen) isShown(k extensionKey) bool { return s.open && s.shown == k }

func (s *extensionScreen) copy(text string) { s.clip.Store(&text) }

func (s *extensionScreen) requestFocus(ext string, d extension.FocusData) {
	s.focus.Store(&extensionFocus{at: extensionKey{ext, d.Surface}, key: d.Key})
}

// takeFocus hands a pending focus to the surface on screen; one for another
// surface is dropped.
func (s *extensionScreen) takeFocus() {
	if f := s.focus.Swap(nil); f != nil && f.at == s.shown {
		s.renderer(s.shown).Focus(f.key)
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
		delete(s.trees, s.shown)
		return extensionAction{}
	}
	s.takeFocus()
	tree, has := s.store.render(s.shown)
	switch extensionViewOf(ext.Status, has) {
	case extensionFailed:
		return s.failed(gtx, th, ext.ID, extensionName(ext), ext.Error, ext.Dir)
	case extensionStarting:
		layout.Center.Layout(gtx, func(gtx C) D { return th.Spinner(gtx, ui.Space(6), th.MutedForeground) })
	case extensionRestarting:
		s.restarting(gtx, th, extensionName(ext), tree.Root)
	case extensionLive:
		for _, h := range pollShortcuts(gtx, tree.Shortcuts, typing || !gtx.Source.Focused(nil)) {
			s.send(ext.ID, extension.FireEvent.New(extension.EventData{Handler: h}))
		}
		s.renderer(s.shown).Layout(gtx, th, tree.Root)
		if !typing && !gtx.Source.Focused(nil) {
			giveBackKeys(gtx)
		}
	}
	return extensionAction{}
}

// restarting draws the last tree dimmed, taking no input, under a strip that
// says why.
func (s *extensionScreen) restarting(gtx C, th *ui.Theme, name string, root extension.Node) {
	inert := gtx.Disabled()
	fade := paint.PushOpacity(gtx.Ops, restartingDim)
	s.renderer(s.shown).Layout(inert, th, root)
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
func (s *extensionScreen) failed(gtx C, th *ui.Theme, id, name, reason, dir string) extensionAction {
	var act extensionAction
	if s.restart.Clicked(gtx) {
		act.restart = id
	}
	if s.folder.Clicked(gtx) {
		act.folder = dir
	}
	restart := th.Button(&s.restart, i18n.T("shell.extensionScreen.restart"))
	folder := th.Button(&s.folder, i18n.T("shell.extensionScreen.openFolder"))
	folder.Variant = ui.VariantOutline
	actions := func(gtx C) D {
		return layout.Flex{}.Layout(gtx, layout.Rigid(folder.Layout), ui.Gap(2), layout.Rigid(restart.Layout))
	}
	empty := th.EmptyState(icExtensionFailed, i18n.T("shell.extensionScreen.failedTitle", i18n.Args{"name": name}), reason, actions)
	empty.MaxLines = 0
	empty.Layout(gtx)
	return act
}
