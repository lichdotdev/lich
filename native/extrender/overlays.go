package extrender

import (
	"image"
	"strconv"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/ui"
)

func init() {
	register(map[string]drawFunc{
		extension.Tooltip.Name:       drawTooltip,
		extension.DropdownMenu.Name:  drawDropdownMenu,
		extension.ContextMenu.Name:   drawContextMenu,
		extension.DropdownPanel.Name: drawDropdownPanel,
		extension.Dialog.Name:        drawDialog,
	})
}

// defaultPanelWidth is w-60, the panel width the kit's own screens use, for
// a DropdownPanel that names none.
var defaultPanelWidth = ui.Space(60)

// drawTooltip wraps its children in a tooltip: Text in a label, or Content
// in a card, which a card variant with Text alone also draws.
func drawTooltip(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.TooltipProps](n)
	s := stateOf[ui.Tooltip](r, id)
	t := th.Tooltip(r.layer(), s, p.Text)
	pop := th.On(th.Popover)
	switch {
	case p.Content != nil:
		t = th.TooltipCard(r.layer(), s, r.extra(pop, *p.Content, id+"/content"))
	case p.Variant == extension.TooltipCard && p.Text != "":
		t = th.TooltipCard(r.layer(), s, pop.Text(ui.TextXS, p.Text).Layout)
	}
	t.Side = side(p.Side)
	return t.Layout(gtx, r.body(th, n, id))
}

func side(s extension.Side) ui.Side {
	switch s {
	case extension.SideBottom:
		return ui.SideBottom
	case extension.SideLeft:
		return ui.SideLeft
	case extension.SideRight:
		return ui.SideRight
	}
	return ui.SideTop
}

func drawDropdownMenu(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.DropdownMenuProps](n)
	s := stateOf[ui.MenuState](r, id)
	entries := r.menuEntries(th.On(th.Popover), p.Entries, id+"/entry")
	return th.DropdownMenu(r.layer(), s, entries).Layout(gtx, r.body(th, n, id))
}

func drawContextMenu(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.ContextMenuProps](n)
	s := stateOf[ui.MenuState](r, id)
	entries := func() []ui.MenuEntry { return r.menuEntries(th.On(th.Popover), p.Entries, id+"/entry") }
	return th.ContextMenuLazy(r.layer(), s, entries).Layout(gtx, r.body(th, n, id))
}

// menuEntries maps entries onto the kit's; pop is the popover they draw on,
// which a Lead node is drawn with.
func (r *Renderer) menuEntries(pop *ui.Theme, es []extension.MenuEntry, parent string) []ui.MenuEntry {
	out := make([]ui.MenuEntry, len(es))
	for i, e := range es {
		out[i] = r.menuEntry(pop, e, parent+"/"+strconv.Itoa(i))
	}
	return out
}

func (r *Renderer) menuEntry(pop *ui.Theme, e extension.MenuEntry, id string) ui.MenuEntry {
	m := ui.MenuEntry{
		Kind: menuKind(e.Kind), Label: e.Label, Icon: r.icon(e.Icon), Shortcut: e.Shortcut,
		ShortcutDestructive: e.ShortcutDestructive, Variant: variant(e.Variant), Inset: e.Inset,
		Disabled: e.Disabled, Checked: e.Checked, Columns: e.Columns,
	}
	if e.Lead != nil {
		lead := *e.Lead
		m.Lead = func(gtx C, size unit.Dp) D {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(size), gtx.Dp(size)))
			return r.node(gtx, pop, lead, id+"/lead")
		}
	}
	if e.Swatch != "" {
		m.Swatch = r.color(pop, e.Swatch, pop.Foreground)
	}
	if len(e.Sub) > 0 {
		m.Sub = r.menuEntries(pop, e.Sub, id)
	}
	onSelect, onChecked := e.OnSelect, e.OnCheckedChange
	m.OnClick = func() { r.fire(onSelect, extension.EventData{}) }
	m.OnCheckedChange = func(c bool) { r.fire(onChecked, extension.EventData{Checked: c}) }
	return m
}

func menuKind(k extension.MenuEntryKind) ui.MenuEntryKind {
	switch k {
	case extension.MenuCheckboxItem:
		return ui.MenuCheckboxItem
	case extension.MenuRadioItem:
		return ui.MenuRadioItem
	case extension.MenuSubTrigger:
		return ui.MenuSubTrigger
	case extension.MenuLabel:
		return ui.MenuLabel
	case extension.MenuSeparator:
		return ui.MenuSeparator
	case extension.MenuSwatch:
		return ui.MenuSwatch
	}
	return ui.MenuItem
}

func drawDropdownPanel(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.DropdownPanelProps](n)
	s := stateOf[ui.DropdownPanelState](r, id)
	width := unit.Dp(p.Width)
	if width <= 0 {
		width = defaultPanelWidth
	}
	content := func(gtx C, pop *ui.Theme) D { return r.body(pop, n, id)(gtx) }
	panel := th.DropdownPanel(r.layer(), s, width, content)
	panel.AlignEnd = p.AlignEnd
	return panel.Layout(gtx, r.extra(th, p.Trigger, id+"/trigger"))
}

// dialogState holds the dialog shut from the frame the person dismissed it
// until the extension sends Open again, so it does not hang open for the
// round trip to the extension.
type dialogState struct {
	dialog    ui.DialogState
	open      synced[bool]
	dismissed bool
}

// drawDialog lays the dialog over the box it is drawn in and takes no room
// there: put it first in the surface's root box, where that box is the whole
// surface and the dialog claims Escape ahead of what follows.
func drawDialog(r *Renderer, _ C, th *ui.Theme, n extension.Node, id string) D {
	r.dialogs = append(r.dialogs, func(gtx C) D { return layoutDialog(r, gtx, th, n, id) })
	return D{}
}

func layoutDialog(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.DialogProps](n)
	s := stateOf[dialogState](r, id)
	if s.open.take(p.Open) {
		s.dismissed = false
	}
	pop := th.On(th.Popover)
	d := pop.Dialog(&s.dialog, p.Open && !s.dismissed, p.Title, p.Description)
	if p.MaxWidth > 0 {
		d.MaxWidth = unit.Dp(p.MaxWidth)
	}
	d.HideClose, d.Top, d.Scrim, d.Root = p.HideClose, float32(p.Top), p.Scrim, r.layer()
	switch {
	case p.Raw:
		d.Raw = r.body(pop, n, id)
	case len(n.Children) > 0:
		d.Body = r.body(pop, n, id)
	}
	d.Actions = r.nodes(pop, p.Actions, id+"/actions")
	if d.Layout(gtx) {
		s.dismissed = true
		r.fire(p.OnClose, extension.EventData{})
	}
	return D{}
}
