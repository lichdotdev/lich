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
