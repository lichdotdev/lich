package main

import (
	"context"
	"log"
	"strconv"

	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The Editor section's global settings.
const (
	hideWhitespaceKey = "editor.hideWhitespace"
	diffLayoutKey     = "editor.diffLayout"
	vimKey            = "editor.vim"
	relativeKey       = "editor.relativeNumbers"
	diffUnified       = "unified"
	diffSplit         = "split"
)

// editorPrefs are the Editor section's values, which the review panel reads.
type editorPrefs struct {
	hideWhitespace bool
	diffLayout     string
	vim            bool
	relative       bool
}

func (p editorPrefs) split() bool { return p.diffLayout == diffSplit }

// parseDiffLayout is readDiffLayout: unified until chosen otherwise.
func parseDiffLayout(raw string) string {
	if raw == diffSplit {
		return diffSplit
	}
	return diffUnified
}

func loadEditorPrefs(ctx context.Context, c *backend.Client) (editorPrefs, error) {
	hide, err := loadHideWhitespace(ctx, c)
	if err != nil {
		return editorPrefs{}, err
	}
	layout, err := c.GetSetting(ctx, diffLayoutKey, "")
	if err != nil {
		return editorPrefs{}, err
	}
	vim, err := c.GetSetting(ctx, vimKey, "")
	if err != nil {
		return editorPrefs{}, err
	}
	relative, err := c.GetSetting(ctx, relativeKey, "")
	return editorPrefs{hideWhitespace: hide, diffLayout: parseDiffLayout(layout), vim: vim == "true", relative: relative == "true"}, err
}

// editorSettings is the Editor section's controls over prefs.
type editorSettings struct {
	prefs    editorPrefs
	hide     ui.SwitchState
	relative ui.SwitchState
	layout   ui.Enum
	vim      ui.SwitchState
	// nvim is the config file of a Neovim the editor runs, asked the first
	// time the row shows; an error when there is none on the PATH.
	nvim   remote[string]
	config ui.Clickable
}

// row answers the description and the control of the Editor row for key.
func (e *editorSettings) row(gtx C, key string, s *settingsScreen) (string, func(gtx C, th *ui.Theme) D) {
	switch key {
	case hideWhitespaceKey:
		return i18n.T("settings.settingsEditor.hideWhitespaceDescription"), func(gtx C, th *ui.Theme) D { return e.hideControl(gtx, th, s) }
	case diffLayoutKey:
		return i18n.T("settings.diffLayoutSetting.description"), func(gtx C, th *ui.Theme) D { return e.layoutControl(gtx, th, s) }
	case relativeKey:
		return i18n.T("settings.settingsEditor.relativeNumbersDescription"), func(gtx C, th *ui.Theme) D { return e.relativeControl(gtx, th, s) }
	case vimKey:
		return e.vimDescription(s), func(gtx C, th *ui.Theme) D { return e.vimControl(gtx, th, s) }
	}
	panic("settings: no Editor row for " + key)
}

func (e *editorSettings) hideControl(gtx C, th *ui.Theme, s *settingsScreen) D {
	if e.hide.Update(gtx) {
		e.prefs.hideWhitespace = e.hide.Value
		e.commit(s, hideWhitespaceKey, strconv.FormatBool(e.hide.Value))
	}
	e.hide.Value = e.prefs.hideWhitespace
	return th.Switch(&e.hide).Layout(gtx)
}

func (e *editorSettings) relativeControl(gtx C, th *ui.Theme, s *settingsScreen) D {
	if e.relative.Update(gtx) {
		e.prefs.relative = e.relative.Value
		e.commit(s, relativeKey, strconv.FormatBool(e.relative.Value))
	}
	e.relative.Value = e.prefs.relative
	return th.Switch(&e.relative).Layout(gtx)
}

func (e *editorSettings) layoutControl(gtx C, th *ui.Theme, s *settingsScreen) D {
	if e.layout.Update(gtx) {
		e.prefs.diffLayout = parseDiffLayout(e.layout.Value)
		e.commit(s, diffLayoutKey, e.prefs.diffLayout)
	}
	e.layout.Value = e.prefs.diffLayout
	return th.ToggleGroup(&e.layout,
		ui.ToggleItem{Value: diffUnified, Text: i18n.T("settings.diffLayoutSetting.layout.unified")},
		ui.ToggleItem{Value: diffSplit, Text: i18n.T("settings.diffLayoutSetting.layout.split")},
	).Layout(gtx)
}

// vimFound is whether Neovim is on the PATH, false until the check lands.
func (e *editorSettings) vimFound(s *settingsScreen) (found, checking bool) {
	e.nvim.invalidate = s.invalidate
	e.nvim.want("nvim", "nvim", func(context.Context) (string, error) {
		if err := vimAvailable(); err != nil {
			return "", err
		}
		return vimConfigFile()
	})
	_, loading, err := e.nvim.get()
	return !loading && err == nil, loading
}

func (e *editorSettings) vimDescription(s *settingsScreen) string {
	if found, checking := e.vimFound(s); !found && !checking {
		return i18n.T("settings.settingsEditor.vimMissing")
	}
	return i18n.T("settings.settingsEditor.vimDescription")
}

func (e *editorSettings) vimControl(gtx C, th *ui.Theme, s *settingsScreen) D {
	if e.vim.Update(gtx) {
		e.prefs.vim = e.vim.Value
		e.commit(s, vimKey, strconv.FormatBool(e.vim.Value))
	}
	e.vim.Value = e.prefs.vim
	sw := th.Switch(&e.vim)
	found, _ := e.vimFound(s)
	// Vim left on with Neovim gone still turns off.
	sw.Disabled = !found && !e.prefs.vim
	if !found || !e.prefs.vim || s.editVimConfig == nil {
		return sw.Layout(gtx)
	}
	path, _, _ := e.nvim.get()
	if e.config.Clicked(gtx) {
		s.editVimConfig(path)
	}
	edit := th.Button(&e.config, i18n.T("settings.settingsEditor.vimEditConfig"))
	edit.Size, edit.Variant = ui.SizeSM, ui.VariantGhost
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(edit.Layout),
		ui.Gap(2),
		layout.Rigid(sw.Layout),
	)
}

// commit hands the change to the review panel at once and stores it.
func (e *editorSettings) commit(s *settingsScreen, key, value string) {
	if s.applyEditor != nil {
		s.applyEditor(key, e.prefs)
	}
	go func() {
		if err := s.client.SetSetting(s.ctx, key, "", value); err != nil {
			log.Printf("settings: save %s: %v", key, err)
		}
	}()
}
