package routes

import (
	"context"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/rpc"
	"github.com/lichdotdev/lich/internal/system"
	"github.com/lichdotdev/lich/internal/tray"
)

// System exposes the system service as api declares it.
func System(d *rpc.Handler, s *system.Service) {
	rpc.Handle(d, api.SystemShowWindow, func(_ context.Context, o api.ShowWindowOptions) (api.None, error) {
		return api.None{}, s.ShowWindow(o)
	})
	rpc.Handle(d, api.SystemQuit, func(_ context.Context, o api.QuitOptions) (api.None, error) { return api.None{}, s.Quit(o) })
	rpc.Handle(d, api.SystemAttachNative, func(_ context.Context, o api.AttachNativeOptions) (api.None, error) {
		s.AttachNative(o)
		return api.None{}, nil
	})
	rpc.Handle(d, api.SystemDiagnostics, func(context.Context, api.None) (api.Diagnostics, error) { return s.Diagnostics(), nil })
	rpc.Handle(d, api.SystemNotify, func(_ context.Context, o api.NotifyOptions) (api.None, error) {
		return api.None{}, s.Notify(o.Summary, o.Detail)
	})
	rpc.Handle(d, api.SystemOpenExternal, func(_ context.Context, o api.OpenExternalOptions) (api.None, error) {
		return api.None{}, s.OpenExternal(o.URL)
	})
	rpc.Handle(d, api.SystemOpenFile, func(_ context.Context, o api.OpenFileOptions) (api.None, error) {
		return api.None{}, s.OpenFile(o.Dir, o.Rel)
	})
	rpc.Handle(d, api.SystemOpenFolder, func(_ context.Context, o api.OpenFolderOptions) (api.None, error) {
		return api.None{}, s.OpenFolder(o.Dir)
	})
	rpc.Handle(d, api.SystemOpenFolderInEditor, func(_ context.Context, o api.OpenFolderOptions) (string, error) {
		return s.OpenFolderInEditor(o.Dir)
	})
	rpc.Handle(d, api.SystemRevealLog, func(context.Context, api.None) (api.None, error) { return api.None{}, s.RevealLog() })
	rpc.Handle(d, api.SystemSandboxBackend, func(context.Context, api.None) (string, error) { return s.SandboxBackend(), nil })
	rpc.Handle(d, api.SystemSSHAgentKeys, func(context.Context, api.None) ([]string, error) { return s.SSHAgentKeys(), nil })
	rpc.Handle(d, api.SystemTakeUncleanExit, func(context.Context, api.None) (bool, error) { return s.TakeUncleanExit(), nil })
}

// Tray exposes the tray as api declares it. tray.Labels keeps its own type for
// its methods; the conversion holds its fields to api's.
func Tray(d *rpc.Handler, t *tray.Tray) {
	rpc.Handle(d, api.TraySetLabels, func(_ context.Context, o api.TrayLabels) (api.None, error) {
		t.SetLabels(tray.Labels(o))
		return api.None{}, nil
	})
}
