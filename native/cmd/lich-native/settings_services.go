package main

import (
	"sync"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const sectionServices = "services"

// servicesKey names the Services row for the search; no setting is stored
// under it, the services live in their own table.
const servicesKey = "services.list"

// servicesSettings is the Services pane, the project's services as a list,
// and the dialog that adds or edits one.
type servicesSettings struct {
	store *servicesStore
	// home is the id of the project Home shows, whose services Global edits:
	// Home has no scope of its own, and its services run in the home
	// directory, which is as global as a service gets.
	home func() string

	open        bool
	focus       bool // the name asks for focus the frame after opening
	dialog      ui.DialogState
	svc         backend.Service // what the dialog edits; a new one has no ID
	projectName string
	name        widget.Editor
	command     widget.Editor
	withLich    ui.SwitchState
	why         ui.Tooltip
	save        ui.Clickable
	cancel      ui.Clickable
	del         ui.Clickable
	add         ui.Clickable
	rows        map[string]*ui.Clickable

	mu     sync.Mutex
	saving bool
	err    string
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

// servicesPaneRows is the pane under its title: the project's services, then
// Add service.
func (s *settingsScreen) servicesPaneRows(th *ui.Theme) []layout.Widget {
	p := &s.services
	if p.rows == nil {
		p.rows = map[string]*ui.Clickable{}
	}
	projectID := p.projectOf(s.scope)
	var rows []layout.Widget
	if projectID == "" || p.store == nil {
		return rows
	}
	for _, v := range p.store.of(projectID) {
		rows = append(rows, func(gtx C) D { return s.serviceRow(gtx, th, v) })
	}
	rows = append(rows, func(gtx C) D {
		if p.add.Clicked(gtx) {
			p.begin(serviceDialogOf(p.store, projectID, ""), s.projectName(projectID))
		}
		b := th.Button(&p.add, i18n.T("settings.servicesSettings.add"))
		b.Variant, b.Size, b.Icon = ui.VariantGhost, ui.SizeSM, icPlus
		return layout.Inset{Top: ui.Space(1)}.Layout(gtx, func(gtx C) D { return layout.W.Layout(gtx, b.Layout) })
	})
	return rows
}

// serviceRow is one saved service, a click that opens its dialog: its name,
// Starts with lich when it does, and its command.
func (s *settingsScreen) serviceRow(gtx C, th *ui.Theme, v backend.Service) D {
	p := &s.services
	click := clickable(p.rows, v.ID)
	if click.Clicked(gtx) {
		p.begin(v, s.projectName(v.ProjectID))
	}
	name := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, v.Name).Weight(font.Medium).Layout)}
	if v.StartWithLich {
		name = append(name, ui.Gap(2), layout.Rigid(th.Text(ui.TextXS, i18n.T("settings.servicesSettings.startsWithLich")).Muted().Layout))
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, func(gtx C) D {
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, cardLayer(th, "", false, click.Hovered()), ui.RadiusMD) },
			func(gtx C) D {
				return layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, name...) }),
						ui.VGap(0.5),
						layout.Rigid(th.Text(ui.TextXS, v.Command).Mono().Muted().Layout),
					)
				})
			},
		)
	})
}

// projectName is the name projectID goes by, for the dialog's subtitle.
func (s *settingsScreen) projectName(projectID string) string {
	for _, p := range s.projects {
		if p.id == projectID {
			return p.name
		}
	}
	return ""
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
	p.open = false
	s.invalidate()
}

func (s *settingsScreen) deleteService(id string) {
	if err := s.client.DeleteService(s.ctx, id); err != nil {
		s.services.setErr(backend.ErrorText(err))
		s.invalidate()
		return
	}
	s.services.open = false
	s.invalidate()
}
