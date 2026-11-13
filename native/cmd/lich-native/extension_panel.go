package main

import (
	"context"
	"encoding/json"
	"image"
	"maps"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// extPanelHeightsKey is the global setting holding each panel surface's
// height in rem, as JSON by "extension/surface". A panel starts at the Git
// panel's default and keeps its bounds.
const extPanelHeightsKey = "lich.extension.panels"

// extensionPanel is an extension's panel surface in the Git panel's place,
// over the footer that opens it: one panel at a time, each with its own
// height. It reads the active session as the dock does.
type extensionPanel struct {
	root       *ui.Root
	extensions *extensionScreen

	open bool
	key  extensionKey
	// heights is each surface's height in rem, the stored ones and those
	// dragged since.
	heights  map[string]float64
	resize   ui.ResizeHandle
	close    ui.IconActionState
	full     ui.IconActionState
	expanded bool
	// slide opens the panel and grow expands it, as the Git panel's do;
	// covers and pendingPx mean what they mean there.
	slide, grow dockSlide
	covers      bool
	pendingPx   int
	// act is what the surface's failed state asked for this frame.
	act extensionAction
}

func newExtensionPanel(root *ui.Root, extensions *extensionScreen) *extensionPanel {
	return &extensionPanel{root: root, extensions: extensions, heights: map[string]float64{}, resize: ui.ResizeHandle{Edge: ui.PanelEdgeTop}}
}

func panelHeightKey(k extensionKey) string { return k.ext + "/" + k.surface }

// shows is whether the panel is open on surface k.
func (p *extensionPanel) shows(k extensionKey) bool { return p != nil && p.open && p.key == k }

// toggle opens the panel on surface k, and closes it while it shows k.
func (p *extensionPanel) toggle(k extensionKey) {
	if p.shows(k) {
		p.open = false
		return
	}
	p.open, p.key = true, k
}

// moved is whether this frame began or ended a slide; a window built without
// the panel has none, as gitPanel.moved says.
func (p *extensionPanel) moved() bool {
	return p != nil && (p.slide.began || p.slide.landed || p.grow.began || p.grow.landed)
}

func (p *extensionPanel) pending() int {
	if p == nil {
		return 0
	}
	return p.pendingPx
}

func (p *extensionPanel) holdsKeys(gtx C) bool {
	return p != nil && p.open && p.extensions.holdsKeys(gtx, p.key)
}

// takeAction is what the surface's failed state asked for, once.
func (p *extensionPanel) takeAction() extensionAction {
	if p == nil {
		return extensionAction{}
	}
	act := p.act
	p.act = extensionAction{}
	return act
}

// height is what the panel takes of a column maxY tall, with the Git panel's
// bounds, reading the frame's drag; released says the drag ended.
func (p *extensionPanel) height(gtx C, maxY int) (h int, released bool) {
	ceiling := min(float32(gitMaxRem*remDp), float32(gtx.Metric.PxToDp(maxY))-gitStageFloorRem*remDp)
	floor := float32(gitMinRem * remDp)
	ceiling = max(ceiling, floor)
	rem, ok := p.heights[panelHeightKey(p.key)]
	if !ok {
		rem = gitDefaultRem
	}
	got, released := p.resize.Update(gtx, unit.Dp(rem*remDp), unit.Dp(floor), unit.Dp(ceiling))
	rem = min(float64(got)/remDp, float64(ceiling)/remDp)
	p.heights[panelHeightKey(p.key)] = rem
	return gtx.Dp(unit.Dp(rem * remDp)), released
}

// act runs the header's clicks before the window reads whether the panel is
// open and how much it covers.
func (p *extensionPanel) actHeader(gtx C) {
	if p == nil {
		return
	}
	for p.close.Click.Clicked(gtx) {
		p.open = false
	}
	for p.full.Click.Clicked(gtx) {
		p.expanded = !p.expanded
	}
}

// layout draws the panel in gtx.Constraints.Max: the Git panel's header with
// the surface's title, over the surface.
func (p *extensionPanel) layout(gtx C, th *ui.Theme, surface placedSurface) {
	size := gtx.Constraints.Max
	gtx.Constraints = layout.Exact(size)
	defer clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(ui.RadiusLG)).Push(gtx.Ops).Pop()
	ui.Fill(gtx, th.Sidebar, 0)
	on := th.On(th.Sidebar)
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return p.header(gtx, on, surface) }),
		layout.Rigid(on.Separator),
		layout.Flexed(1, func(gtx C) D {
			p.act = p.extensions.placed(gtx, on, placePanel, p.key)
			return D{Size: gtx.Constraints.Max}
		}),
	)
	if !p.expanded {
		p.resize.Layout(gtx, on)
	}
}

func (p *extensionPanel) header(gtx C, th *ui.Theme, surface placedSurface) D {
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(dockHeaderHeight)))
	return layout.Inset{Left: ui.Space(3.5), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextSM, surface.title).Layout),
			layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
			layout.Rigid(func(gtx C) D {
				ic, label := icDockMaximize, i18n.T("dock.rightDock.fullScreen")
				if p.expanded {
					ic, label = icDockMinimize, i18n.T("dock.rightDock.exitFullScreen")
				}
				return th.IconAction(p.root, &p.full, ic, label, false).Layout(gtx)
			}),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D {
				return th.IconAction(p.root, &p.close, icDockClose, i18n.T("dock.rightDock.closePanel"), false).Layout(gtx)
			}),
		)
	})
}

// parsePanelHeights reads the stored heights, dropping what is not a size
// within the panel's bounds.
func parsePanelHeights(raw string) map[string]float64 {
	var stored map[string]float64
	_ = json.Unmarshal([]byte(raw), &stored)
	out := map[string]float64{}
	for k, rem := range stored {
		if rem > 0 {
			out[k] = min(gitMaxRem, max(gitMinRem, rem))
		}
	}
	return out
}

func loadPanelHeights(ctx context.Context, c *backend.Client) (map[string]float64, error) {
	raw, err := c.GetSetting(ctx, extPanelHeightsKey, "")
	return parsePanelHeights(raw), err
}

func savePanelHeights(ctx context.Context, c *backend.Client, heights map[string]float64) error {
	data, err := json.Marshal(heights)
	if err != nil {
		return err
	}
	return c.SetSetting(ctx, extPanelHeightsKey, "", string(data))
}

// extPanelShown is whether the extension panel is open on a panel surface that
// still exists, with room to draw.
func (u *window) extPanelShown(v stageView, placed []placedSurface) (placedSurface, bool) {
	p := u.extPanel
	if !p.open || v.home || u.settings.open {
		return placedSurface{}, false
	}
	surface, ok := findPlaced(placedIn(placed, extension.PlacePanel), p.key)
	return surface, ok
}

// onePanel leaves one panel over the footer when a click opened the other
// beside it: the one just opened takes the place of the one there, without a
// slide down and up. gitWas is whether the Git panel was open on the frame
// before.
func onePanel(g *gitPanel, p *extensionPanel, gitWas bool) {
	if g == nil || p == nil || !g.open || !p.open {
		return
	}
	if gitWas {
		g.open = false
		g.slide.snap(false)
		p.slide.snap(true)
		return
	}
	p.open = false
	p.slide.snap(false)
	g.slide.snap(true)
}

// extPanelRow is the extension panel between the stage and the footer, as
// gitPanelRow lays the Git panel.
func (u *window) extPanelRow(gtx C, v stageView) D {
	p := u.extPanel
	if p == nil {
		return D{}
	}
	surface, shown := u.extPanelShown(v, v.in.placed)
	open := p.slide.at(gtx, shown)
	grow := p.grow.at(gtx, shown && p.expanded)
	p.covers, p.pendingPx = false, 0
	if open == 0 {
		return D{}
	}
	if !shown {
		// Sliding out: draw the surface it closed from.
		surface, _ = findPlaced(v.in.placed, p.key)
		if surface.glyph == nil {
			return D{}
		}
	}
	maxY := gtx.Constraints.Max.Y
	h, released := p.height(gtx, maxY)
	if released {
		heights := maps.Clone(p.heights)
		go func() { u.logErr("extension panel height", savePanelHeights(u.ctx, u.client, heights)) }()
	}
	if open == 1 && grow == 1 {
		p.covers = true
		return D{}
	}
	full := max(maxY-gtx.Dp(gitFooterHeight)-gtx.Dp(gutter), h)
	slot := int(open * float32(h+gtx.Dp(gutter)+int(grow*float32(full-h))))
	if shown {
		p.pendingPx = h + gtx.Dp(gutter) - slot
	} else {
		p.pendingPx = -slot
	}
	size := image.Pt(gtx.Constraints.Max.X, h+int(grow*float32(full-h)))
	defer clip.Rect{Max: image.Pt(size.X, slot)}.Push(gtx.Ops).Pop()
	defer op.Offset(image.Pt(0, slot-size.Y)).Push(gtx.Ops).Pop()
	panel := gtx
	panel.Constraints = layout.Exact(size)
	p.layout(panel, u.theme, surface)
	return D{Size: image.Pt(size.X, slot)}
}

// layoutExtPanelCovering draws the panel over the stage once it has finished
// expanding.
func (u *window) layoutExtPanelCovering(gtx C, v stageView) {
	if surface, ok := u.extPanelShown(v, v.in.placed); ok {
		u.extPanel.layout(gtx, u.theme, surface)
	}
}
