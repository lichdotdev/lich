package routes

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/rpc"
	"github.com/lichdotdev/lich/internal/store"
)

// Store exposes the store service as api declares it.
func Store(d *rpc.Handler, s *store.Service) {
	storeProjects(d, s)
	storeSessions(d, s)
	storeSessionMarks(d, s)
	rpc.Handle(d, api.StoreCostTotals, s.CostTotals)
	rpc.Handle(d, api.StoreGetSetting, s.GetSetting)
	rpc.Handle(d, api.StoreSetSetting, s.SetSetting)
}

func storeProjects(d *rpc.Handler, s *store.Service) {
	rpc.Handle(d, api.StoreLoadState, s.LoadState)
	rpc.Handle(d, api.StoreAddProject, s.AddProject)
	rpc.Handle(d, api.StoreCloseProject, s.CloseProject)
	rpc.Handle(d, api.StoreReorderProjects, s.ReorderProjects)
	rpc.Handle(d, api.StoreRecentProjects, s.RecentProjects)
	rpc.Handle(d, api.StoreClosedProjectCount, s.ClosedProjectCount)
	rpc.Handle(d, api.StoreRenameFolder, s.RenameFolder)
}

func storeSessions(d *rpc.Handler, s *store.Service) {
	rpc.Handle(d, api.StoreAddSession, s.AddSession)
	rpc.Handle(d, api.StoreAddSessionInFolder, s.AddSessionInFolder)
	rpc.Handle(d, api.StoreAddSessionFrom, s.AddSessionFrom)
	rpc.Handle(d, api.StoreSetActiveSession, s.SetActiveSession)
	rpc.Handle(d, api.StoreCloseSession, s.CloseSession)
	rpc.Handle(d, api.StoreReopenSession, s.ReopenSession)
	rpc.Handle(d, api.StoreReopenWorktreeSession, s.ReopenWorktreeSession)
	rpc.Handle(d, api.StorePurgeWorktreeSessions, s.PurgeWorktreeSessions)
	rpc.Handle(d, api.StoreClosedSessions, s.ClosedSessions)
	rpc.Handle(d, api.StoreExternalSessions, s.ExternalSessions)
	rpc.Handle(d, api.StoreAdoptExternalSession, s.AdoptExternalSession)
	rpc.Handle(d, api.StoreForgetSession, s.ForgetSession)
	rpc.Handle(d, api.StoreMarkForked, s.MarkForked)
	rpc.Handle(d, api.StoreReorderSessions, s.ReorderSessions)
}

func storeSessionMarks(d *rpc.Handler, s *store.Service) {
	rpc.Handle(d, api.StoreRenameSession, s.RenameSession)
	rpc.Handle(d, api.StoreSetSessionColor, s.SetSessionColor)
	rpc.Handle(d, api.StoreSetSessionFolder, s.SetSessionFolder)
	rpc.Handle(d, api.StoreSetSessionPinned, s.SetSessionPinned)
	rpc.Handle(d, api.StoreSetSessionUnread, s.SetSessionUnread)
}
