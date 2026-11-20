package main

// The dock's Code tab: a tree of the active session's checkout, tracked and
// untracked in a repository, whatever is on disk in a plain folder, with one
// field that filters it by name or searches the files' text. A file picked
// opens in the stage editor; the checkout's review batch sits under the tree.

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/filetree"
)

// plainFolderTick is how often a plain folder's tree re-reads. A plain folder
// has no git status to notice a file the agent just wrote, so it re-reads on
// the cadence an idle repository's poll settles at.
const plainFolderTick = 3 * time.Second

const (
	browseByName = "name"
	browseByText = "text"
)

type fileListing = backend.FileListing

// treeData is one read of the checkout: its files and each changed file's
// counts.
type treeData struct {
	listing fileListing
	stats   map[string]filetree.Delta
}

type codeTab struct {
	client   *backend.Client
	comments *commentStore
	notify   func(ui.Toast)
	// openFile opens rel of checkout in the stage editor, on line (1-based,
	// 0 for the top).
	openFile func(checkout, rel string, line int)

	// checkout is the one the browse below belongs to; another one starts it
	// over.
	checkout   string
	query      widget.Editor
	clearQuery ui.Clickable
	mode       ui.Enum
	toggled    map[string]bool
	selected   string // the file last opened, which reads as current
	// focusQuery asks the next browse box to take the keyboard.
	focusQuery bool

	tree    remote[*treeData]
	treeKey string
	search  remote[searchResult]
	settle  settler

	rows     filetree.Rows
	rowsFrom *treeData
	rowsFor  string
	hits     hitList
	batch    commentBatch
}

func newCodeTab(client *backend.Client, invalidate func(), comments *commentStore, notify func(ui.Toast), openFile func(checkout, rel string, line int)) *codeTab {
	c := &codeTab{client: client, comments: comments, notify: notify, openFile: openFile, toggled: map[string]bool{}}
	c.mode.Value = browseByName
	c.tree.invalidate, c.search.invalidate = invalidate, invalidate
	c.rows.List.Axis, c.hits.list.Axis = layout.Vertical, layout.Vertical
	return c
}

// hidden stops the reads while the tab is off screen; what they answered
// stays, so coming back draws it at once.
func (c *codeTab) hidden() {
	c.tree.stop()
	c.search.stop()
}

func (c *codeTab) layout(gtx C, th *ui.Theme, in dockInput) {
	c.follow(in.checkout)
	c.fetchTree(gtx, in)
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			return c.browse(gtx, th)
		}),
		layout.Rigid(func(gtx C) D { return c.batch.layout(gtx, th, c.comments, in.checkout, in.submit, c.notify) }),
	)
}

func (c *codeTab) focused(gtx C) bool {
	if gtx.Focused(&c.query) || gtx.Focused(&c.batch.note) {
		return true
	}
	for i := range c.hits.hits {
		if c.hits.clicks[i].Focused(gtx) {
			return true
		}
	}
	return false
}

// follow starts the browse over for another checkout: a worktree's folds and
// current file mean nothing in the next one.
func (c *codeTab) follow(checkout string) {
	if checkout == c.checkout {
		return
	}
	c.checkout, c.selected = checkout, ""
	c.query.SetText("")
	clear(c.toggled)
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

func readTree(ctx context.Context, client *backend.Client, path string) (*treeData, error) {
	l, err := client.Tree(ctx, path)
	if err != nil {
		return nil, err
	}
	// The diff only feeds the rows' +/- badges: a checkout with nothing to
	// diff answers an error here, and that means no badges, never no tree.
	stats := map[string]filetree.Delta{}
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

// browseBox is the one field over the tab and the switch saying how it reads.
func (c *codeTab) browseBox(gtx C, th *ui.Theme) D {
	c.fieldKeys(gtx)
	if c.focusQuery {
		c.focusQuery = false
		gtx.Execute(key.FocusCmd{Tag: &c.query})
		c.query.SetCaret(0, c.query.Len())
	}
	c.mode.Update(gtx)
	text := c.mode.Value == browseByText
	placeholder := i18n.T("dock.filesPanel.filterPlaceholder")
	if text {
		placeholder = i18n.T("dock.filesPanel.searchPlaceholder")
	}
	field := th.Input(&c.query, placeholder)
	field.Leading, field.Height, field.TextSize, field.Clear = icSearch, ui.Space(7), ui.TextXS, &c.clearQuery
	modes := th.ToggleGroup(&c.mode, ui.ToggleItem{Value: browseByName, Text: i18n.T("dock.filesPanel.byName")}, ui.ToggleItem{Value: browseByText, Text: i18n.T("dock.filesPanel.byText")})
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

// open shows rel in the stage editor, on line (1-based, 0 for the top).
func (c *codeTab) open(rel string, line int) {
	c.selected = rel
	c.openFile(c.checkout, rel, line)
}

// showing marks rel of checkout as the file the stage editor opened from
// elsewhere, a path in a terminal or an agent's show_code. It follows the
// checkout first: the session asked for may be the one the dock is about to
// come to.
func (c *codeTab) showing(checkout, rel string) {
	c.follow(checkout)
	c.selected = rel
}

// unfold opens folder rel of checkout and every folder over it, for a folder
// a path in an answer names.
func (c *codeTab) unfold(checkout, rel string) {
	c.follow(checkout)
	for d := rel; d != "." && d != "/"; d = path.Dir(d) {
		c.toggled[d] = true
	}
}

// requestSearch is FilesPanel's file-search request: the text search, with
// its field focused and selected.
func (c *codeTab) requestSearch() {
	c.mode.Value = browseByText
	c.focusQuery = true
}
