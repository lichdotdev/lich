package main

// The stage editor's language server while Vim is off (Vim's Neovim runs
// the person's own): the file's server from godemirror's catalog, found on
// the PATH or among those lich installed, bound to the editor with
// lsp.Attach, and its state in the header.

import (
	"image/color"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gioui.org/layout"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/godemirror/lsp"
	"github.com/lichdotdev/godemirror/lsp/servers"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// editorLsp is an editor's binding to its server. gen is the editor's text
// it was chosen for: a new file, or the disk's text laid in, chooses again.
type editorLsp struct {
	gen  int
	root string
	// name is the chosen server, "" for a file none serves; missing is set
	// when its program is on no PATH, and install is how lich installs it.
	name    string
	missing bool
	install *servers.Install
	srv     *lspServer
	doc     *lsp.Document

	installing <-chan error
	installErr string

	installBtn ui.Clickable
	reason     ui.Tooltip
	errors     ui.Clickable
	warnings   ui.Clickable
}

// syncLSP binds the editor to its file's server, once the server is up.
func (f *fileEditor) syncLSP() {
	if f.env.lsp == nil || f.vimOn() || f.kind != blobNone || f.ed == nil || f.loaded != f.opens || f.dir != "" {
		f.lspOff()
		return
	}
	f.lspInstalled()
	l := &f.lsp
	if l.gen != f.gen {
		f.lspOff()
		f.lspChoose()
	}
	if l.doc != nil || l.srv == nil {
		return
	}
	c, err, _ := l.srv.state()
	if c == nil || err != nil {
		return
	}
	doc, err := lsp.Attach(c, f.ed, lspURI(f.absPath()), lsp.Options{LanguageID: servers.LanguageID(servers.FileType(f.rel)),
		Invalidate: f.invalidate, OnError: func(err error) { log.Printf("lsp %s: %v", l.name, err) }})
	if err != nil {
		log.Printf("lsp %s: attach %s: %v", l.name, f.rel, err)
		l.srv = nil
		return
	}
	l.doc = doc
	f.ed.OnDefinition = f.lspGoTo
	f.ed.OnWorkspaceEdit = f.lspApplyElsewhere
}

// lspChoose picks the file's server: the first of the catalog's whose
// program is found, or else the first with a command, as missing.
func (f *fileEditor) lspChoose() {
	l := &f.lsp
	*l = editorLsp{gen: f.gen, installErr: l.installErr, installing: l.installing}
	for _, s := range servers.Lookup(f.rel) {
		if command := lspCommand(s); command != nil {
			l.name, l.root = s.Name, lspRoot(f.checkout, f.rel, s.RootMarkers)
			l.srv = f.env.lsp.server(l.root, s, command)
			return
		}
		if l.name == "" && len(s.Command) > 0 {
			l.name, l.missing, l.install = s.Name, true, lspInstallable(s.Install)
		}
	}
}

func (f *fileEditor) lspOff() {
	l := &f.lsp
	if l.doc != nil {
		_ = l.doc.Close()
		f.ed.OnDefinition, f.ed.OnWorkspaceEdit = nil, nil
	}
	l.doc, l.srv, l.gen, l.name = nil, nil, 0, ""
}

func (f *fileEditor) absPath() string {
	return filepath.Join(f.root(), filepath.FromSlash(f.rel))
}

// lspRoot is the project folder a server works on for rel: the nearest
// folder from rel's up to the checkout holding one of a level of markers,
// the levels tried in order, else the checkout.
func lspRoot(checkout, rel string, markers [][]string) string {
	for _, level := range markers {
		for dir := filepath.Dir(filepath.Join(checkout, filepath.FromSlash(rel))); strings.HasPrefix(dir, checkout); dir = filepath.Dir(dir) {
			for _, name := range level {
				if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
					return dir
				}
			}
			if dir == checkout {
				break
			}
		}
	}
	return checkout
}

func lspURI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

// lspPathOf is the file a URI names, "" for one that is not a file.
func lspPathOf(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return ""
	}
	return filepath.FromSlash(u.Path)
}

// lspGoTo opens where a definition lies in another file: the checkout's in
// this editor, any other read only.
func (f *fileEditor) lspGoTo(loc godemirror.Location) {
	path := lspPathOf(loc.URI)
	if path == "" {
		return
	}
	line := loc.Start.Line + 1
	if rel, err := filepath.Rel(f.checkout, path); err == nil && !strings.HasPrefix(rel, "..") {
		f.open(filepath.ToSlash(rel), line, line, agentNote{})
		return
	}
	f.openReference(filepath.Dir(path), filepath.Base(path), line)
}

// lspCounts is how many errors and warnings are reported on the file, by
// the editor's server or by Vim's.
func (f *fileEditor) lspCounts() (errs, warns int) {
	if f.ed == nil {
		return 0, 0
	}
	for _, d := range f.ed.Diagnostics() {
		switch d.Severity {
		case godemirror.SeverityError:
			errs++
		case godemirror.SeverityWarning:
			warns++
		}
	}
	return errs, warns
}

var icCircleX = icons.Lucide("circle-x")

// lspStatus is the header's say on the file's server: the errors and the
// warnings it reports, each a jump to the next, and its name; or why there
// is none, with Install where lich can install it.
func (f *fileEditor) lspStatus(gtx C, th *ui.Theme) D {
	l := &f.lsp
	errs, warns := f.lspCounts()
	if l.name == "" && errs+warns == 0 {
		return D{}
	}
	if l.installBtn.Clicked(gtx) && l.installing == nil {
		f.lspInstall()
	}
	if l.errors.Clicked(gtx) {
		f.ed.NextDiagnostic(godemirror.SeverityError)
	}
	if l.warnings.Clicked(gtx) {
		f.ed.NextDiagnostic(godemirror.SeverityWarning)
	}
	text, reason, install := f.lspText()
	children := []layout.FlexChild{}
	if errs > 0 {
		children = append(children, layout.Rigid(lspCount(th, &l.errors, icCircleX, errs, th.Destructive)), ui.Gap(1))
	}
	if warns > 0 {
		children = append(children, layout.Rigid(lspCount(th, &l.warnings, icTriangleAlert, warns, th.ToneWait)), ui.Gap(1))
	}
	label := th.Text(ui.TextSM, text).Mono().Muted()
	if text == "" {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	}
	children = append(children, layout.Rigid(func(gtx C) D {
		if reason == "" {
			return label.Layout(gtx)
		}
		return th.Tooltip(f.env.root, &l.reason, reason).Layout(gtx, label.Layout)
	}))
	if install {
		btn := th.Button(&l.installBtn, i18n.T("dock.editor.lspInstall"))
		btn.Size, btn.Variant = ui.SizeXS, ui.VariantOutline
		children = append(children, ui.Gap(2), layout.Rigid(btn.Layout))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// lspText is what the header says of the server, the tooltip's reason
// behind it, and whether Install shows.
func (f *fileEditor) lspText() (text, reason string, install bool) {
	l := &f.lsp
	name := i18n.Args{"name": l.name}
	switch {
	case l.installing != nil:
		return i18n.T("dock.editor.lspInstalling", name), "", false
	case l.installErr != "":
		return i18n.T("dock.editor.lspInstallFailed", name), l.installErr, l.install != nil
	case l.missing:
		return i18n.T("dock.editor.lspMissing", name), "", l.install != nil
	}
	if l.srv != nil {
		if _, err, why := l.srv.state(); err != nil {
			return i18n.T("dock.editor.lspFailed", name), why, false
		}
	}
	return l.name, "", false
}

func lspCount(th *ui.Theme, c *ui.Clickable, icon *icons.Icon, n int, tone color.NRGBA) layout.Widget {
	return func(gtx C) D {
		return c.Layout(gtx, func(gtx C) D {
			return layout.UniformInset(ui.Space(1)).Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return icon.Layout(gtx, ui.Space(3.5), tone) }),
					ui.Gap(1),
					layout.Rigid(th.Text(ui.TextSM, strconv.Itoa(n)).Mono().In(tone).Layout))
			})
		})
	}
}
