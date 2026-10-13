package main

import (
	"fmt"
	"image"
	"runtime"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui"
)

var (
	// statusDot is the strip's size-[0.4375rem] dot.
	statusDot = unit.Dp(7)
	// proseCh is max-w-prose, a ch taken as 0.6em as settingsDescCh is.
	proseCh = 65
)

// sandboxLadder is SandboxSettings above the grants: the machine's answer,
// then, on a machine that can confine, one rung row per enabled provider and
// the hint under them. Nothing is drawn until the machine has answered.
func (s *settingsScreen) sandboxLadder(gtx C, th *ui.Theme) D {
	m := s.sandbox.snapshot()
	if !m.loaded {
		return D{}
	}
	children := []layout.FlexChild{layout.Rigid(func(gtx C) D {
		return layout.Inset{Left: rowPadX, Right: rowPadX}.Layout(gtx, func(gtx C) D { return sandboxStatus(gtx, th, m.backend) })
	})}
	if m.backend == "" {
		_, advice := cannotConfineCopy(runtime.GOOS)
		children = append(children, ui.VGap(2), layout.Rigid(proseLine(th, advice)))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	values := s.snapshot()
	children = append(children, ui.VGap(2), layout.Rigid(proseLine(th, sandboxSummary)),
		layout.Rigid(groupHeading(th, confinedTitle, false)))
	for i, p := range s.providers.book.launchProviders() {
		children = append(children, layout.Rigid(func(gtx C) D { return s.rungRow(gtx, th, m, p, values, i == 0) }))
	}
	children = append(children, ui.VGap(2), layout.Rigid(proseLine(th, confinedHint)))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// rungRow is one provider's ladder, its scope state read from its own key.
func (s *settingsScreen) rungRow(gtx C, th *ui.Theme, m sandboxMachine, p launchProvider, values scopedValues, first bool) D {
	e := settingEntry{title: p.name, key: sandboxKey(p.id), scope: scopeOverridable}
	r := s.rowFor(e, values)
	if r.kind == rowChanged {
		r.globalValue = rungLabel(sandboxLevel(m.globals[e.key]))
		if r.reset.Clicked(gtx) {
			s.useGlobal(e.key, s.scope)
			s.refresh()
		}
	}
	level := sandboxLevel(effectiveValue(values, m.globals, e.key, s.scope))
	enum := s.sandbox.rungEnum(p.id)
	if enum.Update(gtx) {
		s.chooseRung(p, level, enum.Value)
	}
	enum.Value = level
	items := make([]ui.ToggleItem, len(sandboxRungs))
	for i, rung := range sandboxRungs {
		items[i] = ui.ToggleItem{Value: rung.value, Text: rung.label}
	}
	return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D {
		g := th.ToggleGroup(enum, items...)
		g.Size = ui.ToggleSizeSM
		return g.Layout(gtx)
	})
}

func (s *sandboxSettings) rungEnum(providerID string) *widget.Enum {
	if s.rungs == nil {
		s.rungs = map[string]*widget.Enum{}
	}
	if s.rungs[providerID] == nil {
		s.rungs[providerID] = new(widget.Enum)
	}
	return s.rungs[providerID]
}

// chooseRung writes a tightening straight through and asks before a
// loosening: putting a session back in the sandbox is never the harder way.
func (s *settingsScreen) chooseRung(p launchProvider, current, next string) {
	if loosens(current, next) {
		s.sandbox.change = rungChange{providerID: p.id, providerName: p.name, level: next}
		s.sandbox.asking = true
		s.invalidate() // the dialog is laid out before the pane, a frame behind the click
		return
	}
	s.writeSandbox(sandboxKey(p.id), next)
}

// grantRow is Grant: one credential handed back to a confined session, with
// the line naming what exactly it hands over.
func (s *settingsScreen) grantRow(gtx C, th *ui.Theme, e settingEntry, r settingRow, first bool) D {
	m, values := s.sandbox.snapshot(), s.snapshot()
	sw, desc, detail := &s.sandbox.ssh, sshDesc, agentKeysLine(m.agentKeys)
	if e.key == ghTokenKey {
		account, _ := values.set(vcsAccountKey, s.scope)
		sw, desc, detail = &s.sandbox.gh, ghDesc, ghAccountLine(account)
	}
	if sw.Update(gtx) {
		s.writeSandbox(e.key, strconv.FormatBool(sw.Value))
	}
	sw.Value = effectiveValue(values, m.globals, e.key, s.scope) == "true"
	r.globalValue = grantLabel(m.globals[e.key])
	r.sample = func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(rowDescSize, desc).Muted().Leading(20)) }),
			ui.VGap(1),
			layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(ui.TextXS, detail).Muted().Mono().Leading(18)) }),
		)
	}
	return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D { return th.Switch(sw).Layout(gtx) })
}

func wrapped(gtx C, l ui.LabelStyle) D {
	l.MaxLines = 0
	return l.Layout(gtx)
}

// sandboxStatus is StatusStrip: the machine's answer drawn as state, the
// backend named because the two have different holes.
func sandboxStatus(gtx C, th *ui.Theme, backend string) D {
	dot, title, detail := th.TonePass, canConfineText, backend
	if backend == "" {
		dot, title = th.MutedForeground, cannotConfineText
		detail, _ = cannotConfineCopy(runtime.GOOS)
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			insetBorder(gtx, gtx.Constraints.Min, gtx.Dp(ui.RadiusMD), th.Over(th.Border))
			return D{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx C) D {
			return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2.5), Bottom: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(statusDot), gtx.Dp(statusDot)))
						return ui.Fill(gtx, dot, statusDot/2)
					}),
					ui.Gap(2.5),
					layout.Rigid(th.Text(ui.TextXS, title).Weight(font.Medium).Layout),
					ui.Gap(2.5),
					layout.Flexed(1, th.Text(ui.TextXS, statusDetailPrefix+detail).Muted().Layout),
				)
			})
		}),
	)
}

// proseLine is the pane's text-xs muted max-w-prose paragraph.
func proseLine(th *ui.Theme, text string) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Left: rowPadX, Right: rowPadX}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Sp(ui.TextXS)*proseCh*6/10)
			return wrapped(gtx, th.Text(ui.TextXS, text).Muted().Leading(16))
		})
	}
}

// layoutSandboxDialog is the rung's ConfirmDialog. The window lays it out
// with its own constraints, as every dialog.
func (s *settingsScreen) layoutSandboxDialog(gtx C, th *ui.Theme) {
	sb := &s.sandbox
	if sb.confirm.Clicked(gtx) && sb.asking {
		s.writeSandbox(sandboxKey(sb.change.providerID), sb.change.level)
		sb.asking = false
	}
	if sb.cancel.Clicked(gtx) {
		sb.asking = false
	}
	c := sb.change
	on := th.On(th.Popover)
	cancel := on.Button(&sb.cancel, dialogCancel)
	cancel.Variant = ui.VariantGhost
	confirm := on.Button(&sb.confirm, rungConfirm)
	confirm.Variant = ui.VariantDestructive
	dlg := th.Dialog(&sb.dialog, sb.asking && s.open, fmt.Sprintf(rungConfirmTitle, c.providerName),
		fmt.Sprintf(rungConfirmDesc, rungLabel(c.level), rungExposes(c.level)))
	dlg.MaxWidth = ui.DialogWidthXL
	dlg.Actions = []layout.Widget{cancel.Layout, confirm.Layout}
	if dlg.Layout(gtx) {
		sb.asking = false
	}
}
