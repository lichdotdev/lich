package main

import (
	"context"
	"math"
	"strconv"

	"gioui.org/unit"

	"github.com/omartelo/lich/native/lichclient"
)

// ponytail: English until the native window has i18n.
const toastTerminalFontFailed = "Couldn't draw the terminal in %s: %v"

// The Appearance settings beyond the theme. The web keeps zoom, the terminal
// text size and font, and the spend ceiling in its own localStorage
// (providers/settings.tsx), out of this window's reach, so these keys are the
// native window's, named after the web's storage keys; only the close action
// is the web's own workspace setting.
const (
	zoomKey             = "lich.appearance.zoom"
	terminalFontSizeKey = "lich.terminal.fontSize"
	terminalFontKey     = "lich.terminal.font"
	costBudgetKey       = "lich.footer.costBudget"
	closeActionKey      = "window.closeAction" // close-request.ts CLOSE_ACTION_SETTING_KEY
)

// providers/settings.tsx's limits.
const (
	zoomMin             = 0.5
	zoomMax             = 2
	zoomStep            = 0.1
	defaultZoom         = 1
	terminalFontSizeMin = 8
	terminalFontSizeMax = 32
	terminalFontStep    = 1
	defaultTerminalSize = 14
)

// close-request.ts CLOSE_ACTIONS.
const (
	closeAsk        = "ask"
	closeBackground = "background"
	closeQuit       = "quit"
)

// appearancePrefs are the Appearance values the window applies.
type appearancePrefs struct {
	zoom        float64
	fontSize    int
	font        string
	budget      float64 // 0 is no ceiling
	closeAction string
}

// clampZoom is settings.tsx clampZoom: bounded, snapped to one decimal.
func clampZoom(v float64) float64 {
	return math.Round(math.Min(zoomMax, math.Max(zoomMin, v))*10) / 10
}

func clampTerminalFontSize(v int) int { return min(terminalFontSizeMax, max(terminalFontSizeMin, v)) }

// clampCostBudget is settings.tsx clampCostBudget: cents, never negative,
// and none for what is not a number.
func clampCostBudget(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(math.Max(0, v)*100) / 100
}

// closeActionOf is close-request.ts closeActionOf: nothing stored, or a value
// from a newer lich, asks.
func closeActionOf(stored string) string {
	switch stored {
	case closeBackground, closeQuit:
		return stored
	}
	return closeAsk
}

// closeQuits is close-request.ts closeOutcome: with no session running, or
// quit as the kept answer, closing quits; otherwise the user is asked.
func closeQuits(action string, live int) bool { return live == 0 || action == closeQuit }

// parseNumber is parseNumberPref: what does not parse is the fallback.
func parseNumber(raw string, fallback float64) float64 {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return v
}

// loadAppearancePrefs reads the stored values; fontFallback is the family
// the terminal opens in until one is chosen.
func loadAppearancePrefs(ctx context.Context, c *lichclient.Client, fontFallback string) (appearancePrefs, error) {
	raw := map[string]string{}
	for _, key := range []string{zoomKey, terminalFontSizeKey, terminalFontKey, costBudgetKey, closeActionKey} {
		v, err := c.GetSetting(ctx, key, "")
		if err != nil {
			return appearancePrefs{}, err
		}
		raw[key] = v
	}
	p := appearancePrefs{
		zoom:        clampZoom(parseNumber(raw[zoomKey], defaultZoom)),
		fontSize:    clampTerminalFontSize(int(math.Round(parseNumber(raw[terminalFontSizeKey], defaultTerminalSize)))),
		font:        raw[terminalFontKey],
		budget:      clampCostBudget(parseNumber(raw[costBudgetKey], 0)),
		closeAction: closeActionOf(raw[closeActionKey]),
	}
	if p.font == "" {
		p.font = fontFallback
	}
	return p, nil
}

// loadAppearance reads the Appearance values and applies them before the
// first frame; Settings hands every later change to applyAppearance.
func (u *window) loadAppearance() error {
	p, err := loadAppearancePrefs(u.ctx, u.client, *flagFont)
	if err != nil {
		return err
	}
	u.settings.look.prefs, u.settings.look.apply = p, u.applyAppearance
	u.applyAppearance(p)
	return nil
}

// applyAppearance puts p on screen: the interface zoom, the terminals'
// font, and the ceiling the cost readout colours against.
func (u *window) applyAppearance(p appearancePrefs) {
	u.zoom = p.zoom
	if err := u.terms.setFont(p.font, unit.Sp(p.fontSize)); err != nil {
		u.toaster.Show(failure(toastTerminalFontFailed, p.font, err))
	}
	u.footer.setBudget(p.budget)
	u.win.Invalidate()
}
