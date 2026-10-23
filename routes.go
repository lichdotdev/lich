package main

import (
	"context"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/rpc"
	"github.com/lichdotdev/lich/internal/themes"
)

// handleThemes exposes the themes service as api declares it.
func handleThemes(d *rpc.Handler, s *themes.Service) {
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
