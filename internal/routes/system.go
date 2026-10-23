package routes

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/rpc"
	"github.com/lichdotdev/lich/internal/system"
	"github.com/lichdotdev/lich/internal/tray"
)

// System exposes the system service as api declares it.
func System(d *rpc.Handler, s *system.Service) {
	rpc.Handle(d, api.SystemShowWindow, s.ShowWindow)
	rpc.Handle(d, api.SystemQuit, s.Quit)
	rpc.Handle(d, api.SystemAttachNative, s.AttachNative)
	rpc.Handle(d, api.SystemDiagnostics, s.Diagnostics)
	rpc.Handle(d, api.SystemNotify, s.Notify)
	rpc.Handle(d, api.SystemOpenExternal, s.OpenExternal)
	rpc.Handle(d, api.SystemOpenFile, s.OpenFile)
	rpc.Handle(d, api.SystemOpenFolder, s.OpenFolder)
	rpc.Handle(d, api.SystemOpenFolderInEditor, s.OpenFolderInEditor)
	rpc.Handle(d, api.SystemRevealLog, s.RevealLog)
	rpc.Handle(d, api.SystemSandboxBackend, s.SandboxBackend)
	rpc.Handle(d, api.SystemSSHAgentKeys, s.SSHAgentKeys)
	rpc.Handle(d, api.SystemTakeUncleanExit, s.TakeUncleanExit)
}

// Tray exposes the tray as api declares it. tray.Labels keeps its own type for
// its methods; the conversion holds its fields to api's.
func Tray(d *rpc.Handler, t *tray.Tray) {
	rpc.Handle(d, api.TraySetLabels, t.SetLabels)
}
