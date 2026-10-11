package main

import (
	"fmt"
	"strings"

	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

var (
	icMenuRename = icons.Lucide("pencil")
	icMenuColor  = icons.Lucide("palette")
	icMenuPin    = icons.Lucide("pin")
	icMenuUnpin  = icons.Lucide("pin-off")
	icOpenIn     = icons.Lucide("folder-open")
	icInTerminal = icons.Lucide("terminal")
	icInEditor   = icons.Lucide("folder-code")
	icMenuPR     = icons.Lucide("git-pull-request-arrow")
	icMenuClose  = icons.Lucide("x")
	icTakeOut    = icons.Lucide("folder-minus")
	icNewFolder  = icons.Lucide("plus")
)

// ponytail: English until the native window has i18n.
const (
	moveToFolderLabel = "Move to folder"
	takeOutOfLabel    = "Take out of %s"
	newFolderLabel    = "New folder…"
)

// cardMenu is the session card's right-click menu, cut to what this window
// keeps; a pick lands in act, but Rename only turns the label into a field and
// New folder… asks for a name first. A pinned card offers no close: closing is
// what the pin withholds.
//
// ponytail: English labels until the native window has i18n.
func cardMenu(sv *sessionView, folders []string, act *sidebarAction, startRename, newFolder func()) []ui.MenuEntry {
	pin := ui.MenuEntry{Label: "Pin", Icon: icMenuPin, OnClick: func() { act.pin, act.pinTo = sv.ID, true }}
	if sv.Pinned {
		pin = ui.MenuEntry{Label: "Unpin", Icon: icMenuUnpin, OnClick: func() { act.pin, act.pinTo = sv.ID, false }}
	}
	entries := []ui.MenuEntry{
		{Label: "Rename", Icon: icMenuRename, OnClick: startRename},
		pin,
		{Kind: ui.MenuSubTrigger, Label: "Color", Icon: icMenuColor, Columns: 3, Sub: colorSwatches(sv.Color, func(name string) {
			act.color, act.colorTo = sv.ID, name
		})},
		{Kind: ui.MenuSubTrigger, Label: moveToFolderLabel, Icon: icFolder, Sub: cardFolderMenu(sv, folders, act, newFolder)},
		{Kind: ui.MenuSubTrigger, Label: "Open in", Icon: icOpenIn, Sub: openInMenu(sv, act)},
		pullRequestItem(sv, act),
	}
	if !sv.Pinned {
		entries = append(entries,
			ui.MenuEntry{Kind: ui.MenuSeparator},
			ui.MenuEntry{Label: "Close session", Icon: icMenuClose, Variant: ui.VariantDestructive, OnClick: func() { act.close = sv.ID }},
		)
	}
	return entries
}

// colorSwatches is CardColorMenu's grid: the theme first, then the palette,
// current checked.
func colorSwatches(current string, pick func(name string)) []ui.MenuEntry {
	picker := func(name string) func() { return func() { pick(name) } }
	swatches := []ui.MenuEntry{{Kind: ui.MenuSwatch, Label: "Theme", Checked: current == "", OnClick: picker("")}}
	for _, name := range cardColorNames {
		swatches = append(swatches, ui.MenuEntry{
			Kind: ui.MenuSwatch, Label: strings.ToUpper(name[:1]) + name[1:], Swatch: cardColors[name],
			Checked: current == name, OnClick: picker(name),
		})
	}
	return swatches
}

// cardFolderMenu is the card's Move to folder: every other folder, the way
// out of its own, then a new one.
func cardFolderMenu(sv *sessionView, folders []string, act *sidebarAction, newFolder func()) []ui.MenuEntry {
	var sub []ui.MenuEntry
	for _, name := range folders {
		sub = append(sub, ui.MenuEntry{Label: name, Icon: icFolder, OnClick: func() { act.file = fileRequest{ids: []string{sv.ID}, folder: name} }})
	}
	if sv.Folder != "" {
		sub = append(sub, ui.MenuEntry{Label: fmt.Sprintf(takeOutOfLabel, sv.Folder), Icon: icTakeOut,
			OnClick: func() { act.file = fileRequest{ids: []string{sv.ID}} }})
	}
	if len(sub) > 0 {
		sub = append(sub, ui.MenuEntry{Kind: ui.MenuSeparator})
	}
	return append(sub, ui.MenuEntry{Label: newFolderLabel, Icon: icNewFolder, OnClick: newFolder})
}

// openInMenu opens the directory the card shows. A shell session has no use
// for another terminal there.
func openInMenu(sv *sessionView, act *sidebarAction) []ui.MenuEntry {
	var sub []ui.MenuEntry
	if sv.Kind != "shell" {
		sub = append(sub, ui.MenuEntry{Label: "Terminal", Icon: icInTerminal, OnClick: func() { act.terminalAt = sv.shown }})
	}
	return append(sub,
		ui.MenuEntry{Label: "Editor", Icon: icInEditor, OnClick: func() { act.editorAt = sv.shown }},
		ui.MenuEntry{Label: "File manager", Icon: icOpenIn, OnClick: func() { act.folderAt = sv.shown }},
	)
}

// pullRequestItem opens the branch's PR in the browser, this window having no
// Pulls screen to open it in; disabled while the branch has none.
func pullRequestItem(sv *sessionView, act *sidebarAction) ui.MenuEntry {
	item := ui.MenuEntry{Label: "Pull request", Icon: icMenuPR, Disabled: sv.pr == nil}
	if sv.pr != nil {
		url := sv.pr.URL
		item.OnClick = func() { act.openURL = url }
	}
	return item
}
