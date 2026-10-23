package routes

import (
	"context"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/rpc"
	"github.com/lichdotdev/lich/internal/terminal"
)

// Terminal exposes the terminal service as api declares it. A close on behalf
// of another session and the mod's command queue stay out: they skip the
// checks spawn.Close and spawn.Control make.
func Terminal(d *rpc.Handler, s *terminal.Service) {
	rpc.Handle(d, api.TerminalStart, func(_ context.Context, o api.StartOptions) (api.None, error) {
		return api.None{}, s.Start(o.ID, o.ProjectID, o.Cwd, o.Kind, o.Resume, o.Fork, o.Cols, o.Rows)
	})
	rpc.Handle(d, api.TerminalClose, func(_ context.Context, o api.SessionIDOptions) (api.None, error) { return api.None{}, s.Close(o.ID) })
	rpc.Handle(d, api.TerminalWrite, func(_ context.Context, o api.WriteOptions) (api.None, error) {
		return api.None{}, s.Write(o.ID, o.Data)
	})
	rpc.Handle(d, api.TerminalResize, func(_ context.Context, o api.ResizeOptions) (api.None, error) {
		return api.None{}, s.Resize(o.ID, o.Cols, o.Rows)
	})
	rpc.Handle(d, api.TerminalSetVisible, func(_ context.Context, o api.SetVisibleOptions) (api.None, error) {
		return api.None{}, s.SetVisible(o.ID, o.Visible)
	})
	rpc.Handle(d, api.TerminalReplay, func(_ context.Context, o api.SessionIDOptions) (string, error) { return s.Replay(o.ID) })
	rpc.Handle(d, api.TerminalLive, func(_ context.Context, o api.SessionIDOptions) (bool, error) { return s.Live(o.ID), nil })
	rpc.Handle(d, api.TerminalLiveCount, func(context.Context, api.None) (int, error) { return s.LiveCount(), nil })
	rpc.Handle(d, api.TerminalReady, func(_ context.Context, o api.SessionIDOptions) (bool, error) { return s.Ready(o.ID), nil })
	rpc.Handle(d, api.TerminalHandsOn, func(_ context.Context, o api.SessionIDOptions) (int64, error) { return s.HandsOn(o.ID) })
	rpc.Handle(d, api.TerminalLastTurnSaid, func(_ context.Context, o api.SessionIDOptions) (api.LastSaid, error) {
		return s.LastTurnSaid(o.ID)
	})
	rpc.Handle(d, api.TerminalTurnStates, func(context.Context, api.None) ([]api.TurnState, error) { return s.TurnStates(), nil })
	rpc.Handle(d, api.TerminalContinueIn, func(_ context.Context, o api.ContinueInOptions) (api.None, error) {
		return api.None{}, s.ContinueIn(o.ID, o.Profile)
	})
	rpc.Handle(d, api.TerminalSearchTranscripts, func(_ context.Context, o api.SearchTranscriptsOptions) ([]api.TranscriptMatch, error) {
		return s.SearchTranscripts(o.IDs, o.Query), nil
	})
	rpc.Handle(d, api.TerminalWorkdirMissing, func(_ context.Context, o api.WorkdirOptions) (bool, error) {
		return s.WorkdirMissing(o.Cwd), nil
	})
	rpc.Handle(d, api.TerminalResumeAvailable, func(_ context.Context, o api.ResumeAvailableOptions) (bool, error) {
		return s.ResumeAvailable(o.Kind, o.ProviderSessionID, o.Cwd), nil
	})
}
