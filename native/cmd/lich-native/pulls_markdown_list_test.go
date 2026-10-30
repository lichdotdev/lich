package main

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

func TestMarkdownListsKeepNestedItemsAndLinks(t *testing.T) {
	blocks := renderBlocks(markdownTheme(), "9. First item with a [link](https://example.com)\n   - Nested item\n   - Another item\n10. Second item")
	if len(blocks) != 1 || blocks[0].list == nil {
		t.Fatal("expected an ordered list")
	}
	items := blocks[0].list.items
	if len(items) != 2 || items[0].marker.spans[0].Content != "9." || items[1].marker.spans[0].Content != "10." {
		t.Fatalf("ordered list markers or items are incorrect: %+v", items)
	}
	if len(items[0].blocks) != 2 || items[0].blocks[1].list == nil || len(items[0].blocks[1].list.items) != 2 {
		t.Fatal("nested list was not retained inside its parent item")
	}
	for _, span := range items[0].blocks[0].prose.spans {
		if span.Interactive && span.Content == "link" {
			return
		}
	}
	t.Fatal("link in list item was lost")
}

func TestMarkdownListWrapsInsideMarkerColumn(t *testing.T) {
	th := markdownTheme()
	list := renderBlocks(th, "- A long list item that needs several lines at this narrow width.")[0].list
	gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(180, 800)}}
	item := &list.items[0]
	markerWidth := measureCell(gtx, th, &item.marker)
	body := gtx
	body.Constraints.Max.X -= markerWidth + gtx.Dp(ui.Space(2))
	bodySize := item.layoutBody(body, th, nil).Size
	got := list.layout(gtx, th, nil).Size
	if bodySize.Y <= gtx.Sp(markdownBodySize)*2 {
		t.Fatalf("test item did not wrap: height %d", bodySize.Y)
	}
	if got.Y != bodySize.Y || got.X < bodySize.X+markerWidth+gtx.Dp(ui.Space(2)) {
		t.Errorf("list size %v does not include marker column and wrapped body %v", got, bodySize)
	}
}

func TestMarkdownTaskItemsDrawABoxInTheMarkersPlace(t *testing.T) {
	list := renderBlocks(markdownTheme(), "- [x] done\n- [ ] todo\n- plain")[0].list
	if len(list.items) != 3 {
		t.Fatalf("got %d items, want 3", len(list.items))
	}
	for i, want := range []struct {
		task, checked bool
		text          string
	}{{true, true, "done"}, {true, false, "todo"}, {false, false, "plain"}} {
		item := list.items[i]
		if (item.task != nil) != want.task || (item.task != nil && item.task.Value != want.checked) {
			t.Errorf("item %d: task %v, want task %v checked %v", i, item.task, want.task, want.checked)
		}
		var text string
		for _, span := range item.blocks[0].prose.spans {
			text += span.Content
		}
		if text != want.text {
			t.Errorf("item %d reads %q, want %q", i, text, want.text)
		}
	}
	gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(400, 800)}}
	if w := list.items[0].markerWidth(gtx, markdownTheme()); w < gtx.Dp(ui.Space(4)) {
		t.Errorf("marker column %dpx is narrower than the box", w)
	}
}
