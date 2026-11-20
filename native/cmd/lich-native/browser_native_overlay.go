package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"time"

	"gioui.org/f32"

	"github.com/lichdotdev/lich/native/internal/webview"
	"github.com/lichdotdev/lich/native/ui"
)

// What the notes draw over a page the window draws itself is drawn by the
// page, in a layer of its own that takes no pointer: drawn by the window, it
// stayed under the page's X window, and cut out of it, the cut showed the
// pane's background behind every dash and stroke.

// pageOverlay is that layer's content, in the page's CSS pixels.
type pageOverlay struct {
	Rings []pageRing `json:"rings"`
	// Color is the pick's, which the rings are drawn in; Dash and March are
	// a ring's dash and how long its dashes take to move one along.
	Color string  `json:"color"`
	Dash  float32 `json:"dash"`
	March int64   `json:"march"`
	// Strokes are the sketch's, Width their line.
	Strokes []pageStroke `json:"strokes"`
	Width   float32      `json:"width"`
	// Trail is the eraser's trail, as the outline of its ribbon; Eraser the
	// ring around the pointer while it erases.
	Trail  []float32   `json:"trail"`
	Eraser *pageCircle `json:"eraser"`
	// Hold keeps the wheel from scrolling the page while strokes stand on
	// it: they point at what was on screen.
	Hold bool `json:"hold"`
}

// pageStroke is a sketch stroke: its color and opacity, and the lines it
// draws, each as x, y pairs.
type pageStroke struct {
	Ink     string      `json:"ink"`
	Opacity float32     `json:"opacity"`
	Lines   [][]float32 `json:"lines"`
}

// pageCircle is the eraser's ring at the pointer.
type pageCircle struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	R float32 `json:"r"`
}

// pageOverlayOf is what n draws over the page in this frame.
func pageOverlayOf(n *browserNotes) pageOverlay {
	o := pageOverlay{
		Rings: n.rings, Color: cssColor(notePick), Dash: float32(ui.RingDash), March: ui.RingMarch.Milliseconds(),
		Width: sketchLine, Trail: flatten(n.trailOnPage), Hold: len(n.strokes) > 0,
	}
	if len(o.Rings) > 0 {
		o.Dash = o.Rings[0].Dash
	}
	for _, s := range n.strokes {
		opacity := float32(s.ink.A) / 0xff
		if s.doomed {
			opacity *= sketchDoomedOpacity
		}
		st := pageStroke{Ink: cssColor(s.ink), Opacity: opacity}
		for _, points := range strokeSegments(s) {
			st.Lines = append(st.Lines, flatten(points))
		}
		o.Strokes = append(o.Strokes, st)
	}
	if n.erasing && n.rubOver {
		o.Eraser = &pageCircle{X: n.rubAt.X, Y: n.rubAt.Y, R: rubRing}
	}
	return o
}

// flatten is points as x, y pairs.
func flatten(points []f32.Point) []float32 {
	flat := make([]float32, 0, 2*len(points))
	for _, p := range points {
		flat = append(flat, p.X, p.Y)
	}
	return flat
}

// cssColor is c's color, without its alpha, as CSS hex.
func cssColor(c color.NRGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// pageOverlayScript draws an overlay, or takes the layer away when it has
// nothing. It changes the layer in place, a stroke's line growing as the
// pointer moves: rebuilt on every update, the layer went blank for a frame
// now and then under WebKit's shared-memory renderer (measured on the
// user's display, 24 blank frames in 20 s of drawing). It builds with DOM
// calls rather than markup, which a page enforcing Trusted Types refuses.
const pageOverlayScript = `((o, trail, ring) => {
	const id = 'lich-note-rings';
	let host = document.getElementById(id);
	window.__lichHold = o.hold;
	if (!window.__lichWheel) {
		window.__lichWheel = true;
		addEventListener('wheel', e => { if (window.__lichHold) e.preventDefault(); }, {passive: false, capture: true});
	}
	if (!o.rings.length && !o.strokes.length && !o.trail.length && !o.eraser) {
		if (host) host.remove();
		return;
	}
	const ns = 'http://www.w3.org/2000/svg';
	const el = (tag, attrs, parent) => {
		const e = document.createElementNS(ns, tag);
		set(e, attrs);
		parent.appendChild(e);
		return e;
	};
	const set = (e, attrs) => {
		for (const k in attrs) {
			const v = String(attrs[k]);
			if (e.getAttribute(k) !== v) e.setAttribute(k, v);
		}
	};
	if (!host || !host.__lich) {
		if (host) host.remove();
		host = document.createElement('div');
		host.id = id;
		host.style.cssText = 'position:fixed;inset:0;pointer-events:none;z-index:2147483647';
		const root = host.attachShadow({mode: 'open'});
		const style = document.createElement('style');
		root.appendChild(style);
		const rings = document.createElement('div');
		root.appendChild(rings);
		const page = el('svg', {width: '100%', height: '100%', style: 'left:0;top:0'}, root);
		const lines = document.createElementNS(ns, 'g');
		page.appendChild(lines);
		host.__lich = {style, rings, lines,
			trail: el('polygon', {display: 'none'}, page),
			ring: el('circle', {display: 'none', 'stroke-width': 1.5}, page)};
		document.documentElement.appendChild(host);
	}
	const s = host.__lich;
	const css = 'svg{position:fixed;overflow:visible}@keyframes m{to{stroke-dashoffset:' + (-2 * o.dash) + 'px}}.m{animation:m ' + o.march + 'ms linear infinite}';
	if (s.style.textContent !== css) s.style.textContent = css;
	const rk = JSON.stringify([o.rings, o.color]);
	if (s.rk !== rk) {
		s.rk = rk;
		s.rings.replaceChildren();
		for (const r of o.rings) {
			const svg = el('svg', {width: r.W, height: r.H, style: 'left:' + r.X + 'px;top:' + r.Y + 'px'}, s.rings);
			const box = {width: r.W, height: r.H, rx: r.Radius};
			const dash = {'stroke-dasharray': r.Dash + ' ' + r.Dash};
			if (r.March) dash.class = 'm';
			el('rect', {...box, fill: o.color, 'fill-opacity': r.Fill / 255}, svg);
			el('rect', {...box, fill: 'none', stroke: '#fff', 'stroke-opacity': 0.69, 'stroke-width': r.Width + 1, ...dash}, svg);
			el('rect', {...box, fill: 'none', stroke: o.color, 'stroke-width': r.Width, ...dash}, svg);
		}
	}
	let n = 0;
	for (const st of o.strokes) {
		for (const line of st.lines) {
			const attrs = {points: line.join(' '), fill: 'none', stroke: st.ink, 'stroke-opacity': st.opacity, 'stroke-width': o.width, 'stroke-linecap': 'round', 'stroke-linejoin': 'round'};
			const e = s.lines.children[n];
			if (e) set(e, attrs); else el('polyline', attrs, s.lines);
			n++;
		}
	}
	while (s.lines.children.length > n) s.lines.lastChild.remove();
	if (o.trail.length) set(s.trail, {display: 'inline', points: o.trail.join(' '), fill: trail[0], 'fill-opacity': trail[1]});
	else set(s.trail, {display: 'none'});
	if (o.eraser) set(s.ring, {display: 'inline', cx: o.eraser.x, cy: o.eraser.y, r: o.eraser.r, fill: ring[0], 'fill-opacity': ring[1], stroke: ring[2], 'stroke-opacity': ring[3]});
	else set(s.ring, {display: 'none'});
})`

// overlayScript is the call that draws o over the page, the same text for
// the same overlay, so a page is told only of a change.
func overlayScript(o pageOverlay) string {
	if o.Rings == nil {
		o.Rings = []pageRing{}
	}
	if o.Strokes == nil {
		o.Strokes = []pageStroke{}
	}
	encoded, err := json.Marshal(o)
	if err != nil {
		encoded = []byte(`{"rings":[],"strokes":[],"trail":[]}`)
	}
	alpha := func(c color.NRGBA) float32 { return float32(c.A) / 0xff }
	return fmt.Sprintf("%s(%s, [%q, %g], [%q, %g, %q, %g])", pageOverlayScript, encoded,
		cssColor(rubTrailColor), alpha(rubTrailColor), cssColor(rubRingFill), alpha(rubRingFill), cssColor(rubRingEdge), alpha(rubRingEdge))
}

// overlayVisibility shows or hides the layer, which a shot of the page under
// it leaves out: the sketch's picture draws the strokes itself.
const overlayVisibility = `(h => h && (h.style.visibility = %q))(document.getElementById('lich-note-rings'))`

// pageShotWait bounds a shot of the page for the sketch's picture.
const pageShotWait = 5 * time.Second

// shootUnder is the page as it shows, without the layer the notes draw over
// it. The page takes the hide, the shot and the show in that order.
func shootUnder(ctx context.Context, page *webview.Page) (*image.RGBA, error) {
	ctx, cancel := context.WithTimeout(ctx, pageShotWait)
	defer cancel()
	page.Run(fmt.Sprintf(overlayVisibility, "hidden"))
	defer page.Run(fmt.Sprintf(overlayVisibility, ""))
	return page.Snapshot(ctx)
}
