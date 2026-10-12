package main

// The dock's Code tab (FilesPanel.tsx): a read-only tree of the active
// session's checkout, tracked and untracked in a repository, whatever is on
// disk in a plain folder, with one field that filters it by name or searches
// the files' text, and an in-dock preview that covers it. Clicks only
// navigate, inject references into the session's PTY, or comment on lines for
// the checkout's review batch.

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n; the web's en catalog
// (dock.filesPanel).
const (
	codeSearchPlaceholder = "Search in files"
	codeFilterPlaceholder = "Filter by name"
	codeByName            = "Name"
	codeByText            = "Text"
	codeReadFailed        = "Could not read this folder"
	codeLoading           = "Loading…"
	codeNoMatch           = "No file matches"
	codeEmpty             = "No files here"
)

// plainFolderTick is use-git-status.ts GIT_POLL.slowMs. A plain folder has no
// git status to notice a file the agent just wrote, so its tree re-reads on
// the cadence an idle repository's poll settles at.
const plainFolderTick = 3 * time.Second

const (
	browseByName = "name"
	browseByText = "text"
)

// fileListing mirrors internal/project.FileListing.
type fileListing struct {
	Files  []string `json:"files"`
	Cut    bool     `json:"cut"`
	Hidden []string `json:"hidden"`
}

// treeData is one read of the checkout: its files and each changed file's
// counts.
type treeData struct {
	listing fileListing
	stats   map[string]lineDelta
}

type codeTab struct {
	client   *lichclient.Client
	comments *commentStore
	notify   func(ui.Toast)

	// checkout is the one the browse below belongs to; another one starts it
	// over.
	checkout   string
	query      widget.Editor
	clearQuery widget.Clickable
	mode       widget.Enum
	toggled    map[string]bool
	selected   string // the file last opened, which reads as current on Back
	// focusQuery asks the next browse box to take the keyboard.
	focusQuery bool

	tree    remote[*treeData]
	treeKey string
	search  remote[searchResult]
	settle  settler

	rows    treeRows
	hits    hitList
	preview *preview
	batch   commentBatch
}

func newCodeTab(client *lichclient.Client, invalidate func(), family string, comments *commentStore, notify func(ui.Toast)) (*codeTab, error) {
	view, err := godemirror.New(family, previewFontSize)
	if err != nil {
		return nil, err
	}
	c := &codeTab{client: client, comments: comments, notify: notify, toggled: map[string]bool{}}
	c.mode.Value = browseByName
	c.tree.invalidate, c.search.invalidate = invalidate, invalidate
	c.preview = newPreview(client, view, invalidate)
	c.rows.list.Axis, c.hits.list.Axis = layout.Vertical, layout.Vertical
	return c, nil
}

// hidden stops the reads while the tab is off screen; what they answered
// stays, so coming back draws it at once.
func (c *codeTab) hidden() {
	c.tree.stop()
	c.search.stop()
}

func (c *codeTab) layout(gtx C, th *ui.Theme, root *ui.Root, in dockInput) error {
	c.follow(in.checkout)
	c.fetchTree(gtx, in)
	inject := dockInjector(c.client, in.sessionID)
	var err error
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			if c.preview.isOpen() {
				var d D
				d, err = c.preview.layout(gtx, th, root, previewEnv{checkout: in.checkout, light: in.light, inject: inject, comments: c.comments})
				return d
			}
			return c.browse(gtx, th)
		}),
		layout.Rigid(func(gtx C) D { return c.batch.layout(gtx, th, c.comments, in.checkout, inject, c.notify) }),
	)
	return err
}

func (c *codeTab) focused(gtx C) bool {
	// The composer counts from the moment it opens, before its field takes the
	// focus it asks for on its first layout.
	if gtx.Focused(&c.query) || gtx.Focused(&c.batch.note) || c.preview.composer.isOpen() || c.preview.menu.Opened() {
		return true
	}
	for i := range c.hits.hits {
		if gtx.Focused(&c.hits.clicks[i]) {
			return true
		}
	}
	return false
}

// follow starts the browse over for another checkout: a worktree's folds and
// open file mean nothing in the next one.
func (c *codeTab) follow(checkout string) {
	if checkout == c.checkout {
		return
	}
	c.checkout, c.selected = checkout, ""
	c.query.SetText("")
	clear(c.toggled)
	c.preview.close()
}

// fetchTree keys the tree's read like FilesPanel: on the git status's counts
// in a repository, so a new or removed file shows without a watcher, and on a
// plainFolderTick outside one.
func (c *codeTab) fetchTree(gtx C, in dockInput) {
	path := in.checkout
	key := ""
	switch {
	case path == "":
	case in.isGit:
		key = fmt.Sprintf("%s %d %d %d", path, in.git.Files, in.git.Added, in.git.Deleted)
	default:
		tick := gtx.Now.Truncate(plainFolderTick)
		key = path + " " + strconv.FormatInt(tick.Unix(), 10)
		gtx.Execute(op.InvalidateCmd{At: tick.Add(plainFolderTick)})
	}
	c.treeKey = key
	c.tree.want(key, path, func(ctx context.Context) (*treeData, error) { return readTree(ctx, c.client, path) })
}

func readTree(ctx context.Context, client *lichclient.Client, path string) (*treeData, error) {
	var l fileListing
	if err := client.Call(ctx, "project.Tree", &l, path); err != nil {
		return nil, err
	}
	// The diff only feeds the rows' +/- badges: a checkout with nothing to
	// diff answers an error here, and that means no badges, never no tree.
	stats := map[string]lineDelta{}
	if text, err := client.DiffText(ctx, path); err == nil {
		if files, err := godemirror.ParsePatch(text); err == nil {
			stats = diffStatsByPath(files)
		}
	}
	return &treeData{listing: l, stats: stats}, nil
}

func (c *codeTab) browse(gtx C, th *ui.Theme) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return c.browseBox(gtx, th) }),
		layout.Rigid(th.Separator),
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			if c.mode.Value == browseByText {
				return c.searchBody(gtx, th)
			}
			return c.treeBody(gtx, th)
		}),
	)
}

// browseBox is BrowseBox: the one field over the tab, p-1.5 gap-1.5, and the
// switch saying how it reads.
func (c *codeTab) browseBox(gtx C, th *ui.Theme) D {
	c.fieldKeys(gtx)
	if c.focusQuery {
		c.focusQuery = false
		gtx.Execute(key.FocusCmd{Tag: &c.query})
		c.query.SetCaret(0, c.query.Len())
	}
	text := c.mode.Value == browseByText
	placeholder := codeFilterPlaceholder
	if text {
		placeholder = codeSearchPlaceholder
	}
	field := th.Input(&c.query, placeholder)
	field.Leading, field.Height, field.TextSize, field.Clear = icSearch, ui.Space(7), ui.TextXS, &c.clearQuery
	modes := th.ToggleGroup(&c.mode, ui.ToggleItem{Value: browseByName, Text: codeByName}, ui.ToggleItem{Value: browseByText, Text: codeByText})
	modes.Size, modes.ItemHeight, modes.ItemPadX, modes.ItemText = ui.ToggleSizeSM, ui.Space(5), ui.Space(2), ui.TextXS
	return layout.UniformInset(ui.Space(1.5)).Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, field.Layout),
			ui.Gap(1.5),
			layout.Rigid(modes.Layout),
		)
	})
}

// fieldKeys is BrowseBox's onKeyDown: Escape empties the field, and Down in
// text mode hands focus to the first hit.
func (c *codeTab) fieldKeys(gtx C) {
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &c.query, Name: key.NameEscape}, key.Filter{Focus: &c.query, Name: key.NameDownArrow})
		if !ok {
			return
		}
		e, isKey := ev.(key.Event)
		if !isKey || e.State != key.Press {
			continue
		}
		switch {
		case e.Name == key.NameEscape && c.query.Text() != "":
			c.query.SetText("")
		case e.Name == key.NameDownArrow && c.mode.Value == browseByText:
			c.hits.focus(gtx, 0)
		}
	}
}

// open shows rel in the preview, on line (1-based, 0 for the top).
func (c *codeTab) open(rel string, line int) {
	c.selected = rel
	c.preview.openAt(rel, line)
}

// requestSearch is FilesPanel's file-search request: the text search, over a
// closed preview, with its field focused and selected.
func (c *codeTab) requestSearch() {
	c.mode.Value = browseByText
	c.preview.close()
	c.focusQuery = true
}
