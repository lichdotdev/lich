package routes

import (
	"context"

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
	rpc.Handle(d, api.DropAttach, func(_ context.Context, o api.AttachDropOptions) (api.DropAttachment, error) {
		return s.Attach(o.SessionID, o.Root, o.Confined)
	})
}

// PatchNotes exposes the patch notes as api declares them.
func PatchNotes(d *rpc.Handler, s *patchnotes.Service) {
	rpc.Handle(d, api.PatchNotesCurrent, func(_ context.Context, o api.PatchNotesOptions) (api.PatchNotes, error) {
		return s.Current(o.Locale)
	})
}

// AppUpdate exposes the app update service as api declares it.
func AppUpdate(d *rpc.Handler, s *appupdate.Service) {
	rpc.Handle(d, api.AppUpdateGetStatus, func(context.Context, api.None) (api.AppUpdateStatus, error) { return s.Status(), nil })
	rpc.Handle(d, api.AppUpdateApply, func(context.Context, api.None) (api.None, error) { return api.None{}, s.Apply() })
}

// AgentPlugin exposes the lich plugin's install state as api declares it.
func AgentPlugin(d *rpc.Handler, s *agentplugin.Service) {
	rpc.Handle(d, api.AgentPluginStatus, func(context.Context, api.None) ([]api.PluginStatus, error) { return s.Status(), nil })
	rpc.Handle(d, api.AgentPluginInstall, func(_ context.Context, o api.PluginProviderOptions) (api.None, error) {
		return api.None{}, s.Install(o.Provider)
	})
	rpc.Handle(d, api.AgentPluginUpdate, func(_ context.Context, o api.PluginProviderOptions) (api.None, error) {
		return api.None{}, s.Update(o.Provider)
	})
}

// Providers exposes provider detection as api declares it.
func Providers(d *rpc.Handler, s *providers.Service) {
	rpc.Handle(d, api.ProvidersDetect, func(context.Context, api.None) ([]api.DetectedProvider, error) { return s.Detect() })
	rpc.Handle(d, api.ProvidersOnPath, func(ctx context.Context, o api.ProviderIDOptions) ([]api.ProviderBinary, error) {
		return s.OnPath(ctx, o.ID)
	})
	rpc.Handle(d, api.ProvidersRefreshPath, func(context.Context, api.None) (api.None, error) { return api.None{}, s.RefreshPath() })
	rpc.Handle(d, api.ProvidersVerify, func(_ context.Context, o api.BinaryOptions) (api.BinaryCheck, error) { return s.Verify(o.Bin), nil })
	rpc.Handle(d, api.ProvidersVersion, func(ctx context.Context, o api.BinaryOptions) (string, error) { return s.Version(ctx, o.Bin) })
}
