package main

import (
	"slices"

	"github.com/omartelo/lich/native/lichclient"
)

// The block keys of sidebar-groups.ts. The pinned and root blocks have no path
// to key them by, and a folder is prefixed so one named like a real checkout
// cannot key that checkout's block.
const (
	pinnedBlockKey  = "__pinned__"
	rootBlockKey    = "__root__"
	folderKeyPrefix = "__folder__:"
)

// blockKind is which block a session is drawn in, sidebar-groups.ts blockOf.
type blockKind int

const (
	blockCheckout blockKind = iota
	blockPinned
	blockFolder
)

// The marks a folded block draws for the cards it hides (collapsedMark).
const (
	markNone = ""
	markWait = "wait"
	markDone = "done"
)

// sessionBlock is one sidebar block over the stored session order. path is
// the checkout ("" for the project root) and folder the folder's name; both
// are "" for the kinds they do not apply to.
type sessionBlock struct {
	key      string
	kind     blockKind
	folder   string
	path     string
	sessions []lichclient.Session
}

// blockOf orders the checks by precedence: pin, then folder, then the
// session's own checkout. A wall (split group) outranks the pin on the web
// and would be the first check here once the native window has panes.
func blockOf(s lichclient.Session) blockKind {
	switch {
	case s.Pinned:
		return blockPinned
	case s.Folder != "":
		return blockFolder
	}
	return blockCheckout
}

func folderKey(name string) string { return folderKeyPrefix + name }

func checkoutKey(path string) string {
	if path == "" {
		return rootBlockKey
	}
	return path
}

func blockKeyOf(s lichclient.Session) string {
	if blockOf(s) == blockFolder {
		return folderKey(s.Folder)
	}
	return checkoutKey(s.Path)
}

// sessionBlocks is sidebar-groups.ts sidebarGroups without walls: the pinned
// block first, then folders and checkouts interleaved, each landing where its
// first card sits in the stored list and keeping that order inside. Block
// order lives nowhere else, which is what lets a drag move a folder past a
// checkout by moving its cards in the flat list.
func sessionBlocks(sessions []lichclient.Session) []sessionBlock {
	var blocks []sessionBlock
	var pinned []lichclient.Session
	at := map[string]int{}
	for _, s := range sessions {
		kind := blockOf(s)
		if kind == blockPinned {
			pinned = append(pinned, s)
			continue
		}
		key := blockKeyOf(s)
		i, ok := at[key]
		if !ok {
			i = len(blocks)
			at[key] = i
			b := sessionBlock{key: key, kind: kind, path: s.Path}
			if kind == blockFolder {
				b.folder, b.path = s.Folder, ""
			}
			blocks = append(blocks, b)
		}
		blocks[i].sessions = append(blocks[i].sessions, s)
	}
	if len(pinned) == 0 {
		return blocks
	}
	return append([]sessionBlock{{key: pinnedBlockKey, kind: blockPinned, sessions: pinned}}, blocks...)
}

// collapsedMark is what a folded block says about its hidden cards: a session
// waiting on the user outranks a finished turn nobody read, and neither says
// how many. It reads the notification queue, already "everything worth
// surfacing", so it needs no rule of its own about what counts as news.
func collapsedMark(pending []pendingStatus, ids []string) string {
	done := false
	for _, p := range pending {
		if !slices.Contains(ids, p.ID) {
			continue
		}
		if p.Status == statusWaiting {
			return markWait
		}
		done = done || p.Status == statusDone
	}
	if done {
		return markDone
	}
	return markNone
}

// checkoutsOf names the checkouts ("" for the project root) a block's cards
// belong to, once each in card order: where a folder's + can open a session.
func checkoutsOf(sessions []lichclient.Session) []string {
	var paths []string
	for _, s := range sessions {
		if !slices.Contains(paths, s.Path) {
			paths = append(paths, s.Path)
		}
	}
	return paths
}

// foldersOf names a project's folders in the order their first card sits,
// the order their blocks are drawn in. Read off the unfiltered list: a folder
// hidden by a filter is still one the user can file into.
func foldersOf(sessions []lichclient.Session) []string {
	var names []string
	for _, s := range sessions {
		if s.Folder != "" && !slices.Contains(names, s.Folder) {
			names = append(names, s.Folder)
		}
	}
	return names
}

// sharedColor is card-color.ts sharedColor: the palette color every card of a
// folder wears, "" when they differ or are untinted.
func sharedColor(sessions []lichclient.Session) string {
	if len(sessions) == 0 {
		return ""
	}
	first := sessions[0].Color
	if _, ok := cardColors[first]; !ok {
		return ""
	}
	for _, s := range sessions[1:] {
		if s.Color != first {
			return ""
		}
	}
	return first
}

// reorderSubset hands ids to the slots of the sessions member picks and
// leaves every other session where it is. ids not naming that subset exactly
// yields a repeat rather than a gap, which applyOrder (and the backend's
// caller) rejects whole: that is how a drag raced by a close is dropped.
func reorderSubset(stored []lichclient.Session, ids []string, member func(lichclient.Session) bool) []string {
	queue := slices.Clone(ids)
	out := make([]string, len(stored))
	for i, s := range stored {
		out[i] = s.ID
		if member(s) && len(queue) > 0 {
			out[i], queue = queue[0], queue[1:]
		}
	}
	return out
}

// orderBlocks is the flat id order laying blocks out in keys order, each
// keeping its own. A key naming no block contributes nothing, so the result
// fails applyOrder rather than silently dropping that block's sessions.
func orderBlocks(blocks []sessionBlock, keys []string) []string {
	var ids []string
	for _, key := range keys {
		if i := slices.IndexFunc(blocks, func(b sessionBlock) bool { return b.key == key }); i >= 0 {
			for _, s := range blocks[i].sessions {
				ids = append(ids, s.ID)
			}
		}
	}
	return ids
}

// dragOrder is the stored order that lays the movable blocks (all but the
// pinned one) out in keys. The slots it may fill are the cards those blocks
// drew, so a pinned card keeps its slot and lands back among its neighbors
// once unpinned.
func dragOrder(stored []lichclient.Session, blocks []sessionBlock, keys []string) []string {
	moved := map[string]bool{}
	var movable []sessionBlock
	for _, b := range blocks {
		if b.kind == blockPinned {
			continue
		}
		movable = append(movable, b)
		for _, s := range b.sessions {
			moved[s.ID] = true
		}
	}
	return reorderSubset(stored, orderBlocks(movable, keys), func(s lichclient.Session) bool { return moved[s.ID] })
}

// blockOrder is the order a drag inside block commits: ids fill the slots of
// the cards that block drew, asked of the block rather than restated as a rule
// about pins and paths (SessionSidebar.tsx commitGroupOrder).
func blockOrder(stored []lichclient.Session, block sessionBlock, ids []string) []string {
	drawn := map[string]bool{}
	for _, s := range block.sessions {
		drawn[s.ID] = true
	}
	return reorderSubset(stored, ids, func(s lichclient.Session) bool { return drawn[s.ID] })
}

// applyOrder is lib/reorder.ts applyOrder: sessions in ids order, false when
// ids does not name exactly that set (a close raced the drag).
func applyOrder(sessions []lichclient.Session, ids []string) ([]lichclient.Session, bool) {
	if len(ids) != len(sessions) {
		return nil, false
	}
	next := make([]lichclient.Session, 0, len(ids))
	for _, id := range ids {
		i := slices.IndexFunc(sessions, func(s lichclient.Session) bool { return s.ID == id })
		if i < 0 || slices.ContainsFunc(next, func(s lichclient.Session) bool { return s.ID == id }) {
			return nil, false
		}
		next = append(next, sessions[i])
	}
	return next, true
}

// fileSessions is sessions.ts setSessionsFolder: the sessions ids names filed
// under folder, or unfiled for "", the stored order untouched. false when ids
// names none of them.
func fileSessions(sessions []lichclient.Session, ids []string, folder string) ([]lichclient.Session, bool) {
	return refile(sessions, func(s lichclient.Session) bool { return slices.Contains(ids, s.ID) }, folder)
}

// renameFolder is sessions.ts renameFolder: every session filed under from
// moves to to, or out of any folder for "". An empty from is refused, or it
// would sweep every unfiled session into a folder nobody asked for.
func renameFolder(sessions []lichclient.Session, from, to string) ([]lichclient.Session, bool) {
	if from == "" {
		return sessions, false
	}
	return refile(sessions, func(s lichclient.Session) bool { return s.Folder == from }, to)
}

func refile(sessions []lichclient.Session, picked func(lichclient.Session) bool, folder string) ([]lichclient.Session, bool) {
	if !slices.ContainsFunc(sessions, picked) {
		return sessions, false
	}
	next := slices.Clone(sessions)
	for i := range next {
		if picked(next[i]) {
			next[i].Folder = folder
		}
	}
	return next, true
}
