package main

import (
	"image/color"
	"path/filepath"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icMemoryFile = icons.Lucide("file-text")
	icMemoryNote = icons.Lucide("notebook-pen")
	icAgent      = icons.Lucide("bot")
	// memoryImportIndent is how far in a file another pulled in sits.
	memoryImportIndent = ui.Space(4)
	agentNameW         = ui.Space(40)
)

func (u *window) memoryView(gtx C, th *ui.Theme, p *chatPane, sv sessionView, l api.ChatMemoryList) D {
	t := &p.tools
	if !l.Reported {
		return toolsNote(gtx, th, i18n.T("chat.memory.none", i18n.Args{"provider": u.providerName(sv)}))
	}
	rows := memoryRows(l)
	t.ensureRows(len(rows))
	var lines []layout.Widget
	for i, r := range rows {
		if r.note != nil && (i == 0 || rows[i-1].note == nil) {
			lines = append(lines, toolsGroupLine(th, i18n.T("chat.memory.saved"), ""))
		}
		lines = append(lines, func(gtx C) D {
			return pickerRow(gtx, th, &t.rows[i], i == t.cursor, func(gtx C, th *ui.Theme) D { return memoryRowView(gtx, th, sv.path, r) })
		})
	}
	head := func(gtx C) D {
		return toolsHead(gtx, th, i18n.T("chat.memory.title"), i18n.T("chat.memory.fileCount", i18n.Args{"count": len(l.Files)}), nil)
	}
	return t.scrolled(gtx, th, head, lines, nil)
}

func memoryRowView(gtx C, th *ui.Theme, checkout string, r memoryRow) D {
	switch {
	case r.toggle != nil:
		return memorySettingRow(gtx, th, r.setting, *r.toggle)
	case r.note != nil:
		return toolsLine(gtx, th, icMemoryNote, th.MutedForeground, th.Text(ui.TextSM, strings.TrimSuffix(r.note.Name, ".md")), r.note.Description, 0)
	}
	f := *r.file
	label := memoryFileLabel(checkout, f)
	name, meta := th.Text(ui.TextSM, label), memoryFilePlace(checkout, f.Path)
	if meta == label {
		meta = ""
	}
	if !f.Exists {
		name, meta = name.Muted(), i18n.T("chat.memory.missing")
	}
	var indent unit.Dp
	if f.Imported {
		indent = memoryImportIndent
	}
	return toolsLine(gtx, th, icMemoryFile, th.MutedForeground, name, meta, indent)
}

// memoryFileLabel is what a file's row is called: the person's and the
// project's own by what they are, any other by its path in the checkout.
func memoryFileLabel(checkout string, f api.ChatMemoryFile) string {
	switch f.Kind {
	case api.MemoryUser:
		return i18n.T("chat.memory.user")
	case api.MemoryProject:
		return i18n.T("chat.memory.project")
	}
	return memoryFilePlace(checkout, f.Path)
}

// memoryFilePlace is where a file is: its path in the checkout, or its whole
// path outside it.
func memoryFilePlace(checkout, path string) string {
	if rel, err := filepath.Rel(checkout, path); checkout != "" && err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return displayPath(path)
}

func memorySettingRow(gtx C, th *ui.Theme, setting string, t api.ChatMemoryToggle) D {
	label := i18n.T("chat.memory.auto")
	if setting == api.MemoryDream {
		label = i18n.T("chat.memory.dream")
	}
	ic, ink, name := icCheck, th.TonePass, th.Text(ui.TextSM, label)
	if !t.Enabled {
		ic, ink, name = icToolsOff, th.MutedForeground, name.Muted()
	}
	if t.Locked {
		ink = ui.Alpha(ink, 0.55)
	}
	return toolsLine(gtx, th, ic, ink, name, t.Detail, 0)
}

// toolsLine is a panel row: a glyph, the name, and a muted end.
func toolsLine(gtx C, th *ui.Theme, ic *icons.Icon, ink color.NRGBA, name ui.LabelStyle, meta string, indent unit.Dp) D {
	return layout.Inset{Left: indent}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return ic.Layout(gtx, toolsGlyph, ink) }),
			ui.Gap(2),
			layout.Rigid(name.Truncated().Layout),
			ui.Gap(toolsMetaGap),
			layout.Flexed(1, func(gtx C) D {
				return layout.E.Layout(gtx, th.Text(ui.TextXS, cleanLine(meta)).Muted().Truncated().Layout)
			}),
		)
	})
}

func (u *window) agentsView(gtx C, th *ui.Theme, p *chatPane, sv sessionView, l api.ChatAgentList) D {
	t := &p.tools
	if !l.Reported {
		return toolsNote(gtx, th, i18n.T("chat.agents.none", i18n.Args{"provider": u.providerName(sv)}))
	}
	agents := agentsOrdered(l.Agents)
	t.ensureRows(len(agents))
	var lines []layout.Widget
	group := ""
	for i, a := range agents {
		if g := agentGroup(a.Source); g != group {
			group = g
			lines = append(lines, toolsGroupLine(th, agentGroupLabel(g), agentGroupDir(sv.path, g, a.Path)))
		}
		lines = append(lines, func(gtx C) D {
			return pickerRow(gtx, th, &t.rows[i], i == t.cursor, func(gtx C, th *ui.Theme) D { return agentRow(gtx, th, a) })
		})
	}
	head := func(gtx C) D {
		return toolsHead(gtx, th, i18n.T("chat.agents.title"), i18n.T("chat.agents.count", i18n.Args{"count": len(agents)}), nil)
	}
	var foot layout.Widget
	if len(l.Create) > 0 {
		foot = t.newAgentLine(th, l.Create)
	}
	return t.scrolled(gtx, th, head, lines, foot)
}

func agentGroupLabel(g string) string {
	switch g {
	case api.AgentProject:
		return i18n.T("chat.tools.group.project")
	case api.AgentUser:
		return i18n.T("chat.tools.group.user")
	case api.AgentPlugin:
		return i18n.T("chat.agents.group.plugin")
	case api.AgentManaged:
		return i18n.T("chat.agents.group.managed")
	case api.AgentBuiltIn:
		return i18n.T("chat.agents.group.builtIn")
	}
	return i18n.T("chat.tools.group.misc")
}

// agentGroupDir is the folder a group's files are in, where they all share
// one.
func agentGroupDir(checkout, g, path string) string {
	if path == "" || (g != api.AgentProject && g != api.AgentUser) {
		return ""
	}
	return memoryFilePlace(checkout, filepath.Dir(path))
}

func agentRow(gtx C, th *ui.Theme, a api.ChatAgent) D {
	var meta []string
	if a.Model != "" {
		meta = append(meta, a.Model)
	}
	if len(a.Tools) > 0 {
		meta = append(meta, i18n.T("chat.tools.toolCount", i18n.Args{"count": len(a.Tools)}))
	}
	glyph := func(gtx C) D { return icAgent.Layout(gtx, toolsGlyph, th.MutedForeground) }
	end := []layout.FlexChild{layout.Rigid(th.Text(ui.TextXS, strings.Join(meta, " · ")).Muted().Layout)}
	return descRow(gtx, th, glyph, th.Text(ui.TextSM, a.Name).Truncated().Layout, a.Description, end)
}

// newAgentLine is the /agents panel's foot: the new subagent's name and
// description, and a button for each place it can go.
func (t *chatTools) newAgentLine(th *ui.Theme, create []string) layout.Widget {
	return func(gtx C) D {
		return layout.UniformInset(ui.Space(1.5)).Layout(gtx, func(gtx C) D {
			name := th.Input(&t.mem.name, i18n.T("chat.agents.name"))
			name.Height, name.TextSize = ui.Space(8), ui.TextXS
			desc := th.Input(&t.mem.desc, i18n.T("chat.agents.description"))
			desc.Height, desc.TextSize = ui.Space(8), ui.TextXS
			row := []layout.FlexChild{
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(agentNameW), gtx.Dp(agentNameW)
					return name.Layout(gtx)
				}),
				ui.Gap(1.5),
				layout.Flexed(1, desc.Layout),
			}
			for _, scope := range create {
				label := i18n.T("chat.agents.addUser")
				if scope == api.AgentProject {
					label = i18n.T("chat.agents.addProject")
				}
				b := th.Button(t.act(agentAddKey(scope)), label)
				b.Variant, b.Size = ui.VariantSecondary, ui.SizeSM
				row = append(row, ui.Gap(1.5), layout.Rigid(b.Layout))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
		})
	}
}
