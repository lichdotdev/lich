package routes

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/agentplugin"
	"github.com/lichdotdev/lich/internal/appupdate"
	"github.com/lichdotdev/lich/internal/drop"
	"github.com/lichdotdev/lich/internal/patchnotes"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/rpc"
)

// Drop exposes the drop service as api declares it. A dropped file's bytes
// arrive on /drop, outside the RPC's JSON envelope.
func Drop(d *rpc.Handler, s *drop.Service) {
	rpc.Handle(d, api.DropAttach, s.Attach)
}

// PatchNotes exposes the patch notes as api declares them.
func PatchNotes(d *rpc.Handler, s *patchnotes.Service) {
	rpc.Handle(d, api.PatchNotesCurrent, s.Current)
}

// AppUpdate exposes the app update service as api declares it.
func AppUpdate(d *rpc.Handler, s *appupdate.Service) {
	rpc.Handle(d, api.AppUpdateGetStatus, s.Status)
	rpc.Handle(d, api.AppUpdateApply, s.Apply)
}

// AgentPlugin exposes the lich plugin's install state as api declares it.
func AgentPlugin(d *rpc.Handler, s *agentplugin.Service) {
	rpc.Handle(d, api.AgentPluginStatus, s.Status)
	rpc.Handle(d, api.AgentPluginInstall, s.Install)
	rpc.Handle(d, api.AgentPluginUpdate, s.Update)
}

// Providers exposes provider detection as api declares it.
func Providers(d *rpc.Handler, s *providers.Service) {
	rpc.Handle(d, api.ProvidersDetect, s.Detect)
	rpc.Handle(d, api.ProvidersOnPath, s.OnPath)
	rpc.Handle(d, api.ProvidersRefreshPath, s.RefreshPath)
	rpc.Handle(d, api.ProvidersVerify, s.Verify)
	rpc.Handle(d, api.ProvidersVersion, s.Version)
}
