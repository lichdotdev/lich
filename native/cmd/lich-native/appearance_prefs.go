package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"math"
	"strconv"
	"unicode"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/gioterm"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The Appearance settings beyond the theme, as backend settings. Renaming a key
// would orphan what is already stored under it.
const (
	zoomKey             = "lich.appearance.zoom"
	terminalFontSizeKey = "lich.terminal.fontSize"
	terminalFontKey     = "lich.terminal.font"
	editorFontSizeKey   = "lich.editor.fontSize"
	editorFontKey       = "lich.editor.font"
	uiFontKey           = "lich.appearance.font"
	costBudgetKey       = "lich.footer.costBudget"
	closeActionKey      = "window.closeAction"
)

// Bounds and steps for the zoom and the font sizes.
const (
	zoomMin             = 0.5
	zoomMax             = 2
	zoomStep            = 0.1
	defaultZoom         = 1
	terminalFontSizeMin = 8
	terminalFontSizeMax = 32
	terminalFontStep    = 1
	defaultTerminalSize = 14
	editorFontSizeMin   = 8
	editorFontSizeMax   = 32
	defaultEditorSize   = 14
)

// The values the close action setting takes.
const (
	closeAsk        = "ask"
	closeBackground = "background"
	closeQuit       = "quit"
)

// appearancePrefs are the Appearance values the window applies.
type appearancePrefs struct {
	zoom     float64
	fontSize int
	// The fonts are "" for System: the system's monospace for the terminal and
	// the code views, and its sans for the interface.
	font           string
	editorFontSize int
	editorFont     string
	uiFont         string
	budget         float64 // 0 is no ceiling
	closeAction    string
}

// clampZoom bounds a zoom and snaps it to one decimal.
func clampZoom(v float64) float64 {
	return math.Round(math.Min(zoomMax, math.Max(zoomMin, v))*10) / 10
}

func clampTerminalFontSize(v int) int { return min(terminalFontSizeMax, max(terminalFontSizeMin, v)) }

func clampEditorFontSize(v int) int { return min(editorFontSizeMax, max(editorFontSizeMin, v)) }

// systemOr is family, or the fontconfig alias for the system's when it is
// "".
func systemOr(family, alias string) string {
	if family == "" {
		return alias
	}
	return family
}

func (p appearancePrefs) terminalFamily() string { return systemOr(p.font, fonts.SystemMono) }

func (p appearancePrefs) codeFamily() string { return systemOr(p.editorFont, fonts.SystemMono) }

// clampCostBudget keeps a budget in cents, never negative, and none for what is
// not a number.
func clampCostBudget(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(math.Max(0, v)*100) / 100
}

// closeActionOf reads the stored close action: nothing stored, or a value from
// a newer lich, asks.
func closeActionOf(stored string) string {
	switch stored {
	case closeBackground, closeQuit:
		return stored
	}
	return closeAsk
}

// closeQuits reports whether closing quits: with no session running, or quit as
// the kept answer, it does; otherwise the user is asked.
func closeQuits(action string, live int) bool { return live == 0 || action == closeQuit }

// parseNumber parses raw; what does not parse is the fallback.
func parseNumber(raw string, fallback float64) float64 {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return v
}

// loadAppearancePrefs reads the stored values.
func loadAppearancePrefs(ctx context.Context, c *backend.Client) (appearancePrefs, error) {
	raw := map[string]string{}
	for _, key := range []string{zoomKey, terminalFontSizeKey, terminalFontKey, editorFontSizeKey, editorFontKey, uiFontKey, costBudgetKey, closeActionKey} {
		v, err := c.GetSetting(ctx, key, "")
		if err != nil {
			return appearancePrefs{}, err
		}
		raw[key] = v
	}
	p := appearancePrefs{
		zoom:           clampZoom(parseNumber(raw[zoomKey], defaultZoom)),
		fontSize:       clampTerminalFontSize(int(math.Round(parseNumber(raw[terminalFontSizeKey], defaultTerminalSize)))),
		font:           raw[terminalFontKey],
		editorFontSize: clampEditorFontSize(int(math.Round(parseNumber(raw[editorFontSizeKey], defaultEditorSize)))),
		editorFont:     raw[editorFontKey],
		uiFont:         raw[uiFontKey],
		budget:         clampCostBudget(parseNumber(raw[costBudgetKey], 0)),
		closeAction:    closeActionOf(raw[closeActionKey]),
	}
	return p, nil
}

// loadAppearance reads the Appearance values and applies them before the
// first frame; Settings hands every later change to applyAppearance.
func (u *window) loadAppearance() error {
	p, err := loadAppearancePrefs(u.ctx, u.client)
	if err != nil {
		return err
	}
	u.settings.look.prefs, u.settings.look.apply = p, u.applyAppearance
	u.applyAppearance(p)
	return nil
}

// fontFailure is the error toast for a font the window could not draw in.
func fontFailure(title string) ui.Toast {
	return ui.Toast{Variant: ui.ToastError, Title: title}
}

// applyAppearance puts p on screen: the interface zoom, the terminals'
// font, and the ceiling the cost readout colours against.
func (u *window) applyAppearance(p appearancePrefs) {
	u.zoom = p.zoom
	if u.caption.leftReserve > 0 {
		u.win.Option(app.TrafficLights(topbarHeight * unit.Dp(p.zoom)))
	}
	if err := u.terms.setFont(p.terminalFamily(), unit.Sp(p.fontSize)); err != nil {
		u.toaster.Show(fontFailure(i18n.T("settings.appearancePrefs.terminalFontFailed", i18n.Args{"family": p.terminalFamily(), "error": backend.ErrorText(err)})))
	}
	if err := u.setCodeFont(p.codeFamily(), unit.Sp(p.editorFontSize)); err != nil {
		u.toaster.Show(fontFailure(i18n.T("settings.appearancePrefs.editorFontFailed", i18n.Args{"family": p.codeFamily(), "error": backend.ErrorText(err)})))
	}
	if err := u.setInterfaceFonts(interfaceFonts{sans: p.uiFont, mono: p.codeFamily()}); err != nil {
		u.toaster.Show(fontFailure(i18n.T("settings.appearancePrefs.uiFontFailed", i18n.Args{"family": systemOr(p.uiFont, fonts.SystemSans), "error": backend.ErrorText(err)})))
	}
	u.footer.setBudget(p.budget)
	u.win.Invalidate()
}

// setCodeFont redraws the Code tab and both review panels in family at size.
func (u *window) setCodeFont(family string, size unit.Sp) error {
	if err := u.dock.code.preview.setFont(family, size); err != nil {
		return err
	}
	if err := u.review.setFont(family, size); err != nil {
		return err
	}
	return u.pulls.view.files.setFont(family, size)
}

// onestFamily is the bundled interface face, offered beside the installed
// families.
const onestFamily = "Onest"

// interfaceFonts are the interface's two faces as stored, its sans ("" for
// System) and its monospace, the code's family, the language its CJK faces
// are picked for, and whether the shaper holds them: from the start for a
// CJK language, and for any other once Han, kana or Hangul has come up on
// screen (noteOutsideFaces), since a CJK face is tens of megabytes.
type interfaceFonts struct {
	sans, mono string
	language   i18n.Locale
	han        bool
}

// setInterfaceFonts builds the interface's shaper again when its sans or its
// monospace changed: System's sans for "", Onest from the binary, any other
// through fonts.Load, the code's family for the paths, file names and
// counts the interface sets in monospace, and hanFaces for the CJK the
// other two cannot draw, in the current interface language's own shapes.
func (u *window) setInterfaceFonts(want interfaceFonts) error {
	want.language = i18n.Current()
	want.han = u.cjkSeen.Load() || isCJKLanguage(want.language)
	if u.uiFonts != nil && *u.uiFonts == want {
		return nil
	}
	sans, sansTypeface, err := uiFaces(want.sans)
	if err != nil {
		return err
	}
	mono, err := gioterm.Faces(want.mono)
	if err != nil {
		return err
	}
	var han []font.FontFace
	if want.han {
		if han, err = hanFaces(want.language); err != nil {
			return err
		}
	}
	collection := append(append(append(gofont.Collection(), sans...), mono...), han...)
	shaper := text.NewShaper(text.WithCollection(collection), text.WithOutsideCollection(u.noteOutsideFaces))
	u.th.Shaper, u.theme.Shaper = shaper, shaper
	u.theme.Sans, u.theme.Mono = sansTypeface, mono[0].Font.Typeface
	u.uiFonts = &want
	return nil
}

// noteOutsideFaces hears of a rune the interface's faces lack, which the
// shaper meanwhile draws with a system font if one has it. The first Han,
// kana or Hangul has followLanguage build the shaper again with the CJK
// faces; it runs inside a layout, which a new shaper must not replace, so it
// only marks it and asks for a frame.
func (u *window) noteOutsideFaces(r rune) {
	if !unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
		return
	}
	if u.cjkSeen.CompareAndSwap(false, true) {
		u.win.Invalidate()
	}
}

func isCJKLanguage(l i18n.Locale) bool {
	switch l {
	case i18n.ZhCN, i18n.JA, i18n.KO:
		return true
	}
	return false
}

// reloadInterfaceFonts builds the interface's shaper again from the same
// stored fonts, for a CJK font downloaded since.
func (u *window) reloadInterfaceFonts() error {
	want := *u.uiFonts
	u.uiFonts = nil
	return u.setInterfaceFonts(want)
}

// hanFaces are the faces the shaper falls back to for CJK: an installed one
// for the interface's language (Simplified Chinese's for a language that is
// not CJK), else the Noto Sans set lich downloaded for it, else none. A
// download that no longer hashes right counts as none, so choosing the
// language offers it again.
func hanFaces(language i18n.Locale) ([]font.FontFace, error) {
	installed, err := fonts.InstalledHan(string(language))
	if err != nil || len(installed) > 0 {
		return installed, err
	}
	dir, err := fonts.DownloadDir()
	if err != nil {
		return nil, err
	}
	family, files := fonts.NotoSansFor(string(language))
	downloaded, err := fonts.LoadDownloaded(files, dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		log.Printf("fonts: downloaded %s unusable: %v", family, err)
		return nil, nil
	}
	return downloaded, nil
}

func uiFaces(stored string) ([]font.FontFace, font.Typeface, error) {
	if stored == onestFamily {
		faces, err := fonts.Faces()
		return faces, fonts.Sans, err
	}
	family := systemOr(stored, fonts.SystemSans)
	faces, err := fonts.Load(family)
	return faces, font.Typeface(family), err
}
