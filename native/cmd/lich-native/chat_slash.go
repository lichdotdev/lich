package main

import (
	"image"
	"slices"
	"strings"
	"unicode"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icSlashCommand = icons.Lucide("square-slash")
	icSlashSkill   = icons.Lucide("wand-sparkles")
	// slashMaxH is the tallest the list grows over the composer: past it the
	// commands scroll.
	slashMaxH = unit.Dp(320)
)

// slashSigil opens the list at the start of a line, as it does in the
// providers' TUIs.
const slashSigil = "/"

// chatSlash is the composer's list of what its provider takes typed at its
// prompt, open while the line at the caret is the start of a command.
type chatSlash struct {
	// shown is what the list shows this frame, nil while it is shut; from and
	// to are the runes the command being typed takes, which a pick replaces.
	shown    []backend.ChatCommand
	from, to int
	query    string
	cursor   int
	// dismissed is the command start Escape shut the list on, which keeps it
	// shut until the start changes.
	dismissed string
	rows      []ui.Clickable
	list      widget.List
	moved     bool
	// blocked keeps the list shut while the keys belong to something else.
	blocked bool
	// picking is an Enter or Tab taken before the editor read the keys of
	// its frame: refresh picks once the text holds them ("/m" picked out of
	// "/mcp" typed fast left "cp" behind).
	picking bool
}

// slashQuery is the command being typed at the caret of text: what runs from
// the start of the caret's line to the caret, when that is the sigil and a
// word with no space nor slash after it. from is where it starts, in runes.
// A path typed at the start of a line has a second slash, and is not one.
func slashQuery(text string, caret int) (query string, from int, ok bool) {
	runes := []rune(text)
	caret = min(max(caret, 0), len(runes))
	from = caret
	for from > 0 && runes[from-1] != '\n' {
		from--
	}
	line := string(runes[from:caret])
	word, ok := strings.CutPrefix(line, slashSigil)
	if !ok || strings.ContainsFunc(word, func(r rune) bool { return unicode.IsSpace(r) || r == '/' }) {
		return "", 0, false
	}
	return word, from, true
}

// chatNewCommand is lich's own command in the list: the chat starts an empty
// conversation (api.ChatNew) instead of sending the text.
const chatNewCommand = "/new"

// chatNewAliases are typed for the same: Claude Code's /clear, with its
// aliases (/reset; 2.1.330), starts a new conversation the chat cannot follow,
// so lich starts its own, for every provider alike.
var chatNewAliases = []string{"/clear", "/reset"}

// isNewCommand is whether text asks for an empty conversation.
func isNewCommand(text string) bool {
	return text == chatNewCommand || slices.Contains(chatNewAliases, text)
}

// slashCommands is what the list offers: lich's own commands first, /mcp,
// /skills, /plugin, /memory and /agents where provider kind reports what they
// open, then the provider's, less those they would shadow.
func slashCommands(provider []backend.ChatCommand, kind string) []backend.ChatCommand {
	own := []backend.ChatCommand{{Text: chatNewCommand, Description: i18n.T("chat.slash.new")}}
	if slices.Contains(chatToolKinds, kind) {
		own = append(own,
			backend.ChatCommand{Text: chatMCPCommand, Description: i18n.T("chat.slash.mcp")},
			backend.ChatCommand{Text: chatSkillsCommand, Description: i18n.T("chat.slash.skills")},
			backend.ChatCommand{Text: chatPluginCommand, Description: i18n.T("chat.slash.plugin")})
	}
	if slices.Contains(chatMemoryKinds, kind) {
		own = append(own,
			backend.ChatCommand{Text: chatMemoryCommand, Description: i18n.T("chat.slash.memory")},
			backend.ChatCommand{Text: chatAgentsCommand, Description: i18n.T("chat.slash.agents")})
	}
	shadowed := func(c backend.ChatCommand) bool {
		return slices.ContainsFunc(own, func(o backend.ChatCommand) bool { return o.Text == c.Text })
	}
	return append(own, slices.DeleteFunc(slices.Clone(provider), shadowed)...)
}

// slashName is a command's name without the sigil it is typed with.
func slashName(c backend.ChatCommand) string { return strings.TrimLeft(c.Text, "/$") }

// slashMatches is the commands whose name holds query, those it starts first,
// each group in the provider's order.
func slashMatches(cmds []backend.ChatCommand, query string) []backend.ChatCommand {
	query = strings.ToLower(query)
	var starts, holds []backend.ChatCommand
	for _, c := range cmds {
		name := strings.ToLower(slashName(c))
		switch {
		case strings.HasPrefix(name, query):
			starts = append(starts, c)
		case strings.Contains(name, query):
			holds = append(holds, c)
		}
	}
	return append(starts, holds...)
}

// act runs the clicks and keys the open list takes, before the editor reads
// its own: while it is open, Enter and Tab complete the command under the
// cursor instead of sending, the arrows walk it and Escape shuts it.
func (s *chatSlash) act(gtx C, ed *widget.Editor) {
	for i := range s.shown {
		if s.rows[i].Clicked(gtx) {
			s.pick(ed, i)
			return
		}
	}
	if len(s.shown) > 0 {
		s.keys(gtx, ed)
	}
}

// refresh opens, filters or shuts the list on what the editor holds once it
// has been laid out, which is where it takes typed text, so the frame that
// took a keystroke draws the list it leaves. A list open asks for its keys in
// that frame too: the next key is routed by the filters of this one.
func (s *chatSlash) refresh(gtx C, ed *widget.Editor, cmds []backend.ChatCommand) {
	was := len(s.shown) > 0
	s.shown = nil
	_, caret := ed.Selection()
	query, from, ok := slashQuery(ed.Text(), caret)
	if !ok {
		s.dismissed = ""
	}
	if !ok || s.blocked || len(cmds) == 0 || slashSigil+query == s.dismissed {
		s.picking = false
		return
	}
	shown := slashMatches(cmds, query)
	if len(shown) == 0 {
		s.picking = false
		return
	}
	if query != s.query || !was {
		s.query, s.cursor, s.moved = query, 0, true
	}
	s.shown, s.from, s.to = shown, from, caret
	s.cursor = min(s.cursor, len(shown)-1)
	if len(s.rows) < len(shown) {
		s.rows = make([]ui.Clickable, len(shown))
	}
	s.flush(ed)
	s.keys(gtx, ed)
	s.flush(ed)
}

// flush picks the command under the cursor for an Enter or Tab taken earlier.
func (s *chatSlash) flush(ed *widget.Editor) {
	if s.picking && len(s.shown) > 0 {
		s.pick(ed, s.cursor)
	}
	s.picking = false
}

func (s *chatSlash) keys(gtx C, ed *widget.Editor) {
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
		if !isKey || e.State != key.Press || len(s.shown) == 0 {
			continue
		}
		switch e.Name {
		case key.NameDownArrow:
			s.cursor, s.moved = (s.cursor+1)%len(s.shown), true
		case key.NameUpArrow:
			s.cursor, s.moved = (s.cursor+len(s.shown)-1)%len(s.shown), true
		case key.NameEscape:
			s.dismissed, s.shown = slashSigil+s.query, nil
		default:
			s.picking = true
		}
	}
}

// pick puts command i in place of what was typed of it, with a space for
// its arguments, and shuts the list. It is not sent: Enter sends it, as at
// the provider's own prompt.
func (s *chatSlash) pick(ed *widget.Editor, i int) {
	c := s.shown[i]
	ed.SetCaret(s.from, s.to)
	ed.Insert(c.Text + " ")
	s.shown = nil
}

// layout draws the open list over the composer's box, as wide as it and a gap
// above it, deferred so it stands over the conversation, no taller than room.
func (s *chatSlash) layout(gtx C, th *ui.Theme, room int) {
	if len(s.shown) == 0 {
		return
	}
	skills := slices.ContainsFunc(s.shown, func(c backend.ChatCommand) bool { return c.Skill })
	s.list.Axis = layout.Vertical
	if s.moved {
		s.list.ScrollTo(max(s.cursor-2, 0))
		s.moved = false
	}
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(gtx.Constraints.Max.X, max(min(gtx.Dp(slashMaxH), room), 0))}
	macro := op.Record(gtx.Ops)
	d := th.PopoverPanel(g, func(gtx C, th *ui.Theme) D {
		return th.ThinScrollArea(&s.list).Layout(gtx, len(s.shown), func(gtx C, i int) D {
			return pickerRow(gtx, th, &s.rows[i], i == s.cursor, func(gtx C, th *ui.Theme) D {
				return slashRow(gtx, th, s.shown[i], skills)
			})
		})
	})
	call := macro.Stop()
	m := op.Record(gtx.Ops)
	op.Offset(image.Pt(0, -d.Size.Y-gtx.Dp(ui.Space(2)))).Add(gtx.Ops)
	call.Add(gtx.Ops)
	op.Defer(gtx.Ops, m.Stop())
}

// slashRow is one command: a glyph that tells a skill from a command where
// the provider does, its name in medium with its arguments muted, and its
// description muted on the rest of the line.
func slashRow(gtx C, th *ui.Theme, c backend.ChatCommand, skills bool) D {
	glyph := icSlashCommand
	if skills && c.Skill {
		glyph = icSlashSkill
	}
	row := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return glyph.Layout(gtx, ui.Space(3.5), th.MutedForeground) }),
		ui.Gap(2),
		layout.Rigid(th.Text(ui.TextSM, c.Text).Weight(font.Medium).Layout),
	}
	if c.Hint != "" {
		row = append(row, ui.Gap(1.5), layout.Rigid(th.Text(ui.TextSM, c.Hint).Muted().Layout))
	}
	if c.Description != "" {
		row = append(row, ui.Gap(3), layout.Flexed(1, th.Text(ui.TextXS, cleanLine(c.Description)).Muted().Layout))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
}
