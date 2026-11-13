package main

import (
	"context"
	"os"
	"strings"
	"sync"

	"gioui.org/gesture"
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

// The install rows, the places an install starts from and its error lands
// under. errAtFolder is the title line's Open folder.
const (
	installFromGit = "git"
	installFromDir = "dir"
	errAtFolder    = "folder"
)

// extensionPlaceholder is the URL field's example, a URL and no prose.
const extensionPlaceholder = "https://github.com/owner/repo"

var (
	icExtensionsFolder = icons.Lucide("folder-open")
	icExtensionPick    = icons.Lucide("folder")
	icExtensionMore    = icons.Lucide("ellipsis")
	icExtensionRestart = icons.Lucide("rotate-ccw")
	icExtensionRemove  = icons.Lucide("trash-2")
	icExtensionCheck   = icons.Lucide("refresh-cw")
)

// extensionError is the section's last error and the place it came from: an
// install row, the title line, or an extension's row (errAtRow).
type extensionError struct {
	at, text string
}

// errAtRow is where an error from the row of extension id lands; the prefix
// keeps an id from colliding with the install rows' places.
func errAtRow(id string) string { return "row/" + id }

// extensionTasks is the pane's state the backend's answers change from their
// goroutines; it is read and written under extensionsSettings.mu.
type extensionTasks struct {
	// installing is the install row whose prepare or commit runs, "" for none.
	installing string
	err        extensionError
	// asking is whether the install confirmation is open on preview, the
	// staged install from the row askingFrom. preview outlives asking so the
	// dialog keeps its words while it closes.
	asking     bool
	preview    backend.ExtensionInstallPreview
	askingFrom string
	// replacing is the installed extension the preview updates, nil for a
	// new install.
	replacing *backend.Extension
	// checking is whether Check for updates runs.
	checking bool
	// removal is the extension the remove confirmation names.
	removal *backend.Extension
}

func (t extensionTasks) errorAt(at string) string {
	if t.err.at == at {
		return t.err.text
	}
	return ""
}

// extensionRowState is one installed row's hover, its menu and its controls.
type extensionRowState struct {
	hover          gesture.Hover
	restart, more  widget.Clickable
	update         widget.Clickable
	menu, context  ui.MenuState
	moreTip, state ui.Tooltip
}

// extensionsSettings is Settings › Extensions: Open folder on the title line,
// the two install rows, then every extension the store holds.
type extensionsSettings struct {
	store *extensionStore

	url                     widget.Editor
	choose, install, folder widget.Clickable
	check                   widget.Clickable
	rows                    map[string]*extensionRowState
	icons                   map[string]*icons.Icon

	installDialog, removeDialog ui.DialogState
	cancel, confirm             [2]widget.Clickable
	// deleteData is the remove confirmation's box, unticked each time it opens.
	deleteData      widget.Bool
	deleteDataLabel widget.Clickable

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

func (p *extensionsSettings) row(id string) *extensionRowState {
	if p.rows == nil {
		p.rows = map[string]*extensionRowState{}
	}
	r, ok := p.rows[id]
	if !ok {
		r = new(extensionRowState)
		p.rows[id] = r
	}
	return r
}

func (p *extensionsSettings) fail(at string, err error) {
	p.with(func(t *extensionTasks) { t.err = extensionError{at, backend.ErrorText(err)} })
}

// clearError drops the error that came from at, and only that one.
func (p *extensionsSettings) clearError(at string) {
	p.with(func(t *extensionTasks) {
		if t.err.at == at {
			t.err = extensionError{}
		}
	})
}

// prepareInstall stages src from the install row from and opens the
// confirmation on what it is; nothing happens for an empty source or while an
// install runs.
func (s *settingsScreen) prepareInstall(from, src string) {
	p := &s.extensions
	src = strings.TrimSpace(src)
	start := false
	p.with(func(t *extensionTasks) {
		if src != "" && t.installing == "" {
			start, t.installing = true, from
			if t.err.at == from {
				t.err = extensionError{}
			}
		}
	})
	if !start {
		return
	}
	go func() {
		prepare := s.client.PrepareExtensionFromGit
		if from == installFromDir {
			prepare = s.client.PrepareExtensionFromDir
		}
		preview, err := prepare(s.ctx, src)
		p.with(func(t *extensionTasks) {
			t.installing = ""
			if err != nil {
				t.err = extensionError{from, backend.ErrorText(err)}
				return
			}
			t.asking, t.preview, t.askingFrom, t.replacing = true, preview, from, nil
		})
		s.invalidate()
	}()
}

func (p *extensionsSettings) askRemove(e backend.Extension) {
	p.deleteData.Value = false
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

func (s *settingsScreen) openExtensionsFolder() {
	p := &s.extensions
	p.clearError(errAtFolder)
	dir, err := s.client.ExtensionsDir(s.ctx)
	if err == nil {
		err = os.MkdirAll(dir, 0o755)
	}
	if err == nil {
		err = s.client.OpenFolder(s.ctx, dir)
	}
	if err != nil {
		p.fail(errAtFolder, err)
		s.invalidate()
	}
}

// openExtensionDir opens one extension's own folder.
func (s *settingsScreen) openExtensionDir(e backend.Extension) {
	s.extensions.clearError(errAtRow(e.ID))
	go func() {
		if err := s.client.OpenFolder(s.ctx, e.Dir); err != nil {
			s.extensions.fail(errAtRow(e.ID), err)
			s.invalidate()
		}
	}()
}

// commitInstall is the confirmation's Install: the busy state on the row's
// button while it runs, the backend's words under the row when it fails.
func (s *settingsScreen) commitInstall() {
	p := &s.extensions
	var token, from string
	p.with(func(t *extensionTasks) {
		token, from = t.preview.Token, t.askingFrom
		t.asking, t.installing, t.err = false, from, extensionError{}
	})
	go func() {
		_, err := s.client.CommitExtensionInstall(s.ctx, token)
		p.with(func(t *extensionTasks) {
			t.installing = ""
			if err != nil {
				t.err = extensionError{from, backend.ErrorText(err)}
			}
		})
		if err == nil && from == installFromGit {
			p.url.SetText("")
		}
		s.invalidate()
	}()
}

// discardInstall is the confirmation closed without Install: the staged copy
// goes.
func (s *settingsScreen) discardInstall() {
	p := &s.extensions
	var token, from string
	p.with(func(t *extensionTasks) {
		token, from = t.preview.Token, t.askingFrom
		t.asking = false
	})
	go func() {
		if err := s.client.DiscardExtensionInstall(s.ctx, token); err != nil {
			p.fail(from, err)
			s.invalidate()
		}
	}()
}

// confirmRemoval is the remove confirmation's action, which its box picks.
func (s *settingsScreen) confirmRemoval(e backend.Extension) {
	if s.extensions.deleteData.Value {
		s.removeExtensionAndData(e)
		return
	}
	s.removeExtension(e)
}

func (s *settingsScreen) removeExtension(e backend.Extension) {
	s.runRemoval(e, s.client.RemoveExtension)
}

func (s *settingsScreen) removeExtensionAndData(e backend.Extension) {
	s.runRemoval(e, s.client.RemoveExtension, s.client.DeleteExtensionData)
}

// runRemoval makes the calls in order and stops at the first that fails.
func (s *settingsScreen) runRemoval(e backend.Extension, calls ...func(ctx context.Context, id string) error) {
	p := &s.extensions
	p.with(func(t *extensionTasks) { t.removal = nil })
	p.clearError(errAtRow(e.ID))
	go func() {
		for _, call := range calls {
			if err := call(s.ctx, e.ID); err != nil {
				p.fail(errAtRow(e.ID), err)
				break
			}
		}
		s.invalidate()
	}()
}

func (s *settingsScreen) restartExtension(id string) {
	s.extensions.clearError(errAtRow(id))
	go func() {
		if err := s.client.RestartExtension(s.ctx, id); err != nil {
			s.extensions.fail(errAtRow(id), err)
			s.invalidate()
		}
	}()
}

// pickExtensionDir asks for the folder through the backend's directory
// picker and goes straight to staging it.
func (s *settingsScreen) pickExtensionDir() {
	go func() {
		path, err := s.client.PickDirectory(s.ctx, i18n.T("settings.extensionsSettings.pickTitle"))
		switch {
		case err != nil:
			s.extensions.fail(installFromDir, err)
		case path != "":
			s.prepareInstall(installFromDir, path)
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
