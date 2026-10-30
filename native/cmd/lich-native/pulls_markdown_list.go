package main

import (
	"fmt"
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/x/richtext"
	"github.com/yuin/goldmark/ast"

	"github.com/lichdotdev/lich/native/ui"
)

type markdownList struct {
	items []markdownListItem
}

type markdownListItem struct {
	marker markdownCell
	blocks []markdownBlock
}

func renderList(th *ui.Theme, source []byte, list *ast.List) *markdownList {
	out := &markdownList{}
	index := list.Start
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		marker := "•"
		if list.IsOrdered() {
			marker = fmt.Sprintf("%d.", index)
			index++
		}
		entry := markdownListItem{marker: markdownCell{spans: []richtext.SpanStyle{{
			Font: font.Font{Typeface: th.Sans}, Size: markdownBodySize, Color: markdownProseColor(th), Content: marker,
		}}}}
		for child := item.FirstChild(); child != nil; child = child.NextSibling() {
			if block, ok := renderBlock(th, source, child); ok {
				entry.blocks = append(entry.blocks, block)
			}
		}
		out.items = append(out.items, entry)
	}
	return out
}

func (l *markdownList) layout(gtx C, th *ui.Theme, open func(string)) D {
	markerWidth := 0
	for i := range l.items {
		markerWidth = max(markerWidth, measureCell(gtx, th, &l.items[i].marker))
	}
	children := make([]layout.FlexChild, 0, len(l.items))
	for i := range l.items {
		item := &l.items[i]
		children = append(children, layout.Rigid(func(gtx C) D {
			return item.layout(gtx, th, open, markerWidth)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (item *markdownListItem) layout(gtx C, th *ui.Theme, open func(string), markerWidth int) D {
	return layout.Flex{Alignment: layout.Start}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = markerWidth, markerWidth
			return layout.E.Layout(gtx, func(gtx C) D { return item.marker.layout(gtx, th, nil) })
		}),
		layout.Rigid(layout.Spacer{Width: ui.Space(2)}.Layout),
		layout.Flexed(1, func(gtx C) D { return item.layoutBody(gtx, th, open) }),
	)
}

func (item *markdownListItem) layoutBody(gtx C, th *ui.Theme, open func(string)) D {
	children := make([]layout.FlexChild, 0, len(item.blocks)*2)
	for i, block := range item.blocks {
		if i > 0 {
			children = append(children, ui.VGap(2))
		}
		children = append(children, layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min = image.Point{}
			return block.layout(gtx, th, open)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
