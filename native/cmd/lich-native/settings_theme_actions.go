package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n; the web's en catalog
// (settings.appearanceSettings).
const (
	themeImportPickTitle   = "Import Theme"
	themeTemplatePickTitle = "Save Theme Template"
	themeImported          = "Imported theme: %s"
	themeImportFailed      = "Theme import failed: %v"
	themeInstalledOne      = "Installed %d theme from %s v%s"
	themeInstalledOther    = "Installed %d themes from %s v%s"
	themeInstallFailed     = "Theme install failed: %v"
	themeUpToDate          = "%s is already at v%s"
	themeUpdated           = "Updated %s to v%s"
	themeUpdateFailed      = "Theme update failed: %v"
	themeRemoved           = "Removed theme: %s"
	themeRemoveFailed      = "Theme removal failed: %v"
	themeTemplateSaved     = "Saved theme template to %s"
	themeTemplateFailed    = "Theme template failed: %v"
	themeReloadFailed      = "Couldn't reread the themes: %v"
)

// themeRef is a theme a dialog names.
type themeRef struct{ id, name string }

// themeFile is a picked file whose id is already taken.
type themeFile struct{ path, id string }

// packConflict is a repository whose install found ids already taken.
type packConflict struct {
	url       string
	conflicts []string
}

// themeTasks is the picker's state that the backend's answers change from
// their goroutines; it is read and written under themePicker.mu.
type themeTasks struct {
	importing  bool   // a clone or a file import is running
	updating   string // the theme an update is running for
	importOpen bool
	focusURL   bool // the import dialog's field takes the focus next frame
	removal    *themeRef
	overwrite  *themeFile
	pack       *packConflict
}

func (p *themePicker) snapshot() themeTasks {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.tasks
}

func (p *themePicker) with(change func(t *themeTasks)) {
	p.mu.Lock()
	change(&p.tasks)
	p.mu.Unlock()
	p.look.invalidate()
}

func (p *themePicker) success(format string, args ...any) {
	p.toaster.Show(ui.Toast{Variant: ui.ToastSuccess, Title: fmt.Sprintf(format, args...)})
}

// reloadThemes rereads the list after a change, so the strip and the window
// follow it at once.
func (p *themePicker) reloadThemes() {
	if err := p.look.reload(); err != nil {
		p.toaster.Show(failure(themeReloadFailed, err))
	}
}

func (p *themePicker) openImport() {
	p.dialogs.url.SetText("")
	p.with(func(t *themeTasks) { t.importOpen, t.focusURL = true, true })
}

func (p *themePicker) askRemove(id, name string) {
	p.with(func(t *themeTasks) { t.removal = &themeRef{id: id, name: name} })
}

// installRepository is AppearanceSettings' installRepository: a conflict
// asks before replacing, anything else closes the import dialog with a
// toast.
func (p *themePicker) installRepository(url string, overwrite bool) {
	p.with(func(t *themeTasks) { t.importing = true })
	go func() {
		res, err := p.client.InstallThemes(p.ctx, url, overwrite)
		p.with(func(t *themeTasks) {
			t.importing = false
			if err != nil {
				return
			}
			t.importOpen, t.pack = false, nil
			if len(res.Conflicts) > 0 {
				t.pack = &packConflict{url: url, conflicts: res.Conflicts}
			}
		})
		switch {
		case err != nil:
			p.toaster.Show(failure(themeInstallFailed, err))
		case len(res.Conflicts) == 0:
			p.success(pluralFormat(len(res.Themes), themeInstalledOne, themeInstalledOther), len(res.Themes), res.Pack, res.Version)
			p.reloadThemes()
		}
	}()
}

func pluralFormat(n int, one, other string) string {
	if n == 1 {
		return one
	}
	return other
}

// chooseFile picks a theme file through the backend's picker and imports
// it; an id already taken asks before replacing.
func (p *themePicker) chooseFile() {
	p.with(func(t *themeTasks) { t.importing = true })
	go func() {
		defer p.with(func(t *themeTasks) { t.importing = false })
		path, err := p.client.PickFile(p.ctx, themeImportPickTitle)
		if err != nil || path == "" {
			p.toastIfFailed(themeImportFailed, err)
			return
		}
		res, err := p.client.ImportTheme(p.ctx, path, false)
		if err != nil {
			p.toaster.Show(failure(themeImportFailed, err))
			return
		}
		p.with(func(t *themeTasks) {
			t.importOpen = false
			if res.NeedsOverwrite {
				t.overwrite = &themeFile{path: path, id: res.Theme.ID}
			}
		})
		if !res.NeedsOverwrite {
			p.success(themeImported, res.Theme.Name)
			p.reloadThemes()
		}
	}()
}

func (p *themePicker) toastIfFailed(format string, err error) {
	if err != nil {
		p.toaster.Show(failure(format, err))
	}
}

func (p *themePicker) overwriteFile(f themeFile) {
	go func() {
		res, err := p.client.ImportTheme(p.ctx, f.path, true)
		if err != nil {
			p.toaster.Show(failure(themeImportFailed, err))
			return
		}
		p.with(func(t *themeTasks) { t.overwrite = nil })
		p.success(themeImported, res.Theme.Name)
		p.reloadThemes()
	}()
}

func (p *themePicker) updateTheme(id string) {
	p.with(func(t *themeTasks) { t.updating = id })
	go func() {
		defer p.with(func(t *themeTasks) { t.updating = "" })
		res, err := p.client.UpdateTheme(p.ctx, id)
		switch {
		case err != nil:
			p.toaster.Show(failure(themeUpdateFailed, err))
		case res.UpToDate:
			p.success(themeUpToDate, res.Pack, res.Version)
		default:
			p.success(themeUpdated, res.Pack, res.Version)
			p.reloadThemes()
		}
	}()
}

// removeTheme removes r; removing the theme in use hands the window back
// to System, as the web's removeTheme does.
func (p *themePicker) removeTheme(r themeRef) {
	go func() {
		if err := p.client.RemoveTheme(p.ctx, r.id); err != nil {
			p.toaster.Show(failure(themeRemoveFailed, err))
			return
		}
		p.with(func(t *themeTasks) { t.removal = nil })
		if selected := p.look.view().selected; themeAfterRemoval(r.id, selected) != selected {
			p.look.choose(themeSystem)
		}
		p.success(themeRemoved, r.name)
		p.reloadThemes()
	}()
}

func (p *themePicker) saveTemplate() {
	go func() {
		path, err := p.client.PickSaveFile(p.ctx, themeTemplatePickTitle, themeTemplateName)
		if err != nil || path == "" {
			p.toastIfFailed(themeTemplateFailed, err)
			return
		}
		if err := p.client.SaveThemeTemplate(p.ctx, path); err != nil {
			p.toaster.Show(failure(themeTemplateFailed, err))
			return
		}
		p.success(themeTemplateSaved, path)
	}()
}

func joinConflicts(c *packConflict) string {
	if c == nil {
		return ""
	}
	return strings.Join(c.conflicts, ", ")
}

var flagThemePicker = flag.String("theme-picker", "", "open Settings › Appearance with the theme strip at start, for -shot: open, import, remove (the first imported theme) or replace")

// showThemePicker is -theme-picker.
func (u *window) showThemePicker(mode string) error {
	u.settings.section = sectionAppearance
	u.settings.show("", nil)
	p := u.settings.theme
	p.open = true
	var custom []lichclient.Theme
	for _, e := range u.appearance.view().themes {
		if e.def.Origin == themeOriginCustom {
			custom = append(custom, e.def)
		}
	}
	switch {
	case mode == "open":
	case mode == "import":
		p.openImport()
	case mode == "remove" && len(custom) > 0:
		p.askRemove(custom[0].ID, custom[0].Name)
	case mode == "replace" && len(custom) > 0 && custom[0].Source != nil:
		p.with(func(t *themeTasks) {
			t.pack = &packConflict{url: custom[0].Source.URL, conflicts: []string{custom[0].ID}}
		})
	default:
		return fmt.Errorf("-theme-picker %q: want open, import, or remove/replace with an imported theme", mode)
	}
	return nil
}
