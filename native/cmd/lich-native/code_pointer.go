package main

// Lines of a session's checkout brought to the dock's Code tab, by its agent
// (show_code) or by a Ctrl+click on a path:line in its terminal.

import (
	"context"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/gioterm"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// codeMenuMax bounds the files a click offers: a name like index.ts can match
// hundreds of them in a monorepo.
const codeMenuMax = 10

type codeTarget struct {
	session, rel string
	line, end    int
	comment      string
}

// codeMenu offers the files a clicked path:line could be.
type codeMenu struct {
	state   ui.MenuState
	choices []codeTarget
	total   int
}

// askCode hands do to the frame, the only goroutine that may touch the dock.
func (u *window) askCode(do func()) {
	u.codeAsk.Store(&do)
	u.terms.invalidate()
}

func (u *window) takeCodeAsk() {
	if do := u.codeAsk.Swap(nil); do != nil {
		(*do)()
	}
}

func (u *window) tellShowCode(s api.ShowCode) {
	u.askCode(func() {
		u.agentPointed(codeTarget{session: s.SessionID, rel: s.Rel, line: s.Line, end: s.EndLine, comment: s.Comment})
	})
}

// agentPointed opens the lines when the person is looking at the session. From
// anywhere else it only offers them: taking the screen would pull them off what
// they are typing into.
func (u *window) agentPointed(t codeTarget) {
	if !u.settings.open && u.model.sessionOnScreen(t.session) {
		u.openCode(t)
		return
	}
	if _, ok := u.model.sessionViewOf(t.session); !ok {
		return
	}
	label := u.model.sessionLabel(t.session)
	if label == "" {
		label = i18n.T("shell.projectEvents.unlabeledSession")
	}
	u.toaster.Show(ui.Toast{
		Title:  i18n.T("dock.showCode.pointed", i18n.Args{"session": label, "file": codeLabel(t)}),
		Action: &ui.ToastButton{Label: i18n.T("dock.showCode.show"), OnClick: func() { u.openCode(t) }},
	})
}

func codeLabel(t codeTarget) string {
	name := path.Base(t.rel)
	if t.line == 0 {
		return name
	}
	return name + ":" + formatLineRef(t.line, max(t.line, t.end))
}

// openCode brings the session up if it is not the active one, then its lines.
func (u *window) openCode(t codeTarget) {
	view, ok := u.model.sessionViewOf(t.session)
	if !ok {
		return
	}
	if !u.model.sessionOnScreen(t.session) {
		u.focusSession(view.projectID, t.session)
	}
	u.dock.tab = dockCode
	note := agentNote{text: t.comment, kind: view.Kind, author: u.model.providerName(view.Kind)}
	u.dock.code.showLines(view.path, t.rel, t.line, t.end, note)
}

func (m *model) sessionOnScreen(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, _ := m.sessionAt(id)
	return p != nil && m.shownProject() == p && m.active == id
}

// followCodeRef finds what a clicked path:line names, off the frame: the
// listing is a git call and the fallback reads the disk.
func (u *window) followCodeRef(session string, ref gioterm.CodeRef) {
	go func() {
		var matches []string
		if view, ok := u.model.sessionViewOf(session); ok {
			var err error
			if matches, err = checkoutMatches(u.ctx, u.client, view.path, ref); err != nil {
				u.logErr("list the checkout", err)
				u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("dock.filesPanel.readFailed")})
				return
			}
		}
		u.showCodeMatches(session, ref, matches)
	}()
}

// checkoutMatches searches the listing, then asks the backend about the path
// itself: the listing leaves out what git ignores, and the backend refuses a
// path outside the checkout.
func checkoutMatches(ctx context.Context, c *backend.Client, checkout string, ref gioterm.CodeRef) ([]string, error) {
	listing, err := c.Tree(ctx, checkout)
	if err != nil {
		return nil, err
	}
	if matches := codeRefMatches(listing.Files, checkout, ref); len(matches) > 0 {
		return matches, nil
	}
	rel, ok := codeRefRel(checkout, ref.Path)
	if !ok {
		return nil, nil
	}
	if _, err := c.ReadFile(ctx, checkout, rel); err != nil {
		return nil, nil
	}
	return []string{rel}, nil
}

func (u *window) showCodeMatches(session string, ref gioterm.CodeRef, matches []string) {
	switch len(matches) {
	case 0:
		u.toaster.Show(ui.Toast{Title: i18n.T("dock.showCode.notFound", i18n.Args{"file": filepath.Base(ref.Path)})})
	case 1:
		u.askCode(func() { u.openCode(codeTarget{session: session, rel: matches[0], line: ref.Line}) })
	default:
		var choices []codeTarget
		for _, rel := range matches[:min(len(matches), codeMenuMax)] {
			choices = append(choices, codeTarget{session: session, rel: rel, line: ref.Line})
		}
		u.askCode(func() {
			u.codeMenu.choices, u.codeMenu.total = choices, len(matches)
			u.codeMenu.state.OpenAtPointer(&u.root)
		})
	}
}

func (u *window) layoutCodeMenu(gtx C) {
	m := &u.codeMenu
	if !m.state.Opened() {
		return
	}
	entries := []ui.MenuEntry{{Kind: ui.MenuLabel, Label: i18n.T("dock.showCode.matches", i18n.Args{"count": m.total})}}
	for _, t := range m.choices {
		entries = append(entries, ui.MenuEntry{Label: t.rel + ":" + strconv.Itoa(t.line), OnClick: func() { u.openCode(t) }})
	}
	u.theme.ContextMenu(&u.root, &m.state, entries).Layout(gtx, func(C) D { return D{} })
}

// codeRefMatches is what ref can be among a checkout's files: the path itself
// when it is one, else every file it is the tail of, a program often printing
// only part of the path. A stack frame's package settles a tie, and is ignored
// when it matches nothing: a source root can sit under any directory.
func codeRefMatches(files []string, checkout string, ref gioterm.CodeRef) []string {
	rel, ok := codeRefRel(checkout, ref.Path)
	if !ok {
		return nil
	}
	if slices.Contains(files, rel) {
		return []string{rel}
	}
	tail := "/" + rel
	var matches []string
	for _, f := range files {
		if strings.HasSuffix(f, tail) {
			matches = append(matches, f)
		}
	}
	if len(matches) < 2 || ref.Package == "" {
		return matches
	}
	inPackage := "/" + ref.Package + "/" + path.Base(rel)
	var narrowed []string
	for _, f := range matches {
		if strings.HasSuffix("/"+f, inPackage) {
			narrowed = append(narrowed, f)
		}
	}
	if narrowed == nil {
		return matches
	}
	return narrowed
}

// codeRefRel is printed relative to checkout with forward slashes, false for a
// path outside it.
func codeRefRel(checkout, printed string) (string, bool) {
	if filepath.IsAbs(printed) {
		rel, err := filepath.Rel(checkout, printed)
		if err != nil {
			return "", false
		}
		printed = rel
	}
	rel := path.Clean(filepath.ToSlash(printed))
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return rel, true
}
