package main

import (
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// serviceNameWidth is the name field's width in the dialog: a name is a word,
// the command takes the rest of the line.
const serviceNameWidth unit.Dp = 160

var icServiceHelp = icons.Lucide("circle-question-mark")

// begin opens the dialog on v, a saved service or a new one of its project
// for an empty ID, filled with what it holds. projectName says whose it is.
func (p *servicesSettings) begin(v backend.Service, projectName string) {
	p.open, p.focus = true, true
	p.svc, p.projectName = v, projectName
	p.setErr("")
	p.name.SetText(v.Name)
	p.command.SetText(v.Command)
	p.withLich.Value = v.StartWithLich
}

// layoutServiceDialog is the service's dialog: the name and the command side
// by side, the line that says how a command runs, Start with lich, and
// Delete (for a saved service that is down), Cancel and Save. Lay it out
// before the content, so it claims Escape ahead of anything under it.
func (s *settingsScreen) layoutServiceDialog(gtx C, th *ui.Theme) {
	p := &s.services
	if !p.open && !p.dialog.Animating(gtx) {
		return
	}
	v := p.svc
	if p.cancel.Clicked(gtx) {
		p.open = false
	}
	if p.save.Clicked(gtx) && p.open {
		opts := api.SaveServiceOptions{ID: v.ID, ProjectID: v.ProjectID, Name: p.name.Text(), Command: p.command.Text(), StartWithLich: p.withLich.Value}
		go s.saveService(opts)
	}
	if p.del.Clicked(gtx) && p.open {
		go s.deleteService(v.ID)
	}
	p.mu.Lock()
	saving, errText := p.saving, p.err
	p.mu.Unlock()
	on := th.On(th.Popover)
	title := i18n.T("settings.servicesSettings.add")
	if v.ID != "" {
		title = i18n.T("settings.servicesSettings.editTitle", i18n.Args{"name": v.Name})
	}
	cancel := on.Button(&p.cancel, i18n.T("common.action.cancel"))
	cancel.Variant = ui.VariantGhost
	save := on.Button(&p.save, i18n.T("settings.servicesSettings.save"))
	save.Disabled = saving
	var actions []layout.Widget
	if v.ID != "" && !serviceUp(v) {
		del := on.Button(&p.del, i18n.T("sidebar.services.delete"))
		del.Variant = ui.VariantDestructive
		actions = append(actions, func(gtx C) D { return del.Layout(gtx) })
	}
	d := th.Dialog(&p.dialog, p.open, title, p.projectName)
	d.Body = func(gtx C) D { return s.serviceFields(gtx, on, errText) }
	d.Actions = append(actions, cancel.Layout, save.Layout)
	if d.Layout(gtx) {
		p.open = false
	}
	if p.focus && p.open {
		gtx.Execute(key.FocusCmd{Tag: &p.name})
		p.focus = false
	}
}

// serviceFields is the dialog's body.
func (s *settingsScreen) serviceFields(gtx C, th *ui.Theme, errText string) D {
	p := &s.services
	example := func(value string) string {
		return i18n.T("settings.servicesSettings.example", i18n.Args{"value": value})
	}
	name := th.Input(&p.name, example(i18n.T("settings.servicesSettings.namePlaceholder")))
	command := th.Textarea(&p.command, example(i18n.T("settings.servicesSettings.commandPlaceholder")))
	command.MaxHeight = ui.Space(32)
	field := func(label string, w layout.Widget) layout.Widget {
		return func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(th.Label(label).Layout), ui.VGap(1.5), layout.Rigid(w))
		}
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(serviceNameWidth), gtx.Dp(serviceNameWidth)
					return field(i18n.T("settings.servicesSettings.name"), name.Layout)(gtx)
				}),
				ui.Gap(3),
				layout.Flexed(1, field(i18n.T("settings.servicesSettings.command"), command.Layout)),
			)
		}),
		ui.VGap(1.5),
		layout.Rigid(func(gtx C) D { return s.commandHint(gtx, th) }),
		ui.VGap(4),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(th.Switch(&p.withLich).Layout), ui.Gap(2),
				layout.Rigid(th.Text(ui.TextSM, i18n.T("settings.servicesSettings.startWithLich")).Layout),
			)
		}),
	}
	if errText != "" {
		children = append(children, ui.VGap(3), layout.Rigid(func(gtx C) D {
			l := th.Text(ui.TextXS, errText)
			l.Color = th.Destructive
			return l.Wrapped().Layout(gtx)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// commandHint is how a command runs, in one line, and a help glyph whose
// tooltip says the rest.
func (s *settingsScreen) commandHint(gtx C, th *ui.Theme) D {
	why := th.Tooltip(s.root, &s.services.why, i18n.T("settings.servicesSettings.commandHint"))
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, th.Text(ui.TextXS, i18n.T("settings.servicesSettings.commandHintShort")).Muted().Wrapped().Layout),
		ui.Gap(1.5),
		layout.Rigid(func(gtx C) D {
			return why.Layout(gtx, func(gtx C) D { return icServiceHelp.Layout(gtx, 12, th.MutedForeground) })
		}),
	)
}

// serviceDialogOf is what the dialog opens on for id in project: the saved
// service, or a new one of the project for an empty id.
func serviceDialogOf(store *servicesStore, projectID, id string) backend.Service {
	if v, ok := store.byID(id); ok && id != "" {
		return v
	}
	return backend.Service{ProjectID: projectID}
}
