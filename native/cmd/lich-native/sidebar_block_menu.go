package main

import (
	"slices"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icLaunchTerminal = icons.Lucide("terminal")
	icCheckout       = icons.Lucide("folder-git-2")
	icUngroup        = icons.Lucide("ungroup")
)

// launchEntries is SessionLaunchMenuItems without its Worktree and Run items
// and its sandbox step: every enabled provider, then a terminal. open gets the
// kind, "shell" for the terminal.
func launchEntries(providers []launchProvider, terminal string, open func(kind string)) []ui.MenuEntry {
	entries := make([]ui.MenuEntry, 0, len(providers)+2)
	for _, p := range providers {
		entries = append(entries, ui.MenuEntry{Label: p.name, Icon: providerIcon(p.id), OnClick: func() { open(p.id) }})
	}
	return append(entries,
		ui.MenuEntry{Kind: ui.MenuSeparator},
		ui.MenuEntry{Label: terminal, Icon: icLaunchTerminal, OnClick: func() { open("shell") }},
	)
}

// folderLaunchEntries is FolderLaunchMenuItems: a folder of one checkout opens
// there and says so; with more, each checkout is a submenu. open gets the
// kind and the checkout.
func folderLaunchEntries(folder string, checkouts []launchCheckout, providers []launchProvider, open func(kind, dir string)) []ui.MenuEntry {
	heading := ui.MenuEntry{Kind: ui.MenuLabel, Label: folderLaunchHeading(folder, checkouts)}
	if len(checkouts) == 1 {
		only := checkouts[0]
		return append([]ui.MenuEntry{heading}, launchEntries(providers, i18n.T("sidebar.sessionLaunchMenuItems.newTerminal"), func(kind string) { open(kind, only.path) })...)
	}
	entries := []ui.MenuEntry{heading}
	for _, c := range checkouts {
		entries = append(entries, ui.MenuEntry{
			Kind: ui.MenuSubTrigger, Label: c.label, Icon: icCheckout,
			Sub: launchEntries(providers, i18n.T("sidebar.sessionLaunchMenuItems.newTerminal"), func(kind string) { open(kind, c.path) }),
		})
	}
	return entries
}

// blockLaunch is the + of a block's header: nil for the pinned block, which
// has nowhere to open a session.
func blockLaunch(b blockView, providers []launchProvider, act *sidebarAction) []ui.MenuEntry {
	switch b.kind {
	case blockPinned:
		return nil
	case blockFolder:
		return folderLaunchEntries(b.title, b.checkouts, providers, func(kind, dir string) {
			act.launch = launchRequest{kind: kind, dir: dir, folder: b.folder}
		})
	}
	return launchEntries(providers, i18n.T("sidebar.sessionLaunchMenuItems.newTerminal"), func(kind string) { act.launch = launchRequest{kind: kind, dir: b.path} })
}

// blockMenuHooks are the header's ways to act on its own state rather than
// write anything: start the folder rename, ask a new folder's name.
type blockMenuHooks struct {
	rename    func()
	newFolder func(ids []string)
}

// blockOptions is the … of a block's header (SessionGroupHeader): a folder
// renames, ungroups and recolors itself; a checkout's block files all its
// cards at once. The recolor and the filing are withheld under a filter, where
// the cards drawn are only the ones that matched. nil hides the button.
func blockOptions(b blockView, folders []string, filtering bool, act *sidebarAction, hooks blockMenuHooks) []ui.MenuEntry {
	ids := blockIDs(b)
	switch b.kind {
	case blockPinned:
		return nil
	case blockFolder:
		entries := []ui.MenuEntry{
			{Label: i18n.T("sidebar.sessionGroupHeader.renameFolder"), Icon: icMenuRename, OnClick: hooks.rename},
			{Label: i18n.T("sidebar.sessionGroupHeader.ungroup"), Icon: icUngroup, OnClick: func() { act.folder = folderRename{from: b.folder} }},
		}
		if filtering {
			return entries
		}
		return append(entries, ui.MenuEntry{Kind: ui.MenuSubTrigger, Label: i18n.T("sidebar.cardColor.menu"), Icon: icMenuColor, Columns: 3,
			Sub: colorSwatches(b.color, func(name string) { act.paint = paintRequest{ids: ids, color: name} })})
	}
	if filtering {
		return nil
	}
	return []ui.MenuEntry{{Kind: ui.MenuSubTrigger, Label: i18n.T("sidebar.sessionGroupHeader.moveToFolder"), Icon: icFolder, Sub: groupFolderMenu(ids, folders, act, hooks.newFolder)}}
}

// groupFolderMenu files a checkout's whole block: every folder, then a new one.
func groupFolderMenu(ids, folders []string, act *sidebarAction, newFolder func([]string)) []ui.MenuEntry {
	var sub []ui.MenuEntry
	for _, name := range folders {
		sub = append(sub, ui.MenuEntry{Label: name, Icon: icFolder, OnClick: func() { act.file = fileRequest{ids: ids, folder: name} }})
	}
	if len(sub) > 0 {
		sub = append(sub, ui.MenuEntry{Kind: ui.MenuSeparator})
	}
	return append(sub, ui.MenuEntry{Label: i18n.T("sidebar.sessionGroupHeader.newFolder"), Icon: icNewFolder, OnClick: func() { newFolder(ids) }})
}

func blockIDs(b blockView) []string {
	ids := make([]string, 0, len(b.sessions))
	for _, s := range b.sessions {
		ids = append(ids, s.ID)
	}
	return ids
}

// takesCard is file-drag-store.ts accepts: a folder's header takes any card
// not already in it, a checkout's takes one of its own cards back out of a
// folder, and the pinned block takes none.
func takesCard(target blockView, card sessionView) bool {
	switch target.kind {
	case blockPinned:
		return false
	case blockFolder:
		return target.folder != card.Folder
	}
	return card.Folder != "" && target.path == card.Path
}

// carriedCard is the session a drag holds, looked up in every block.
func carriedCard(blocks []blockView, id string) (sessionView, bool) {
	for _, b := range blocks {
		if i := slices.IndexFunc(b.sessions, func(s sessionView) bool { return s.ID == id }); i >= 0 {
			return b.sessions[i], true
		}
	}
	return sessionView{}, false
}

// mergesInto is the rename note's condition: the typed name is another folder
// of the project, so renaming files this one into it.
func mergesInto(name, typed string, folders []string) bool {
	return typed != "" && typed != name && slices.Contains(folders, typed)
}
