package main

import (
	"context"
	"log"
	"sync"

	"github.com/omartelo/lich/native/lichclient"
)

// themeSettingKey is providers/settings.tsx THEME_SETTING_KEY, a global
// setting: "system" (the web's default), "light", "dark", or the id of a
// custom theme.
const themeSettingKey = "appearance.theme"

const (
	themeLight = "light"
	themeDark  = "dark"
)

// appearance is the window's theme state: the themes the backend has, the
// stored selection and the OS's scheme. Goroutines change it; the frame
// takes the theme it resolves to whenever it changed and repaints.
type appearance struct {
	ctx        context.Context
	client     *lichclient.Client
	invalidate func()

	mu         sync.Mutex
	themes     []themeEntry
	broken     []lichclient.BrokenTheme
	selected   string
	systemDark bool
	changed    bool
}

// appearanceView is what the picker draws from.
type appearanceView struct {
	themes   []themeEntry
	broken   []lichclient.BrokenTheme
	selected string
	resolved themeEntry
}

func (a *appearance) view() appearanceView {
	a.mu.Lock()
	defer a.mu.Unlock()
	return appearanceView{themes: a.themes, broken: a.broken, selected: a.selected,
		resolved: resolveTheme(a.selected, a.themes, a.systemDark)}
}

// take returns the theme to draw with when anything it resolves from
// changed since the last take.
func (a *appearance) take() (themeEntry, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.changed || len(a.themes) == 0 {
		return themeEntry{}, false
	}
	a.changed = false
	return resolveTheme(a.selected, a.themes, a.systemDark), true
}

func (a *appearance) update(change func()) {
	a.mu.Lock()
	change()
	a.changed = true
	a.mu.Unlock()
	a.invalidate()
}

// choose selects id and stores it where the web reads it.
func (a *appearance) choose(id string) {
	a.update(func() { a.selected = id })
	go func() {
		if err := a.client.SetSetting(a.ctx, themeSettingKey, "", id); err != nil {
			log.Printf("appearance: save theme: %v", err)
		}
	}()
}

func (a *appearance) setSystemDark(dark bool) { a.update(func() { a.systemDark = dark }) }

// reload rereads the themes, after an install, an update or a removal.
func (a *appearance) reload() error {
	defs, err := a.client.Themes(a.ctx)
	if err != nil {
		return err
	}
	broken, err := a.client.BrokenThemes(a.ctx)
	if err != nil {
		return err
	}
	themes, broken := readThemes(defs, broken)
	for _, b := range broken {
		log.Printf("appearance: theme %s can't load: %s", b.ID, b.Reason)
	}
	a.update(func() { a.themes, a.broken = themes, broken })
	return nil
}

// load reads the stored selection, the themes and the OS's scheme before the
// first frame, and follows the OS's scheme from then on. A setting never
// written reads "", which is the web's default, System.
func (a *appearance) load() error {
	stored, err := a.client.GetSetting(a.ctx, themeSettingKey, "")
	if err != nil {
		return err
	}
	if stored == "" {
		stored = themeSystem
	}
	a.selected = stored
	a.systemDark = watchSystemDark(a.ctx, a.setSystemDark)
	return a.reload()
}

// paintTheme repaints the whole window from e: the kit theme every widget
// draws with, and the terminals' default colors.
func (u *window) paintTheme(e themeEntry) {
	u.light = e.light()
	p := e.palette
	u.theme.Palette, u.theme.Surface = p, p.Canvas
	u.th.Fg, u.th.Bg = p.Foreground, p.Background
	u.terms.setColors(e.terminal)
}

// takeTheme repaints the window when the theme it resolves to changed.
func (u *window) takeTheme() {
	if e, ok := u.appearance.take(); ok {
		u.paintTheme(e)
	}
}
