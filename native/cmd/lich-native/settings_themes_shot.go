package main

import (
	"flag"
	"fmt"
)

var (
	flagThemeDetail = flag.String("settings-theme-detail", "", "open Settings › Appearance › Themes with this listed pack's detail at start, for -shot")
	flagThemeTry    = flag.String("settings-theme-try", "", "open Settings › Appearance › Themes wearing this listed theme on approval at start, for -shot")
)

// themesShot is -settings-theme-detail and -settings-theme-try: the catalog is
// read before the first frame, so the pack is there to open.
func (u *window) themesShot() error {
	if *flagThemeDetail == "" && *flagThemeTry == "" {
		return nil
	}
	u.settings.section, u.settings.page = sectionAppearance, themesPage
	u.settings.show("", nil)
	cat, err := u.client.ThemeCatalog(u.ctx)
	if err != nil {
		return err
	}
	u.model.themeCatalog.set(cat)
	read, _ := u.model.themeCatalog.read()
	p := u.settings.theme
	if pack, ok := read.pack(*flagThemeDetail); ok {
		p.grid.detail.show(pack, u.appearance.view().selected)
	}
	if *flagThemeTry == "" {
		return nil
	}
	for _, pack := range read.packs {
		for _, t := range pack.themes {
			if t.def.ID == *flagThemeTry {
				p.bar.packID = pack.def.ID
				u.appearance.try(t.themeEntry)
				return nil
			}
		}
	}
	return fmt.Errorf("-settings-theme-try %q: the catalog lists no such theme", *flagThemeTry)
}
