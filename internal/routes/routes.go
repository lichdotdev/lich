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
	"context"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/rpc"
	"github.com/lichdotdev/lich/internal/spawn"
	"github.com/lichdotdev/lich/internal/themes"
)

// Themes exposes the themes service as api declares it.
func Themes(d *rpc.Handler, s *themes.Service) {
	rpc.Handle(d, api.ThemesList, func(context.Context, api.None) ([]api.Theme, error) { return s.List() })
	rpc.Handle(d, api.ThemesListBroken, func(context.Context, api.None) ([]api.BrokenTheme, error) { return s.ListBroken() })
	rpc.Handle(d, api.ThemesImport, func(_ context.Context, o api.ImportThemeOptions) (api.ThemeImportResult, error) {
		return s.Import(o.Path, o.Overwrite)
	})
	rpc.Handle(d, api.ThemesInstallFromGit, func(_ context.Context, o api.InstallThemesOptions) (api.ThemeGitResult, error) {
		return s.InstallFromGit(o.URL, o.Overwrite)
	})
	rpc.Handle(d, api.ThemesUpdateFromGit, func(_ context.Context, o api.UpdateThemeOptions) (api.ThemeGitResult, error) {
		return s.UpdateFromGit(o.ID)
	})
	rpc.Handle(d, api.ThemesRemove, func(_ context.Context, o api.RemoveThemeOptions) (api.None, error) {
		return api.None{}, s.Remove(o.ID)
	})
	rpc.Handle(d, api.ThemesSaveTemplate, func(_ context.Context, o api.SaveThemeTemplateOptions) (api.None, error) {
		return api.None{}, s.SaveTemplate(o.Path)
	})
	rpc.Handle(d, api.ThemesSelection, func(context.Context, api.None) (api.ThemeSelection, error) { return s.Selection() })
	rpc.Handle(d, api.ThemesSelect, func(_ context.Context, o api.SelectThemeOptions) (api.None, error) {
		return api.None{}, s.Select(o)
	})
	rpc.Handle(d, api.ThemesSetFollowSystem, func(_ context.Context, o api.FollowSystemOptions) (api.None, error) {
		return api.None{}, s.SetFollowSystem(o.On)
	})
}

// Spawn exposes the spawn service as api declares it. CloseFinishedWorker
// stays out: it is the relay's, and closes a worker with none of Close's checks.
func Spawn(d *rpc.Handler, s *spawn.Service) {
	rpc.Handle(d, api.SpawnOpen, func(_ context.Context, o api.OpenSessionOptions) (api.OpenedSession, error) { return s.Open(o) })
	rpc.Handle(d, api.SpawnOpenSubagent, func(_ context.Context, o api.OpenSubagentOptions) (api.OpenedSession, error) {
		return s.OpenSubagent(o)
	})
	rpc.Handle(d, api.SpawnClose, func(_ context.Context, o api.CloseSessionOptions) (api.ClosedSession, error) { return s.Close(o) })
	rpc.Handle(d, api.SpawnControl, s.Control)
	rpc.Handle(d, api.SpawnAsk, s.Ask)
	rpc.Handle(d, api.SpawnWorktrees, func(_ context.Context, o api.WorktreesOptions) ([]api.WorktreeCheckout, error) {
		return s.Worktrees(o)
	})
	rpc.Handle(d, api.SpawnFolders, func(_ context.Context, o api.FoldersOptions) ([]api.SessionFolder, error) { return s.Folders(o) })
	rpc.Handle(d, api.SpawnFile, func(_ context.Context, o api.FileSessionOptions) (api.FiledSession, error) { return s.File(o) })
	rpc.Handle(d, api.SpawnRenameFolder, func(_ context.Context, o api.RenameFolderOptions) (api.RenamedFolder, error) {
		return s.RenameFolder(o)
	})
	rpc.Handle(d, api.SpawnColorFolder, func(_ context.Context, o api.ColorFolderOptions) (api.ColoredFolder, error) {
		return s.ColorFolder(o)
	})
	rpc.Handle(d, api.SpawnRename, func(_ context.Context, o api.RenameSessionOptions) (api.RenamedSession, error) { return s.Rename(o) })
}
