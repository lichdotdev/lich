// Package routes exposes the services api declares on an rpc.Handler, one
// function per service. It is its own package so the tests that post as a
// client of another release (internal/cli) answer through the same routes the
// app does.
//
// A service method nothing here routes cannot be reached over /rpc/, and the
// ones kept out are kept out on purpose: they exist for Go callers inside the
// process. Startup wiring (the Set* setters, store.Close) would rewire or end
// what the app runs on; the hooks' state stream and a worker's answer arrive
// over /hook and /mod/*, and called here would act for another session;
// terminal.CloseBy, the mod's command queue and terminal.SubmitPrompt skip the
// checks spawn.Close, spawn.Control and the relay make; terminal.SessionAccount
// returns a session's credentials; and a dropped file's bytes go to /drop,
// outside the JSON envelope.
package routes

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/rpc"
	"github.com/lichdotdev/lich/internal/spawn"
	"github.com/lichdotdev/lich/internal/themes"
)

// Themes exposes the themes service as api declares it.
func Themes(d *rpc.Handler, s *themes.Service) {
	rpc.Handle(d, api.ThemesList, s.List)
	rpc.Handle(d, api.ThemesListBroken, s.ListBroken)
	rpc.Handle(d, api.ThemesImport, s.Import)
	rpc.Handle(d, api.ThemesInstallFromGit, s.InstallFromGit)
	rpc.Handle(d, api.ThemesUpdateFromGit, s.UpdateFromGit)
	rpc.Handle(d, api.ThemesRemove, s.Remove)
	rpc.Handle(d, api.ThemesSaveTemplate, s.SaveTemplate)
	rpc.Handle(d, api.ThemesSelection, s.Selection)
	rpc.Handle(d, api.ThemesSelect, s.Select)
	rpc.Handle(d, api.ThemesSetFollowSystem, s.SetFollowSystem)
}

// Spawn exposes the spawn service as api declares it. CloseFinishedWorker
// stays out: it is the relay's, and closes a worker with none of Close's checks.
func Spawn(d *rpc.Handler, s *spawn.Service) {
	rpc.Handle(d, api.SpawnOpen, s.Open)
	rpc.Handle(d, api.SpawnOpenSubagent, s.OpenSubagent)
	rpc.Handle(d, api.SpawnClose, s.Close)
	rpc.Handle(d, api.SpawnCloseProject, s.CloseProject)
	rpc.Handle(d, api.SpawnControl, s.Control)
	rpc.Handle(d, api.SpawnAsk, s.Ask)
	rpc.Handle(d, api.SpawnWorktrees, s.Worktrees)
	rpc.Handle(d, api.SpawnFolders, s.Folders)
	rpc.Handle(d, api.SpawnFile, s.File)
	rpc.Handle(d, api.SpawnRenameFolder, s.RenameFolder)
	rpc.Handle(d, api.SpawnColorFolder, s.ColorFolder)
	rpc.Handle(d, api.SpawnRename, s.Rename)
}
