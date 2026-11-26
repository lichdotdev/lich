package main

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/api"
)

// handScreens are the screens the sweep hovers, each built the way its
// stale_* test builds it.
func handScreens() map[string]func(*testing.T) *frameRig {
	screens := map[string]func(*testing.T) *frameRig{
		"sidebar": func(t *testing.T) *frameRig { _, r := sidebarFrames(t); return r },
		"git":     func(t *testing.T) *frameRig { _, r := gitFrames(t); return r },
		"pulls":   func(t *testing.T) *frameRig { _, r := pullsFrames(t); return r },
		// The tree is read off the frame: without the wait a slow runner hovers
		// the pane still loading, whose only clickable is its background.
		"code": func(t *testing.T) *frameRig { _, r, _ := codeFrames(t); r.waitFor(t, "a.go"); return r },
		"chat": func(t *testing.T) *frameRig { _, r, _ := chatFrames(t, api.ViewChat); return r },
	}
	for name, tab := range map[string]dockTab{"review": dockReview, "code tab": dockCode, "artifacts": dockArtifacts} {
		screens["dock "+name] = func(t *testing.T) *frameRig { _, r := stageFrames(t, tab); return r }
	}
	for _, section := range []string{sectionGeneral, "general/window", "general/notifications", sectionHotkeys, sectionProviders, sectionSandbox, sectionVersionControl, sectionEditor, sectionUpdates, sectionHelp, sectionServices, sectionExtensions} {
		screens["settings "+section] = func(t *testing.T) *frameRig { _, r := settingsFrames(t, section); return r }
	}
	return screens
}

// A control that answers a click shows the hand over it while it is enabled
// and the arrow when it is not; the sweep finds every control a screen draws
// and hovers it.
func TestEveryClickableShowsTheHand(t *testing.T) {
	var missing []string
	checked := 0
	for name, build := range handScreens() {
		t.Run(name, func(t *testing.T) {
			r := build(t)
			r.settle(t)
			for _, n := range r.nodes() {
				if n.Desc.Gestures&input.ClickGesture == 0 || n.Desc.Gestures&input.ScrollGesture != 0 || n.Desc.Bounds.Empty() {
					continue
				}
				p, ok := r.handProbe(n)
				if !ok {
					continue
				}
				checked++
				r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: p})
				r.frame()
				want := pointer.CursorPointer
				if n.Desc.Disabled || disabledAtRest[fmt.Sprint(name, n.Desc.Bounds)] {
					want = pointer.CursorDefault
				}
				got := r.router.Cursor()
				if got != pointer.CursorDefault && got != pointer.CursorPointer {
					continue // a text field, a resize or a drag area: its own cursor is the right one
				}
				if got != want {
					missing = append(missing, fmt.Sprintf("%s: %q %q at %v: cursor %v, want %v", name, n.Desc.Label, n.Desc.Description, n.Desc.Bounds, got, want))
				}
			}
		})
	}
	t.Logf("%d clickable nodes hovered, %d with the wrong cursor", checked, len(missing))
	if len(missing) > 0 {
		t.Errorf("%d clickables with the wrong cursor:\n%s", len(missing), strings.Join(missing, "\n"))
	}
}

// disabledAtRest are the nodes a disabled control leaves behind: its face has
// no Clickable, so only the tooltip over it takes presses (and passes them
// on), and the arrow is right. Keyed by screen and bounds.
var disabledAtRest = map[string]bool{
	"dock review(10,758)-(42,790)":       true, // Attach, with no session
	"dock code tab(10,758)-(42,790)":     true,
	"dock artifacts(10,758)-(42,790)":    true,
	"git(895,758)-(978,790)":             true, // the pull request, with none
	"settings editor(961,165)-(993,197)": true, // the font stepper's minus, at its minimum
	"settings editor(961,166)-(993,198)": true, // same button at the other text rounding
}

// trackWidth is the widest a scrollbar's track or a resize edge is, in px.
const trackWidth = 12

// handProbe is a point on n worth hovering, or false for what is no control:
// a node with another inside it is a panel, a scrim or a stage that takes
// presses, a node thinner than a scrollbar's track is the track or a resize
// edge, and a point off the window, under a smaller node or outside the view
// of the list n scrolls in belongs to something else.
func (r *frameRig) handProbe(n input.SemanticNode) (f32.Point, bool) {
	nb := n.Desc.Bounds
	c := nb.Min.Add(nb.Max).Div(2)
	if !c.In(image.Rectangle{Max: r.size}) || min(nb.Dx(), nb.Dy()) <= trackWidth {
		return f32.Point{}, false
	}
	for _, o := range r.nodes() {
		b := o.Desc.Bounds
		if o.Desc.Gestures&input.ClickGesture == 0 || o.ID == n.ID {
			continue
		}
		if b != nb && b.In(nb) || c.In(b) && area(b) < area(nb) {
			return f32.Point{}, false
		}
	}
	if r.cutOff(n, c) {
		return f32.Point{}, false
	}
	return f32.Pt(float32(c.X), float32(c.Y)), true
}

// cutOff is whether p lies outside the view of a list n scrolls in: the
// node's bounds run on under the list's edge, where nothing takes a press.
func (r *frameRig) cutOff(n input.SemanticNode, p image.Point) bool {
	nodes := map[input.SemanticID]input.SemanticNode{}
	for _, o := range r.nodes() {
		nodes[o.ID] = o
	}
	for up := n.ParentID; up != 0; up = nodes[up].ParentID {
		if s := nodes[up]; s.Desc.Gestures&input.ScrollGesture != 0 && !p.In(s.Desc.Bounds) {
			return true
		}
	}
	return false
}
