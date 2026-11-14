package main

import (
	"flag"
	"strconv"
	"strings"
)

var flagSettingsCatalog = flag.String("settings-catalog", "", "with -settings extensions, at start, for -shot: detail:<id> opens a listing's detail, install:<repository URL> stages it to its confirmation, filter:<tab>:<query> picks the nth category and types the query")

// catalogShot is -settings-catalog.
func (u *window) catalogShot() {
	mode, arg, _ := strings.Cut(*flagSettingsCatalog, ":")
	g := &u.settings.extensions.grid
	switch mode {
	case "detail":
		g.detail.show(arg)
	case "install":
		u.settings.prepareInstall(installFromCatalog, arg)
	case "filter":
		tab, query, _ := strings.Cut(arg, ":")
		g.tabs.Selected, _ = strconv.Atoi(tab)
		g.query.SetText(query)
	}
}
