package main

import (
	"image/color"

	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// themeSettingKey is providers/settings.tsx THEME_SETTING_KEY, a global
// setting: "system" (the web's default), "light", "dark", or the id of a
// custom theme.
const themeSettingKey = "appearance.theme"

const (
	themeLight = "light"
	themeDark  = "dark"
)

// ponytail: English until the native window has i18n.
const (
	lightThemeHint = "Light theme"
	darkThemeHint  = "Dark theme"
)

var (
	icSun  = icons.Lucide("sun")
	icMoon = icons.Lucide("moon")
)

// termColors are a theme's terminal defaults, themes/<id>.json "terminal".
type termColors struct{ fg, bg color.NRGBA }

var (
	lightTerminal = termColors{fg: rgb(0x1f2328), bg: rgb(0xe8e8ea)}
	darkTerminal  = termColors{fg: rgb(0xe5e7eb), bg: rgb(0x06070f)}
)

func rgb(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}
}

// storedLight maps the web's selection onto the two palettes this window
// has. ponytail: only "light" draws light; "system" would follow the OS,
// which this window does not read yet, and a custom theme's palette is not
// ported, so both draw dark.
func storedLight(stored string) bool { return stored == themeLight }

func storedTheme(light bool) string {
	if light {
		return themeLight
	}
	return themeDark
}

// setLight repaints the whole window from the light or the dark palette:
// the kit theme every widget draws with, and the terminals' default colors.
func (u *window) setLight(light bool) {
	u.light = light
	p, term := ui.Dark, darkTerminal
	if light {
		p, term = ui.Light, lightTerminal
	}
	u.theme.Palette, u.theme.Surface = p, p.Canvas
	u.th.Fg, u.th.Bg = p.Foreground, p.Background
	u.terms.setColors(term)
}

// toggleTheme is the topbar's stand-in for Settings › Appearance: it flips
// the palette and stores the choice where the web reads it.
func (u *window) toggleTheme() {
	u.setLight(!u.light)
	value := storedTheme(u.light)
	go func() { u.logErr("save theme", u.client.SetSetting(u.ctx, themeSettingKey, "", value)) }()
}

// loadTheme reads the stored selection before the first frame.
func (u *window) loadTheme() error {
	stored, err := u.client.GetSetting(u.ctx, themeSettingKey, "")
	if err != nil {
		return err
	}
	u.setLight(storedLight(stored))
	return nil
}
