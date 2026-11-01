package main

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// A tab's skeleton row takes a loaded row's room, so nothing moves when the
// pull request arrives.
func TestTabSkeletonRowsAreLoadedRowsHeight(t *testing.T) {
	th := rowMetaTheme(t)
	gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(1200, 800)}}
	commit := backend.PRCommit{OID: "0123456789abcdef", Headline: "fix: a thing", Name: "ana", Date: rowMetaNow.Format("2006-01-02T15:04:05Z")}
	if loaded, got := commitRow(gtx, th, commit, new(widget.Clickable), false, false, rowMetaNow).Size.Y, skeletonCommitRow(gtx, th, 0).Size.Y; got != loaded {
		t.Errorf("commit skeleton row is %dpx tall, a loaded one %dpx", got, loaded)
	}
	d := backend.PRDetail{CheckRuns: []api.CheckItem{{Name: "test", Description: "Go tests", State: "passed"}}}
	var v pullView
	v.checkClicks = map[string]*widget.Clickable{}
	var act pullsAction
	if loaded, got := v.checkRows(gtx, th, d, &act)[0](gtx).Size.Y, skeletonCheckRow(gtx, th, 0).Size.Y; got != loaded {
		t.Errorf("check skeleton row is %dpx tall, a loaded one %dpx", got, loaded)
	}
}
