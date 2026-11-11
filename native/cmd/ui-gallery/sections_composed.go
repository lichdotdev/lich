package main

import (
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/ui"
)

// composedState is the live state of the components the product composes
// from the kit's primitives.
type composedState struct {
	emptyActions [4]widget.Clickable
}

func (g *gallery) composedSection(gtx C, th *ui.Theme) D {
	return stack(4, g.emptyStates(th)...)(gtx)
}

func (g *gallery) emptyStates(th *ui.Theme) []layout.Widget {
	action := func(i int, text string) layout.Widget {
		return th.Button(&g.emptyActions[i], text).Layout
	}
	return []layout.Widget{
		caption(th, "EmptyState: glyph, description, action"),
		height(200, th.EmptyState(g.icon("folder-open"), "No project open", "Open a folder to get started.", action(0, "Open project")).Layout),
		caption(th, "EmptyState: title and action only"),
		height(160, th.EmptyState(nil, "Nothing here", "", action(1, "Reload")).Layout),
		caption(th, "InlineEmptyState: description and action"),
		th.InlineEmptyState("This file cannot be previewed", "It is a binary format.", action(2, "Open")).Layout,
		caption(th, "InlineEmptyState: action only"),
		th.InlineEmptyState("No preview", "", action(3, "Reveal")).Layout,
	}
}
