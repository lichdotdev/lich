package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/omartelo/lich/native/lichclient"
)

// English until the native window has i18n.

// planQuotaRefresh is how often a watched reading is re-read. The backend
// serves a five-minute cache, so this only decides how quickly a fresh reading
// reaches the screen, never how often a provider is asked.
const planQuotaRefresh = time.Minute

// profileQuotaPrefix starts the key of a Claude Code profile's reading. A
// session id never does.
const profileQuotaPrefix = "profile:"

// sessionLimitEvent is terminal limitEventName: a turn a usage limit ended.
const sessionLimitEvent = "session-limit"

// planQuota shares one poll loop per key across its watchers, ported from
// plan-quota-store.ts, and is the window's only reader of plan quota. The key
// is the session the reading is for, "" the machine's own login (which is
// Personal's), "profile:<slug>" another Claude Code profile's own login:
// a session can spend another account entirely.
// A reading outlives its watchers: coming back to a card draws the numbers it
// had while the fresh read is in flight.
type planQuota struct {
	fetch    func(sessionID string) ([]lichclient.QuotaPlan, error)
	after    func(time.Duration) <-chan time.Time
	onChange func()

	mu      sync.Mutex
	entries map[string]*quotaEntry
}

type quotaEntry struct {
	plans    []lichclient.QuotaPlan
	watchers int
	stop     chan struct{}
	// seq numbers the reads, so one started before the last watcher left
	// cannot publish over a later one.
	seq int
}

// profileQuotaKey is the key of a profile's reading: Personal is the machine's.
func profileQuotaKey(slug string) string {
	if effectiveProfile(slug) == personalProfile {
		return ""
	}
	return profileQuotaPrefix + slug
}

// quotaReader reads the plan a planQuota key stands for. A profile's reading
// is the one plan its own login has, so it comes back as a list of one.
func quotaReader(c *lichclient.Client) func(key string) ([]lichclient.QuotaPlan, error) {
	return func(key string) ([]lichclient.QuotaPlan, error) {
		ctx := context.Background()
		slug, isProfile := strings.CutPrefix(key, profileQuotaPrefix)
		if !isProfile {
			return c.QuotaPlans(ctx, key)
		}
		plan, err := c.ClaudeProfilePlan(ctx, slug)
		if err != nil {
			return nil, err
		}
		return []lichclient.QuotaPlan{plan}, nil
	}
}

// newPlanQuota polls with fetch every planQuotaRefresh; onChange runs, from
// the poll goroutine, whenever a key's reading changed.
func newPlanQuota(fetch func(string) ([]lichclient.QuotaPlan, error), onChange func()) *planQuota {
	return &planQuota{fetch: fetch, after: time.After, onChange: onChange, entries: map[string]*quotaEntry{}}
}

// plans is the last reading for key; nil until one lands.
func (q *planQuota) plans(key string) []lichclient.QuotaPlan {
	q.mu.Lock()
	defer q.mu.Unlock()
	if e := q.entries[key]; e != nil {
		return e.plans
	}
	return nil
}

// watch starts key's poll if nothing was watching it, and returns the call
// that stops watching.
func (q *planQuota) watch(key string) (unwatch func()) {
	q.mu.Lock()
	defer q.mu.Unlock()
	e := q.entries[key]
	if e == nil {
		e = &quotaEntry{}
		q.entries[key] = e
	}
	e.watchers++
	if e.watchers == 1 {
		e.stop = make(chan struct{})
		go q.poll(key, e, e.stop)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			q.mu.Lock()
			defer q.mu.Unlock()
			e.watchers--
			if e.watchers == 0 {
				close(e.stop)
			}
		})
	}
}

// reread reads key again now when something watches it: the session moved to
// another login or took a turn, and its next tick could be a minute away.
func (q *planQuota) reread(key string) {
	q.mu.Lock()
	e := q.entries[key]
	watched := e != nil && e.watchers > 0
	q.mu.Unlock()
	if watched {
		go q.refresh(key, e)
	}
}

// apply reads a session's quota again when its events say the spend moved: a
// turn that ended, a usage limit it hit, or a switch of login. It is the
// store listening to /events, so the code that made the change names no one.
func (q *planQuota) apply(ev lichclient.Event) {
	switch ev.Name {
	case sessionLimitEvent, lichclient.ClaudeProfileEvent, statusEventName:
	default:
		return
	}
	var e struct {
		ID    string `json:"id"`
		State string `json:"state"`
	}
	if json.Unmarshal(ev.Data, &e) != nil || e.ID == "" {
		return
	}
	if ev.Name == statusEventName && e.State != statusDone {
		return
	}
	q.reread(e.ID)
}

// invalidate drops key's reading and reads it again when something watches it:
// a login that just finished, or a profile that is gone, changes whose plan the
// key has, and the old numbers must not be drawn while the new ones load.
func (q *planQuota) invalidate(key string) {
	q.mu.Lock()
	e := q.entries[key]
	if e == nil {
		q.mu.Unlock()
		return
	}
	e.seq++
	e.plans = nil
	watched := e.watchers > 0
	q.mu.Unlock()
	if watched {
		go q.refresh(key, e)
	}
	q.onChange()
}

func (q *planQuota) poll(key string, e *quotaEntry, stop chan struct{}) {
	for {
		q.refresh(key, e)
		select {
		case <-stop:
			return
		case <-q.after(planQuotaRefresh):
		}
		// A tick and the stop can be ready together, and select picks either.
		select {
		case <-stop:
			return
		default:
		}
	}
}

func (q *planQuota) refresh(key string, e *quotaEntry) {
	q.mu.Lock()
	e.seq++
	id := e.seq
	q.mu.Unlock()
	next, err := q.fetch(key)
	if err != nil {
		// A gauge that blanks on one dropped request is worse than one a
		// minute old: the previous numbers stay.
		log.Printf("quota plans %q: %v", key, err)
		return
	}
	q.mu.Lock()
	changed := id == e.seq && next != nil && !sameQuota(e.plans, next)
	if changed {
		e.plans = next
	}
	q.mu.Unlock()
	if changed {
		q.onChange()
	}
}

// sameQuota keeps a reading nobody is spending from redrawing every minute.
// It compares what the web's unchanged() compares, plus the account: a session
// moved to another login can read the same numbers under another name.
func sameQuota(a, b []lichclient.QuotaPlan) bool {
	return slices.EqualFunc(a, b, func(p, o lichclient.QuotaPlan) bool {
		return p.Provider == o.Provider && p.Status == o.Status && p.Plan == o.Plan && p.Account == o.Account &&
			slices.EqualFunc(p.Windows, o.Windows, func(w, v lichclient.QuotaWindow) bool {
				return w.Label == v.Label && w.Percent == v.Percent && w.Seconds == v.Seconds && w.ResetsAt == v.ResetsAt
			})
	})
}

// planFor is usePlanQuotaFor: provider's reading, false for a provider lich
// reads no plan from and while the first reading is in flight.
func planFor(plans []lichclient.QuotaPlan, provider string) (lichclient.QuotaPlan, bool) {
	i := slices.IndexFunc(plans, func(p lichclient.QuotaPlan) bool { return p.Provider == provider })
	if provider == "" || i < 0 {
		return lichclient.QuotaPlan{}, false
	}
	return plans[i], true
}

// hottestWindow is the window a single readout shows: the provider's own
// active verdict when it names one, else the window closest to spent, the
// first on a tie.
func hottestWindow(plan lichclient.QuotaPlan) (lichclient.QuotaWindow, bool) {
	if i := slices.IndexFunc(plan.Windows, func(w lichclient.QuotaWindow) bool { return w.Active }); i >= 0 {
		return plan.Windows[i], true
	}
	if len(plan.Windows) == 0 {
		return lichclient.QuotaWindow{}, false
	}
	hottest := plan.Windows[0]
	for _, w := range plan.Windows[1:] {
		if w.Percent > hottest.Percent {
			hottest = w
		}
	}
	return hottest, true
}

// footerWindow is PlanQuota's pick: a locked window stays visible even when
// another is fuller. False unless the reading is ok.
func footerWindow(plan lichclient.QuotaPlan) (lichclient.QuotaWindow, bool) {
	if plan.Status != "ok" {
		return lichclient.QuotaWindow{}, false
	}
	if i := slices.IndexFunc(plan.Windows, func(w lichclient.QuotaWindow) bool { return w.LockedReason != "" }); i >= 0 {
		return plan.Windows[i], true
	}
	return hottestWindow(plan)
}

// planReadout is PlanQuota's strip text and tooltip title; the tooltip body
// is quotaGauges and accountLine.
func planReadout(plan lichclient.QuotaPlan) (footerReadout, bool) {
	w, ok := footerWindow(plan)
	if !ok {
		return footerReadout{}, false
	}
	r := footerReadout{Label: "Plan usage", Title: plan.Name, Tone: toneOf(float64(w.Percent))}
	if plan.Plan != "" {
		r.Title = plan.Name + " · " + plan.Plan
	}
	reading := fmt.Sprintf("%d%%", w.Percent)
	if w.LockedReason != "" {
		reading, r.Tone = "Locked", toneDestructive
	}
	r.Text = joinNonEmpty(" ", shortWindow(w.Seconds), reading)
	return r, true
}

// quotaGauge is one row of PlanQuota's tooltip (QuotaGauge stacked).
type quotaGauge struct {
	Label   string // "Session · 5h"
	Reading string // "42%" or "Locked"
	Left    string // time to reset, "" when unknown or past
	Percent int    // the bar's fill
	Locked  string // the provider's reason, verbatim
	Ahead   bool   // spending ahead of this window's pace
	Tone    usageTone
}

func quotaGauges(plan lichclient.QuotaPlan, now time.Time) []quotaGauge {
	rows := make([]quotaGauge, 0, len(plan.Windows))
	for _, w := range plan.Windows {
		g := quotaGauge{
			Label: joinNonEmpty(" · ", w.Label, formatWindow(w.Seconds)), Reading: fmt.Sprintf("%d%%", w.Percent),
			Left: timeLeft(w.ResetsAt, now), Percent: w.Percent, Locked: w.LockedReason, Ahead: w.Ahead,
			Tone: toneOf(float64(w.Percent)),
		}
		if w.LockedReason != "" {
			g.Reading = "Locked"
		}
		rows = append(rows, g)
	}
	return rows
}

// accountLine is the account a gauge was read against. A token login has a
// name lich may not ask for, which is worth a word rather than a gap.
func accountLine(plan lichclient.QuotaPlan) string {
	if plan.Account != "" {
		return plan.Account
	}
	if plan.NoAccount == "token-login" {
		return "Token login"
	}
	return ""
}

// shortWindow names a window's length at status-bar width: "5h", "wk", "30d".
func shortWindow(seconds int) string {
	if seconds >= day && roundDiv(seconds, day) == 7 {
		return "wk"
	}
	return formatWindow(seconds)
}

// formatWindow names a window's length in full: "5h", "7d", "30d".
func formatWindow(seconds int) string {
	switch {
	case seconds <= 0:
		return ""
	case seconds < day:
		return fmt.Sprintf("%dh", roundDiv(seconds, hour))
	}
	return fmt.Sprintf("%dd", roundDiv(seconds, day))
}

const (
	hour = 60 * 60
	day  = 24 * hour
)

// roundDiv is Math.round(n / d) for the non-negative n it is given.
func roundDiv(n, d int) int { return int(math.Round(float64(n) / float64(d))) }

// timeLeft is how long until a window starts over, in its two coarsest units:
// "6d 20h", "4h 12m", "41m". Empty when there is no reset time or it has
// passed: a countdown to the past is worse than none.
func timeLeft(resetsAt string, now time.Time) string {
	reset, err := time.Parse(time.RFC3339, resetsAt)
	if err != nil {
		return ""
	}
	seconds := int(math.Floor(reset.Sub(now).Seconds()))
	switch {
	case seconds <= 0:
		return ""
	case seconds >= day:
		return fmt.Sprintf("%dd %dh", seconds/day, seconds%day/hour)
	case seconds >= hour:
		return fmt.Sprintf("%dh %dm", seconds/hour, seconds%hour/60)
	}
	return fmt.Sprintf("%dm", max(1, seconds/60))
}

func joinNonEmpty(sep string, parts ...string) string {
	return strings.Join(slices.DeleteFunc(parts, func(s string) bool { return s == "" }), sep)
}
