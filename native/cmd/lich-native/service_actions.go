package main

import (
	"context"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// applyService carries out what a click asked of a service. The ones the
// backend does run off the frame, a failure in a toast; the store hears what
// changed from the backend, never from here.
func (u *window) applyService(r serviceRequest) {
	switch r.op {
	case serviceNone:
		return
	case serviceShow:
		u.settings.hide()
		u.pulls.hide()
		u.model.openConsole(r.id)
		u.model.invalidate()
	case serviceEdit:
		if v, ok := u.model.services.byID(r.id); ok {
			u.openServiceSettings(v.ProjectID, r.id)
		}
	case serviceAdd:
		u.openServiceSettings(r.project, "")
	default:
		go u.runServiceOp(r)
	}
}

// serviceCalls are the service ops the backend carries out, each as its call.
var serviceCalls = map[serviceOp]func(c *backend.Client, ctx context.Context, r serviceRequest) error{
	serviceStart: func(c *backend.Client, ctx context.Context, r serviceRequest) error { return c.StartService(ctx, r.id) },
	serviceStop:  func(c *backend.Client, ctx context.Context, r serviceRequest) error { return c.StopService(ctx, r.id) },
	serviceRestart: func(c *backend.Client, ctx context.Context, r serviceRequest) error {
		return c.RestartService(ctx, r.id)
	},
	serviceForceStop: func(c *backend.Client, ctx context.Context, r serviceRequest) error {
		return c.ForceStopService(ctx, r.id)
	},
	serviceDelete: func(c *backend.Client, ctx context.Context, r serviceRequest) error {
		return c.DeleteService(ctx, r.id)
	},
	serviceStartAll: func(c *backend.Client, ctx context.Context, r serviceRequest) error {
		return c.StartAllServices(ctx, r.project)
	},
	serviceStopAll: func(c *backend.Client, ctx context.Context, r serviceRequest) error {
		return c.StopAllServices(ctx, r.project)
	},
}

func (u *window) runServiceOp(r serviceRequest) {
	call, ok := serviceCalls[r.op]
	if !ok {
		return
	}
	if err := call(u.client, u.ctx, r); err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("sidebar.services.failedOp", i18n.Args{"error": backend.ErrorText(err)})})
	}
}

// revealService is the tray's Show output: the service's project comes on
// screen with its console, its tab reopened when it was closed, since a
// service keeps running past its project's tab.
func (u *window) revealService(id string) {
	v, ok := u.model.services.byID(id)
	if !ok {
		return
	}
	if !u.model.setActiveProject(v.ProjectID) && !u.reopenServiceProject(v.ProjectID) {
		return
	}
	u.hideSettings.Store(true)
	u.model.openConsole(id)
	u.model.invalidate()
	u.saveScreen(v.ProjectID)
}

// openServiceSettings opens Settings on projectID's Services with service id
// in the form, a new one for "".
func (u *window) openServiceSettings(projectID, id string) {
	u.pulls.hide()
	u.settings.openSection(sectionServices)
	tabs, _ := u.model.projectTabs()
	u.settings.show(projectID, settingsProjectsOf(tabs))
	if id == "" {
		id = newServiceRow
	}
	u.settings.services.begin(id)
	u.model.invalidate()
}

// reopenServiceProject opens the closed project projectID again, and reports
// whether it is on screen.
func (u *window) reopenServiceProject(projectID string) bool {
	recent, err := u.client.RecentProjects(u.ctx)
	if err != nil {
		u.logErr("reopen a service's project", err)
		return false
	}
	for _, r := range recent {
		if r.ID == projectID {
			if err := u.adopt(r); err != nil {
				u.logErr("reopen a service's project", err)
				return false
			}
			return u.model.setActiveProject(projectID)
		}
	}
	return false
}
