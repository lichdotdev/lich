package api

var StoreCostTotals = define[CostTotalsOptions, CostReport]("store.CostTotals")

// CostTotalsOptions is one CostTotals call. It travels as a single JSON object
// rather than as positional arguments because a package manager replaces the
// lich binary under a running backend: a filter a newer `lich cost` adds is
// dropped by an older backend, and one an older `lich cost` does not send is no
// filter, where a positional list refuses both on its count.
type CostTotalsOptions struct {
	Project  string `json:"project"`
	Provider string `json:"provider"`
	Since    int64  `json:"since"`
}

// CostReport is CostTotals' answer: a row per project, and the same numbers
// summed across every row.
//
// The money is a **lower bound** whenever Unpriced is not zero. A session lich
// never priced holds no ledger row at all — its provider files its conversation
// where lich has no reader, a model in it has no price, or the readout was off
// while it ran — and nothing here can guess what it spent. Shipping that count
// beside the sum
// is the whole difference between "this is what it cost" and "this is what lich
// could add up", and a total that omitted it in silence would be the one
// failure mode that makes the number not worth reading.
//
// Why the row is missing is not in here on purpose: that is costMiss
// (internal/terminal/usage_cost.go), which is decided by the live scan and
// never persisted, so the ledger can only say that a session has no price —
// not which of the reasons it was.
//
// Priced and Reported are how many counted sessions each rung contributed, and
// they are what lets a reader weigh the total: the sum is the one place the
// difference between the two is invisible, since both arrive as dollars.
//
// Readout is whether the cost readout is on at all. Off, no transcript is ever
// summed, so every session is unpriced and the total can only be zero — which a
// caller has to be able to say instead of printing a $0.00 that reads as free.
type CostReport struct {
	Projects []CostTotal `json:"projects"`
	Sessions int         `json:"sessions"`
	Unpriced int         `json:"unpriced"`
	Priced   int         `json:"priced"`
	Reported int         `json:"reported"`
	Source   string      `json:"source"`
	CostUSD  float64     `json:"costUsd"`
	Readout  bool        `json:"readout"`
}

// CostTotal is what one project's sessions have cost. Sessions is every session
// in the unit; Unpriced is how many of them carry no price at all, so the money
// covers Sessions-Unpriced of them.
//
// Source is whose arithmetic that money is: providers.CostSourcePriced when
// lich derived every dollar of it from token counts, providers.CostSourceReported
// when the providers that spent it handed their own figures over, and
// CostSourceMixed when the row spans both. Empty is a row with nothing to
// attribute — every session in it unpriced, or its spend earned by a provider
// CLI run by hand inside a shell session, which lich never spawned a rung for.
type CostTotal struct {
	Project  string  `json:"project"`
	Sessions int     `json:"sessions"`
	Unpriced int     `json:"unpriced"`
	Source   string  `json:"source"`
	CostUSD  float64 `json:"costUsd"`
}
