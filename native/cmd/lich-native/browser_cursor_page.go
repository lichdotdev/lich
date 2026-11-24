package main

import (
	"image"
	"image/color"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// A page the window draws itself is a window of the system's over the pane,
// so the agent's cursor the pane draws is under it. The page draws the same
// cursor in its overlay layer, at the same place and moment: the pane's copy
// still shows over the toolbar and the tabs, and one cursor crosses the
// page's edge.

// pageCursor is the agent's cursor in this frame, for the page to draw: in
// its CSS pixels, from its top-left corner, Unit being one dp.
type pageCursor struct {
	X     float32 `json:"x"`
	Y     float32 `json:"y"`
	Unit  float32 `json:"unit"`
	Alpha float32 `json:"alpha"`
	// Scale is the arrow's, the dip of a click.
	Scale   float32 `json:"scale"`
	Tint    string  `json:"tint"`
	Outline string  `json:"outline"`
	Shadow  float32 `json:"shadow"`
	// Rings are the clicks' rings, Chevrons a scroll's.
	Rings    []pageCursorRing `json:"rings"`
	Chevrons *pageChevrons    `json:"chevrons"`
	Badge    *pageBadge       `json:"badge"`
	Look     pageCursorLook   `json:"look"`
}

// pageCursorRing is a click's ring, its radius in dp.
type pageCursorRing struct {
	X     float32 `json:"x"`
	Y     float32 `json:"y"`
	R     float32 `json:"r"`
	Alpha float32 `json:"alpha"`
}

// pageChevrons are a scroll's chevrons: their direction, 1 down, how far they
// slid in dp, and their opacity.
type pageChevrons struct {
	Dir   float32 `json:"dir"`
	Shift float32 `json:"shift"`
	Alpha float32 `json:"alpha"`
}

// pageBadge is the pill beside the arrow.
type pageBadge struct {
	Text  string          `json:"text"`
	Ink   string          `json:"ink"`
	Alpha float32         `json:"alpha"`
	Icon  []icons.SVGPath `json:"icon"`
}

// pageCursorLook is what never changes about the cursor: the arrow's shape and
// the sizes drawAgentCursor draws in, dp unless said.
type pageCursorLook struct {
	Arrow       []icons.SVGPath `json:"arrow"`
	ArrowBox    float32         `json:"arrowBox"`
	ArrowSize   float32         `json:"arrowSize"`
	Tip         [2]float32      `json:"tip"`
	Outline     float32         `json:"outline"`
	RingWidth   float32         `json:"ringWidth"`
	Below       float32         `json:"below"`
	ChevOutline float32         `json:"chevOutline"`
	ChevLine    float32         `json:"chevLine"`
	BadgeAt     [2]float32      `json:"badgeAt"`
	BadgeHeight float32         `json:"badgeHeight"`
	PadLeft     float32         `json:"padLeft"`
	PadRight    float32         `json:"padRight"`
	IconSize    float32         `json:"iconSize"`
	IconBox     float32         `json:"iconBox"`
	IconStroke  float32         `json:"iconStroke"`
	Gap         float32         `json:"gap"`
	BadgeLine   float32         `json:"badgeLine"`
	// Text is the badge's text size, in sp, and Font its face.
	Text float32 `json:"text"`
	Font string  `json:"font"`
}

var cursorLook = pageCursorLook{
	Arrow: cursorArrow.SVGPaths(), ArrowBox: icons.BibataArrowBox, ArrowSize: cursorArrowSize,
	Tip: [2]float32{cursorTip.X, cursorTip.Y}, Outline: cursorArrowOutline, RingWidth: cursorRingWidth,
	Below: cursorChevronsBelow, ChevOutline: cursorChevronOutline, ChevLine: cursorChevronLine,
	BadgeAt:     [2]float32{float32(cursorBadgeX), float32(cursorBadgeY)},
	BadgeHeight: cursorBadgeHeight, PadLeft: float32(cursorBadgePadLeft), PadRight: float32(cursorBadgePadRight),
	IconSize: float32(cursorBadgeIcon), IconBox: icons.SVGBox, IconStroke: icons.SVGStroke,
	Gap: float32(ui.Space(cursorBadgeGap)), BadgeLine: cursorBadgeOutline,
	Text: float32(cursorBadgeText), Font: string(fonts.Sans) + ", " + fonts.SystemSans,
}

// pageCursorOf is c as drawAgentCursor draws it at gtx.Now, moved into the
// page at page, in the pane's pixels, whose CSS pixel is scale of them; nil
// when it is not up.
func pageCursorOf(gtx C, c *agentCursor, tint, outline color.NRGBA, icon *icons.Icon, page image.Rectangle, scale float32) *pageCursor {
	now := gtx.Now
	a := c.alpha(now)
	if !c.shown || a <= 0 {
		return nil
	}
	css := func(p f32.Point) f32.Point { return p.Sub(layout.FPt(page.Min)).Div(scale) }
	at, landed := css(c.position(now)), c.landed(now)
	pc := &pageCursor{
		X: at.X, Y: at.Y, Unit: float32(gtx.Dp(1)) / scale, Alpha: a, Scale: c.dipScale(landed),
		Tint: cssColor(tint), Outline: cssColor(outline), Shadow: float32(cursorShadow.A) / 0xff,
		Rings: []pageCursorRing{}, Look: cursorLook,
	}
	pc.Look.Text = float32(gtx.Sp(unit.Sp(cursorLook.Text))) / float32(gtx.Dp(1))
	for _, r := range c.rings {
		pc.ring(css(r.at), now.Sub(r.start), r.span)
	}
	if c.scroll == 0 {
		pc.ring(at, landed, c.span(cursorRing))
	} else if shift, alpha, ok := chevronMotion(landed, c.span(cursorScroll), c.scroll); ok {
		pc.Chevrons = &pageChevrons{Dir: float32(c.scroll), Shift: shift, Alpha: alpha}
	}
	if verb, shown := cursorVerb(c.action), c.badgeAlpha(now); verb != "" && shown > 0 {
		pc.Badge = &pageBadge{Text: verb, Ink: cssColor(inkOn(tint, outline)), Alpha: shown, Icon: icon.SVGPaths()}
	}
	return pc
}

// ring adds a click's ring at at, since into its span, while it shows.
func (pc *pageCursor) ring(at f32.Point, since, span time.Duration) {
	if r, alpha, ok := ringMotion(since, span); ok {
		pc.Rings = append(pc.Rings, pageCursorRing{X: at.X, Y: at.Y, R: r, Alpha: alpha})
	}
}
