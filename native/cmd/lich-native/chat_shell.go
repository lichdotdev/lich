package main

import (
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// A line that starts with "!" runs in the shell, as Claude Code's prompt does
// in its shell mode (2.1.330): the box wears the mode while the text starts
// so, and Enter runs the rest with no model and no permission check. Deleting
// the "!" leaves the mode, as Backspace does on Claude Code's empty prompt.

// chatRunShellKinds are the providers whose protocol runs a command of the
// person's own (internal/chat/shell.go); in any other chat a "!" line goes
// to the agent as typed.
var chatRunShellKinds = []string{claudeKind, codexKind, opencodeKind}

const chatShellMark = "!"

// shellLine is the command the composer holds when it is one, for a chat of
// kind: the text after a leading "!", with no file attached, which a command
// could not carry.
func (p *chatPane) shellLine(kind string) (string, bool) {
	text := p.editor.Text()
	if len(p.attached) > 0 || !strings.HasPrefix(text, chatShellMark) || !slices.Contains(chatRunShellKinds, kind) {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(text, chatShellMark)), true
}

// takeShell runs the composer's command in session id's shell when it holds
// one, and reports whether it did; "!" alone runs nothing. A refused command
// goes back into the box. It never waits in the queue: joined with the
// messages there, it would reach the agent as text.
func (u *window) takeShell(p *chatPane, id string) bool {
	sv, ok := u.model.sessionViewOf(id)
	if !ok {
		return false
	}
	command, ok := p.shellLine(sv.Kind)
	if !ok || command == "" {
		return ok
	}
	m := p.composed()
	p.clearComposer()
	go func() {
		if err := u.client.ChatRunShell(u.ctx, id, command); err != nil {
			u.chatRefused(p, m, err)
		}
	}()
	return true
}

// claudeCycle is the order Shift+Tab walks Claude Code's modes in, its own
// (2.1.330, not the order it lists them in): plan, then bypass where the
// process may run it, then auto where the model has it, and back to default;
// dontAsk is left for default.
var claudeCycle = []string{"default", "acceptEdits", "plan", "bypassPermissions", "auto"}

// nextMode is the mode Shift+Tab moves opt to in a chat of kind: the next
// one the option offers, in Claude Code's order there and in the provider's
// list elsewhere. A dangerous choice is skipped: lich starts a process
// allowed to bypass only for a person who picked it in the menu, and a key
// must not restart the agent into it.
func nextMode(kind string, opt backend.ChatOption) (string, bool) {
	order := claudeCycle
	if kind != claudeKind {
		order = make([]string, len(opt.Choices))
		for i, c := range opt.Choices {
			order[i] = c.ID
		}
	}
	offered := func(id string) bool {
		i := slices.IndexFunc(opt.Choices, func(c api.ChatChoice) bool { return c.ID == id })
		return i >= 0 && !opt.Choices[i].Danger && id != opt.Current
	}
	at := slices.Index(order, opt.Current)
	for step := 1; step <= len(order); step++ {
		if id := order[(at+step)%len(order)]; offered(id) {
			return id, true
		}
	}
	return "", false
}

// cycleMode is Shift+Tab in session id's box: the mode moves to the next,
// as its chip's menu would set it. While the agent works the chip takes no
// click, and the key does nothing either.
func (u *window) cycleMode(id string) {
	sv, ok := u.model.sessionViewOf(id)
	opt, has := u.model.chat.optionOf(id, api.OptionMode)
	if !ok || !has || chatWorking(sv, u.model.chat.isRunning(id)) {
		return
	}
	if next, ok := nextMode(sv.Kind, opt); ok {
		u.setChatOption(id, opt.ID, next)
	}
}
