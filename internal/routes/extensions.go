package routes

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/extensions"
	"github.com/lichdotdev/lich/internal/rpc"
)

// Extensions exposes the extensions as api declares them.
func Extensions(d *rpc.Handler, s *extensions.Service) {
	rpc.Handle(d, api.ExtensionsList, s.List)
	rpc.Handle(d, api.ExtensionsSend, s.Send)
	rpc.Handle(d, api.ExtensionsRestart, s.Restart)
}
