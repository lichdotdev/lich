package routes

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/rpc"
	"github.com/lichdotdev/lich/internal/terminal"
)

// Terminal exposes the terminal service as api declares it. A close on behalf
// of another session and the mod's command queue stay out: they skip the
// checks spawn.Close and spawn.Control make.
func Terminal(d *rpc.Handler, s *terminal.Service) {
	rpc.Handle(d, api.TerminalStart, s.Start)
	rpc.Handle(d, api.TerminalClose, s.Close)
	rpc.Handle(d, api.TerminalWrite, s.Write)
	rpc.Handle(d, api.TerminalResize, s.Resize)
	rpc.Handle(d, api.TerminalSetVisible, s.SetVisible)
	rpc.Handle(d, api.TerminalReplay, s.Replay)
	rpc.Handle(d, api.TerminalLive, s.Live)
	rpc.Handle(d, api.TerminalLiveCount, s.LiveCount)
	rpc.Handle(d, api.TerminalReady, s.Ready)
	rpc.Handle(d, api.TerminalHandsOn, s.HandsOn)
	rpc.Handle(d, api.TerminalLastTurnSaid, s.LastTurnSaid)
	rpc.Handle(d, api.TerminalTurnStates, s.TurnStates)
	rpc.Handle(d, api.TerminalContinueIn, s.ContinueIn)
	rpc.Handle(d, api.TerminalSearchTranscripts, s.SearchTranscripts)
	rpc.Handle(d, api.TerminalWorkdirMissing, s.WorkdirMissing)
	rpc.Handle(d, api.TerminalResumeAvailable, s.ResumeAvailable)
}
