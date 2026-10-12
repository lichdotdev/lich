package main

import (
	"context"
	"log"
	"strconv"

	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// The Editor section's global settings. The web keeps the diff layout in
// localStorage (lich.diff.layout), so the backend has no key to share with
// it; these are the native window's.
const (
	hideWhitespaceKey = "editor.hideWhitespace"
	diffLayoutKey     = "editor.diffLayout"
	diffUnified       = "unified"
	diffSplit         = "split"
)

// ponytail: English until the native window has i18n (diffLayoutSetting.layout).
const (
	diffUnifiedLabel = "Unified"
	diffSplitLabel   = "Side-by-side"
)

// editorPrefs are the Editor section's values, which the review panel reads.
type editorPrefs struct {
	hideWhitespace bool
	diffLayout     string
}

func (p editorPrefs) split() bool { return p.diffLayout == diffSplit }

// parseDiffLayout is readDiffLayout: unified until chosen otherwise.
func parseDiffLayout(raw string) string {
	if raw == diffSplit {
		return diffSplit
	}
	return diffUnified
}

func loadEditorPrefs(ctx context.Context, c *lichclient.Client) (editorPrefs, error) {
	hide, err := loadHideWhitespace(ctx, c)
	if err != nil {
		return editorPrefs{}, err
	}
	layout, err := c.GetSetting(ctx, diffLayoutKey, "")
	return editorPrefs{hideWhitespace: hide, diffLayout: parseDiffLayout(layout)}, err
}

// editorSettings is the Editor section's controls over prefs.
type editorSettings struct {
	prefs  editorPrefs
	hide   ui.SwitchState
	layout widget.Enum
}

// row answers the description and the control of the Editor row for key.
func (e *editorSettings) row(gtx C, key string, s *settingsScreen) (string, func(gtx C, th *ui.Theme) D) {
	switch key {
	case hideWhitespaceKey:
		return hideWhitespaceDesc, func(gtx C, th *ui.Theme) D { return e.hideControl(gtx, th, s) }
	case diffLayoutKey:
		return diffLayoutDesc, func(gtx C, th *ui.Theme) D { return e.layoutControl(gtx, th, s) }
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

func (e *editorSettings) layoutControl(gtx C, th *ui.Theme, s *settingsScreen) D {
	if e.layout.Update(gtx) {
		e.prefs.diffLayout = parseDiffLayout(e.layout.Value)
		e.commit(s, diffLayoutKey, e.prefs.diffLayout)
	}
	e.layout.Value = e.prefs.diffLayout
	return th.ToggleGroup(&e.layout,
		ui.ToggleItem{Value: diffUnified, Text: diffUnifiedLabel},
		ui.ToggleItem{Value: diffSplit, Text: diffSplitLabel},
	).Layout(gtx)
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
