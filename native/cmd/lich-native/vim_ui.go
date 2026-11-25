package main

// What Neovim's UI events say beyond the buffer: the cursor's shape in each
// mode (the person's 'guicursor'), the mode's message, the partial command,
// the command line and the last message. The grid itself is never drawn:
// the editor draws the text.

import (
	"strings"
)

// vimCursor is a mode's cursor: "block", "horizontal" or "vertical", and
// the share of the cell the last two take, in percent.
type vimCursor struct {
	shape   string
	percent int
}

type vimCmdline struct {
	// text is the line as shown: its first character (":", "/", "?") or
	// prompt, lead bytes long, then what was typed; pos is the cursor's byte.
	text string
	pos  int
	lead int
}

// vimPum is Neovim's completion menu: its items and the one chosen, -1 for
// none.
type vimPum struct {
	items    []vimPumItem
	selected int
}

// vimPumItem is a completion: the word it puts in, and its kind as the
// source names it ("func", "v").
type vimPumItem struct {
	word, kind string
}

type vimUI struct {
	pum      *vimPum
	buf      int
	cursors  []vimCursor
	mode     int
	showmode string
	showcmd  string
	cmdline  *vimCmdline
	msg      string
	msgError bool
}

// cursor is the shape the current mode draws, a block when Neovim named none.
func (u vimUI) cursor() vimCursor {
	if u.mode >= 0 && u.mode < len(u.cursors) && u.cursors[u.mode].shape != "" {
		return u.cursors[u.mode]
	}
	return vimCursor{shape: "block", percent: 100}
}

// vimErrorKinds are msg_show's kinds of an error.
var vimErrorKinds = map[string]bool{"emsg": true, "echoerr": true, "lua_error": true, "rpc_error": true}

func (h *vimHost) redraw(updates ...[]any) {
	h.mu.Lock()
	for _, u := range updates {
		h.ui.apply(u)
	}
	h.mu.Unlock()
	h.invalidate()
}

// apply takes one redraw update: its name, then one argument tuple per call.
func (u *vimUI) apply(update []any) {
	if len(update) == 0 {
		return
	}
	name, _ := update[0].(string)
	for _, raw := range update[1:] {
		args, _ := raw.([]any)
		u.applyOne(name, args)
	}
}

func (u *vimUI) applyOne(name string, args []any) {
	switch name {
	case "mode_info_set":
		u.cursors = vimCursorsOf(argAt(args, 1))
	case "mode_change":
		u.mode = intOf(argAt(args, 1))
	case "msg_showmode":
		u.showmode = chunksText(argAt(args, 0))
	case "msg_showcmd":
		u.showcmd = chunksText(argAt(args, 0))
	case "cmdline_show":
		firstc, _ := argAt(args, 2).(string)
		prompt, _ := argAt(args, 3).(string)
		lead := firstc + prompt
		u.cmdline = &vimCmdline{text: lead + chunksText(argAt(args, 0)), pos: len(lead) + intOf(argAt(args, 1)), lead: len(lead)}
	case "cmdline_pos":
		if u.cmdline != nil {
			u.cmdline.pos = u.cmdline.lead + intOf(argAt(args, 0))
		}
	case "cmdline_hide":
		u.cmdline = nil
	case "msg_show":
		kind, _ := argAt(args, 0).(string)
		text := chunksText(argAt(args, 1))
		if kind == "return_prompt" || text == "" {
			return
		}
		// Messages pile up until Neovim clears them, as :ls or :reg write
		// theirs a line at a time.
		if replace, _ := argAt(args, 2).(bool); replace || u.msg == "" {
			u.msg, u.msgError = text, vimErrorKinds[kind]
		} else {
			u.msg, u.msgError = u.msg+"\n"+text, u.msgError || vimErrorKinds[kind]
		}
	case "msg_history_show":
		u.msg, u.msgError = vimHistoryText(argAt(args, 0)), false
	case "popupmenu_show":
		// The command line's completion (grid -1) is not drawn.
		if intOf(argAt(args, 4)) == -1 {
			u.pum = nil
			return
		}
		u.pum = &vimPum{items: vimPumItems(argAt(args, 0)), selected: intOf(argAt(args, 1))}
	case "popupmenu_select":
		if u.pum != nil {
			u.pum.selected = intOf(argAt(args, 0))
		}
	case "popupmenu_hide":
		u.pum = nil
	case "msg_clear":
		u.msg, u.msgError = "", false
	}
}

func vimCursorsOf(raw any) []vimCursor {
	infos, _ := raw.([]any)
	out := make([]vimCursor, len(infos))
	for i, info := range infos {
		m, _ := info.(map[string]any)
		shape, _ := m["cursor_shape"].(string)
		out[i] = vimCursor{shape: shape, percent: intOf(m["cell_percentage"])}
	}
	return out
}

// chunksText joins a message's [attr, text, hl] chunks into its text.
func chunksText(raw any) string {
	chunks, _ := raw.([]any)
	var b strings.Builder
	for _, c := range chunks {
		if parts, ok := c.([]any); ok && len(parts) > 1 {
			s, _ := parts[1].(string)
			b.WriteString(s)
		}
	}
	return b.String()
}

func argAt(args []any, i int) any {
	if i < len(args) {
		return args[i]
	}
	return nil
}

func intOf(v any) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case int:
		return n
	}
	return 0
}

// The graph lanes a mode's badge takes, so its colors follow the scheme as
// the git graph's do: blue, green, amber, purple, cyan, red.
const (
	vimLaneNormal  = 0
	vimLaneInsert  = 1
	vimLaneCommand = 2
	vimLaneVisual  = 3
	vimLaneOther   = 4
	vimLaneReplace = 5
)

// vimModeNames are Vim's modes by nvim_get_mode's spelling, as Vim's own
// statusline plugins name them, and the lane their badge wears; the first
// spelling that starts the mode wins.
var vimModeNames = []struct {
	prefix, name string
	lane         int
}{
	{"no", "O-PENDING", vimLaneOther}, {"n", "NORMAL", vimLaneNormal},
	{"v", "VISUAL", vimLaneVisual}, {"V", "V-LINE", vimLaneVisual}, {"\x16", "V-BLOCK", vimLaneVisual},
	{"s", "SELECT", vimLaneVisual}, {"S", "S-LINE", vimLaneVisual}, {"\x13", "S-BLOCK", vimLaneVisual},
	{"i", "INSERT", vimLaneInsert}, {"Rv", "V-REPLACE", vimLaneReplace}, {"R", "REPLACE", vimLaneReplace},
	{"c", "COMMAND", vimLaneCommand}, {"r", "PROMPT", vimLaneOther}, {"!", "SHELL", vimLaneOther},
	{"t", "TERMINAL", vimLaneOther},
}

// vimModeLabel is mode as the strip names it and the lane its badge wears,
// "" for one Vim has no name for.
func vimModeLabel(mode string) (string, int) {
	for _, m := range vimModeNames {
		if strings.HasPrefix(mode, m.prefix) {
			return m.name, m.lane
		}
	}
	return "", 0
}

// vimShowmodeRest is what 'showmode' says past the mode the label already
// names: "-- INSERT --recording @q" keeps "recording @q".
func vimShowmodeRest(s string) string {
	if rest, ok := strings.CutPrefix(s, "-- "); ok {
		if i := strings.Index(rest, " --"); i >= 0 {
			return strings.TrimSpace(rest[i+len(" --"):])
		}
	}
	return s
}

// vimHistoryText is :messages' entries, one per line.
func vimHistoryText(raw any) string {
	entries, _ := raw.([]any)
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		if parts, ok := e.([]any); ok && len(parts) > 1 {
			lines = append(lines, chunksText(parts[1]))
		}
	}
	return strings.Join(lines, "\n")
}

// vimMessageLines is how many lines of a long message the strip shows, the
// last ones, as Vim's pager ends on them.
const vimMessageLines = 15

// vimMessageTail is msg cut to its last vimMessageLines lines.
func vimMessageTail(msg string) string {
	lines := strings.Split(msg, "\n")
	if len(lines) <= vimMessageLines {
		return msg
	}
	return strings.Join(lines[len(lines)-vimMessageLines:], "\n")
}

// vimPumItems reads popupmenu_show's [word, kind, menu, info] items.
func vimPumItems(raw any) []vimPumItem {
	items, _ := raw.([]any)
	out := make([]vimPumItem, 0, len(items))
	for _, it := range items {
		parts, _ := it.([]any)
		word, _ := argAt(parts, 0).(string)
		kind, _ := argAt(parts, 1).(string)
		out = append(out, vimPumItem{word: word, kind: kind})
	}
	return out
}
