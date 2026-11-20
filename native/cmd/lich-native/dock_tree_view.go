package main

import (
	"maps"

	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/filetree"
)

// buildRows is the tree for data, colored by the git status's changes,
// rebuilt only when one of them changed: a big checkout is thousands of paths
// to nest every frame.
func (c *codeTab) buildRows(data *treeData) {
	if data == c.rowsFrom && maps.Equal(c.changes, c.rowsWith) {
		return
	}
	c.rowsFrom, c.rowsWith = data, c.changes
	c.rows.Tree = filetree.Build(data.listing.Files)
	c.rows.Stats = filetree.WithDirStats(c.rows.Tree, data.stats)
	c.rows.States = filetree.WithDirStates(c.rows.Tree, treeStates(c.changes))
}

// treeBody is TreeBody: the states the tree reads as, the rows, and the
// footnote a listing that left something out owes the reader.
func (c *codeTab) treeBody(gtx C, th *ui.Theme) D {
	data, loading, err := c.tree.get()
	switch {
	case err != nil:
		return th.Notice(i18n.T("dock.filesPanel.readFailed")).Layout(gtx)
	case loading || data == nil:
		return th.Notice(i18n.T("dock.filesPanel.loading")).Layout(gtx)
	}
	c.buildRows(data)
	c.treeClicks(gtx)
	rows := filetree.VisibleRows(c.rows.Tree, func(path string) bool { return c.toggled[path] })
	note := treeFootnote(data.listing.Cut, data.listing.Hidden)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			if len(rows) == 0 {
				return th.Notice(i18n.T("dock.filesPanel.empty")).Layout(gtx)
			}
			return c.treeList(gtx, th, rows)
		}),
		layout.Rigid(func(gtx C) D {
			if note == "" {
				return D{}
			}
			return th.Footnote(note).Layout(gtx)
		}),
	)
}

// treeClicks folds a folder or opens a file, before the rows are drawn.
func (c *codeTab) treeClicks(gtx C) {
	for _, node := range c.rows.Clicked(gtx) {
		if node.Dir {
			c.toggled[node.Path] = !c.toggled[node.Path]
			continue
		}
		c.open(node.Path, 0)
	}
}

func (c *codeTab) treeList(gtx C, th *ui.Theme, rows []filetree.Row) D {
	return c.rows.Layout(gtx, th, rows, c.selected)
}
