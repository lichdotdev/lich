package routes

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/rpc"
	"github.com/lichdotdev/lich/internal/services"
)

// Services exposes the project services as api declares them.
func Services(d *rpc.Handler, s *services.Service) {
	rpc.Handle(d, api.ServicesList, s.List)
	rpc.Handle(d, api.ServicesSave, s.Save)
	rpc.Handle(d, api.ServicesDelete, s.Delete)
	rpc.Handle(d, api.ServicesStart, s.Start)
	rpc.Handle(d, api.ServicesStop, s.Stop)
	rpc.Handle(d, api.ServicesForceStop, s.ForceStop)
	rpc.Handle(d, api.ServicesRestart, s.Restart)
	rpc.Handle(d, api.ServicesStartAll, s.StartAll)
	rpc.Handle(d, api.ServicesStopAll, s.StopAll)
	rpc.Handle(d, api.ServicesOutput, s.Output)
}
