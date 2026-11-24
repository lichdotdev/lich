package main

// The stage editor: a checkout's file on the whole stage, read and edited in
// the conflict resolver's godemirror editor. There is one per checkout, kept
// for as long as its node is in the tree. An image or a PDF shows read-only,
// and a Markdown file can be read rendered.

import (
	"context"
	"fmt"
	"image"
	"strconv"
	"time"
	"unicode/utf8"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/codeview"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

// editorLoad is a read of the file and, the first time, the editor and the
// disk change's diff, built off the UI goroutine since both load fonts.
type editorLoad struct {
	text string
	ed   *godemirror.Editor
	diff *godemirror.DiffView
}

type fileEditor struct {
	client     *backend.Client
	invalidate func()
	family     string
	size       unit.Sp
	checkout   string
	// dir is the folder outside the checkout the open file is read from, for
	// an agent's artifact, which the editor only shows: "" for a checkout's
	// file. writable is set on one the person edits there, a memory or
	// subagent file of the chat's /memory and /agents panels.
	dir      string
	writable bool
	// onDirty hears the editor start or stop holding unsaved changes, onSaved
	// a save that landed, and onQuit Vim's :q, discard set when its changes
	// are to be dropped.
	onDirty, onSaved func()
	onQuit           func(discard bool)

	rel   string
	kind  blobKind
	opens int
	// line and endLine are the lines an open asked for, 1-based, 0 for the
	// top; land selects them once the file is in the editor. note is what
	// the agent said about them.
	line, endLine int
	land          bool
	note          agentNote

	file remote[editorLoad]
	ed   *godemirror.Editor
	diff *godemirror.DiffView
	// loaded is the open whose read is in the editor; base is the text it
	// read or last saved, in the editor's "\n" lines, and sep the break the
	// file is written back with. gen counts the changes to the text or to
	// base, which the margin's marks follow.
	loaded       int
	base, sep    string
	dirty        bool
	gen, decoGen int
	// lines is the text weighed against base as of gen, decoKey what the
	// decorations were last drawn for, and blame the cursor line's blame.
	lines   lineDiff
	decoKey string
	blame   editorBlame
	focus   bool
	disk    diskWatch
	saving  editorSaving
	vim     vimBuffer
	// editedAt is the last edit, which autoSave counts from on now's clock.
	editedAt time.Time
	now      func() time.Time
	image    *blobView
	exits    fileExits
	scroll   widget.List
	md       codeMarkdown
	menu     ui.MenuState
	composer commentComposer
	env      editorEnv
	th       *ui.Theme
}

// editorEnv is what the editor reads of the window each frame.
type editorEnv struct {
	root     *ui.Root
	code     godemirror.Theme
	inject   func(string) bool
	comments *commentStore
	notify   func(ui.Toast)
	vim      *vimHost
}

func newFileEditor(client *backend.Client, invalidate func(), family string, size unit.Sp, checkout string) *fileEditor {
	f := &fileEditor{client: client, invalidate: invalidate, family: family, size: size, checkout: checkout,
		image: newBlobView(invalidate), onDirty: func() {}, onSaved: func() {}, onQuit: func(bool) {}, now: time.Now}
	f.file.invalidate, f.disk.read.invalidate, f.blame.read.invalidate = invalidate, invalidate, invalidate
	f.scroll.Axis = layout.Vertical
	f.md.rendered = markdown.Cache{}
	f.md.scroll.Axis = layout.Vertical
	return f
}

// open shows rel, on lines line to end. Another file starts over: the caller
// has asked about the changes this one holds.
func (f *fileEditor) open(rel string, line, end int, note agentNote) {
	f.openFrom("", rel, line, end, note)
}

// openArtifact shows the agent's file name of dir, read only, rendered when
// it is Markdown.
func (f *fileEditor) openArtifact(dir, name string) {
	f.openFrom(dir, name, 0, 0, agentNote{})
}

// openOutside shows file name of dir, outside the checkout, to edit and save
// in place.
func (f *fileEditor) openOutside(dir, name string) {
	f.openFrom(dir, name, 0, 0, agentNote{})
	f.writable, f.md.mode = true, mdSource
}

// readOnly is whether the open file is one the editor only shows.
func (f *fileEditor) readOnly() bool { return f.dir != "" && !f.writable }

// holds is whether the editor shows rel of dir ("" for the checkout).
func (f *fileEditor) holds(dir, rel string) bool { return f.rel == rel && f.dir == dir }

func (f *fileEditor) openFrom(dir, rel string, line, end int, note agentNote) {
	f.writable = false
	if !f.holds(dir, rel) {
		f.dir, f.rel, f.kind = dir, rel, previewKindOf(rel)
		f.opens++
		f.md.mode = markdownModeFor(rel, line, note)
		f.composer.close()
		f.disk.read.stop()
		f.disk.seen, f.disk.theirs = "", nil
		f.setDirty(false)
	} else {
		// An agent that edits a file and points at it again has to show
		// the edit.
		f.recheck()
	}
	f.line, f.endLine, f.note, f.land, f.focus = line, end, note, true, true
}

// root is the folder rel is read from.
func (f *fileEditor) root() string {
	if f.dir != "" {
		return f.dir
	}
	return f.checkout
}

func (f *fileEditor) fileKey() string {
	return f.root() + "\x00" + f.rel + "\x00" + strconv.Itoa(f.opens)
}

// read asks for the file's bytes: an image's, or the text and, the first
// time, the editor to hold it.
func (f *fileEditor) read() {
	switch f.kind {
	case blobImage:
		f.image.want(f.client, f.root(), f.rel, "", strconv.Itoa(f.opens))
		return
	case blobPDF:
		return
	}
	key, client, root, rel := f.fileKey(), f.client, f.root(), f.rel
	build, family, size := f.ed == nil, f.family, f.size
	f.file.want(key, key, func(ctx context.Context) (editorLoad, error) {
		text, err := client.ReadFile(ctx, root, rel)
		if err != nil || !build {
			return editorLoad{text: text}, err
		}
		l := editorLoad{text: text}
		if l.ed, err = codeview.NewTextEditor(family, size); err != nil {
			return l, err
		}
		l.diff, err = codeview.NewDiffEditor(family, size, godemirror.DiffOptions{HighlightChanges: true, StyleTags: codeview.StyleTags, Collapse: &diskDiffContext})
		return l, err
	})
}

// take lays the read into the editor once it lands, on the lines asked for.
func (f *fileEditor) take() error {
	if f.kind != blobNone {
		return nil
	}
	if f.loaded == f.opens {
		f.landLines()
		return nil
	}
	l, loading, err := f.file.get()
	if loading || err != nil {
		return nil
	}
	if f.ed == nil {
		if l.ed == nil {
			return nil
		}
		f.ed, f.diff = l.ed, l.diff
		f.ed.OnUpdate = f.updated
	}
	f.loaded, f.disk.seen = f.opens, l.text
	if err := f.setText(l.text); err != nil {
		return err
	}
	f.landLines()
	return nil
}

// setText puts text in the editor from its top, as what the disk holds.
func (f *fileEditor) setText(text string) error {
	f.sep = lineBreak(text)
	err := f.ed.SetState(godemirror.EditorStateConfig{Doc: text, Filename: f.rel, LineSeparator: f.sep,
		StyleTags: codeview.StyleTags, AllowMultipleSelections: true})
	f.base = f.ed.State().Doc()
	f.gen++
	f.blame.gen++
	f.setDirty(false)
	f.vimSet()
	return err
}

// landLines selects the lines an open asked for and scrolls them in.
func (f *fileEditor) landLines() {
	if !f.land || f.ed == nil {
		return
	}
	f.land = false
	st := f.ed.State()
	if f.line <= 0 || st.Lines() == 0 {
		return
	}
	from, to := st.Line(min(f.line, st.Lines())), st.Line(min(max(f.line, f.endLine), st.Lines()))
	_ = f.ed.Dispatch(godemirror.TransactionSpec{Selection: godemirror.SingleSelection(from.From, to.To), ScrollIntoView: true, UserEvent: "select"})
}

// updated follows the user's edits: whether the text is still the disk's,
// and the marks on the lines that are not.
func (f *fileEditor) updated(u godemirror.Update) {
	f.vimPointer(u)
	if !u.DocChanged() {
		return
	}
	if f.readOnly() {
		// An agent's file is not the checkout's to write: typing in its
		// source changes nothing.
		if u.State.Doc() != f.base {
			f.reloadFrom(f.base)
		}
		return
	}
	f.gen++
	f.editedAt = f.now()
	f.setDirty(u.State.Doc() != f.base)
}

func (f *fileEditor) setDirty(dirty bool) {
	if f.dirty != dirty {
		f.dirty = dirty
		f.onDirty()
	}
}

// redecorate marks, in the margin, the lines that differ from base, and
// hangs the blame at the end of the cursor's line.
func (f *fileEditor) redecorate(th *ui.Theme, now time.Time) {
	if f.ed == nil || f.loaded != f.opens {
		return
	}
	st := f.ed.State()
	if f.decoGen != f.gen {
		f.decoGen, f.lines = f.gen, diffLines(f.base, st.Doc())
	}
	cursor := st.LineAt(st.Selection().Main().Head())
	text, hash := "", ""
	if b, loading, err := f.blame.read.get(); f.dir == "" && !loading && err == nil {
		text, hash = blameLine(b, f.lines.kept, cursor.Number-1, now)
		f.blame.commit = b.Commits[hash]
	}
	key := fmt.Sprint(f.gen, cursor.Number, text, th.ToneWait)
	if key == f.decoKey {
		return
	}
	f.decoKey = key
	f.blame.text, f.blame.hash, f.blame.cells = text, hash, utf8.RuneCountInString(cursor.Text)
	var deco godemirror.Decorations
	for _, i := range f.lines.marks {
		if i < st.Lines() {
			deco.Lines = append(deco.Lines, godemirror.LineDecorationAt{Pos: st.Line(i + 1).From, Decoration: godemirror.LineDecoration{GutterStrip: th.ToneWait}})
		}
	}
	if text != "" {
		deco.Widgets = []godemirror.LineWidgetAt{{Pos: cursor.From, Placement: godemirror.AfterText, Widget: f.blameWidget}}
	}
	_ = f.ed.Dispatch(godemirror.TransactionSpec{Decorations: &deco, SkipHistory: true})
}

func (f *fileEditor) layout(gtx C, th *ui.Theme, env editorEnv) error {
	f.env, f.th = env, th
	f.exits.update(gtx, f.client, env.notify, f.root(), f.rel)
	f.read()
	if err := f.take(); err != nil {
		return err
	}
	f.takeSave()
	f.syncVim()
	f.actions(gtx)
	f.autoSave(gtx)
	f.watchDisk(gtx)
	f.readBlame()
	f.redecorate(th, gtx.Now)
	if f.focus && f.ed != nil && f.kind == blobNone {
		f.focus = false
		f.ed.Focus(gtx)
	}
	var err error
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return f.header(gtx, th) }),
		layout.Rigid(th.Separator),
		layout.Rigid(func(gtx C) D { return f.diskBanner(gtx, th) }),
		layout.Rigid(func(gtx C) D { return f.notes(gtx, th) }),
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			var d D
			d, err = f.body(gtx, th)
			return d
		}),
		layout.Rigid(func(gtx C) D { return f.vimStatus(gtx, th) }),
	)
	return err
}

// editable is a text file in the editor, which Undo and Save act on.
func (f *fileEditor) editable() bool {
	return f.kind == blobNone && f.ed != nil && f.loaded == f.opens && f.md.mode != mdRendered && !f.readOnly()
}

func (f *fileEditor) body(gtx C, th *ui.Theme) (D, error) {
	switch f.kind {
	case blobImage:
		return th.ScrollArea(&f.scroll).Layout(gtx, 1, func(gtx C, _ int) D {
			return layout.UniformInset(ui.Space(3)).Layout(gtx, func(gtx C) D { return f.image.layout(gtx, th) })
		}), nil
	case blobPDF:
		return binaryState(gtx, th, i18n.T("diff.binaryState.pdf"), i18n.T("diff.binaryState.pdfHint"), &f.exits), nil
	}
	_, loading, err := f.file.get()
	switch {
	case f.loaded == f.opens && f.md.mode == mdRendered:
		return f.markdownBody(gtx, th), nil
	case f.loaded == f.opens:
	case err != nil:
		return binaryState(gtx, th, backend.ErrorText(err), "", &f.exits), nil
	case loading || f.loaded != f.opens:
		return th.Notice(i18n.T("dock.filesPanel.loading")).Layout(gtx), nil
	}
	if f.md.mode != mdSplit {
		return f.source(gtx, th)
	}
	var viewErr error
	d := layout.Flex{}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			var d D
			d, viewErr = f.source(gtx, th)
			return d
		}),
		layout.Rigid(th.VerticalRule),
		layout.Flexed(1, func(gtx C) D { return f.markdownBody(gtx, th) }),
	)
	return d, viewErr
}

// source is the text in the editor, with its menu when it is the checkout's.
func (f *fileEditor) source(gtx C, th *ui.Theme) (D, error) {
	var viewErr error
	code := func(gtx C) D {
		gtx.Constraints.Min = gtx.Constraints.Max
		f.ed.Theme = f.env.code
		if f.vimOn() {
			f.ed.Theme = vimHideCaret(f.ed.Theme)
		}
		d, err := f.ed.Layout(gtx)
		viewErr = err
		if err == nil {
			paint.FillShape(gtx.Ops, th.Over(th.Border), clip.Rect(gutterRule(f.ed.Geometry(), d.Size.Y)).Op())
			f.paintVimCursor(gtx)
			f.vimResize(d.Size)
		}
		return d
	}
	if f.dir != "" {
		// Its lines are not the checkout's, so there is no reference to hand a session.
		return code(gtx), viewErr
	}
	lines := f.selectedLines()
	entries := injectEntries(f.rel, lines, f.env.inject, func() { f.composer.open(f.rel, lines) })
	dims := th.ContextMenu(f.env.root, &f.menu, entries).Layout(gtx, code)
	return dims, viewErr
}

// selectedLines is the selection as formatLineRef spells it, "" for a cursor.
func (f *fileEditor) selectedLines() string {
	st := f.ed.State()
	main := st.Selection().Main()
	if main.Empty() {
		return ""
	}
	from, to := st.LineAt(main.From()), st.LineAt(main.To())
	if main.To() == to.From && to.Number > from.Number {
		to = st.Line(to.Number - 1)
	}
	return formatLineRef(from.Number, to.Number)
}

// holdsKeys is whether the comment box or the menu has the keyboard; the
// editor itself has it whenever it is on the stage.
func (f *fileEditor) holdsKeys(gtx C) bool {
	return f.composer.holdsKeys(gtx) || f.menu.Opened()
}

// setFont swaps the editor for one in family at size on the next read,
// keeping nothing but the file: a font change is rare, and the text is the
// disk's again once it lands.
func (f *fileEditor) setFont(family string, size unit.Sp) {
	if family == f.family && size == f.size {
		return
	}
	f.family, f.size = family, size
	if f.dirty {
		// shortcut: the unsaved text keeps the old font until the file is
		// opened again; rebuilding the editor would drop it.
		return
	}
	f.ed, f.diff = nil, nil
	f.opens++
	f.land = true
}

// gutterRule is the 1px line between the line numbers and the code: the
// gutter's last column, down the whole code area.
func gutterRule(g godemirror.Geometry, height int) image.Rectangle {
	return image.Rect(g.GutterWidth-1, 0, g.GutterWidth, height)
}
