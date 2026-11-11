package extrender

import (
	"maps"
	"slices"
	"strconv"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/codeview"
	"github.com/lichdotdev/lich/native/ui/diffview"
	"github.com/lichdotdev/lich/native/ui/filetree"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

func init() {
	register(map[string]drawFunc{
		extension.Markdown.Name: drawMarkdown,
		extension.CodeView.Name: drawCodeView,
		extension.DiffView.Name: drawDiffView,
		extension.FileTree.Name: drawFileTree,
	})
}

// Code is how the window draws code: its theme's code palette and the
// editor's font family and size, which a view's own Size overrides.
type Code struct {
	Theme  godemirror.Theme
	Family string
	Size   unit.Sp
}

// SetCode is the code palette and font the code and diff views draw with; the
// window calls it on every change of theme or editor font.
func (r *Renderer) SetCode(c Code) { r.code = c }

func (r *Renderer) codeSize(sp float64) unit.Sp { return textSize(sp, r.code.Size) }

type markdownState struct{ cache markdown.Cache }

func drawMarkdown(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.MarkdownProps](n)
	s := stateOf[markdownState](r, id)
	if s.cache == nil {
		s.cache = markdown.Cache{}
	}
	open := func(url string) { r.fire(p.OnOpen, extension.EventData{Text: url}) }
	return s.cache.Layout(gtx, th, id, p.Text, open)
}

// codeState is a code view and what it last showed. blocks are the widgets
// its source hangs under lines, swapped every frame so a block draws the tree
// as it is now while the source is only set again when it changed.
type codeState struct {
	view   *codeview.View
	failed bool
	shown  [2]string
	afters []int
	blocks []layout.Widget
}

func drawCodeView(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.CodeViewProps](n)
	s := stateOf[codeState](r, id)
	if !s.ready(r.code.Family, r.codeSize(p.Size)) {
		return D{}
	}
	s.blocks = s.blocks[:0]
	afters := make([]int, len(p.Blocks))
	for i, b := range p.Blocks {
		s.blocks = append(s.blocks, r.slot(th, b.Content, id+"/b"+strconv.Itoa(i)))
		afters[i] = b.After
	}
	shown := [2]string{p.Filename, p.Text}
	if shown != s.shown || !slices.Equal(afters, s.afters) {
		s.view.UpdateSource(codeview.Source{Filename: p.Filename, Text: p.Text, Blocks: s.blockWidgets(afters)})
		s.shown, s.afters = shown, afters
	}
	gtx.Constraints.Min = gtx.Constraints.Max
	d, _ := s.view.Layout(gtx, th, r.code.Theme)
	return d
}

// ready builds the view on its first frame and swaps its font on a change;
// a font that does not load leaves the view out rather than retrying each
// frame.
func (s *codeState) ready(family string, size unit.Sp) bool {
	if s.failed {
		return false
	}
	if s.view == nil {
		v, err := codeview.New(family, size)
		s.view, s.failed = v, err != nil
		return !s.failed
	}
	changed, err := s.view.SetFont(family, size)
	if err != nil {
		s.failed = true
		return false
	}
	if changed {
		s.shown, s.afters = [2]string{}, nil
	}
	return true
}

func (s *codeState) blockWidgets(afters []int) []godemirror.BlockWidget {
	out := make([]godemirror.BlockWidget, len(afters))
	for i, after := range afters {
		out[i] = godemirror.BlockWidget{After: after, Widget: func(gtx C) D {
			if i >= len(s.blocks) {
				return D{}
			}
			return s.blocks[i](gtx)
		}}
	}
	return out
}

// diffState is a diff view, the font it was built in and the patch it shows,
// parsed once per change of its text.
type diffState struct {
	view    *diffview.View
	family  string
	size    unit.Sp
	failed  bool
	text    string
	patch   godemirror.FilePatch
	blocks  diffview.Blocks
	buttons *diffview.Buttons
}

// diffLabels are the words on a diff's buttons, the window's own.
var diffLabels = diffview.Labels{
	Expand: func(count int) string { return i18n.T("git.codemirror.expand", i18n.Args{"count": count}) },
	Revert: func() string { return i18n.T("git.codemirror.revert") },
}

func drawDiffView(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.DiffViewProps](n)
	s := stateOf[diffState](r, id)
	if !s.ready(r.code.Family, r.codeSize(p.Size)) {
		return D{}
	}
	if p.Patch != s.text {
		s.text, s.patch = p.Patch, firstFile(p.Patch)
	}
	items := make([]diffview.Block, len(p.Blocks))
	for i, b := range p.Blocks {
		at := godemirror.LineRef{Side: godemirror.NewSide, Line: b.Line}
		if b.Side == extension.DiffOld {
			at.Side = godemirror.OldSide
		}
		items[i] = diffview.Block{Key: b.Key, At: at, Widget: r.slot(th, b.Content, id+"/k:"+b.Key)}
	}
	d, changed, _ := s.view.Layout(gtx, diffview.Frame{
		Theme: th, Code: r.code.Theme, Patch: s.patch, Split: p.Split,
		Height: gtx.Dp(unit.Dp(p.Height)), Blocks: &s.blocks, Items: items,
	})
	if changed {
		gtx.Execute(op.InvalidateCmd{})
	}
	return d
}

// ready builds the view, again on a change of font: a diff view cannot swap
// its font. The diff offers neither expand nor revert, which need the file
// and the checkout an extension's patch does not come with.
func (s *diffState) ready(family string, size unit.Sp) bool {
	if s.view != nil && family == s.family && size == s.size {
		return true
	}
	if s.failed && family == s.family && size == s.size {
		return false
	}
	if s.buttons == nil {
		s.buttons = diffview.NewButtons(diffLabels)
	}
	v, err := diffview.New(family, size, diffview.Options{Buttons: s.buttons})
	s.view, s.family, s.size, s.failed = v, family, size, err != nil
	return !s.failed
}

// firstFile is the first file of a unified diff, an empty one for a patch
// that names none or does not parse.
func firstFile(patch string) godemirror.FilePatch {
	files, err := godemirror.ParsePatch(patch)
	if err != nil || len(files) == 0 {
		return godemirror.FilePatch{}
	}
	return files[0]
}

// treeState is the tree's rows and the paths and stats they were built from.
type treeState struct {
	rows  filetree.Rows
	paths []string
	stats map[string]extension.FileStat
}

func drawFileTree(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.FileTreeProps](n)
	s := stateOf[treeState](r, id)
	s.rows.List.Axis = layout.Vertical
	for _, node := range s.rows.Clicked(gtx) {
		if node.Dir {
			r.fire(p.OnToggle, extension.EventData{Text: node.Path})
			continue
		}
		r.fire(p.OnSelect, extension.EventData{Text: node.Path})
	}
	s.build(p)
	closed := map[string]bool{}
	for _, path := range p.ClosedPaths {
		closed[path] = true
	}
	rows := filetree.VisibleRows(s.rows.Tree, func(path string) bool { return !closed[path] })
	return s.rows.Layout(gtx, th, rows, p.Selected)
}

// build makes the tree again only when its paths or stats changed.
func (s *treeState) build(p extension.FileTreeProps) {
	if s.rows.Tree != nil && slices.Equal(p.Paths, s.paths) && maps.Equal(p.Stats, s.stats) {
		return
	}
	s.paths, s.stats = slices.Clone(p.Paths), maps.Clone(p.Stats)
	s.rows.Tree = filetree.Build(p.Paths)
	deltas := make(map[string]filetree.Delta, len(p.Stats))
	for path, st := range p.Stats {
		deltas[path] = filetree.Delta{Added: st.Added, Deleted: st.Deleted}
	}
	s.rows.Stats = filetree.WithDirStats(s.rows.Tree, deltas)
}

// slot is n, a node held in a prop rather than among the children, drawn
// under id.
func (r *Renderer) slot(th *ui.Theme, n extension.Node, id string) layout.Widget {
	return func(gtx C) D { return r.node(gtx, th, n, id) }
}
