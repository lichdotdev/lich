package main

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/ui"
)

// mentionSigil opens the list of the checkout's paths at the start of a word,
// as Claude Code's prompt does.
const mentionSigil = "@"

// chatMention is the composer's list of the checkout's folders and files,
// open while the word at the caret is an @ and a path being typed. The text
// goes to the agent as typed: the agent reads the path.
type chatMention struct {
	// shown is what the list shows this frame, nil while it is shut; from and
	// to are the runes the @ and its path take, which a pick replaces.
	shown    []string
	from, to int
	query    string
	cursor   int
	// dismissed is the word Escape shut the list on, which keeps it shut
	// until the word changes.
	dismissed string
	rows      []ui.Clickable
	list      widget.List
	moved     bool
	motion    panelMotion
	// blocked keeps the list shut while the keys belong to something else.
	blocked bool
	// taking is an Enter (mentionPick) or Tab (mentionComplete) taken before
	// the editor read the keys of its frame: refresh does it once the text
	// holds them, as the slash list does.
	taking int
	// index is files' ranking, and found what foundFor found in it last.
	files    *chatFiles
	index    *mentionIndex
	found    []string
	foundFor string
}

// What an Enter or a Tab taken early does once the text is current.
const (
	mentionPick = iota + 1
	mentionComplete
)

// mentionQuery is the path being typed at the caret of text: the @ the caret
// stands after, at the start of the text or after a space as Claude Code
// takes it, with only path characters between. from is where the @ is and to
// where the path ends, past the caret if it runs on.
func mentionQuery(text string, caret int) (query string, from, to int, ok bool) {
	runes := []rune(text)
	caret = min(max(caret, 0), len(runes))
	from = caret - 1
	for from >= 0 && runes[from] != '@' {
		if !mentionPathRune(runes[from]) {
			return "", 0, 0, false
		}
		from--
	}
	if from < 0 || from > 0 && !mentionBoundary(runes[from-1]) {
		return "", 0, 0, false
	}
	to = caret
	for to < len(runes) && mentionPathRune(runes[to]) {
		to++
	}
	return string(runes[from+1 : to]), from, to, true
}

// mentionPathRune is what Claude Code reads as part of an @ path.
func mentionPathRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) || strings.ContainsRune(`_-./\()[]~:`, r)
}

// mentionBoundary is what may stand before an @: a space, or the CJK full
// stop, comma, question and exclamation marks Claude Code also takes.
func mentionBoundary(r rune) bool {
	return unicode.IsSpace(r) || strings.ContainsRune("。、？！", r)
}

// mentionText is p as a pick puts it in the text, quoted when it holds a
// space, and the space after it.
func mentionText(p string) string {
	if strings.Contains(p, " ") {
		return mentionSigil + `"` + p + `" `
	}
	return mentionSigil + p + " "
}

// act runs the clicks and keys the open list takes, before the editor reads
// its own, as the slash list does.
func (m *chatMention) act(gtx C, ed *widget.Editor) {
	for i := range m.shown {
		if m.rows[i].Clicked(gtx) {
			m.pick(ed, i)
			return
		}
	}
	if len(m.shown) > 0 {
		m.keys(gtx, ed)
	}
}

// refresh opens, filters or shuts the list on what the editor holds once it
// has been laid out, the slash list's way.
func (m *chatMention) refresh(gtx C, ed *widget.Editor, files *chatFiles) {
	was := len(m.shown) > 0
	m.shown = nil
	_, caret := ed.Selection()
	query, from, to, ok := mentionQuery(ed.Text(), caret)
	if !ok {
		m.dismissed = ""
	}
	if !ok || m.blocked || files == nil || mentionSigil+query == m.dismissed {
		m.taking = 0
		return
	}
	shown := m.matches(files, query)
	if len(shown) == 0 {
		m.taking = 0
		return
	}
	if query != m.query || !was {
		m.query, m.cursor, m.moved = query, 0, true
	}
	m.shown, m.from, m.to = shown, from, to
	m.cursor = min(m.cursor, len(shown)-1)
	if len(m.rows) < len(shown) {
		m.rows = make([]ui.Clickable, len(shown))
	}
	m.flush(ed)
	m.keys(gtx, ed)
	m.flush(ed)
}

// flush does the Enter or Tab taken earlier, on the list the text now holds.
func (m *chatMention) flush(ed *widget.Editor) {
	switch {
	case len(m.shown) == 0:
	case m.taking == mentionPick:
		m.pick(ed, m.cursor)
	case m.taking == mentionComplete:
		m.complete(ed)
	}
	m.taking = 0
}

// matches is query's paths in files, searched again only when either changes:
// refresh runs every frame.
func (m *chatMention) matches(files *chatFiles, query string) []string {
	if files != m.files {
		m.files, m.index, m.found = files, newMentionIndex(files.files, files.dirs), nil
	}
	if m.found == nil || query != m.foundFor {
		m.found, m.foundFor = m.index.search(query, mentionLimit), query
	}
	return m.found
}

func (m *chatMention) keys(gtx C, ed *widget.Editor) {
	var filters []event.Filter
	for _, name := range []key.Name{key.NameUpArrow, key.NameDownArrow, key.NameReturn, key.NameEnter, key.NameTab, key.NameEscape} {
		filters = append(filters, key.Filter{Focus: ed, Name: name})
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		e, isKey := ev.(key.Event)
		if !isKey || e.State != key.Press || len(m.shown) == 0 {
			continue
		}
		switch e.Name {
		case key.NameDownArrow:
			m.cursor, m.moved = (m.cursor+1)%len(m.shown), true
		case key.NameUpArrow:
			m.cursor, m.moved = (m.cursor+len(m.shown)-1)%len(m.shown), true
		case key.NameEscape:
			m.dismissed, m.shown = mentionSigil+m.query, nil
		case key.NameTab:
			m.taking = mentionComplete
		default:
			m.taking = mentionPick
		}
	}
}

// complete is Tab, as in Claude Code: what every path shown starts with, when
// it is longer than what was typed, goes in with the list left open on it;
// otherwise the path under the cursor is picked.
func (m *chatMention) complete(ed *widget.Editor) {
	common := m.shown[0]
	for _, p := range m.shown[1:] {
		n := 0
		for n < len(common) && n < len(p) && common[n] == p[n] {
			n++
		}
		common = common[:n]
	}
	for !utf8.ValidString(common) {
		common = common[:len(common)-1]
	}
	if len([]rune(common)) <= len([]rune(m.query)) {
		m.pick(ed, m.cursor)
		return
	}
	ed.SetCaret(m.from, m.to)
	ed.Insert(mentionSigil + common)
	m.shown = nil
}

// pick puts path i in place of the @ and what was typed of it, with a space
// after, and shuts the list. A folder is put in the same way, as Claude Code
// puts it.
func (m *chatMention) pick(ed *widget.Editor, i int) {
	ed.SetCaret(m.from, m.to)
	ed.Insert(mentionText(m.shown[i]))
	m.shown = nil
}

// layout draws the open list where the slash list stands.
func (m *chatMention) layout(gtx C, th *ui.Theme, room int) {
	shown := m.shown
	m.motion.track(gtx.Now, len(shown) > 0, "")
	if len(shown) == 0 {
		return
	}
	m.list.Axis = layout.Vertical
	if m.moved {
		m.list.ScrollTo(max(m.cursor-2, 0))
		m.moved = false
	}
	surfaceOverBox(gtx, th, &m.motion, max(min(gtx.Dp(slashMaxH), room), 0), func(gtx C, th *ui.Theme) D {
		return th.ThinScrollArea(&m.list).Layout(gtx, len(shown), func(gtx C, i int) D {
			return toolsPick(gtx, th, &m.rows[i], i == m.cursor, func(gtx C, th *ui.Theme) D {
				return mentionRow(gtx, th, shown[i])
			})
		})
	})
}

// mentionRow is one path: a folder's glyph or the file's icon, its name in
// medium and the folder it sits in muted on the rest of the line, so a deep
// path cuts its folder and never its name.
func mentionRow(gtx C, th *ui.Theme, p string) D {
	dir, name := mentionParts(p)
	glyph := th.FileIcon(p, ui.Space(3.5)).Layout
	if strings.HasSuffix(p, "/") {
		glyph = func(gtx C) D { return icFolder.Layout(gtx, ui.Space(3.5), th.MutedForeground) }
	}
	row := []layout.FlexChild{
		layout.Rigid(glyph),
		ui.Gap(2),
		layout.Rigid(th.Text(ui.TextSM, name).Weight(font.Medium).Layout),
	}
	if dir != "" {
		row = append(row, ui.Gap(3), layout.Flexed(1, th.Text(ui.TextXS, dir).Muted().Layout))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
}
