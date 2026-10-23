package routes

import (
	"context"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/rpc"
	"github.com/lichdotdev/lich/internal/store"
)

// Store exposes the store service as api declares it.
func Store(d *rpc.Handler, s *store.Service) {
	rpc.Handle(d, api.StoreCostTotals, func(_ context.Context, o api.CostTotalsOptions) (api.CostReport, error) { return s.CostTotals(o) })
}
