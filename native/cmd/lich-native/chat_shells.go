package main

import (
	"log"
	"slices"
	"time"

	"gioui.org/io/key"
	"gioui.org/op"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// The background shells of a chat (api.ChatShells) are counted by a chip in
// the composer's foot while one runs, Claude Code's "2 shells", and listed in
// a panel in the slash list's place that /tasks opens too, Claude Code's
// dialog: each shell's command, state and running time, and in its detail
// the end of its output and Stop.

const (
	// chatShellsPanel is the panel's kind in chatTools, named after the
	// command that opens it.
	chatShellsPanel = "/tasks"
	// chatBashesCommand is /tasks' other name in Claude Code.
	chatBashesCommand = "/bashes"
	chipShells        = "shells"
)

// chatShellKinds is the providers whose chat reports its background shells.
var chatShellKinds = []string{claudeKind}

// chatShellPoll is how often an open shell's output is read while it runs.
const chatShellPoll = time.Second

// chatShellOut is the output of the shell the panel shows, read off the
// frame.
type chatShellOut struct {
	id   string
	text string
	read bool
	// state is the shell's state at the last read: one more read follows its end.
	state    string
	fetching bool
	// gen counts the reads that landed, scrolled the one the list was
	// scrolled to the end for.
	gen, scrolled int
	next          time.Time
	stop          ui.Clickable
}

// openShells opens the shells' panel. The box keeps what was typed: typing
// more shuts it.
func (p *chatPane) openShells() {
	t := &p.tools
	t.open(chatShellsPanel)
	t.mu.Lock()
	t.loaded, t.base = true, p.editor.Len()
	t.mu.Unlock()
}

// runningShells counts the shells that run, the monitors apart.
func runningShells(shells []backend.ChatShell) (n, monitors int) {
	for _, s := range shells {
		switch {
		case s.State != api.ShellRunning:
		case s.Monitor:
			monitors++
		default:
			n++
		}
	}
	return n, monitors
}

// shellsOrdered is the shells as the panel lists them: those running first,
// each group newest first.
func shellsOrdered(shells []backend.ChatShell) []backend.ChatShell {
	out := slices.Clone(shells)
	slices.Reverse(out)
	slices.SortStableFunc(out, func(a, b backend.ChatShell) int {
		return boolRank(b.State == api.ShellRunning) - boolRank(a.State == api.ShellRunning)
	})
	return out
}

func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}

// shellsChipActs opens or shuts the panel on a click of the chip.
func (u *window) shellsChipActs(gtx C, p *chatPane) {
	if !p.shellsChip.click.Clicked(gtx) {
		return
	}
	if p.tools.kind == chatShellsPanel {
		p.tools.shut()
		return
	}
	p.openShells()
}

// shellActs runs the shells' panel: a row opens its shell, and in a shell
// Back goes back and Stop stops it.
func (u *window) shellActs(gtx C, p *chatPane, id string) (int, func(int)) {
	t := &p.tools
	shells := shellsOrdered(u.model.chat.shellsOf(id))
	if t.detail == "" {
		return len(shells), func(i int) { t.detail = shells[i].ID }
	}
	i := slices.IndexFunc(shells, func(s backend.ChatShell) bool { return s.ID == t.detail })
	if t.back.Clicked(gtx) || i < 0 {
		t.detail = ""
		return 0, func(int) {}
	}
	if t.shell.stop.Clicked(gtx) && shells[i].State == api.ShellRunning {
		shell := shells[i].ID
		go func() { u.chatFailed(u.client.ChatStopShell(u.ctx, id, shell)) }()
	}
	return 0, func(int) {}
}

// backgroundKeys takes Ctrl+B from the composer ahead of the editor while the
// turn waits on a command it moves, Claude Code's key on every system, and
// moves the command to the background.
func (u *window) backgroundKeys(gtx C, p *chatPane, id string) {
	if !u.model.chat.isRunning(id) || !movable(u.model.chat.of(id)) {
		return
	}
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &p.editor, Name: "B", Required: key.ModCtrl})
		if !ok {
			return
		}
		if e, isKey := ev.(key.Event); isKey && e.State == key.Press {
			go func() { u.chatFailed(u.client.ChatBackground(u.ctx, id)) }()
		}
	}
}

// pollShell reads the output of the shell the panel shows: at once, every
// chatShellPoll while it runs, and once more when it ends.
func (u *window) pollShell(gtx C, p *chatPane, id string, s backend.ChatShell) {
	o := &p.tools.shell
	p.tools.mu.Lock()
	if o.id != s.ID {
		o.id, o.text, o.read, o.state, o.next = s.ID, "", false, "", time.Time{}
	}
	due := !o.read || o.state != s.State || s.State == api.ShellRunning && !gtx.Now.Before(o.next)
	fetch := due && !o.fetching
	if fetch {
		o.fetching, o.next = true, gtx.Now.Add(chatShellPoll)
	}
	next := o.next
	p.tools.mu.Unlock()
	if s.State == api.ShellRunning && !next.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: next})
	}
	if !fetch {
		return
	}
	state := s.State
	go func() {
		text, err := u.client.ChatShellOutput(u.ctx, id, s.ID)
		p.tools.mu.Lock()
		o.fetching = false
		if o.id == s.ID {
			o.read, o.state = true, state
			if err == nil {
				o.text = text
				o.gen++
			}
		}
		p.tools.mu.Unlock()
		if err != nil {
			log.Printf("output of shell %s: %v", s.ID, err)
		}
		u.model.invalidate()
	}()
}
