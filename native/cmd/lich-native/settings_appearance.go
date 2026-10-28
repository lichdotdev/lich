package main

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// closeChoices are CloseSetting's three answers, each with the keys of its
// label and of the line saying what it does.
var closeChoices = []struct{ value, labelKey, consequenceKey string }{
	{closeAsk, "settings.closeSetting.choice.ask.label", "settings.closeSetting.choice.ask.consequence"},
	{closeBackground, "settings.closeSetting.choice.background.label", "settings.closeSetting.choice.background.consequence"},
	{closeQuit, "settings.closeSetting.choice.quit.label", "settings.closeSetting.choice.quit.consequence"},
}

var (
	icZoomOut = icons.Lucide("zoom-out")
	icZoomIn  = icons.Lucide("zoom-in")
	icMinus   = icons.Lucide("minus")
	// ceilingWidth is the input's w-40.
	ceilingWidth = ui.Space(40)
)

// appearanceControls are the Appearance rows past the theme and the footer,
// over the values the window applies; apply hands it each change.
type appearanceControls struct {
	prefs      appearancePrefs
	apply      func(appearancePrefs)
	zoom       stepperState
	size       stepperState
	font       fontPicker
	editorSize stepperState
	editorFont fontPicker
	uiFont     fontPicker
	replicate  fontReplication
	budget     widget.Editor
	typed      bool // the ceiling field holds the user's text, not the stored value
	closeTo    widget.Enum
	copy       ui.SwitchState
}

// commit applies p at once and stores the setting that changed.
func (s *settingsScreen) commitAppearance(p appearancePrefs, key, value string) {
	s.look.prefs = p
	if s.look.apply != nil {
		s.look.apply(p)
	}
	go func() {
		if err := s.client.SetSetting(s.ctx, key, "", value); err != nil {
			log.Printf("settings: save %s: %v", key, err)
		}
	}()
}

// appearanceRow draws the rows of the keys this file owns, reporting false
// for any other.
func (s *settingsScreen) appearanceRow(gtx C, th *ui.Theme, e settingEntry, r settingRow, first bool) (D, bool) {
	var control func(gtx C, th *ui.Theme) D
	switch e.key {
	case zoomKey:
		control = s.zoomControl
	case terminalFontSizeKey:
		control = s.textSizeControl
	case terminalFontKey:
		r.sample = s.look.font.sample(th, s.look.prefs.font)
		control = func(gtx C, th *ui.Theme) D {
			return s.fontControl(gtx, th, &s.look.font, s.look.prefs.font, func(family string) {
				p := s.look.prefs
				p.font = family
				s.commitAppearance(p, terminalFontKey, family)
			})
		}
	case uiFontKey:
		r.sample = s.look.uiFont.sample(th, s.look.prefs.uiFont)
		control = func(gtx C, th *ui.Theme) D {
			return s.fontControl(gtx, th, &s.look.uiFont, s.look.prefs.uiFont, func(family string) {
				p := s.look.prefs
				p.uiFont = family
				s.commitAppearance(p, uiFontKey, family)
				s.offerFontReplication(family, p)
			})
		}
	case editorFontSizeKey:
		control = s.editorSizeControl
	case editorFontKey:
		r.sample = s.look.editorFont.sample(th, s.look.prefs.editorFont)
		control = func(gtx C, th *ui.Theme) D {
			return s.fontControl(gtx, th, &s.look.editorFont, s.look.prefs.editorFont, func(family string) {
				p := s.look.prefs
				p.editorFont = family
				s.commitAppearance(p, editorFontKey, family)
			})
		}
	case costBudgetKey:
		r.description = i18n.T("settings.footerSettings.ceilingDescription")
		control = s.ceilingControl
	case copyOnSelectKey:
		r.description = i18n.T("settings.appearanceSettings.copyOnSelectDescription")
		control = s.copyOnSelectControl
	case closeActionKey:
		return s.closeBlock(gtx, th, r, first), true
	default:
		return D{}, false
	}
	return r.layout(gtx, th, first, control), true
}

func (s *settingsScreen) zoomControl(gtx C, th *ui.Theme) D {
	p := s.look.prefs
	return s.look.zoom.layout(gtx, th, s.root, stepperSpec{
		value: p.zoom, min: zoomMin, max: zoomMax, step: zoomStep, fallback: defaultZoom,
		display: fmt.Sprintf("%d%%", int(math.Round(p.zoom*100))), reset: i18n.T("settings.appearanceSettings.resetZoom"),
		dec: icZoomOut, inc: icZoomIn, decLabel: i18n.T("settings.appearanceSettings.zoomOut"), incLabel: i18n.T("settings.appearanceSettings.zoomIn"),
		onChange: func(v float64) {
			p.zoom = clampZoom(v)
			s.commitAppearance(p, zoomKey, strconv.FormatFloat(p.zoom, 'f', -1, 64))
		},
	})
}

func (s *settingsScreen) textSizeControl(gtx C, th *ui.Theme) D {
	p := s.look.prefs
	return s.look.size.layout(gtx, th, s.root, stepperSpec{
		value: float64(p.fontSize), min: terminalFontSizeMin, max: terminalFontSizeMax, step: terminalFontStep,
		fallback: defaultTerminalSize, display: fmt.Sprintf("%dpx", p.fontSize), reset: i18n.T("settings.appearanceSettings.resetTextSize"),
		dec: icMinus, inc: icPlus, decLabel: i18n.T("settings.appearanceSettings.textSmaller"), incLabel: i18n.T("settings.appearanceSettings.textLarger"),
		onChange: func(v float64) {
			p.fontSize = clampTerminalFontSize(int(math.Round(v)))
			s.commitAppearance(p, terminalFontSizeKey, strconv.Itoa(p.fontSize))
		},
	})
}

// copyOnSelectControl is the switch that copies a terminal selection as soon
// as it is made.
func (s *settingsScreen) copyOnSelectControl(gtx C, th *ui.Theme) D {
	if s.look.copy.Update(gtx) {
		p := s.look.prefs
		p.copyOnSelect = s.look.copy.Value
		s.commitAppearance(p, copyOnSelectKey, strconv.FormatBool(p.copyOnSelect))
	}
	s.look.copy.Value = s.look.prefs.copyOnSelect
	return th.Switch(&s.look.copy).Layout(gtx)
}

func (s *settingsScreen) editorSizeControl(gtx C, th *ui.Theme) D {
	p := s.look.prefs
	return s.look.editorSize.layout(gtx, th, s.root, stepperSpec{
		value: float64(p.editorFontSize), min: editorFontSizeMin, max: editorFontSizeMax, step: terminalFontStep,
		fallback: defaultEditorSize, display: fmt.Sprintf("%dpx", p.editorFontSize), reset: i18n.T("settings.settingsAppearance.editorSizeReset"),
		dec: icMinus, inc: icPlus, decLabel: i18n.T("settings.settingsAppearance.editorSmaller"), incLabel: i18n.T("settings.settingsAppearance.editorLarger"),
		onChange: func(v float64) {
			p.editorFontSize = clampEditorFontSize(int(math.Round(v)))
			s.commitAppearance(p, editorFontSizeKey, strconv.Itoa(p.editorFontSize))
		},
	})
}

// ceilingControl is SpendCeiling's w-40 mono field: an amount in dollars,
// empty for none. The typed text stays as typed, so a half-written decimal
// survives while the stored amount stays a number.
func (s *settingsScreen) ceilingControl(gtx C, th *ui.Theme) D {
	ed := &s.look.budget
	if !s.look.typed {
		ed.SetText(budgetText(s.look.prefs.budget))
		s.look.typed = true
	}
	ed.Filter = "0123456789."
	for {
		ev, ok := ed.Update(gtx)
		if !ok {
			break
		}
		if _, changed := ev.(widget.ChangeEvent); changed {
			p := s.look.prefs
			p.budget = clampCostBudget(parseNumber(strings.TrimSpace(ed.Text()), 0))
			s.commitAppearance(p, costBudgetKey, strconv.FormatFloat(p.budget, 'f', -1, 64))
		}
	}
	in := th.Input(ed, i18n.T("settings.footerSettings.ceilingPlaceholder"))
	in.Mono, in.Width = true, ceilingWidth
	return in.Layout(gtx)
}

func budgetText(budget float64) string {
	if budget <= 0 {
		return ""
	}
	return strconv.FormatFloat(budget, 'f', -1, 64)
}

// closeBlock is CloseSetting, a SettingBlock: the title and description,
// the three answers under them, and what the chosen one does.
func (s *settingsScreen) closeBlock(gtx C, th *ui.Theme, r settingRow, first bool) D {
	r.description = i18n.T("settings.closeSetting.description")
	if r.kind == rowGlobalOnly {
		return r.layout(gtx, th, first, nil)
	}
	g := &s.look.closeTo
	if g.Update(gtx) {
		p := s.look.prefs
		p.closeAction = closeActionOf(g.Value)
		s.commitAppearance(p, closeActionKey, p.closeAction)
	}
	g.Value = s.look.prefs.closeAction
	items := make([]ui.ToggleItem, len(closeChoices))
	consequence := ""
	for i, c := range closeChoices {
		items[i] = ui.ToggleItem{Value: c.value, Text: i18n.T(c.labelKey)}
		if c.value == g.Value {
			consequence = i18n.T(c.consequenceKey)
		}
	}
	under := func(gtx C, th *ui.Theme) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(th.ToggleGroup(g, items...).Layout),
			ui.VGap(2),
			layout.Rigid(th.Text(ui.TextXS, consequence).Muted().Layout),
		)
	}
	return r.layoutBlock(gtx, th, first, under)
}

// rowApplies drops a row that has nothing to set: the spend ceiling while
// the footer shows no cost, as FooterSettings mounts it, and the sandbox
// grants on a machine that cannot confine.
func (s *settingsScreen) rowApplies(e settingEntry) bool {
	switch e.key {
	case costBudgetKey:
		l, _ := s.footer.snapshot()
		return l.has("cost")
	case sshAgentKey, ghTokenKey:
		return s.sandbox.canConfine()
	}
	return true
}
