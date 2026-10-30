package main

import (
	"fmt"
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/x/richtext"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"

	"github.com/lichdotdev/lich/native/ui"
)

type markdownList struct {
	items []markdownListItem
}

type markdownListItem struct {
	marker markdownCell
	// task is a task list item's box, drawn in the marker's place; nil
	// for any other item.
	task   *widget.Bool
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
		if box := taskCheckBox(item); box != nil {
			entry.task = &widget.Bool{Value: box.IsChecked}
		}
		for child := item.FirstChild(); child != nil; child = child.NextSibling() {
			if block, ok := renderBlock(th, source, child); ok {
				entry.blocks = append(entry.blocks, block)
			}
		}
		out.items = append(out.items, entry)
	}
	return out
}

// taskCheckBox is the [ ] or [x] that opens item's first line, if it opens
// with one.
func taskCheckBox(item ast.Node) *extast.TaskCheckBox {
	first := item.FirstChild()
	if first == nil {
		return nil
	}
	box, _ := first.FirstChild().(*extast.TaskCheckBox)
	return box
}

func (l *markdownList) layout(gtx C, th *ui.Theme, open func(string)) D {
	markerWidth := 0
	for i := range l.items {
		markerWidth = max(markerWidth, l.items[i].markerWidth(gtx, th))
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
			if item.task != nil {
				return item.layoutTask(gtx, th)
			}
			return layout.E.Layout(gtx, func(gtx C) D { return item.marker.layout(gtx, th, nil) })
		}),
		layout.Rigid(layout.Spacer{Width: ui.Space(2)}.Layout),
		layout.Flexed(1, func(gtx C) D { return item.layoutBody(gtx, th, open) }),
	)
}

// markerWidth is the width the item's marker, or its box, takes.
func (item *markdownListItem) markerWidth(gtx C, th *ui.Theme) int {
	if item.task == nil {
		return measureCell(gtx, th, &item.marker)
	}
	call := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	dims := item.layoutTask(gtx, th)
	call.Stop()
	return dims.Size.X
}

// layoutTask is the item's box, read-only as GitHub draws it, centred on the
// first line.
func (item *markdownListItem) layoutTask(gtx C, th *ui.Theme) D {
	box := th.Checkbox(item.task)
	box.Disabled = true
	line := int(float32(gtx.Sp(markdownBodySize)) * markdownLineHeightScale)
	return layout.E.Layout(gtx, func(gtx C) D {
		call := op.Record(gtx.Ops)
		dims := box.Layout(gtx)
		c := call.Stop()
		defer op.Offset(image.Pt(0, (line-dims.Size.Y)/2)).Push(gtx.Ops).Pop()
		c.Add(gtx.Ops)
		return D{Size: image.Pt(dims.Size.X, line)}
	})
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
