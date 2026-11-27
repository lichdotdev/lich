package routes

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/rpc"
	"github.com/lichdotdev/lich/internal/subagent"
)

// Subagent exposes the vetoes /subagent wrote, for a chat to draw.
func Subagent(d *rpc.Handler, s *subagent.Service) {
	rpc.Handle(d, api.SubagentVetoed, s.Vetoed)
}

// SubagentCards exposes Settings › Subagents.
func SubagentCards(d *rpc.Handler, c *subagent.Cards) {
	rpc.Handle(d, api.SubagentCards, c.List)
	rpc.Handle(d, api.SubagentSetCards, c.SetCards)
	rpc.Handle(d, api.SubagentSetBring, c.SetBring)
	rpc.Handle(d, api.SubagentCardsOn, c.CardsOn)
}
