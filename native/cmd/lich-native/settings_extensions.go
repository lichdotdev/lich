package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

const sectionExtensions = "extensions"

// extensionsInstallKey and extensionsListKey name the section's two rows for
// the search; nothing is stored under them.
const (
	extensionsInstallKey = "extensions.install"
	extensionsListKey    = "extensions.list"
)

// The install box's two sources, the values of its toggle.
const (
	installFromGit = "git"
	installFromDir = "dir"
)

// extensionPlaceholder is the URL field's example, a URL and no prose.
const extensionPlaceholder = "https://github.com/owner/repo"

var (
	icExtensionsFolder = icons.Lucide("folder-open")
	icExtensionPick    = icons.Lucide("folder")
)

// extensionTasks is the pane's state the backend's answers change from their
// goroutines; it is read and written under extensionsSettings.mu.
type extensionTasks struct {
	installing bool
	err        string
	// asking is the source the install confirmation names, "" while it is shut.
	asking string
	// removal is the extension the remove confirmation names.
	removal *backend.Extension
}

// extensionsSettings is Settings › Extensions: the install box, then every
// extension the store holds.
type extensionsSettings struct {
	store *extensionStore

	from                    widget.Enum
	url, dir                widget.Editor
	choose, install, folder widget.Clickable
	restarts, removes       map[string]*widget.Clickable
	icons                   map[string]*icons.Icon

	installDialog, removeDialog ui.DialogState
	cancel, confirm             [2]widget.Clickable

	mu    sync.Mutex
	tasks extensionTasks
}

func (p *extensionsSettings) snapshot() extensionTasks {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.tasks
}

func (p *extensionsSettings) with(change func(t *extensionTasks)) {
	p.mu.Lock()
	change(&p.tasks)
	p.mu.Unlock()
}

// source is what the box would install: the URL or the folder, trimmed.
func (p *extensionsSettings) source() string {
	if p.from.Value == installFromDir {
		return strings.TrimSpace(p.dir.Text())
	}
	return strings.TrimSpace(p.url.Text())
}

// askInstall opens the confirmation for what the box holds; nothing happens
// for an empty field or while an install runs.
func (p *extensionsSettings) askInstall() {
	src := p.source()
	p.with(func(t *extensionTasks) {
		if src != "" && !t.installing {
			t.asking, t.err = src, ""
		}
	})
}

func (p *extensionsSettings) askRemove(e backend.Extension) {
	p.with(func(t *extensionTasks) { t.removal = &e })
}

// extensionSourceLine is the meta line under an extension: the repository
// and release for one installed from GitHub, the folder for one copied from
// one, its own directory for one placed by hand.
func extensionSourceLine(e backend.Extension) string {
	switch {
	case e.Source == "":
		return e.Dir
	case e.Release != "":
		return strings.TrimPrefix(e.Source, "https://") + " · " + e.Release
	}
	return e.Source
}

// extensionsPaneRows is the pane under its title: the header line with Open
// folder, the install box, then the list or its empty state.
func (s *settingsScreen) extensionsPaneRows(th *ui.Theme) []layout.Widget {
	p := &s.extensions
	if p.from.Value == "" {
		p.from.Value = installFromGit
	}
	rows := []layout.Widget{
		func(gtx C) D { return s.extensionsHeader(gtx, th) },
		func(gtx C) D { return s.extensionsInstallBox(gtx, th) },
	}
	if p.store == nil {
		return rows
	}
	list := p.store.all()
	if len(list) == 0 {
		return append(rows, func(gtx C) D {
			e := th.InlineEmptyState(i18n.T("settings.extensionsSettings.emptyTitle"), i18n.T("settings.extensionsSettings.emptyDescription"), nil)
			e.Pad = 0
			return layout.Inset{Top: ui.Space(6)}.Layout(gtx, e.Layout)
		})
	}
	rows = append(rows, layout.Spacer{Height: ui.Space(4)}.Layout)
	for _, e := range list {
		rows = append(rows, func(gtx C) D { return s.extensionRow(gtx, th, e) })
	}
	return rows
}

// extensionsHeader is what an extension may do, and Open folder.
func (s *settingsScreen) extensionsHeader(gtx C, th *ui.Theme) D {
	p := &s.extensions
	if p.folder.Clicked(gtx) {
		go s.openExtensionsFolder()
	}
	open := th.Button(&p.folder, i18n.T("settings.extensionsSettings.openFolder"))
	open.Variant, open.Size, open.Icon = ui.VariantOutline, ui.SizeSM, icExtensionsFolder
	return layout.Inset{Top: -ui.Space(2), Bottom: ui.Space(4)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Sp(settingsDescSize)*settingsDescCh*6/10)
				l := th.Text(settingsDescSize, i18n.T("settings.extensionsSettings.description")).Muted().Leading(20)
				l.MaxLines = 0
				return l.Layout(gtx)
			}),
			ui.VGap(3),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = 0
				return open.Layout(gtx)
			}),
		)
	})
}

func (s *settingsScreen) openExtensionsFolder() {
	dir, err := backend.ExtensionsDir(os.Getenv)
	if err == nil {
		err = os.MkdirAll(dir, 0o755)
	}
	if err == nil {
		err = s.client.OpenFolder(s.ctx, dir)
	}
	if err != nil {
		s.extensions.with(func(t *extensionTasks) { t.err = backend.ErrorText(err) })
		s.invalidate()
	}
}

// installExtension runs the confirmed install from src: the busy state while
// it runs, the backend's words under the box when it fails.
func (s *settingsScreen) installExtension(fromDir bool, src string) {
	p := &s.extensions
	p.with(func(t *extensionTasks) { t.asking, t.installing, t.err = "", true, "" })
	go func() {
		var err error
		if fromDir {
			_, err = s.client.InstallExtensionFromDir(s.ctx, src)
		} else {
			_, err = s.client.InstallExtensionFromGit(s.ctx, src)
		}
		p.with(func(t *extensionTasks) {
			t.installing = false
			if err != nil {
				t.err = backend.ErrorText(err)
			}
		})
		if err == nil {
			p.url.SetText("")
			p.dir.SetText("")
		}
		s.invalidate()
	}()
}

func (s *settingsScreen) removeExtension(e backend.Extension) {
	p := &s.extensions
	p.with(func(t *extensionTasks) { t.removal = nil })
	go func() {
		err := s.client.RemoveExtension(s.ctx, e.ID)
		if err != nil {
			p.with(func(t *extensionTasks) { t.err = backend.ErrorText(err) })
		}
		s.invalidate()
	}()
}

func (s *settingsScreen) restartExtension(id string) {
	go func() {
		if err := s.client.RestartExtension(s.ctx, id); err != nil {
			s.extensions.with(func(t *extensionTasks) { t.err = backend.ErrorText(err) })
			s.invalidate()
		}
	}()
}

// pickExtensionDir asks for the folder through the backend's file picker,
// on the manifest it holds: the backend has no folder picker but the one that
// opens a project.
func (s *settingsScreen) pickExtensionDir() {
	go func() {
		path, err := s.client.PickFile(s.ctx, i18n.T("settings.extensionsSettings.pickTitle"))
		switch {
		case err != nil:
			s.extensions.with(func(t *extensionTasks) { t.err = backend.ErrorText(err) })
		case path != "":
			s.extensions.dir.SetText(filepath.Dir(path))
		}
		s.invalidate()
	}()
}

// extensionStatusText names status in the interface language.
func extensionStatusText(status string) string {
	switch status {
	case api.ExtensionRunning:
		return i18n.T("settings.extensionsSettings.status.running")
	case api.ExtensionRestarting:
		return i18n.T("settings.extensionsSettings.status.restarting")
	case api.ExtensionFailed:
		return i18n.T("settings.extensionsSettings.status.failed")
	case api.ExtensionInvalid:
		return i18n.T("settings.extensionsSettings.status.invalid")
	}
	return i18n.T("settings.extensionsSettings.status.stopped")
}

// extensionIcon is the first surface's Lucide glyph, the puzzle for none.
func (p *extensionsSettings) extensionIcon(e backend.Extension) *icons.Icon {
	name := ""
	if len(e.Surfaces) > 0 {
		name = e.Surfaces[0].Icon
	}
	if p.icons == nil {
		p.icons = map[string]*icons.Icon{}
	}
	ic, ok := p.icons[name]
	if !ok {
		if ic, ok = icons.LookupLucide(name); !ok {
			ic = icExtension
		}
		p.icons[name] = ic
	}
	return ic
}

// extensionNameLine is the row's first line: name, version, the status dot
// and its word.
func extensionNameLine(gtx C, th *ui.Theme, e backend.Extension) D {
	children := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, e.Name).Weight(font.Medium).Layout)}
	if e.Version != "" {
		children = append(children, ui.Gap(2), layout.Rigid(th.Text(ui.TextXS, e.Version).Mono().Muted().Layout))
	}
	children = append(children, ui.Gap(3),
		layout.Rigid(func(gtx C) D { return extensionDot(gtx, th, e.Status) }),
		ui.Gap(1),
		layout.Rigid(th.Text(ui.TextXS, extensionStatusText(e.Status)).Muted().Layout),
	)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// extensionDot is the status dot: running beats in the pass tone.
func extensionDot(gtx C, th *ui.Theme, status string) D {
	switch status {
	case api.ExtensionRunning:
		return th.BeatingDot(gtx, th.TonePass)
	case api.ExtensionRestarting:
		return ui.StatusDot(gtx, th.ToneWait)
	case api.ExtensionFailed, api.ExtensionInvalid:
		return ui.StatusDot(gtx, th.Destructive)
	}
	return ui.HollowDot(gtx, th.MutedForeground)
}

// extensionRow is one extension: its glyph, the name line, the source line
// and its error, Restart for a failed one and Remove.
func (s *settingsScreen) extensionRow(gtx C, th *ui.Theme, e backend.Extension) D {
	p := &s.extensions
	if p.restarts == nil {
		p.restarts, p.removes = map[string]*widget.Clickable{}, map[string]*widget.Clickable{}
	}
	restart, remove := clickable(p.restarts, e.ID), clickable(p.removes, e.ID)
	if restart.Clicked(gtx) {
		s.restartExtension(e.ID)
	}
	if remove.Clicked(gtx) {
		p.askRemove(e)
		// The dialogs were laid out before this row took the click.
		gtx.Execute(op.InvalidateCmd{})
	}
	actions := []layout.FlexChild{}
	if e.Status == api.ExtensionFailed {
		b := th.Button(restart, i18n.T("settings.extensionsSettings.restart"))
		b.Variant, b.Size = ui.VariantOutline, ui.SizeSM
		actions = append(actions, layout.Rigid(b.Layout), ui.Gap(2))
	}
	rm := th.Button(remove, i18n.T("settings.extensionsSettings.remove"))
	rm.Variant, rm.Size = ui.VariantGhost, ui.SizeSM
	actions = append(actions, layout.Rigid(rm.Layout))
	return layout.Inset{Top: ui.Space(2.5), Bottom: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Start}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: ui.Space(0.5)}.Layout(gtx, func(gtx C) D { return p.extensionIcon(e).Layout(gtx, ui.Space(4), th.MutedForeground) })
			}),
			ui.Gap(3),
			layout.Flexed(1, func(gtx C) D { return extensionDetails(gtx, th, e) }),
			ui.Gap(3),
			layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, actions...) }),
		)
	})
}

// extensionDetails is the name line over the source line and the error.
func extensionDetails(gtx C, th *ui.Theme, e backend.Extension) D {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return extensionNameLine(gtx, th, e) }),
		ui.VGap(0.5),
		layout.Rigid(th.Text(ui.TextXS, extensionSourceLine(e)).Mono().Muted().Layout),
	}
	if e.Error != "" {
		children = append(children, ui.VGap(1), layout.Rigid(func(gtx C) D {
			l := th.Text(ui.TextXS, e.Error)
			l.Color = th.Destructive
			return l.Wrapped().Layout(gtx)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
