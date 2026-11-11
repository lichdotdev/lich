package main

import (
	"fmt"
	"image"

	"gioui.org/layout"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/codeview"
	"github.com/lichdotdev/lich/native/ui/diffview"
	"github.com/lichdotdev/lich/native/ui/filetree"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

const richMarkdown = "## Markdown\n\nA paragraph with **bold**, *italics*, `inline code` and a [link](https://example.com).\n\n" +
	"- [x] a task done\n- [ ] a task to do\n  - and a nested item\n\n| Viewer | Input |\n|---|---|\n| Markdown | text |\n| Code | source and filename |\n"

// richState holds what the rich content viewers keep between frames.
type richState struct {
	md      markdown.Cache
	code    *codeview.View
	codeErr error

	diff      *diffview.View
	diffBlock diffview.Blocks
	diffPatch godemirror.FilePatch
	diffErr   error

	tree       filetree.Rows
	treeClosed map[string]bool
	treeOpen   string
}

func (g *gallery) markdownSection(gtx C, th *ui.Theme) D {
	if g.rich.md == nil {
		g.rich.md = markdown.Cache{}
	}
	return g.rich.md.Layout(gtx, th, "gallery", richMarkdown, nil)
}

const richSource = `// Package demo is what the code view draws in the gallery.
package demo

import "fmt"

// Greet says hello to name.
func Greet(name string) string {
	return fmt.Sprintf("hello, %s", name)
}
`

const richCodeHeight = 240

func (g *gallery) codeSection(gtx C, th *ui.Theme) D {
	r := &g.rich
	if r.code == nil && r.codeErr == nil {
		r.code, r.codeErr = codeview.New(fonts.SystemMono, 13)
		if r.codeErr == nil {
			r.code.SetSource(codeview.Source{Filename: "demo.go", Text: richSource})
		}
	}
	if r.codeErr != nil {
		return th.Text(ui.TextSM, r.codeErr.Error()).Muted().Layout(gtx)
	}
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(richCodeHeight)))
	dims, err := r.code.Layout(gtx, th, godemirror.DarkTheme)
	if err != nil {
		r.codeErr = err
	}
	return dims
}

const richPatch = `diff --git a/demo.go b/demo.go
--- a/demo.go
+++ b/demo.go
@@ -1,6 +1,6 @@
 package demo

-// Greet says hello.
-func Greet(name string) string {
+// Hello says hello.
+func Hello(name string) string {
 	return "hello, " + name
 }
`

var richDiffLabels = diffview.Labels{
	Expand: func(count int) string { return fmt.Sprintf("Expand %d lines", count) },
	Revert: func() string { return "Revert" },
}

func (g *gallery) diffSection(gtx C, th *ui.Theme) D {
	r := &g.rich
	if r.diff == nil && r.diffErr == nil {
		r.diffErr = r.buildDiff()
	}
	if r.diffErr != nil {
		return th.Text(ui.TextSM, r.diffErr.Error()).Muted().Layout(gtx)
	}
	dims, _, err := r.diff.Layout(gtx, diffview.Frame{
		Theme: th, Code: godemirror.DarkTheme, Patch: r.diffPatch,
		Height: gtx.Dp(richCodeHeight), Blocks: &r.diffBlock,
	})
	r.diffErr = err
	return dims
}

func (r *richState) buildDiff() error {
	files, err := godemirror.ParsePatch(richPatch)
	if err != nil {
		return err
	}
	r.diffPatch = files[0]
	r.diff, err = diffview.New(fonts.SystemMono, 13, diffview.Options{Buttons: diffview.NewButtons(richDiffLabels)})
	return err
}

var richPaths = []string{"native/ui/markdown/markdown.go", "native/ui/codeview/view.go", "native/ui/tabs.go", "README.md"}

func (g *gallery) treeSection(gtx C, th *ui.Theme) D {
	r := &g.rich
	if r.tree.Tree == nil {
		r.tree.List.Axis = layout.Vertical
		r.tree.Tree = filetree.Build(richPaths)
		r.tree.Stats = filetree.WithDirStats(r.tree.Tree, map[string]filetree.Delta{"native/ui/tabs.go": {Added: 12, Deleted: 3}})
		r.treeClosed = map[string]bool{}
	}
	for _, n := range r.tree.Clicked(gtx) {
		if n.Dir {
			r.treeClosed[n.Path] = !r.treeClosed[n.Path]
			continue
		}
		r.treeOpen = n.Path
	}
	rows := filetree.VisibleRows(r.tree.Tree, func(path string) bool { return !r.treeClosed[path] })
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(richCodeHeight)))
	return r.tree.Layout(gtx, th, rows, r.treeOpen)
}
