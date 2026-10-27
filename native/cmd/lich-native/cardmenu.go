package main

import (
	"slices"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icMenuRename = icons.Lucide("pencil")
	icMenuColor  = icons.Lucide("palette")
	icMenuPin    = icons.Lucide("pin")
	icMenuUnpin  = icons.Lucide("pin-off")
	icOpenIn     = icons.Lucide("folder-open")
	icBrowser    = icons.Lucide("globe")
	icInTerminal = icons.Lucide("terminal")
	icInEditor   = icons.Lucide("folder-code")
	icMenuPR     = icons.Lucide("git-pull-request-arrow")
	icMenuClose  = icons.Lucide("x")
	icTakeOut    = icons.Lucide("folder-minus")
	icNewFolder  = icons.Lucide("plus")
	icMenuFork   = icons.Lucide("git-fork")
)

// cardMenu is the session card's right-click menu, cut to what this window
// keeps; a pick lands in act, but Rename only turns the label into a field and
// New folder… asks for a name first. A pinned card offers no close: closing is
// what the pin withholds.
//
// extra are entries the window adds after Pull request, such as Continue in.
func cardMenu(sv *sessionView, folders []string, act *sidebarAction, startRename, newFolder func(), extra ...ui.MenuEntry) []ui.MenuEntry {
	pin := ui.MenuEntry{Label: i18n.T("sidebar.sessionCard.pin"), Icon: icMenuPin, OnClick: func() { act.pin, act.pinTo = sv.ID, true }}
	if sv.Pinned {
		pin = ui.MenuEntry{Label: i18n.T("sidebar.sessionCard.unpin"), Icon: icMenuUnpin, OnClick: func() { act.pin, act.pinTo = sv.ID, false }}
	}
	entries := []ui.MenuEntry{
		{Label: i18n.T("sidebar.sessionCard.rename"), Icon: icMenuRename, OnClick: startRename},
		pin,
		{Kind: ui.MenuSubTrigger, Label: i18n.T("sidebar.cardColor.menu"), Icon: icMenuColor, Columns: 3, Sub: colorSwatches(sv.Color, func(name string) {
			act.color, act.colorTo = sv.ID, name
		})},
		{Kind: ui.MenuSubTrigger, Label: i18n.T("sidebar.sessionCard.moveToFolder"), Icon: icFolder, Sub: cardFolderMenu(sv, folders, act, newFolder)},
		{Label: i18n.T("sidebar.sessionCard.browser"), Icon: icBrowser, OnClick: func() { act.browser = sv.ID }},
		{Kind: ui.MenuSubTrigger, Label: i18n.T("sidebar.sessionCard.openIn"), Icon: icOpenIn, Sub: openInMenu(sv, act)},
		pullRequestItem(sv, act),
	}
	if fork, ok := forkItem(sv, act); ok {
		entries = append(entries, fork)
	}
	entries = append(entries, extra...)
	if !sv.Pinned {
		entries = append(entries,
			ui.MenuEntry{Kind: ui.MenuSeparator},
			ui.MenuEntry{Label: i18n.T("sidebar.sessionCard.closeSession"), Icon: icMenuClose, Variant: ui.VariantDestructive, OnClick: func() { act.close = sv.ID }},
		)
	}
	return entries
}

// colorSwatches is CardColorMenu's grid: the theme first, then the palette,
// current checked.
func colorSwatches(current string, pick func(name string)) []ui.MenuEntry {
	picker := func(name string) func() { return func() { pick(name) } }
	swatches := []ui.MenuEntry{{Kind: ui.MenuSwatch, Label: i18n.T("sidebar.cardColor.color.theme"), Checked: current == "", OnClick: picker("")}}
	for _, name := range cardColorNames {
		swatches = append(swatches, ui.MenuEntry{
			Kind: ui.MenuSwatch, Label: cardColorLabel(name), Swatch: cardColors[name],
			Checked: current == name, OnClick: picker(name),
		})
	}
	return swatches
}

// cardColorLabel is a palette entry named in the interface language.
func cardColorLabel(name string) string {
	switch name {
	case "red":
		return i18n.T("sidebar.cardColor.color.red")
	case "orange":
		return i18n.T("sidebar.cardColor.color.orange")
	case "amber":
		return i18n.T("sidebar.cardColor.color.amber")
	case "green":
		return i18n.T("sidebar.cardColor.color.green")
	case "teal":
		return i18n.T("sidebar.cardColor.color.teal")
	case "blue":
		return i18n.T("sidebar.cardColor.color.blue")
	case "violet":
		return i18n.T("sidebar.cardColor.color.violet")
	}
	return i18n.T("sidebar.cardColor.color.pink")
}

// cardFolderMenu is the card's Move to folder: every other folder, the way
// out of its own, then a new one.
func cardFolderMenu(sv *sessionView, folders []string, act *sidebarAction, newFolder func()) []ui.MenuEntry {
	var sub []ui.MenuEntry
	for _, name := range folders {
		sub = append(sub, ui.MenuEntry{Label: name, Icon: icFolder, OnClick: func() { act.file = fileRequest{ids: []string{sv.ID}, folder: name} }})
	}
	if sv.Folder != "" {
		sub = append(sub, ui.MenuEntry{Label: i18n.T("sidebar.sessionCard.takeOutOf", i18n.Args{"folder": sv.Folder}), Icon: icTakeOut,
			OnClick: func() { act.file = fileRequest{ids: []string{sv.ID}} }})
	}
	if len(sub) > 0 {
		sub = append(sub, ui.MenuEntry{Kind: ui.MenuSeparator})
	}
	return append(sub, ui.MenuEntry{Label: i18n.T("sidebar.sessionCard.newFolder"), Icon: icNewFolder, OnClick: newFolder})
}

// openInMenu opens the directory the card shows. A shell session has no use
// for another terminal there.
func openInMenu(sv *sessionView, act *sidebarAction) []ui.MenuEntry {
	var sub []ui.MenuEntry
	if sv.Kind != "shell" {
		sub = append(sub, ui.MenuEntry{Label: i18n.T("sidebar.sessionCard.terminal"), Icon: icInTerminal, OnClick: func() { act.terminalAt = sv.shown }})
	}
	return append(sub,
		ui.MenuEntry{Label: i18n.T("sidebar.sessionCard.editor"), Icon: icInEditor, OnClick: func() { act.editorAt = sv.shown }},
		ui.MenuEntry{Label: i18n.T("sidebar.sessionCard.fileManager"), Icon: icOpenIn, OnClick: func() { act.folderAt = sv.shown }},
	)
}

// pullRequestItem opens the branch's PR on the Pulls screen, the session
// brought up with it; disabled while the branch has none.
// forkItem is Fork ▸ Here / Worktree… on a session whose provider branches
// conversations and that has one to branch. A provider that only resumes
// keeps the row, dead and saying why, so a missing Fork never reads as an
// oversight; a shell has nothing to fork and gets no row.
func forkItem(sv *sessionView, act *sidebarAction) (ui.MenuEntry, bool) {
	label := i18n.T("sidebar.sessionCard.fork")
	if !canFork(sv.Kind) {
		if !slices.Contains(resumableKinds, sv.Kind) {
			return ui.MenuEntry{}, false
		}
		return ui.MenuEntry{Label: label, Icon: icMenuFork, Disabled: true, Shortcut: i18n.T("sidebar.sessionCard.forkResumeOnly")}, true
	}
	if sv.ProviderSessionID == "" {
		return ui.MenuEntry{}, false
	}
	id := sv.ID
	return ui.MenuEntry{Kind: ui.MenuSubTrigger, Label: label, Icon: icMenuFork, Sub: []ui.MenuEntry{
		{Label: i18n.T("sidebar.sessionCard.forkHere"), Icon: icFolder, OnClick: func() { act.forkHere = id }},
		{Label: i18n.T("sidebar.sessionCard.forkWorktree"), Icon: icBranch, OnClick: func() { act.forkWorktree = id }},
	}}, true
}

func pullRequestItem(sv *sessionView, act *sidebarAction) ui.MenuEntry {
	item := ui.MenuEntry{Label: i18n.T("sidebar.sessionCard.pullRequest"), Icon: icMenuPR, Disabled: sv.pr == nil}
	if sv.pr != nil {
		id := sv.ID
		number, url := prTarget(sv.pr, sv.shown, sv.path)
		item.OnClick = func() { act.activate, act.openPR, act.openPRURL = id, number, url }
	}
	return item
}
