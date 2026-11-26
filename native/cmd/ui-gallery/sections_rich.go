package main

import (
	"fmt"
	"image"
	"math"
	"strings"

	"gioui.org/layout"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/codeview"
	"github.com/lichdotdev/lich/native/ui/diffview"
	"github.com/lichdotdev/lich/native/ui/filetree"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

var richMarkdown = "## Markdown\n\nA paragraph with **bold**, *italics*, `inline code` and a [link](https://example.com).\n\n" +
	"Paths in code wear their icon: `app.go`, `main.ts`, `README.md`, `config.json`, `notes.zzz` and `docs/`.\n\n" +
	"- [x] a task done\n- [ ] a task to do\n  - and a nested item\n\n| Viewer | Input |\n|---|---|\n| Markdown | text |\n| Code | source and filename |\n\n" +
	"```go\n" + richSource + "```\n\n" +
	"> [!NOTE]\n> Sessions inherit the login shell's `PATH`.\n\n> [!TIP]\n> `task --list` shows every task.\n\n" +
	"> [!IMPORTANT]\n> The hook runs before the spawn.\n\n> [!WARNING]\n> The migration drops `legacy_id`.\n\n> [!CAUTION]\n> `git reset --hard` discards the other sessions' changes.\n\n" +
	"```diff\n--- a/internal/relay/wait.go\n+++ b/internal/relay/wait.go\n@@ -41,4 +41,4 @@\n \tr.mu.Lock()\n-\tdeadline := time.Now().Add(30 * time.Second)\n+\tdeadline, _ := ctx.Deadline()\n \tfor !r.done(t) {\n```\n\n" +
	"```\n" + strings.Repeat("a line of a long log\n", 30) + "```\n"

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

const richDocument = "---\nname: lich-ux-declutter\ndescription: Cut a native window screen down to what a visit needs, from a screenshot of it. Use before calling any UI change done in `native/`, after adding or moving a control, a label, a row or a dialog, and whenever you look at a shot of your own UI.\n---\n\n" +
	"# Declutter a screen\n\n`native/DESIGN.md` says how to draw. This skill says what to cut.\n\n## 1. Shoot it\n\nTake a `-shot` of the screen in its common state.\n\n---\n\n" +
	"## 2. Inventory\n\nPut each one through four questions:\n\n1. **Remove?** Does it earn its place?\n2. **Merge?** Is the same fact shown twice?\n3. **Shrink?** Can a value sit at the end of its row?\n"

func (g *gallery) markdownDocumentSection(gtx C, th *ui.Theme) D {
	if g.rich.md == nil {
		g.rich.md = markdown.Cache{}
	}
	return g.rich.md.LayoutDocumentWindow(gtx, th, "gallery-document", richDocument, nil, markdown.Window{Top: math.MinInt, Bottom: math.MaxInt})
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

func init() {
	alerts := map[markdown.Alert]string{markdown.AlertNote: "Note", markdown.AlertTip: "Tip", markdown.AlertImportant: "Important", markdown.AlertWarning: "Warning", markdown.AlertCaution: "Caution"}
	markdown.Words = markdown.Labels{
		Alert: func(a markdown.Alert) string { return alerts[a] },
		More:  func(count int) string { return fmt.Sprintf("Show %d more lines", count) },
		Less:  func() string { return "Show less" },
	}
}

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

var richPaths = []string{"native/ui/markdown/markdown.go", "native/ui/codeview/view.go", "native/ui/tabs.go", "native/ui/pill.go", "native/ui/merge.go", "README.md", "NOTES.md"}

func (g *gallery) treeSection(gtx C, th *ui.Theme) D {
	r := &g.rich
	if r.tree.Tree == nil {
		r.tree.List.Axis = layout.Vertical
		r.tree.Tree = filetree.Build(richPaths)
		r.tree.Stats = filetree.WithDirStats(r.tree.Tree, map[string]filetree.Delta{"native/ui/tabs.go": {Added: 12, Deleted: 3}, "native/ui/pill.go": {Added: 40}, "native/ui/merge.go": {Added: 9, Deleted: 2}, "NOTES.md": {Added: 1}})
		r.tree.States = filetree.WithDirStates(r.tree.Tree, map[string]filetree.State{"native/ui/tabs.go": filetree.Modified, "native/ui/pill.go": filetree.Added, "native/ui/merge.go": filetree.Conflict, "NOTES.md": filetree.Untracked})
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
