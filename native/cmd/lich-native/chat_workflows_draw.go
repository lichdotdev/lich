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

// chatWorkflow draws the line of session id's workflow over the composer, none
// while it has none to show.
func (u *window) chatWorkflow(gtx C, th *ui.Theme, p *chatPane, id string) D {
	wf, ok := workflowShown(u.model.chat.workflowsOf(id), u.model.chat.of(id))
	if !ok {
		return D{}
	}
	s := &p.workflow
	if s.toggle.Clicked(gtx) {
		p.toggleWorkflows()
	}
	fg := th.MutedForeground
	if s.toggle.Hovered() || p.tools.kind == chatWorkflowsPanel {
		fg = th.Foreground
	}
	ic, ink := icWorkflow, fg
	end := th.Text(ui.TextSM, workflowRunning(wf)).Truncated().Shimmer()
	if wf.State == api.WorkflowFailed {
		ic, ink = icToolsFailed, th.Destructive
		end = th.Text(ui.TextSM, i18n.T("chat.workflow.failed")).In(th.Destructive)
	}
	count := i18n.T("chat.workflow.count", i18n.Args{"name": wf.Name, "done": workflowDone(wf), "total": len(wf.Agents)})
	line := func(gtx C) D {
		return s.toggle.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(1), Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return ic.Layout(gtx, chatCrumbGlyph, ink) }),
					ui.Gap(2),
					layout.Rigid(th.Text(ui.TextSM, count).In(fg).Layout),
					ui.Gap(2),
					layout.Flexed(1, end.Layout),
					ui.Gap(2),
					layout.Rigid(th.Text(ui.TextXS, took(gtx, wf.Started, wf.Ended)).Mono().Muted().Layout),
				)
			})
		})
	}
	return layout.Inset{Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return th.Tooltip(&u.root, &s.tip, workflowTip(wf, gtx.Now)).Layout(gtx, line)
	})
}

// workflowTip is the line's tooltip: the phase running, how many agents and
// when the workflow started.
func workflowTip(wf backend.ChatWorkflow, now time.Time) string {
	parts := []string{
		i18n.T("chat.workflow.agents", i18n.Args{"count": len(wf.Agents)}),
		i18n.T("chat.workflow.started", i18n.Args{"time": datedClock(time.UnixMilli(wf.Started).In(now.Location()), now)}),
	}
	if ph := workflowPhase(wf); ph != "" {
		parts = append([]string{ph}, parts...)
	}
	return strings.Join(parts, " · ")
}

// agentState is an agent's glyph and its ink.
func agentState(th *ui.Theme, a api.ChatWorkflowAgent) (*icons.Icon, color.NRGBA) {
	switch a.State {
	case api.WorkflowRunning:
		return icToolsPending, th.ToneWait
	case api.WorkflowCompleted:
		return icCheck, th.TonePass
	case api.WorkflowFailed:
		return icToolsFailed, th.Destructive
	}
	return icPlanPending, th.MutedForeground
}

// workflowsView is the panel: the workflow's name and count, then its agents
// under a line per phase, or one agent's detail.
func (u *window) workflowsView(gtx C, th *ui.Theme, p *chatPane, sv sessionView) D {
	t := &p.tools
	wf, ok := workflowShown(u.model.chat.workflowsOf(sv.ID), u.model.chat.of(sv.ID))
	if !ok {
		return D{}
	}
	if i := slices.IndexFunc(wf.Agents, func(a api.ChatWorkflowAgent) bool { return agentKey(a) == t.detail }); i >= 0 {
		return agentDetail(gtx, th, t, wf.Agents[i])
	}
	t.ensureRows(len(wf.Agents))
	var lines []layout.Widget
	phase := ""
	for i, a := range wf.Agents {
		if a.Phase != phase {
			phase = a.Phase
			lines = append(lines, toolsGroupLine(th, a.Phase, ""))
		}
		lines = append(lines, func(gtx C) D {
			return toolsPick(gtx, th, &t.rows[i], i == t.cursor, func(gtx C, th *ui.Theme) D { return workflowAgentRow(gtx, th, a) })
		})
	}
	head := func(gtx C) D {
		return toolsHead(gtx, th, wf.Name, i18n.T("chat.workflow.progress", i18n.Args{"done": workflowDone(wf), "total": len(wf.Agents)}), nil)
	}
	return t.scrolled(gtx, th, head, lines, nil)
}

func workflowAgentRow(gtx C, th *ui.Theme, a api.ChatWorkflowAgent) D {
	ic, ink := agentState(th, a)
	label := th.Text(ui.TextSM, a.Label).Weight(font.Medium)
	if a.State == api.WorkflowRunning {
		label = label.Shimmer()
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return ic.Layout(gtx, toolsGlyph, ink) }),
		ui.Gap(2),
		layout.Flexed(1, label.Truncated().Layout),
		ui.Gap(toolsMetaGap),
		layout.Rigid(th.Text(ui.TextXS, took(gtx, a.Started, a.Ended)).Mono().Muted().Layout),
	)
}

// agentDetail is one agent: Back and its label, then its phase, model, time
// and counts, the call it makes or made last, its task, and its answer or
// why it failed.
func agentDetail(gtx C, th *ui.Theme, t *chatTools, a api.ChatWorkflowAgent) D {
	head := func(gtx C) D {
		return toolsBackHead(gtx, th, &t.back, i18n.T("chat.workflow.back"), th.Text(ui.TextSM, a.Label).Weight(font.SemiBold), took(gtx, a.Started, a.Ended), nil)
	}
	return t.scrolled(gtx, th, head, agentFacts(gtx, th, a), nil)
}

func agentFacts(gtx C, th *ui.Theme, a api.ChatWorkflowAgent) []layout.Widget {
	var lines []layout.Widget
	text := func(label, value string, mono bool) {
		if value == "" {
			return
		}
		l := th.Text(ui.TextSM, value).Truncated()
		if mono {
			l = th.Text(ui.TextXS, value).Mono().Truncated()
		}
		lines = append(lines, mcpFact(th, label, l.Layout))
	}
	text(i18n.T("chat.workflow.fact.phase"), a.Phase, false)
	text(i18n.T("chat.workflow.fact.model"), formatModel(a.Model), false)
	text(i18n.T("chat.workflow.fact.time"), agentTime(gtx, a), true)
	call := strings.TrimSuffix(a.Tool+" · "+cleanLine(a.ToolInput), " · ")
	if a.State == api.WorkflowRunning {
		text(i18n.T("chat.workflow.fact.now"), call, true)
	} else {
		text(i18n.T("chat.workflow.fact.last"), call, true)
	}
	for _, f := range []struct{ label, text string }{
		{i18n.T("chat.workflow.fact.task"), a.Prompt},
		{i18n.T("chat.workflow.fact.result"), a.Result},
	} {
		if f.text != "" {
			lines = append(lines, mcpFact(th, f.label, th.Text(ui.TextSM, f.text).Muted().Wrapped().Layout))
		}
	}
	if a.Error != "" {
		lines = append(lines, toolsFootNoteIn(th, a.Error, th.Destructive))
	}
	return lines
}

// agentTime is how long the agent has run, its calls and its tokens.
func agentTime(gtx C, a api.ChatWorkflowAgent) string {
	var parts []string
	if d := took(gtx, a.Started, a.Ended); d != "" {
		parts = append(parts, d)
	}
	if a.ToolCalls > 0 {
		parts = append(parts, i18n.T("chat.workflow.calls", i18n.Args{"count": a.ToolCalls}))
	}
	if a.Tokens > 0 {
		parts = append(parts, i18n.T("chat.workflow.tokens", i18n.Args{"n": compactCount(a.Tokens, i18n.Current())}))
	}
	return strings.Join(parts, " · ")
}

func toolsFootNoteIn(th *ui.Theme, s string, ink color.NRGBA) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1)}.Layout(gtx, th.Text(ui.TextXS, s).In(ink).Wrapped().Layout)
	}
}
