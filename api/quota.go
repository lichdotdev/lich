package api

var (
	QuotaPlans            = define[QuotaPlansOptions, []QuotaPlan]("quota.Plans")
	QuotaProfilePlan      = define[ProfilePlanOptions, QuotaPlan]("quota.ProfilePlan")
	QuotaCodexProfilePlan = define[ProfilePlanOptions, QuotaPlan]("quota.CodexProfilePlan")
	QuotaUseReset         = define[UseResetOptions, UseResetResult]("quota.UseReset")
)

// UseResetOptions names the Claude Code profile ("personal" for the machine's
// own) and the grant of limit resets (QuotaReset.ID) to spend one of.
type UseResetOptions struct {
	Profile string `json:"profile"`
	GrantID string `json:"grantId"`
}

// UseResetResult is how the provider answered, one of the UseReset* statuses.
// CooldownUntil is RFC 3339 and only set with UseResetCooldown, when the
// provider said when to try again.
type UseResetResult struct {
	Status        string `json:"status"`
	CooldownUntil string `json:"cooldownUntil,omitempty"`
}

// What a spend can come back as. Reset and AlreadyUsed both mean the limit is
// cleared (the second is the same request answered twice); the rest cleared
// nothing.
const (
	UseResetReset       = "reset"
	UseResetAlreadyUsed = "already_used"
	UseResetNotLimited  = "not_limited"
	UseResetIneligible  = "ineligible"
	UseResetCooldown    = "cooldown"
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
	// Extra is Claude's pay-as-you-go spend past the plan, present only when the
	// account has it switched on. ResetCredits is how many windows Codex lets
	// the account reset on demand; 0 is none or not reported.
	Extra        *QuotaExtra `json:"extra,omitempty"`
	ResetCredits int         `json:"resetCredits,omitempty"`
	// Resets are the limit resets Claude Code lets the account spend: one entry
	// per grant still good, empty when the account is not offered any.
	Resets []QuotaReset `json:"resets,omitempty"`
	Status string       `json:"status"`
}

// QuotaReset is one grant of limit resets. ID is the provider's own, what a
// spend names. Clears says which limits a reset of it clears, as the
// QuotaClears* names. Kind names a grant lich names itself
// (QuotaResetFull), which the window translates; Label is the provider's own
// name for the rest, shown verbatim (a model a grant is scoped to, or the
// grant's own text); empty when Kind is set. Left is how many resets remain,
// EndsAt when the grant lapses, RFC 3339. Held is a grant that cannot be spent
// now: paused, or not usable at the moment.
type QuotaReset struct {
	ID     string   `json:"id"`
	Clears []string `json:"clears,omitempty"`
	Kind   string   `json:"kind,omitempty"`
	Label  string   `json:"label,omitempty"`
	Left   int      `json:"left"`
	EndsAt string   `json:"endsAt"`
	Held   bool     `json:"held,omitempty"`
}

// QuotaExtra is the spend past a plan's windows, in the currency's minor units
// (cents for USD). Limit is nil for an account with no monthly cap. Decimals is
// how many of those digits sit after the point, 2 unless the provider says so.
type QuotaExtra struct {
	Used     int64  `json:"used"`
	Limit    *int64 `json:"limit,omitempty"`
	Currency string `json:"currency,omitempty"`
	Decimals int    `json:"decimals"`
}

// QuotaWindow is one metered window of a plan: how much of it is spent, how long it
// runs, and when it starts over.
//
// Kind names a window lich names itself, one of the QuotaWindow* kinds, which the
// window translates. Label is the provider's own name for the rest (the model a
// scoped weekly cap belongs to, an Antigravity group), shown verbatim; empty when
// Kind is set. Seconds is its length, so the window can print the metered window
// it is looking at; 0 when the provider does not report one. ResetsAt is RFC 3339, empty when unreported.
// Active is the provider's own verdict on which window is the binding one, when
// it reports one at all. LockedReason is non-empty only when the provider says
// this window cannot be spent past regardless of its percentage. RunsOutAt is
// lich's own reading of the rate rather than the provider's, RFC 3339 and set
// only when spending at the pace so far empties the window before ResetsAt (see
// pace.go).
type QuotaWindow struct {
	Kind         string `json:"kind,omitempty"`
	Label        string `json:"label,omitempty"`
	Seconds      int    `json:"seconds"`
	Percent      int    `json:"percent"`
	ResetsAt     string `json:"resetsAt,omitempty"`
	Active       bool   `json:"active,omitempty"`
	LockedReason string `json:"lockedReason,omitempty"`
	RunsOutAt    string `json:"runsOutAt,omitempty"`
}

const (
	QuotaWindowSession      = "session"
	QuotaWindowWeekly       = "weekly"
	QuotaWindowMonthly      = "monthly"
	QuotaWindowUsage        = "usage"
	QuotaWindowCursorModels = "cursorModels"
	QuotaWindowOtherModels  = "otherModels"
	QuotaWindowOnDemand     = "onDemand"
)

// QuotaResetFull is a grant that clears the weekly window too, not only a
// scoped one.
const QuotaResetFull = "full"

// The limits a reset can clear: the five-hour window, the weekly one and the
// weekly cap of a model.
const (
	QuotaClearsSession = "session"
	QuotaClearsWeekly  = "weekly"
	QuotaClearsModel   = "model"
)
