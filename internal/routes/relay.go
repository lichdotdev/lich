package routes

import (
	"context"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/relay"
	"github.com/lichdotdev/lich/internal/rpc"
)

// Relay exposes the relay service as api declares it. The hooks' state
// stream and a worker's answer stay out: they arrive over /hook and
// /mod/answer, and called here would act for another session.
func Relay(d *rpc.Handler, s *relay.Service) {
	rpc.Handle(d, api.RelayPeers, func(_ context.Context, o api.PeersOptions) ([]api.Peer, error) { return s.Peers(o) })
	rpc.Handle(d, api.RelaySelf, func(_ context.Context, o api.SelfOptions) (api.Peer, error) { return s.Self(o) })
	rpc.Handle(d, api.RelaySend, s.Send)
	rpc.Handle(d, api.RelaySendPrivate, s.SendPrivate)
	rpc.Handle(d, api.RelaySendSubagent, s.SendSubagent)
	rpc.Handle(d, api.RelayWait, s.Wait)
	rpc.Handle(d, api.RelayCollect, s.Collect)
	rpc.Handle(d, api.RelayCollectNow, func(_ context.Context, o api.CollectNowOptions) (api.CollectedResults, error) {
		return s.CollectNow(o)
	})
	rpc.Handle(d, api.RelayReply, func(_ context.Context, o api.ReplyOptions) (api.None, error) { return api.None{}, s.Reply(o) })
	rpc.Handle(d, api.RelayInsert, func(_ context.Context, o api.InsertTextOptions) (api.InsertedText, error) { return s.Insert(o) })
	rpc.Handle(d, api.RelayFocus, func(_ context.Context, o api.FocusSessionOptions) (api.FocusedSession, error) { return s.Focus(o) })
	rpc.Handle(d, api.RelayAnnounceMerge, func(_ context.Context, o api.AnnounceMergeOptions) (api.None, error) {
		return api.None{}, s.AnnounceMerge(o.Checkout, o.Number, o.Title, o.Branch, o.Base)
	})
	rpc.Handle(d, api.RelaySubmit, func(_ context.Context, o api.SubmitOptions) (api.None, error) {
		return api.None{}, s.Submit(o.SessionID, o.Message)
	})
}
