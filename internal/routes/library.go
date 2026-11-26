package routes

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/library"
	"github.com/lichdotdev/lich/internal/rpc"
)

// Library exposes lich's library of skills and MCP servers as api declares it.
func Library(d *rpc.Handler, s *library.Service) {
	rpc.Handle(d, api.LibraryList, s.List)
	rpc.Handle(d, api.LibraryMoveIn, s.MoveIn)
	rpc.Handle(d, api.LibrarySet, s.Set)
	rpc.Handle(d, api.LibrarySeen, s.Seen)
}
