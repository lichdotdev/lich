package extrender

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/ui"
)

const samplePatch = `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +1,3 @@
 package main
-var x = 1
+var x = 2
 func main() {}
`

// recorder is a renderer whose events land in got, drawing code as the
// window does.
func recorder(got *[]extension.EventData) *Renderer {
	r := New(nil, func(e extension.EventData) { *got = append(*got, e) })
	r.SetCode(Code{Theme: godemirror.DarkTheme, Family: fonts.SystemMono, Size: 13})
	return r
}

// tap presses and releases the primary button at x, y and draws the frame
// that reads it.
func tap(r *Renderer, router *input.Router, tree extension.Node, x, y float32) {
	frame(r, router, tree)
	router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(x, y)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(x, y)},
	)
	frame(r, router, tree)
}

func onlyEvent(t *testing.T, got []extension.EventData, want extension.EventData) {
	t.Helper()
	if len(got) != 1 || got[0].Handler != want.Handler || got[0].Text != want.Text {
		t.Fatalf("events = %+v, want only %+v", got, want)
	}
}

func TestEveryViewerDrawsWithEveryPropSet(t *testing.T) {
	block := extension.Label.New(extension.LabelProps{Text: "note"})
	tree := extension.Box.New(extension.BoxProps{},
		extension.Markdown.New(extension.MarkdownProps{Text: "# Title\n\n[link](https://example.com)", OnOpen: "open"}),
		extension.Box.New(extension.BoxProps{Height: 80}, extension.CodeView.New(extension.CodeViewProps{
			Filename: "main.go", Text: "package main\n\nfunc main() {}\n", Size: 12,
			Blocks: []extension.CodeBlock{{After: 0, Content: block}},
		})),
		extension.DiffView.New(extension.DiffViewProps{
			Patch: samplePatch, Split: true, Height: 60, Size: 12,
			Blocks: []extension.DiffBlock{{Key: "a", Side: extension.DiffOld, Line: 2, Content: block}, {Key: "b", Line: 2, Content: block}},
		}),
		extension.FileTree.New(extension.FileTreeProps{
			Paths: []string{"a.go", "dir/b.go"}, Stats: map[string]extension.FileStat{"a.go": {Added: 1, Deleted: 2}},
			Selected: "a.go", ClosedPaths: []string{"dir"}, OnSelect: "select", OnToggle: "toggle",
		}),
	)
	var got []extension.EventData
	r := recorder(&got)
	router := new(input.Router)
	frame(r, router, tree)
	frame(r, router, tree)
	r.SetCode(Code{Theme: godemirror.LightTheme, Family: fonts.SystemMono, Size: 15})
	frame(r, router, tree)
}

func TestADiffThatDoesNotParseDrawsEmpty(t *testing.T) {
	if f := firstFile("not a patch"); f.Path != "" {
		t.Errorf("firstFile of no patch = %+v, want none", f)
	}
	if f := firstFile(samplePatch); f.Path != "main.go" {
		t.Errorf("firstFile of a patch = %q, want main.go", f.Path)
	}
	var got []extension.EventData
	frame(recorder(&got), new(input.Router), extension.DiffView.New(extension.DiffViewProps{Patch: "@@ nonsense", Height: 40}))
}

func TestAViewWhoseFontDoesNotLoadDrawsNothing(t *testing.T) {
	r := New(nil, func(extension.EventData) {})
	r.SetCode(Code{Theme: godemirror.DarkTheme, Family: "no-such-family-\x00", Size: 13})
	tree := extension.Box.New(extension.BoxProps{},
		extension.CodeView.New(extension.CodeViewProps{Text: "x"}),
		extension.DiffView.New(extension.DiffViewProps{Patch: samplePatch, Height: 40}))
	router := new(input.Router)
	frame(r, router, tree)
	frame(r, router, tree)
}

// frameIn is frame in one theme across frames: Markdown parses its text
// again for another shaper, which a click spanning two frames would not
// survive.
func frameIn(th *ui.Theme, r *Renderer, router *input.Router, tree extension.Node) {
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(400, 300)), Source: router.Source()}
	r.Layout(gtx, th, tree)
	router.Frame(&ops)
}

func TestALinkClickedInMarkdownSendsItsURL(t *testing.T) {
	var got []extension.EventData
	r := recorder(&got)
	tree := extension.Markdown.New(extension.MarkdownProps{Text: "[docs](https://example.com/docs)", OnOpen: "open"})
	router, th := new(input.Router), testTheme()
	frameIn(th, r, router, tree)
	router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(8, 8)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(8, 8)},
	)
	frameIn(th, r, router, tree)
	onlyEvent(t, got, extension.EventData{Handler: "open", Text: "https://example.com/docs"})
}

func TestAFileClickedInTheTreeSendsItsPath(t *testing.T) {
	var got []extension.EventData
	r := recorder(&got)
	tree := extension.FileTree.New(extension.FileTreeProps{Paths: []string{"main.go"}, OnSelect: "select", OnToggle: "toggle"})
	tap(r, new(input.Router), tree, 40, 12)
	onlyEvent(t, got, extension.EventData{Handler: "select", Text: "main.go"})
}

func TestAFolderClickedInTheTreeSendsItsPath(t *testing.T) {
	var got []extension.EventData
	r := recorder(&got)
	tree := extension.FileTree.New(extension.FileTreeProps{Paths: []string{"dir/a.go", "dir/b.go"}, OnSelect: "select", OnToggle: "toggle"})
	tap(r, new(input.Router), tree, 40, 12)
	onlyEvent(t, got, extension.EventData{Handler: "toggle", Text: "dir"})
}
