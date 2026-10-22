package main

import (
	"log"
	"slices"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// fontReplication asks, after a monospace interface font, whether the
// terminal and the editor take it too.
type fontReplication struct {
	state       ui.DialogState
	keep, apply widget.Clickable
	// family is the one offered, "" while closed; settingsScreen.mu guards it.
	family string
}

// offersFontReplication reports whether choosing family for the interface
// asks the question: a monospace family the terminal or the editor lacks.
// System is never offered, the interface's being a sans.
func offersFontReplication(family string, mono []string, p appearancePrefs) bool {
	if family == "" || !slices.Contains(mono, family) {
		return false
	}
	return p.font != family || p.editorFont != family
}

// offerFontReplication opens the question for family once the monospace
// families are listed, off the frame: the list is a fontconfig call.
func (s *settingsScreen) offerFontReplication(family string, p appearancePrefs) {
	go func() {
		mono, err := s.look.font.find()
		if err != nil {
			log.Printf("settings: list monospace fonts: %v", err)
			return
		}
		if !offersFontReplication(family, mono, p) {
			return
		}
		s.mu.Lock()
		s.look.replicate.family = family
		s.mu.Unlock()
		s.invalidate()
	}()
}

// layoutFontReplication draws the question. It stays closed once the
// interface moved on to another family before the list came back.
func (s *settingsScreen) layoutFontReplication(gtx C, th *ui.Theme) {
	r := &s.look.replicate
	s.mu.Lock()
	family := r.family
	s.mu.Unlock()
	open := family != "" && family == s.look.prefs.uiFont
	closed := r.keep.Clicked(gtx)
	if r.apply.Clicked(gtx) && open {
		p := s.look.prefs
		p.font, p.editorFont = family, family
		s.commitAppearance(p, terminalFontKey, family)
		s.commitAppearance(p, editorFontKey, family)
		closed = true
	}
	on := th.On(th.Popover)
	keep := on.Button(&r.keep, i18n.T("settings.appearancePrefs.replicateFontKeep"))
	keep.Variant = ui.VariantGhost
	apply := on.Button(&r.apply, i18n.T("settings.appearancePrefs.replicateFontApply"))
	dlg := th.Dialog(&r.state, open, i18n.T("settings.appearancePrefs.replicateFontTitle", i18n.Args{"family": family}),
		i18n.T("settings.appearancePrefs.replicateFontDescription"))
	dlg.Actions = []layout.Widget{keep.Layout, apply.Layout}
	if dlg.Layout(gtx) || closed {
		s.mu.Lock()
		r.family = ""
		s.mu.Unlock()
	}
}
