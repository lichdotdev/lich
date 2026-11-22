package main

import (
	"image/color"
	"slices"
	"strings"
	"time"

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

// shellsChip is the chip that counts the shells running, lit while their
// panel is open; none while none runs.
func (u *window) shellsChip(gtx C, th *ui.Theme, p *chatPane, sv sessionView) D {
	n := runningShells(u.model.chat.shellsOf(sv.ID))
	if n == 0 {
		return D{}
	}
	p.shellsChip.pressed = p.tools.kind == chatShellsPanel
	label := i18n.T("chat.shells.count", i18n.Args{"count": n})
	return footerButton(gtx, th, &u.root, &p.shellsChip, i18n.T("chat.shells.title"), true, false, glyphText(icToolRun, label))
}

// shellState is a shell's glyph and its state in words.
func shellState(th *ui.Theme, s backend.ChatShell) (*icons.Icon, color.NRGBA, string) {
	switch s.State {
	case api.ShellRunning:
		return icToolsPending, th.ToneWait, i18n.T("chat.shells.state.running")
	case api.ShellCompleted:
		return icCheck, th.TonePass, i18n.T("chat.shells.state.completed")
	case api.ShellFailed:
		return icToolsFailed, th.Destructive, i18n.T("chat.shells.state.failed")
	}
	return icToolsOff, th.MutedForeground, i18n.T("chat.shells.state.stopped")
}

// shellTook is how long s has run, or ran.
func shellTook(gtx C, s backend.ChatShell) string {
	start := time.UnixMilli(s.Started)
	if s.Ended == 0 {
		chatTick(gtx, start)
		return chatElapsed(gtx.Now.Sub(start))
	}
	return chatElapsed(time.UnixMilli(s.Ended).Sub(start))
}

func (u *window) shellsView(gtx C, th *ui.Theme, p *chatPane, sv sessionView) D {
	t := &p.tools
	shells := shellsOrdered(u.model.chat.shellsOf(sv.ID))
	if i := slices.IndexFunc(shells, func(s backend.ChatShell) bool { return s.ID == t.detail }); i >= 0 {
		return u.shellDetail(gtx, th, p, sv.ID, shells[i])
	}
	t.list.ScrollToEnd = false
	if len(shells) == 0 {
		return toolsNote(gtx, th, i18n.T("chat.shells.none"))
	}
	t.ensureRows(len(shells))
	lines := make([]layout.Widget, len(shells))
	for i, s := range shells {
		lines[i] = func(gtx C) D {
			return pickerRow(gtx, th, &t.rows[i], i == t.cursor, func(gtx C, th *ui.Theme) D { return shellRow(gtx, th, s) })
		}
	}
	head := func(gtx C) D {
		return toolsHead(gtx, th, i18n.T("chat.shells.title"), i18n.T("chat.shells.count", i18n.Args{"count": runningShells(shells)}), nil)
	}
	return t.scrolled(gtx, th, head, lines, nil)
}

func shellRow(gtx C, th *ui.Theme, s backend.ChatShell) D {
	ic, ink, state := shellState(th, s)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return ic.Layout(gtx, toolsGlyph, ink) }),
		ui.Gap(2),
		layout.Flexed(1, th.Text(ui.TextSM, cleanLine(s.Command)).Mono().Truncated().Layout),
		ui.Gap(toolsMetaGap),
		layout.Rigid(th.Text(ui.TextXS, state+" · "+shellTook(gtx, s)).Muted().Layout),
	)
}

// shellDetail is one shell: its command, state and running time, what the
// agent said it is for, the end of its output following what comes, and Stop
// while it runs. Escape or Back goes back to the list.
func (u *window) shellDetail(gtx C, th *ui.Theme, p *chatPane, id string, s backend.ChatShell) D {
	t := &p.tools
	u.pollShell(gtx, p, id, s)
	t.mu.Lock()
	out, read := t.shell.text, t.shell.read
	t.mu.Unlock()
	back := th.Button(&t.back, i18n.T("chat.shells.back"))
	back.Icon, back.Variant, back.Size = icToolsBack, ui.VariantGhost, ui.SizeXS
	ic, ink, state := shellState(th, s)
	head := func(gtx C) D {
		return layout.Inset{Left: ui.Space(1), Right: ui.Space(2), Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(back.Layout),
				ui.Gap(2),
				layout.Rigid(func(gtx C) D { return ic.Layout(gtx, toolsGlyph, ink) }),
				ui.Gap(1.5),
				layout.Flexed(1, th.Text(ui.TextSM, cleanLine(s.Command)).Mono().Weight(font.SemiBold).Truncated().Layout),
				ui.Gap(toolsMetaGap),
				layout.Rigid(th.Text(ui.TextXS, state+" · "+shellTook(gtx, s)).Muted().Layout),
			)
		})
	}
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
	t.list.ScrollToEnd = true
	var foot layout.Widget
	if s.State == api.ShellRunning {
		foot = func(gtx C) D {
			b := th.Button(&t.shell.stop, i18n.T("chat.shells.stop"))
			b.Variant, b.Size, b.Icon = ui.VariantSecondary, ui.SizeSM, icChatStop
			return layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx, layout.Rigid(b.Layout))
			})
		}
	}
	return t.scrolled(gtx, th, head, lines, foot)
}

func shellLine(th *ui.Theme, line string) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, th.Text(ui.TextXS, expandTabs(line)).Mono().Wrapped().Layout)
	}
}
