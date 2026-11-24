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

type vimUI struct {
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
		u.msg, u.msgError = text, vimErrorKinds[kind]
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
