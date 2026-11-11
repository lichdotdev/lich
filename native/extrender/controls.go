package extrender

import (
	"image"
	"slices"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

func init() {
	register(map[string]drawFunc{
		extension.Scroll.Name:       drawScroll,
		extension.Wrap.Name:         drawWrap,
		extension.WrapItem.Name:     drawWrapItem,
		extension.Separator.Name:    drawSeparator,
		extension.FormLabel.Name:    drawFormLabel,
		extension.Glyph.Name:        drawGlyph,
		extension.Skeleton.Name:     drawSkeleton,
		extension.Progress.Name:     drawProgress,
		extension.Spinner.Name:      drawSpinner,
		extension.BusyRing.Name:     drawBusyRing,
		extension.Chip.Name:         drawChip,
		extension.Checkbox.Name:     drawCheckbox,
		extension.Switch.Name:       drawSwitch,
		extension.Toggle.Name:       drawToggle,
		extension.ToggleGroup.Name:  drawToggleGroup,
		extension.Tabs.Name:         drawTabs,
		extension.Collapse.Name:     drawCollapse,
		extension.Presence.Name:     drawPresence,
		extension.SortableList.Name: drawSortableList,
		extension.ResizeHandle.Name: drawResizeHandle,
	})
}

// defaultIconSize is size-4, the kit's icon.
var defaultIconSize = ui.Space(4)

// blankIcon draws nothing at an icon's size, for a kit component that needs
// an icon the extension named wrong.
var blankIcon = &icons.Icon{}

// synced is the value of a prop the person can change on screen. The
// extension's value wins only when it is news: one it sent before may since
// have been changed by the person, and a stale render must not undo that.
type synced[T comparable] struct {
	last T
	set  bool
}

// take reports whether v differs from what the extension last sent.
func (s *synced[T]) take(v T) bool {
	if s.set && s.last == v {
		return false
	}
	s.last, s.set = v, true
	return true
}

// layer is the window's overlay layer, or one of the renderer's own when it
// was made without one, as in a test: popups then open in a layer no window
// lays out, which draws them nowhere but never panics.
func (r *Renderer) layer() *ui.Root {
	if r.root == nil {
		r.root = new(ui.Root)
	}
	return r.root
}

// body is n's children as one widget: the child itself when there is one,
// stacked when there are more.
func (r *Renderer) body(th *ui.Theme, n extension.Node, id string) layout.Widget {
	if len(n.Children) == 1 {
		return r.child(th, n.Children[0], id, 0)
	}
	return ui.Stack(0, r.children(th, n, id)...)
}

// extra draws a node a prop carries, outside the children, under id.
func (r *Renderer) extra(th *ui.Theme, n extension.Node, id string) layout.Widget {
	return func(gtx C) D { return r.node(gtx, th, n, id) }
}

// nodes is a list of nodes a prop carries as widgets, under parent.
func (r *Renderer) nodes(th *ui.Theme, ns []extension.Node, parent string) []layout.Widget {
	ws := make([]layout.Widget, len(ns))
	for i, c := range ns {
		ws[i] = r.child(th, c, parent, i)
	}
	return ws
}

// iconOr is icon, or the blank icon for a component that cannot draw none.
func (r *Renderer) iconOr(name extension.Icon) *icons.Icon {
	if ic := r.icon(name); ic != nil {
		return ic
	}
	return blankIcon
}

func iconSize(size float64) unit.Dp {
	if size <= 0 {
		return defaultIconSize
	}
	return unit.Dp(size)
}

func drawScroll(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.ScrollProps](n)
	list := stateOf[widget.List](r, id)
	list.Axis = layout.Vertical
	if p.Direction == extension.DirectionRow {
		list.Axis = layout.Horizontal
	}
	kids := r.children(th, n, id)
	return th.ScrollArea(list).Layout(gtx, len(kids), func(gtx C, i int) D { return kids[i](gtx) })
}

func drawWrap(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.WrapProps](n)
	kids := make([]ui.WrapChild, len(n.Children))
	for i, c := range n.Children {
		w := r.child(th, c, id, i)
		if c.Type == extension.WrapItem.Name && props[extension.WrapItemProps](c).MarginAuto {
			kids[i] = ui.WrapMLAuto(w)
			continue
		}
		kids[i] = ui.WrapItem(w)
	}
	wrap := ui.Wrap{GapX: unit.Dp(p.GapX), GapY: unit.Dp(p.GapY), Justify: wrapJustify(p.Justify), ContentStart: p.ContentStart}
	return wrap.Layout(gtx, kids...)
}

// wrapJustify maps Justify onto the Wrap's; the kit's Wrap has no center, so
// a centered wrap starts at the start.
func wrapJustify(j extension.Justify) ui.Justify {
	switch j {
	case extension.JustifyEnd:
		return ui.JustifyEnd
	case extension.JustifyBetween:
		return ui.JustifyBetween
	}
	return ui.JustifyStart
}

// drawWrapItem draws its children; the Wrap around it reads MarginAuto.
func drawWrapItem(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	return r.body(th, n, id)(gtx)
}

func drawSeparator(_ *Renderer, gtx C, th *ui.Theme, _ extension.Node, _ string) D {
	return th.Separator(gtx)
}

func drawFormLabel(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.FormLabelProps](n)
	l := th.Label(p.Text)
	if p.Size > 0 {
		l.Size = unit.Sp(p.Size)
	}
	if p.Weight != "" {
		l.Weight = weight(p.Weight)
	}
	l.Uppercase, l.Disabled = p.Uppercase, p.Disabled
	return l.Layout(gtx)
}

// drawGlyph draws any set's icon, a devicon included; a name no set has
// keeps its square empty, so the row around it does not shift.
func drawGlyph(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.GlyphProps](n)
	size, fg := iconSize(p.Size), r.color(th, p.Color, th.Foreground)
	if name, ok := strings.CutPrefix(string(p.Name), deviconPrefix); ok {
		if b, ok := icons.LookupDevicon(name); ok {
			return b.Layout(gtx, size, fg)
		}
		return D{Size: image.Pt(gtx.Dp(size), gtx.Dp(size))}
	}
	return r.iconOr(p.Name).Layout(gtx, size, fg)
}

func drawSkeleton(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.SkeletonProps](n)
	return th.Skeleton(unit.Dp(p.Width), unit.Dp(p.Height)).Layout(gtx)
}

func drawProgress(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	return th.Progress(props[extension.ProgressProps](n).Percent).Layout(gtx)
}

func drawSpinner(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.SpinnerProps](n)
	return th.Spinner(gtx, iconSize(p.Size), r.color(th, p.Color, th.Foreground))
}

func drawBusyRing(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.BusyRingProps](n)
	fg := r.color(th, p.Color, th.Foreground)
	return th.BusyRing(gtx, iconSize(p.Size), fg, r.color(th, p.Track, th.RingTrack(fg)))
}

func drawChip(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.ChipProps](n)
	click := stateOf[widget.Clickable](r, id)
	if click.Clicked(gtx) {
		r.fire(p.OnPress, extension.EventData{})
	}
	return th.Chip(click, p.Label, p.Count, p.On).Layout(gtx)
}

type boolState struct {
	value widget.Bool
	sent  synced[bool]
}

// update takes the extension's value and fires a change the person made.
func (s *boolState) update(r *Renderer, gtx C, value bool, h extension.Handler) {
	if s.sent.take(value) {
		s.value.Value = value
	}
	if s.value.Update(gtx) {
		r.fire(h, extension.EventData{Checked: s.value.Value})
	}
}

func drawCheckbox(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.CheckboxProps](n)
	s := stateOf[boolState](r, id)
	s.update(r, gtx, p.Checked, p.OnChange)
	c := th.Checkbox(&s.value)
	c.Disabled = p.Disabled
	return c.Layout(gtx)
}

type switchState struct {
	value ui.SwitchState
	sent  synced[bool]
}

func drawSwitch(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.SwitchProps](n)
	s := stateOf[switchState](r, id)
	if s.sent.take(p.Checked) {
		s.value.Value = p.Checked
	}
	if s.value.Update(gtx) {
		r.fire(p.OnChange, extension.EventData{Checked: s.value.Value})
	}
	sw := th.Switch(&s.value)
	sw.Disabled = p.Disabled
	if p.Size == extension.SwitchSizeSM {
		sw.Size = ui.SwitchSizeSM
	}
	return sw.Layout(gtx)
}

func drawToggle(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.ToggleProps](n)
	s := stateOf[boolState](r, id)
	s.update(r, gtx, p.Pressed, p.OnChange)
	t := th.Toggle(&s.value, p.Text)
	t.Icon, t.Variant, t.Size, t.Disabled = r.icon(p.Icon), toggleVariant(p.Variant), toggleSize(p.Size), p.Disabled
	t.Height, t.PadX, t.TextSize = unit.Dp(p.Height), unit.Dp(p.PadX), unit.Sp(p.TextSize)
	return t.Layout(gtx)
}

type enumState struct {
	value widget.Enum
	sent  synced[string]
}

func drawToggleGroup(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.ToggleGroupProps](n)
	s := stateOf[enumState](r, id)
	if s.sent.take(p.Value) {
		s.value.Value = p.Value
	}
	if s.value.Update(gtx) {
		r.fire(p.OnChange, extension.EventData{Text: s.value.Value})
	}
	items := make([]ui.ToggleItem, len(p.Items))
	for i, it := range p.Items {
		items[i] = ui.ToggleItem{Value: it.Value, Text: it.Text, Icon: r.icon(it.Icon), Disabled: it.Disabled}
	}
	g := th.ToggleGroup(&s.value, items...)
	g.Variant, g.Size = toggleVariant(p.Variant), toggleSize(p.Size)
	g.ItemHeight, g.ItemPadX, g.ItemText = unit.Dp(p.ItemHeight), unit.Dp(p.ItemPadX), unit.Sp(p.ItemText)
	return g.Layout(gtx)
}

func toggleVariant(v extension.ToggleVariant) ui.ToggleVariant {
	if v == extension.ToggleVariantOutline {
		return ui.ToggleVariantOutline
	}
	return ui.ToggleVariantDefault
}

func toggleSize(s extension.ToggleSize) ui.ToggleSize {
	switch s {
	case extension.ToggleSizeSM:
		return ui.ToggleSizeSM
	case extension.ToggleSizeLG:
		return ui.ToggleSizeLG
	}
	return ui.ToggleSizeDefault
}

type tabsState struct {
	tabs ui.TabsState
	sent synced[int]
}

// drawTabs draws the triggers and, under them, the child of the selected
// tab: a tab's children are its panels, one each, in order.
func drawTabs(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.TabsProps](n)
	s := stateOf[tabsState](r, id)
	if s.sent.take(p.Selected) {
		s.tabs.Selected = p.Selected
	}
	triggers := make([]ui.TabsTrigger, len(p.Triggers))
	for i, tr := range p.Triggers {
		triggers[i] = ui.TabsTrigger{Text: tr.Text, Icon: r.icon(tr.Icon), Disabled: tr.Disabled}
		if tr.Trailing != nil {
			triggers[i].Trailing = r.extra(th, *tr.Trailing, id+"/trailing/"+strconv.Itoa(i))
		}
	}
	t := th.Tabs(&s.tabs, triggers...)
	t.Variant, t.Orientation = tabsVariant(p.Variant), ui.TabsHorizontal
	if p.Orientation == extension.TabsVertical {
		t.Orientation = ui.TabsVertical
	}
	d := t.Layout(gtx, r.children(th, n, id)...)
	if s.tabs.Changed() {
		r.fire(p.OnChange, extension.EventData{Index: s.tabs.Selected})
	}
	return d
}

func tabsVariant(v extension.TabsVariant) ui.TabsVariant {
	switch v {
	case extension.TabsVariantLine:
		return ui.TabsVariantLine
	case extension.TabsVariantGhost:
		return ui.TabsVariantGhost
	}
	return ui.TabsVariantDefault
}

func drawCollapse(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	c := stateOf[ui.Collapse](r, id)
	return c.Layout(gtx, props[extension.CollapseProps](n).Open, r.body(th, n, id))
}

// presenceState keeps each child the Presence still draws, so one the tree
// dropped plays its exit from the node it last was.
type presenceState struct {
	motion ui.Presence
	kids   map[string]presenceKid
}

type presenceKid struct {
	node  extension.Node
	index int
}

func drawPresence(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	s := stateOf[presenceState](r, id)
	if s.kids == nil {
		s.kids = map[string]presenceKid{}
	}
	live := make([]string, len(n.Children))
	for i, c := range n.Children {
		live[i] = childKey(c, i)
		s.kids[live[i]] = presenceKid{node: c, index: i}
	}
	shown := s.motion.Update(gtx.Now, live)
	for k := range s.kids {
		if !slices.Contains(shown, k) {
			delete(s.kids, k)
		}
	}
	ws := make([]layout.Widget, len(shown))
	for j, k := range shown {
		kid := s.kids[k]
		w := r.child(th, kid.node, id, kid.index)
		ws[j] = func(gtx C) D { return s.motion.Layout(gtx, k, w) }
	}
	return ui.Stack(0, ws...)(gtx)
}

// childKey names a child among its siblings: its key, else its place.
func childKey(c extension.Node, i int) string {
	if c.Key != "" {
		return c.Key
	}
	return strconv.Itoa(i)
}

func drawSortableList(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.SortableListProps](n)
	s := stateOf[ui.Sortable](r, id)
	keys := make([]string, len(n.Children))
	for i, c := range n.Children {
		keys[i] = childKey(c, i)
	}
	if d, ok := s.Update(gtx); ok {
		if order, moved := reorder(keys, d); moved {
			r.fire(p.OnReorder, extension.EventData{Order: order})
		}
	}
	list := ui.SortList{Gap: unit.Dp(p.Gap), Horizontal: p.Horizontal, Within: p.Within, Leaves: p.Leaves}
	return s.List(gtx, list, keys, func(gtx C, i int) D {
		return s.Item(gtx, keys[i], r.child(th, n.Children[i], id, i), nil)
	})
}

// reorder is keys with the dropped one moved to the slot of the one it was
// dropped over, as dnd-kit's arrayMove; a drop on itself or off the list
// moves nothing.
func reorder(keys []string, d ui.SortDrop) ([]string, bool) {
	from, to := slices.Index(keys, d.ID), slices.Index(keys, d.Over)
	if from < 0 || to < 0 || from == to {
		return nil, false
	}
	out := slices.Delete(slices.Clone(keys), from, from+1)
	return slices.Insert(out, to, d.ID), true
}

type resizeState struct {
	handle ui.ResizeHandle
	width  unit.Dp
	sent   synced[float64]
}

// drawResizeHandle draws its children as the panel, Width wide, and the
// handle on its Edge. The panel follows the drag; the extension hears the
// width once, on release, rather than on every frame of the drag.
func drawResizeHandle(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.ResizeHandleProps](n)
	s := stateOf[resizeState](r, id)
	if s.sent.take(p.Width) {
		s.width = unit.Dp(p.Width)
	}
	s.handle.Edge = ui.PanelEdgeRight
	if p.Edge == extension.PanelEdgeLeft {
		s.handle.Edge = ui.PanelEdgeLeft
	}
	ceiling := unit.Dp(p.MaxWidth)
	if ceiling <= 0 {
		ceiling = gtx.Metric.PxToDp(gtx.Constraints.Max.X)
	}
	next, released := s.handle.Update(gtx, s.width, unit.Dp(p.MinWidth), ceiling)
	s.width = next
	if released {
		r.fire(p.OnResize, extension.EventData{Width: float64(next)})
	}
	px := min(gtx.Dp(s.width), gtx.Constraints.Max.X)
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = px, px
	d := r.body(th, n, id)(gtx)
	box := gtx
	box.Constraints.Max = image.Pt(px, d.Size.Y)
	s.handle.Layout(box, th)
	return D{Size: image.Pt(px, d.Size.Y), Baseline: d.Baseline}
}
