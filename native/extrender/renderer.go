// Package extrender draws an extension's tree with the UI kit and turns what
// the person does on it into events for the extension.
package extrender

import (
	"image"
	"strconv"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/ui"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

// drawFunc draws one node; id is the node's path in the tree, the key its
// widget state lives under.
type drawFunc func(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D

// drawers is filled by each family's file in its own init, so the families
// stay in their own files.
var drawers = map[string]drawFunc{}

func register(fs map[string]drawFunc) {
	for name, f := range fs {
		if _, dup := drawers[name]; dup {
			panic("extrender: " + name + " registered twice")
		}
		drawers[name] = f
	}
}

// Renderer draws one surface. It keeps each node's widget state between
// frames and drops the state of a node the tree no longer has.
type Renderer struct {
	root  *ui.Root
	send  func(extension.EventData)
	state map[string]any
	seen  map[string]bool
	code  Code
	// focus is the key whose field takes the keyboard this frame.
	focus string
	// dialogs are the dialogs the tree opened this frame, drawn after it over
	// the whole surface: a dialog covers what it is drawn in, and a node does
	// not know where in the surface it sits.
	dialogs []layout.Widget
	drag    dragPointer
	// scrollRoom is, on each axis, how far the viewport of the scroll being
	// drawn runs on from the start of the child being drawn; 0 outside one.
	scrollRoom image.Point
}

// New returns a renderer that hands every event to send. root is the window's
// overlay layer, where menus, tooltips and dialogs open.
func New(root *ui.Root, send func(extension.EventData)) *Renderer {
	return &Renderer{root: root, send: send, state: map[string]any{}, seen: map[string]bool{}}
}

// Layout draws tree filling gtx.
func (r *Renderer) Layout(gtx C, th *ui.Theme, tree extension.Node) D {
	clear(r.seen)
	r.dialogs = r.dialogs[:0]
	r.readPointer(gtx)
	r.holdGroups(gtx)
	area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, &r.drag)
	d := r.node(gtx, th, tree, "")
	for i := 0; i < len(r.dialogs); i++ {
		r.dialogs[i](gtx)
	}
	area.Pop()
	r.releaseStranded(gtx)
	for id := range r.state {
		if !r.seen[id] {
			delete(r.state, id)
		}
	}
	r.focus = ""
	return d
}

// Focus gives the keyboard to the field under k on the next frame; a k no
// field on that frame has is forgotten.
func (r *Renderer) Focus(k string) { r.focus = k }

// takeFocus focuses ed when n is the field Focus asked for.
func (r *Renderer) takeFocus(gtx C, n extension.Node, ed *widget.Editor) {
	if r.focus == "" || n.Key != r.focus {
		return
	}
	r.focus = ""
	gtx.Execute(key.FocusCmd{Tag: ed})
}

// node draws n. A type with no drawer is one the backend let through from a
// newer contract than this window speaks, so it draws nothing.
func (r *Renderer) node(gtx C, th *ui.Theme, n extension.Node, id string) D {
	draw, ok := drawers[n.Type]
	if !ok {
		return D{}
	}
	return draw(r, gtx, th, n, id)
}

// child is the i-th child of the node at id as a widget, under its key when
// it has one so its state follows it when siblings move.
func (r *Renderer) child(th *ui.Theme, c extension.Node, parent string, i int) layout.Widget {
	id := parent + "/" + strconv.Itoa(i)
	if c.Key != "" {
		id = parent + "/k:" + c.Key
	}
	return func(gtx C) D { return r.node(gtx, th, c, id) }
}

// children is every child of n as widgets.
func (r *Renderer) children(th *ui.Theme, n extension.Node, id string) []layout.Widget {
	ws := make([]layout.Widget, len(n.Children))
	for i, c := range n.Children {
		ws[i] = r.child(th, c, id, i)
	}
	return ws
}

// fire sends e to the extension under h; a node with no handler sends nothing.
func (r *Renderer) fire(h extension.Handler, e extension.EventData) {
	if h == "" {
		return
	}
	e.Handler = h
	r.send(e)
}

// stateOf is the widget state of the node at id, made on its first frame.
func stateOf[T any](r *Renderer, id string) *T {
	r.seen[id] = true
	if s, ok := r.state[id].(*T); ok {
		return s
	}
	s := new(T)
	r.state[id] = s
	return s
}

// props is n's props as P; the contract decoded them into exactly that type.
func props[P any](n extension.Node) P {
	p, _ := n.Props.(P)
	return p
}
