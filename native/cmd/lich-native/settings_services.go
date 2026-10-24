package main

import (
	"image"
	"sync"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

const sectionServices = "services"

// servicesKey names the Services row for the search; no setting is stored
// under it, the services live in their own table.
const servicesKey = "services.list"

// newServiceRow is the id the form edits when it adds a service.
const newServiceRow = "+new"

var icServiceEdit = icons.Lucide("pencil")

// servicesSettings is the Services pane: the project's services as a list,
// one of them, or a new one, open in a form.
type servicesSettings struct {
	store *servicesStore
	// home is the id of the project Home shows, whose services Global edits:
	// Home has no scope of its own, and its services run in the home
	// directory, which is as global as a service gets.
	home func() string

	editing  string // a service id, newServiceRow, or "" for none
	name     widget.Editor
	command  widget.Editor
	withLich widget.Bool
	save     widget.Clickable
	cancel   widget.Clickable
	add      widget.Clickable
	edits    map[string]*widget.Clickable
	deletes  map[string]*widget.Clickable

	mu     sync.Mutex
	saving bool
	err    string
}

// begin opens the form on service id, newServiceRow for a new one, filled
// with what the store holds for it.
func (p *servicesSettings) begin(id string) {
	p.editing = id
	p.setErr("")
	v, _ := p.store.byID(id)
	p.name.SetText(v.Name)
	p.command.SetText(v.Command)
	p.withLich.Value = v.StartWithLich
}

// projectOf is the project whose services scope edits: the scope's own, or
// Home's in Global; "" while Home is not known.
func (p *servicesSettings) projectOf(scope string) string {
	if scope != "" || p.home == nil {
		return scope
	}
	return p.home()
}

func (p *servicesSettings) setErr(text string) {
	p.mu.Lock()
	p.err = text
	p.mu.Unlock()
}

// servicesPaneRows is the pane under its title: what a service is, then the
// project's services, then Add service. Global has none to show.
func (s *settingsScreen) servicesPaneRows(th *ui.Theme) []layout.Widget {
	p := &s.services
	if p.edits == nil {
		p.edits, p.deletes = map[string]*widget.Clickable{}, map[string]*widget.Clickable{}
	}
	desc := i18n.T("settings.servicesSettings.description")
	if s.scope == "" {
		desc = i18n.T("settings.servicesSettings.globalDescription")
	}
	projectID := p.projectOf(s.scope)
	rows := []layout.Widget{func(gtx C) D {
		return layout.Inset{Top: -ui.Space(2), Bottom: ui.Space(4)}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Sp(settingsDescSize)*settingsDescCh*6/10)
			l := th.Text(settingsDescSize, desc).Muted().Leading(20)
			l.MaxLines = 0
			return l.Layout(gtx)
		})
	}}
	if projectID == "" || p.store == nil {
		return rows
	}
	for _, v := range p.store.of(projectID) {
		rows = append(rows, func(gtx C) D { return s.serviceRow(gtx, th, v) })
	}
	if p.editing == newServiceRow {
		rows = append(rows, func(gtx C) D {
			return s.serviceForm(gtx, th, backend.Service{ServiceConfig: api.ServiceConfig{ProjectID: projectID}})
		})
		return rows
	}
	rows = append(rows, func(gtx C) D {
		if p.add.Clicked(gtx) {
			p.begin(newServiceRow)
		}
		b := th.Button(&p.add, i18n.T("settings.servicesSettings.add"))
		b.Variant, b.Size, b.Icon = ui.VariantOutline, ui.SizeSM, icPlus
		return layout.Inset{Top: ui.Space(3)}.Layout(gtx, b.Layout)
	})
	return rows
}

// serviceRow is one saved service: its name and command, Edit and Delete; or
// the form, while it is the one being edited.
func (s *settingsScreen) serviceRow(gtx C, th *ui.Theme, v backend.Service) D {
	p := &s.services
	if p.editing == v.ID {
		return s.serviceForm(gtx, th, v)
	}
	edit, del := clickable(p.edits, v.ID), clickable(p.deletes, v.ID)
	if edit.Clicked(gtx) {
		p.begin(v.ID)
	}
	if del.Clicked(gtx) {
		go s.deleteService(v.ID)
	}
	name := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, v.Name).Weight(font.Medium).Layout)}
	if v.StartWithLich {
		name = append(name, ui.Gap(2), layout.Rigid(th.Text(ui.TextXS, i18n.T("settings.servicesSettings.startsWithLich")).Muted().Layout))
	}
	return layout.Inset{Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, name...) }),
					ui.VGap(0.5),
					layout.Rigid(th.Text(ui.TextXS, v.Command).Mono().Muted().Layout),
				)
			}),
			layout.Rigid(func(gtx C) D { return cornerButton(gtx, th, edit, icServiceEdit, false) }),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D {
				if serviceUp(v) {
					return D{Size: image.Pt(gtx.Dp(16), gtx.Dp(16))}
				}
				return cornerButton(gtx, th, del, icServiceDelete, false)
			}),
		)
	})
}

// serviceForm edits v, or a new service for a zero v: name, command, start
// with lich, and the reason a save was refused.
func (s *settingsScreen) serviceForm(gtx C, th *ui.Theme, v backend.Service) D {
	p := &s.services
	if p.cancel.Clicked(gtx) {
		p.editing = ""
	}
	if p.save.Clicked(gtx) {
		opts := api.SaveServiceOptions{ID: v.ID, ProjectID: v.ProjectID, Name: p.name.Text(), Command: p.command.Text(), StartWithLich: p.withLich.Value}
		go s.saveService(opts)
	}
	p.mu.Lock()
	saving, errText := p.saving, p.err
	p.mu.Unlock()
	name := th.Input(&p.name, i18n.T("settings.servicesSettings.namePlaceholder"))
	command := th.Textarea(&p.command, i18n.T("settings.servicesSettings.commandPlaceholder"))
	command.MaxHeight = ui.Space(32)
	save := th.Button(&p.save, i18n.T("settings.servicesSettings.save"))
	save.Size, save.Disabled = ui.SizeSM, saving
	cancel := th.Button(&p.cancel, i18n.T("common.action.cancel"))
	cancel.Variant, cancel.Size = ui.VariantGhost, ui.SizeSM
	children := []layout.FlexChild{
		layout.Rigid(th.Label(i18n.T("settings.servicesSettings.name")).Layout), ui.VGap(1.5),
		layout.Rigid(name.Layout), ui.VGap(3),
		layout.Rigid(th.Label(i18n.T("settings.servicesSettings.command")).Layout), ui.VGap(1.5),
		layout.Rigid(command.Layout), ui.VGap(1.5),
		layout.Rigid(func(gtx C) D {
			return wrapped(gtx, th.Text(ui.TextXS, i18n.T("settings.servicesSettings.commandHint")).Muted())
		}),
		ui.VGap(3),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(th.Checkbox(&p.withLich).Layout), ui.Gap(2),
				layout.Rigid(th.Text(ui.TextSM, i18n.T("settings.servicesSettings.startWithLich")).Layout),
			)
		}),
	}
	if errText != "" {
		children = append(children, ui.VGap(2), layout.Rigid(func(gtx C) D {
			l := th.Text(ui.TextXS, errText)
			l.Color = th.Destructive
			return wrapped(gtx, l)
		}))
	}
	children = append(children, ui.VGap(3), layout.Rigid(func(gtx C) D {
		return layout.Flex{}.Layout(gtx, layout.Rigid(cancel.Layout), ui.Gap(2), layout.Rigid(save.Layout))
	}))
	return layout.Inset{Top: ui.Space(2), Bottom: ui.Space(3)}.Layout(gtx, func(gtx C) D {
		return layout.UniformInset(ui.Space(3)).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (s *settingsScreen) saveService(opts api.SaveServiceOptions) {
	p := &s.services
	p.mu.Lock()
	p.saving = true
	p.mu.Unlock()
	_, err := s.client.SaveService(s.ctx, opts)
	p.mu.Lock()
	p.saving = false
	p.mu.Unlock()
	if err != nil {
		p.setErr(backend.ErrorText(err))
		s.invalidate()
		return
	}
	p.setErr("")
	p.editing = ""
	s.invalidate()
}

func (s *settingsScreen) deleteService(id string) {
	if err := s.client.DeleteService(s.ctx, id); err != nil {
		s.services.setErr(backend.ErrorText(err))
	}
	s.invalidate()
}
