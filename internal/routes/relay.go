package routes

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/relay"
	"github.com/lichdotdev/lich/internal/rpc"
)

// Relay exposes the relay service as api declares it. The hooks' state
// stream and a worker's answer stay out: they arrive over /hook and
// /mod/answer, and called here would act for another session.
func Relay(d *rpc.Handler, s *relay.Service) {
	rpc.Handle(d, api.RelayPeers, s.Peers)
	rpc.Handle(d, api.RelaySelf, s.Self)
	rpc.Handle(d, api.RelaySend, s.Send)
	rpc.Handle(d, api.RelaySendPrivate, s.SendPrivate)
	rpc.Handle(d, api.RelaySendSubagent, s.SendSubagent)
	rpc.Handle(d, api.RelayWait, s.Wait)
	rpc.Handle(d, api.RelayCollect, s.Collect)
	rpc.Handle(d, api.RelayCollectNow, s.CollectNow)
	rpc.Handle(d, api.RelayReply, s.Reply)
	rpc.Handle(d, api.RelayInsert, s.Insert)
	rpc.Handle(d, api.RelayFocus, s.Focus)
	rpc.Handle(d, api.RelayAnnounceMerge, s.AnnounceMerge)
	rpc.Handle(d, api.RelaySubmit, s.Submit)
}
