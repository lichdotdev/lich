package api

var (
	QuotaPlans            = define[QuotaPlansOptions, []QuotaPlan]("quota.Plans")
	QuotaProfilePlan      = define[ProfilePlanOptions, QuotaPlan]("quota.ProfilePlan")
	QuotaCodexProfilePlan = define[ProfilePlanOptions, QuotaPlan]("quota.CodexProfilePlan")
)

// QuotaPlansOptions names the session whose account's plans are read; ""
// reads the machine's own login.
type QuotaPlansOptions struct {
	SessionID string `json:"sessionId"`
}

// ProfilePlanOptions names the profile whose plan is read, "personal" for the
// machine's own.
type ProfilePlanOptions struct {
	Profile string `json:"profile"`
}

// QuotaPlan is one provider's quota reading. Provider is a providers.Registry id, so
// the window resolves the icon and the login command it already knows.
//
// Account names the login the windows were read against — the whole point of
// the custom-binary path being that a session can spend an account lich's own
// environment does not name. It is empty whenever the provider will not say
// which, which is a reading without a name and never an error: a gauge that
// cannot name its account is exactly what lich drew before this field existed.
//
// NoAccount carries why it is empty, for the one absence a reader can act on.
// Left empty, an unnamed account is drawn the same whether the provider names
// nobody or the session's own login is one lich may not ask about, and nothing
// on the gauge told the two apart.
type QuotaPlan struct {
	Provider  string        `json:"provider"`
	Name      string        `json:"name"`
	Plan      string        `json:"plan,omitempty"`
	Account   string        `json:"account,omitempty"`
	NoAccount string        `json:"noAccount,omitempty"`
	Windows   []QuotaWindow `json:"windows,omitempty"`
	Status    string        `json:"status"`
}

// QuotaWindow is one metered window of a plan: how much of it is spent, how long it
// runs, and when it starts over.
//
// Label is the window's name as the provider frames it ("Session", "Weekly"), or
// the display name of the model a scoped window belongs to. Seconds is its
// length, so the window can print the metered window it is looking at; 0 when the
// provider does not report one. ResetsAt is RFC 3339, empty when unreported.
// Active is the provider's own verdict on which window is the binding one, when
// it reports one at all. LockedReason is non-empty only when the provider says
// this window cannot be spent past regardless of its percentage. Ahead is
// lich's own reading of the rate rather than the provider's — see pace.go.
type QuotaWindow struct {
	Label        string `json:"label"`
	Seconds      int    `json:"seconds"`
	Percent      int    `json:"percent"`
	ResetsAt     string `json:"resetsAt,omitempty"`
	Active       bool   `json:"active,omitempty"`
	LockedReason string `json:"lockedReason,omitempty"`
	Ahead        bool   `json:"ahead,omitempty"`
}
