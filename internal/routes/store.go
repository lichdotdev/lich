package routes

import (
	"context"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/rpc"
	"github.com/lichdotdev/lich/internal/store"
)

// Store exposes the store service as api declares it.
func Store(d *rpc.Handler, s *store.Service) {
	storeProjects(d, s)
	storeSessions(d, s)
	storeSessionMarks(d, s)
	rpc.Handle(d, api.StoreCostTotals, func(_ context.Context, o api.CostTotalsOptions) (api.CostReport, error) { return s.CostTotals(o) })
	rpc.Handle(d, api.StoreGetSetting, func(_ context.Context, o api.GetSettingOptions) (string, error) {
		return s.GetSetting(o.Key, o.ProjectID)
	})
	rpc.Handle(d, api.StoreSetSetting, func(_ context.Context, o api.SetSettingOptions) (api.None, error) {
		return api.None{}, s.SetSetting(o.Key, o.ProjectID, o.Value)
	})
}

func storeProjects(d *rpc.Handler, s *store.Service) {
	rpc.Handle(d, api.StoreLoadState, func(context.Context, api.None) ([]api.Project, error) { return s.LoadState() })
	rpc.Handle(d, api.StoreAddProject, func(_ context.Context, o api.AddProjectOptions) (api.None, error) {
		return api.None{}, s.AddProject(o.ID, o.Name, o.Path)
	})
	rpc.Handle(d, api.StoreCloseProject, func(_ context.Context, o api.ProjectIDOptions) (api.None, error) {
		return api.None{}, s.CloseProject(o.ID)
	})
	rpc.Handle(d, api.StoreReorderProjects, func(_ context.Context, o api.ReorderProjectsOptions) (api.None, error) {
		return api.None{}, s.ReorderProjects(o.IDs)
	})
	rpc.Handle(d, api.StoreRecentProjects, func(_ context.Context, o api.SearchTermOptions) ([]api.RecentProject, error) {
		return s.RecentProjects(o.Term)
	})
	rpc.Handle(d, api.StoreClosedProjectCount, func(_ context.Context, o api.SearchTermOptions) (int, error) {
		return s.ClosedProjectCount(o.Term)
	})
	rpc.Handle(d, api.StoreRenameFolder, func(_ context.Context, o api.RenameProjectFolderOptions) ([]string, error) {
		return s.RenameFolder(o.ProjectID, o.From, o.To)
	})
}

func storeSessions(d *rpc.Handler, s *store.Service) {
	rpc.Handle(d, api.StoreAddSession, func(_ context.Context, o api.AddSessionOptions) (api.None, error) {
		return api.None{}, s.AddSession(o.ProjectID, o.SessionID, o.Label, o.Kind, o.Path, o.NextSeq, o.Sandbox)
	})
	rpc.Handle(d, api.StoreAddSessionInFolder, func(_ context.Context, o api.AddSessionInFolderOptions) (api.None, error) {
		return api.None{}, s.AddSessionInFolder(o.ProjectID, o.SessionID, o.Label, o.Kind, o.Path, o.NextSeq, o.Sandbox, o.Folder)
	})
	rpc.Handle(d, api.StoreAddSessionFrom, func(_ context.Context, o api.AddSessionFromOptions) (api.None, error) {
		return api.None{}, s.AddSessionFrom(o.ProjectID, o.SessionID, o.Label, o.Kind, o.Path, o.NextSeq, o.OriginID, o.OriginLabel, o.Folder)
	})
	rpc.Handle(d, api.StoreSetActiveSession, func(_ context.Context, o api.ActiveSessionOptions) (api.None, error) {
		return api.None{}, s.SetActiveSession(o.ProjectID, o.SessionID)
	})
	rpc.Handle(d, api.StoreCloseSession, func(_ context.Context, o api.ParkSessionOptions) (api.None, error) {
		return api.None{}, s.CloseSession(o.ProjectID, o.SessionID, o.ActiveID)
	})
	rpc.Handle(d, api.StoreReopenSession, func(_ context.Context, o api.ReopenSessionOptions) (*api.Session, error) {
		return s.ReopenSession(o.SessionID, o.NewSessionID)
	})
	rpc.Handle(d, api.StoreReopenWorktreeSession, func(_ context.Context, o api.ReopenWorktreeSessionOptions) (*api.Session, error) {
		return s.ReopenWorktreeSession(o.ProjectID, o.Path, o.NewSessionID)
	})
	rpc.Handle(d, api.StorePurgeWorktreeSessions, func(_ context.Context, o api.WorktreeSessionsOptions) (api.None, error) {
		return api.None{}, s.PurgeWorktreeSessions(o.ProjectID, o.Path)
	})
	rpc.Handle(d, api.StoreClosedSessions, func(_ context.Context, o api.SearchTermOptions) (api.ClosedHistory, error) {
		return s.ClosedSessions(o.Term)
	})
	rpc.Handle(d, api.StoreExternalSessions, func(context.Context, api.None) ([]api.ExternalSession, error) { return s.ExternalSessions() })
	rpc.Handle(d, api.StoreAdoptExternalSession, func(_ context.Context, o api.AdoptExternalSessionOptions) (api.None, error) {
		return api.None{}, s.AdoptExternalSession(o.ProjectID, o.SessionID, o.Kind, o.Path, o.ProviderSessionID, o.Label)
	})
	rpc.Handle(d, api.StoreForgetSession, func(_ context.Context, o api.SessionIDOptions) (api.None, error) {
		return api.None{}, s.ForgetSession(o.ID)
	})
	rpc.Handle(d, api.StoreMarkForked, func(_ context.Context, o api.SessionIDOptions) (api.None, error) {
		return api.None{}, s.MarkForked(o.ID)
	})
	rpc.Handle(d, api.StoreReorderSessions, func(_ context.Context, o api.ReorderSessionsOptions) (api.None, error) {
		return api.None{}, s.ReorderSessions(o.ProjectID, o.IDs)
	})
}

func storeSessionMarks(d *rpc.Handler, s *store.Service) {
	rpc.Handle(d, api.StoreRenameSession, func(_ context.Context, o api.LabelSessionOptions) (api.None, error) {
		return api.None{}, s.RenameSession(o.SessionID, o.Label)
	})
	rpc.Handle(d, api.StoreSetSessionColor, func(_ context.Context, o api.SessionColorOptions) (api.None, error) {
		return api.None{}, s.SetSessionColor(o.SessionID, o.Color)
	})
	rpc.Handle(d, api.StoreSetSessionFolder, func(_ context.Context, o api.SessionFolderOptions) (api.None, error) {
		return api.None{}, s.SetSessionFolder(o.SessionID, o.Folder)
	})
	rpc.Handle(d, api.StoreSetSessionPinned, func(_ context.Context, o api.SessionPinnedOptions) (api.None, error) {
		return api.None{}, s.SetSessionPinned(o.SessionID, o.Pinned)
	})
	rpc.Handle(d, api.StoreSetSessionUnread, func(_ context.Context, o api.SessionUnreadOptions) (api.None, error) {
		return api.None{}, s.SetSessionUnread(o.SessionID, o.Unread)
	})
}
