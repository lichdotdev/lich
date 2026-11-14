package main

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var icExtension = icons.Lucide("puzzle")

// surfaceButton is one extension surface's button on the strip.
type surfaceButton struct {
	key                 extensionKey
	title, icon, status string
	// image, when set, is drawn in place of icon.
	image *icons.Bitmap
}

// surfaceButtons is a button per full screen surface of every extension that
// is not invalid, in the list's order: an invalid one never started, so it has
// nothing to show. A dock or panel surface opens from the footer instead.
func surfaceButtons(list []backend.Extension) []surfaceButton {
	var out []surfaceButton
	for _, e := range list {
		if e.Status == api.ExtensionInvalid {
			continue
		}
		for _, s := range e.Surfaces {
			if s.Place != string(extension.PlaceScreen) {
				continue
			}
			out = append(out, surfaceButton{key: extensionKey{e.ID, s.ID}, title: surfaceTitle(e, s), icon: s.Icon, status: e.Status})
		}
	}
	return out
}

// surfaceDot is the corner dot's color for an extension in status, false for
// none: waiting while it restarts, destructive once it failed.
func surfaceDot(th *ui.Theme, status string) (color.NRGBA, bool) {
	switch status {
	case api.ExtensionRestarting:
		return th.ToneWait, true
	case api.ExtensionFailed:
		return th.Destructive, true
	}
	return color.NRGBA{}, false
}

// extensionButtons are the surfaces' buttons as the strip's children, each
// followed by its gap.
func (t *topbar) extensionButtons(th *ui.Theme, f topbarFrame, act *topbarAction) []layout.FlexChild {
	var kids []layout.FlexChild
	for _, b := range surfaceButtons(f.extensions) {
		b.image = f.surfaceImages[b.key]
		kids = append(kids, layout.Rigid(func(gtx C) D { return t.surfaceButton(gtx, th, b, f.extension == b.key, act) }), ui.Gap(1))
	}
	return kids
}

// surfaceButton opens its surface, and closes it while it is open.
func (t *topbar) surfaceButton(gtx C, th *ui.Theme, b surfaceButton, open bool, act *topbarAction) D {
	click := t.surfaceClick(b.key)
	if click.Clicked(gtx) {
		act.extension = b.key
	}
	var ic icons.Glyph = surfaceGlyph(b.icon)
	if b.image != nil {
		ic = b.image
	}
	tip := th.Tooltip(t.root, t.hint("extension:"+b.key.ext+"/"+b.key.surface), b.title)
	tip.Side = ui.SideBottom
	return tip.Layout(gtx, func(gtx C) D {
		return layout.Stack{}.Layout(gtx,
			layout.Stacked(th.BarIconButton(click, ic, open).Layout),
			layout.Expanded(func(gtx C) D {
				if c, ok := surfaceDot(th, b.status); ok {
					return th.CornerDot(gtx, c)
				}
				return D{Size: gtx.Constraints.Min}
			}),
		)
	})
}

func (t *topbar) surfaceClick(k extensionKey) *widget.Clickable {
	if t.surfaceClicks == nil {
		t.surfaceClicks = map[extensionKey]*widget.Clickable{}
	}
	c, ok := t.surfaceClicks[k]
	if !ok {
		c = new(widget.Clickable)
		t.surfaceClicks[k] = c
	}
	return c
}
