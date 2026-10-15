package main

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n (settings.appearanceSettings,
// settings.footerSettings, settings.closeSetting, common.stepper).
const (
	zoomTitle     = "Zoom"
	zoomOutLabel  = "Zoom out"
	zoomInLabel   = "Zoom in"
	zoomReset     = "Reset the zoom"
	textSizeTitle = "Terminal text size"
	textSmaller   = "Smaller terminal text"
	textLarger    = "Larger terminal text"
	textSizeReset = "Reset the terminal text size"

	editorSizeTitle = "Editor text size"
	editorSizeAlso  = "code review font size"
	editorSmaller   = "Smaller code text"
	editorLarger    = "Larger code text"
	editorSizeReset = "Reset the editor text size"
	editorFontTitle = "Editor font"
	uiFontTitle     = "Interface font"
	uiFontAlso      = "ui sans text"
	editorFontAlso  = "code review typeface monospace"
	stepperDefault  = "Default"

	ceilingTitle       = "Spend ceiling"
	ceilingAlso        = "cost budget usd"
	ceilingDesc        = "Warn as the session's API cost nears this amount: amber at 80%, red at 95%. Empty for none."
	ceilingPlaceholder = "No ceiling"

	closeSettingTitle = "When the window closes"
	closeAlso         = "quit exit background close window tray keep running"
	closeDesc         = "What lich does with its sessions when you close its window. With none running, closing the window quits lich."
)

// closeChoices are CloseSetting's three answers, each with its label and
// the line saying what it does.
var closeChoices = []struct{ value, label, consequence string }{
	{closeAsk, "Ask", "lich asks each time whether to keep running in the background."},
	{closeBackground, "Keep running", "The window closes and the sessions keep working; opening lich again brings them back."},
	{closeQuit, "Quit", "Closing the window ends lich and every session in it."},
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
	budget     widget.Editor
	typed      bool // the ceiling field holds the user's text, not the stored value
	closeTo    widget.Enum
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
		r.description = ceilingDesc
		control = s.ceilingControl
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
		display: fmt.Sprintf("%d%%", int(math.Round(p.zoom*100))), reset: zoomReset,
		dec: icZoomOut, inc: icZoomIn, decLabel: zoomOutLabel, incLabel: zoomInLabel,
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
		fallback: defaultTerminalSize, display: fmt.Sprintf("%dpx", p.fontSize), reset: textSizeReset,
		dec: icMinus, inc: icPlus, decLabel: textSmaller, incLabel: textLarger,
		onChange: func(v float64) {
			p.fontSize = clampTerminalFontSize(int(math.Round(v)))
			s.commitAppearance(p, terminalFontSizeKey, strconv.Itoa(p.fontSize))
		},
	})
}

func (s *settingsScreen) editorSizeControl(gtx C, th *ui.Theme) D {
	p := s.look.prefs
	return s.look.editorSize.layout(gtx, th, s.root, stepperSpec{
		value: float64(p.editorFontSize), min: editorFontSizeMin, max: editorFontSizeMax, step: terminalFontStep,
		fallback: defaultEditorSize, display: fmt.Sprintf("%dpx", p.editorFontSize), reset: editorSizeReset,
		dec: icMinus, inc: icPlus, decLabel: editorSmaller, incLabel: editorLarger,
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
	in := th.Input(ed, ceilingPlaceholder)
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
	r.description = closeDesc
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
		items[i] = ui.ToggleItem{Value: c.value, Text: c.label}
		if c.value == g.Value {
			consequence = c.consequence
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
