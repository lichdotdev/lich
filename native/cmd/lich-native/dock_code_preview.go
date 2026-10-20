package main

// The Code tab's FilePreview: the file read-only in a godemirror view over
// the tree, a back button, and the inject menu on its selection, with the
// session comment's composer hung under the selection's last line.

import (
	"context"
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var icPreviewBack = icons.Lucide("chevron-left")

type preview struct {
	client *lichclient.Client
	view   *godemirror.View
	family string
	size   unit.Sp
	file   remote[string]
	image  *blobView
	exits  fileExits
	scroll widget.List

	rel  string // "" while closed
	line int    // the line to land on, 1-based; 0 for the top
	// shown is the read on the view, its key; content is what was set there.
	shown   string
	content godemirror.Content

	back     widget.Clickable
	backTip  ui.Tooltip
	menu     ui.MenuState
	composer commentComposer
	// blockAt is the document line the composer hangs under, -1 for none.
	blockAt int
	env     previewEnv
	blockTh *ui.Theme
}

// previewEnv is what the preview reads of the dock each frame.
type previewEnv struct {
	checkout string
	code     godemirror.Theme
	inject   func(string) bool
	comments *commentStore
	notify   func(ui.Toast)
}

func newPreview(client *lichclient.Client, view *godemirror.View, family string, size unit.Sp, invalidate func()) *preview {
	p := &preview{client: client, view: view, family: family, size: size, blockAt: -1, image: newBlobView(invalidate)}
	p.file.invalidate = invalidate
	p.scroll.Axis = layout.Vertical
	return p
}

func (p *preview) isOpen() bool { return p.rel != "" }

func (p *preview) openAt(rel string, line int) {
	p.rel, p.line, p.shown = rel, line, ""
	p.composer.close()
}

func (p *preview) close() {
	p.rel = ""
	p.composer.close()
	p.file.stop()
	p.image.read.stop()
}

func (p *preview) layout(gtx C, th *ui.Theme, root *ui.Root, env previewEnv) (D, error) {
	p.env, p.blockTh = env, th
	if p.back.Clicked(gtx) {
		p.close()
		return D{Size: gtx.Constraints.Max}, nil
	}
	key, path, rel := env.checkout+"\x00"+p.rel, env.checkout, p.rel
	kind := previewKindOf(rel)
	p.exits.update(gtx, p.client, env.notify, path, rel)
	switch kind {
	case blobImage:
		p.image.want(p.client, path, rel, "", "")
	case blobNone:
		p.file.want(key, key, func(ctx context.Context) (string, error) {
			var text string
			return text, p.client.Call(ctx, "project.ReadFile", &text, path, rel)
		})
	}
	var err error
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return p.header(gtx, th, root) }),
		layout.Rigid(th.Separator),
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			switch kind {
			case blobImage:
				return p.imageBody(gtx, th)
			case blobPDF:
				return binaryState(gtx, th, i18n.T("diff.binaryState.pdf"), i18n.T("diff.binaryState.pdfHint"), &p.exits)
			}
			var d D
			d, err = p.body(gtx, th, root, key)
			return d
		}),
	)
	return dims, err
}

// header is the h-8 strip: back, the path in mono, and the read-only mark.
func (p *preview) header(gtx C, th *ui.Theme, root *ui.Root) D {
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(ui.Space(8))))
	return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.W.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return p.headerRow(gtx, th, root)
		})
	})
}

func (p *preview) headerRow(gtx C, th *ui.Theme, root *ui.Root) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return p.backButton(gtx, th, root) }),
		ui.Gap(1.5),
		layout.Flexed(1, func(gtx C) D {
			l := th.Text(ui.TextXS, p.rel).Mono()
			l.MaxLines = 1
			return l.Layout(gtx)
		}),
		ui.Gap(1.5),
		layout.Rigid(th.Text(text2XS, strings.ToUpper(i18n.T("dock.filesPanel.readOnly"))).Leading(text2XSLeading).Muted().Layout),
	)
}

// backButton is a rounded button, its chevron muted until hovered.
func (p *preview) backButton(gtx C, th *ui.Theme, root *ui.Root) D {
	tip := th.Tooltip(root, &p.backTip, i18n.T("dock.filesPanel.backToTree"))
	return tip.Layout(gtx, func(gtx C) D {
		return p.back.Layout(gtx, func(gtx C) D {
			side := gtx.Dp(ui.Space(5))
			gtx.Constraints = layout.Exact(image.Pt(side, side))
			fg := th.MutedForeground
			if p.back.Hovered() {
				ui.Fill(gtx, th.Accent, ui.RadiusSM)
				fg = th.AccentForeground
			}
			return layout.Center.Layout(gtx, func(gtx C) D { return icPreviewBack.Layout(gtx, ui.Space(4), fg) })
		})
	})
}

func (p *preview) body(gtx C, th *ui.Theme, root *ui.Root, key string) (D, error) {
	text, loading, err := p.file.get()
	switch {
	case err != nil:
		return binaryState(gtx, th, lichclient.ErrorText(err), "", &p.exits), nil
	case loading:
		return notice(gtx, th, i18n.T("dock.filesPanel.loading")), nil
	}
	p.show(key, text)
	p.syncComposer()
	p.view.Theme = previewTheme(th, p.env.code)
	var viewErr error
	lines := ""
	if from, to, ok := p.view.SelectedLines(); ok {
		lines = formatLineRef(from+1, to+1)
	}
	entries := injectEntries(p.rel, lines, p.env.inject, func() { p.composer.open(p.rel, lines) })
	dims := th.ContextMenu(root, &p.menu, entries).Layout(gtx, func(gtx C) D {
		return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			d, err := p.view.Layout(gtx)
			viewErr = err
			return d
		})
	})
	return dims, viewErr
}

// imageBody is the image in p-3, scrolling when it is taller than the panel.
func (p *preview) imageBody(gtx C, th *ui.Theme) D {
	return th.ScrollArea(&p.scroll).Layout(gtx, 1, func(gtx C, _ int) D {
		return layout.UniformInset(ui.Space(3)).Layout(gtx, func(gtx C) D { return p.image.layout(gtx, th) })
	})
}

// show puts a new read on the view, landing on the line a search hit named,
// selected: the selection is what the inject menu and the comment read, so the
// line found is one right-click from both.
func (p *preview) show(key, text string) {
	if key == p.shown {
		return
	}
	p.shown, p.blockAt = key, -1
	p.content = godemirror.Content{Filename: p.rel, Text: text, StyleTags: codeStyleTags}
	p.view.SetContent(p.content)
	if p.line > 0 {
		p.view.SelectLines(p.line-1, p.line-1)
		p.view.ScrollToLine(p.line-1, godemirror.AlignCenter)
	}
}

// syncComposer hangs the composer under its line while it is open, and takes
// it down once it closed itself. Opening sets the content outright and
// scrolls to it: UpdateContent would keep the scroll but swap the block in
// only once the file is highlighted again, a frame the view would scroll
// against the old content in.
func (p *preview) syncComposer() {
	at := -1
	if p.composer.isOpen() {
		at = p.composer.at() - 1
	}
	if at == p.blockAt {
		return
	}
	p.blockAt = at
	if at < 0 {
		p.view.UpdateContent(p.content)
		return
	}
	c := p.content
	c.Blocks = []godemirror.BlockWidget{{After: at, Widget: p.composerBlock}}
	p.view.SetContent(c)
	p.view.ScrollToLine(at, godemirror.AlignCenter)
}

func (p *preview) composerBlock(gtx C) D {
	return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return p.composer.layout(gtx, p.blockTh, p.env.comments, p.env.checkout)
	})
}

// codeStyleTags tags more of the code than CodeMirror's grammars do, so a
// theme can color it as an IDE does: Java's enum constants, annotations,
// fields and import paths, parameter declarations, and each language's
// builtins.
var codeStyleTags = []godemirror.StyleTags{
	godemirror.RicherJavaTags(),
	godemirror.RicherGoTags(), godemirror.GoBuiltinTags(),
	godemirror.RicherJavaScriptTags(), godemirror.JavaScriptBuiltinTags(),
	godemirror.RicherPythonTags(), godemirror.PythonBuiltinTags(),
	godemirror.RicherRustTags(), godemirror.RustBuiltinTags(),
}

// previewTheme is the window theme's code palette on the theme's surface.
func previewTheme(th *ui.Theme, code godemirror.Theme) godemirror.Theme {
	code.Background, code.GutterBackground = th.Surface, th.Surface
	return code
}

// setFont swaps the view for one in family at size, keeping what it shows.
func (p *preview) setFont(family string, size unit.Sp) error {
	if family == p.family && size == p.size {
		return nil
	}
	view, err := newCodeView(family, size)
	if err != nil {
		return err
	}
	p.view, p.family, p.size = view, family, size
	if p.content.Text != "" {
		p.view.SetContent(p.content)
	}
	return nil
}
