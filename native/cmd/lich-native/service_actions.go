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
		u.extension.hide()
		u.model.openConsole(r.id)
		u.model.invalidate()
	case serviceEdit:
		if v, ok := u.model.services.byID(r.id); ok {
			u.openServiceDialog(v.ProjectID, r.id)
		}
	case serviceAdd:
		u.openServiceDialog(r.project, "")
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
// screen with its console. A running service's project has a tab open, since
// closing the tab stops it.
func (u *window) revealService(id string) {
	v, ok := u.model.services.byID(id)
	if !ok || !u.model.setActiveProject(v.ProjectID) {
		return
	}
	u.hideSettings.Store(true)
	u.model.openConsole(id)
	u.model.invalidate()
	u.saveScreen(v.ProjectID)
}

// openServiceDialog opens the service dialog where the window is, on
// projectID's service id, a new one for "".
func (u *window) openServiceDialog(projectID, id string) {
	u.settings.services.begin(serviceDialogOf(&u.model.services, projectID, id), u.model.projectName(projectID))
	u.model.invalidate()
}
