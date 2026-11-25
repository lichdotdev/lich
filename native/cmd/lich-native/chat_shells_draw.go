package main

import (
	"image/color"
	"slices"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// shellTailLines is how many of the output's last lines a shell's detail
// draws.
const shellTailLines = 200

// shellsChip is the chip that counts the shells running and the monitors,
// lit while their panel is open; none while none runs.
func (u *window) shellsChip(gtx C, th *ui.Theme, p *chatPane, sv sessionView) D {
	n, monitors := runningShells(u.model.chat.shellsOf(sv.ID))
	if n+monitors == 0 {
		return D{}
	}
	p.shellsChip.pressed = p.tools.kind == chatShellsPanel
	var counts []string
	if n > 0 {
		counts = append(counts, i18n.T("chat.shells.count", i18n.Args{"count": n}))
	}
	if monitors > 0 {
		counts = append(counts, i18n.T("chat.shells.monitors", i18n.Args{"count": monitors}))
	}
	label := strings.Join(counts, " · ")
	return footerButton(gtx, th, &u.root, &p.shellsChip, i18n.T("chat.shells.title"), true, false, glyphText(icToolRun, label))
}

// shellState is a shell's glyph and how it ended in words where the glyph
// alone does not say it: "" while it runs and once it completed, a failure's
// exit code where the provider named it.
func shellState(th *ui.Theme, s backend.ChatShell) (*icons.Icon, color.NRGBA, string) {
	switch s.State {
	case api.ShellRunning:
		return icToolsPending, th.ToneWait, ""
	case api.ShellCompleted:
		return icCheck, th.TonePass, ""
	case api.ShellFailed:
		if s.ExitCode != nil {
			return icToolsFailed, th.Destructive, i18n.T("chat.shells.exit", i18n.Args{"code": *s.ExitCode})
		}
		return icToolsFailed, th.Destructive, i18n.T("chat.shells.state.failed")
	}
	return icToolsOff, th.MutedForeground, i18n.T("chat.shells.state.stopped")
}

// shellTook is how long s has run, or ran.
func shellTook(gtx C, s backend.ChatShell) string { return took(gtx, s.Started, s.Ended) }

// shellMeta is what a shell's line says at its end: a monitor named one, how
// it ended where the glyph does not say it, and how long it has run.
func shellMeta(gtx C, s backend.ChatShell, state string) string {
	meta := []string{shellTook(gtx, s)}
	if state != "" {
		meta = append([]string{state}, meta...)
	}
	if s.Monitor {
		meta = append([]string{i18n.T("chat.shells.monitor")}, meta...)
	}
	return strings.Join(meta, " · ")
}

func (u *window) shellsView(gtx C, th *ui.Theme, p *chatPane, sv sessionView) D {
	t := &p.tools
	shells := shellsOrdered(u.model.chat.shellsOf(sv.ID))
	if i := slices.IndexFunc(shells, func(s backend.ChatShell) bool { return s.ID == t.detail }); i >= 0 {
		return u.shellDetail(gtx, th, p, sv.ID, shells[i])
	}
	if len(shells) == 0 {
		return toolsNote(gtx, th, i18n.T("chat.shells.none"))
	}
	t.ensureRows(len(shells))
	running := slices.IndexFunc(shells, func(s backend.ChatShell) bool { return s.State != api.ShellRunning })
	if running < 0 {
		running = len(shells)
	}
	var lines []layout.Widget
	for i, s := range shells {
		if i == 0 && running > 0 {
			lines = append(lines, toolsGroupLine(th, i18n.T("chat.tools.group.running"), ""))
		}
		if i == running {
			lines = append(lines, toolsGroupLine(th, i18n.T("chat.tools.group.ended"), ""))
		}
		lines = append(lines, func(gtx C) D {
			return toolsPick(gtx, th, &t.rows[i], i == t.cursor, func(gtx C, th *ui.Theme) D { return shellRow(gtx, th, s) })
		})
	}
	count := ""
	if running > 0 {
		count = i18n.T("chat.tools.running", i18n.Args{"count": running})
	}
	head := func(gtx C) D {
		return toolsHead(gtx, th, i18n.T("chat.shells.title"), count, nil)
	}
	return t.scrolled(gtx, th, head, lines, nil)
}

// shellRow is a shell's line: a failure's words in Destructive, the rest of
// its facts muted.
func shellRow(gtx C, th *ui.Theme, s backend.ChatShell) D {
	ic, ink, state := shellState(th, s)
	meta := shellMeta(gtx, s, "")
	if s.State != api.ShellFailed {
		meta, state = shellMeta(gtx, s, state), ""
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return ic.Layout(gtx, toolsGlyph, ink) }),
		ui.Gap(2),
		layout.Flexed(1, th.Text(ui.TextSM, cleanLine(s.Command)).Mono().Truncated().Layout),
		ui.Gap(toolsMetaGap),
		layout.Rigid(func(gtx C) D {
			l := th.Text(ui.TextXS, state)
			l.Color = th.Destructive
			return l.Layout(gtx)
		}),
		layout.Rigid(th.Text(ui.TextXS, shellMetaSep(state)+meta).Muted().Layout),
	)
}

// shellMetaSep joins a failure's words to the rest of the line's facts.
func shellMetaSep(state string) string {
	if state == "" {
		return ""
	}
	return " · "
}

// shellDetail is one shell: its command, state and running time, what the
// agent said it is for, the end of its output following what comes, and Stop
// while it runs. Escape or Back goes back to the list.
func (u *window) shellDetail(gtx C, th *ui.Theme, p *chatPane, id string, s backend.ChatShell) D {
	t := &p.tools
	u.pollShell(gtx, p, id, s)
	t.mu.Lock()
	out, read, gen := t.shell.text, t.shell.read, t.shell.gen
	t.mu.Unlock()
	head := func(gtx C) D { return shellHead(gtx, th, t, s) }
	var lines []layout.Widget
	if s.Description != "" && s.Description != s.Command {
		lines = append(lines, toolsFootNote(th, s.Description))
	}
	text := strings.TrimRight(out, "\n")
	switch {
	case !read:
		lines = append(lines, toolsFootNote(th, i18n.T("chat.tools.loading")))
	case text == "":
		lines = append(lines, toolsFootNote(th, i18n.T("chat.shells.noOutput")))
	default:
		all := strings.Split(text, "\n")
		for _, l := range all[max(len(all)-shellTailLines, 0):] {
			lines = append(lines, shellLine(th, l))
		}
	}
	// each read lands with the list at its end, where the output grows
	if t.shell.scrolled != gen {
		t.shell.scrolled = gen
		t.list.ScrollTo(len(lines))
	}
	return t.scrolled(gtx, th, head, lines, nil)
}

// shellHead is Back, then the shell's command and time.
func shellHead(gtx C, th *ui.Theme, t *chatTools, s backend.ChatShell) D {
	_, _, state := shellState(th, s)
	title := th.Text(ui.TextSM, cleanLine(s.Command)).Mono().Weight(font.Medium)
	return toolsBackHead(gtx, th, &t.back, i18n.T("chat.shells.back"), title, shellMeta(gtx, s, state), nil)
}

// shellStopAct is Stop on the keys' line while the shell the panel shows
// runs.
func (u *window) shellStopAct(th *ui.Theme, p *chatPane, id string) layout.Widget {
	t := &p.tools
	shells := u.model.chat.shellsOf(id)
	i := slices.IndexFunc(shells, func(s backend.ChatShell) bool { return s.ID == t.detail })
	if i < 0 || shells[i].State != api.ShellRunning {
		return nil
	}
	return func(gtx C) D {
		b := th.Button(&t.shell.stop, i18n.T("chat.shells.stop"))
		b.Variant, b.Size, b.Icon = ui.VariantSecondary, ui.SizeSM, icChatStop
		return b.Layout(gtx)
	}
}

func shellLine(th *ui.Theme, line string) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, th.Text(ui.TextXS, expandTabs(line)).Mono().Wrapped().Layout)
	}
}
