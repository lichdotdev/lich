package main

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/omartelo/lich/native/lichclient"
)

// claudeProfileKey is store.claudeProfileKey: the profile new Claude Code
// sessions are born in. Globally "" and "personal" both mean Personal; in a
// project "" inherits and "personal" names Personal over a global profile.
const claudeProfileKey = "provider.claude.profile"

// claudeKind is the provider id of Claude Code.
const claudeKind = "claude"

// personalProfile is claudeprofile.Personal, the slug of the default profile.
const personalProfile = "personal"

// profilePlanRefresh is how long a profile's plan reading is kept before the
// next one is asked for: the backend serves it from a five-minute cache.
const profilePlanRefresh = 5 * time.Minute

// sessionWindowSeconds is the five-hour window of a Claude Code plan, the one
// a "Continue in" row quotes.
const sessionWindowSeconds = 5 * hour

// ponytail: English until the native window has i18n (claudeProfile.*).
const (
	profileUsageLimit   = "limit"
	profileNoPlugins    = "no plugins"
	profileOnePlugin    = "1 plugin"
	profileManyPlugins  = "%d plugins"
	profileNotLoggedIn  = "Not logged in"
	profileAccountSplit = " · "
)

// profileBook is the Claude Code profiles as the window knows them: who is
// logged in where, and each profile's plan reading. The window loads it at
// start and again after anything that changes a login; Settings and the card
// menu read it. Every method is safe from any goroutine.
type profileBook struct {
	client     *lichclient.Client
	invalidate func()

	mu       sync.Mutex
	list     []lichclient.ClaudeProfile
	loaded   bool
	plans    map[string]profileReading
	fetching map[string]bool
}

type profileReading struct {
	plan lichclient.QuotaPlan
	at   time.Time
}

func newProfileBook(c *lichclient.Client, invalidate func()) *profileBook {
	return &profileBook{client: c, invalidate: invalidate, plans: map[string]profileReading{}, fetching: map[string]bool{}}
}

// reload asks the backend for the profiles. It runs one CLI per profile, so
// callers keep it off the frame.
func (b *profileBook) reload(ctx context.Context) error {
	list, err := b.client.ClaudeProfiles(ctx)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.list, b.loaded = list, true
	for slug := range b.plans {
		if !slices.ContainsFunc(list, func(p lichclient.ClaudeProfile) bool { return p.Slug == slug }) {
			delete(b.plans, slug)
		}
	}
	b.mu.Unlock()
	b.invalidate()
	return nil
}

func (b *profileBook) snapshot() ([]lichclient.ClaudeProfile, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.list), b.loaded
}

// find is the profile a stored value names; "" is Personal.
func (b *profileBook) find(stored string) (lichclient.ClaudeProfile, bool) {
	slug := effectiveProfile(stored)
	b.mu.Lock()
	defer b.mu.Unlock()
	i := slices.IndexFunc(b.list, func(p lichclient.ClaudeProfile) bool { return p.Slug == slug })
	if i < 0 {
		return lichclient.ClaudeProfile{}, false
	}
	return b.list[i], true
}

// nameOf is what a stored value is called on screen: the profile's name, the
// slug of one the book does not know (a deleted profile still marks the
// sessions born in it).
func (b *profileBook) nameOf(stored string) string {
	if p, ok := b.find(stored); ok {
		return p.Name
	}
	if slug := effectiveProfile(stored); slug != personalProfile {
		return slug
	}
	return "Personal"
}

// loggedIn is every profile with a login, in the book's order.
func (b *profileBook) loggedIn() []lichclient.ClaudeProfile {
	list, _ := b.snapshot()
	return slices.DeleteFunc(list, func(p lichclient.ClaudeProfile) bool { return !p.LoggedIn })
}

// plan is the last reading of a profile's plan, false until the first lands.
// An older one is asked for again behind the answer.
func (b *profileBook) plan(ctx context.Context, slug string) (lichclient.QuotaPlan, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.plans[slug]
	if (!ok || time.Since(r.at) > profilePlanRefresh) && !b.fetching[slug] {
		b.fetching[slug] = true
		go b.fetchPlan(ctx, slug)
	}
	return r.plan, ok
}

func (b *profileBook) fetchPlan(ctx context.Context, slug string) {
	plan, err := b.client.ClaudeProfilePlan(ctx, slug)
	b.mu.Lock()
	b.fetching[slug] = false
	if err == nil {
		b.plans[slug] = profileReading{plan: plan, at: time.Now()}
	}
	b.mu.Unlock()
	if err != nil {
		log.Printf("claude profile plan %s: %v", slug, err)
		return
	}
	b.invalidate()
}

// forgetPlans drops every reading, so the next ask reads again: a login that
// just finished changes whose plan a profile has.
func (b *profileBook) forgetPlans() {
	b.mu.Lock()
	b.plans = map[string]profileReading{}
	b.mu.Unlock()
}

// effectiveProfile is the slug a stored value stands for: an unset value is
// Personal.
func effectiveProfile(stored string) string {
	if stored == "" {
		return personalProfile
	}
	return stored
}

// profileChoiceValue is what picking slug writes at scope: globally Personal is
// the unset value, in a project it is named, since unset there inherits.
func profileChoiceValue(slug, scope string) string {
	if slug == personalProfile && scope == "" {
		return ""
	}
	return slug
}

// profileAccountLine is a profile row's second line: the account, its plan and
// how many plugins it carries. "" for a profile nobody is logged in to.
func profileAccountLine(p lichclient.ClaudeProfile) string {
	if !p.LoggedIn {
		return ""
	}
	parts := []string{p.Email}
	if p.Plan != "" {
		parts = append(parts, strings.ToUpper(p.Plan[:1])+p.Plan[1:])
	}
	switch p.Plugins {
	case 0:
		parts = append(parts, profileNoPlugins)
	case 1:
		parts = append(parts, profileOnePlugin)
	default:
		parts = append(parts, fmt.Sprintf(profileManyPlugins, p.Plugins))
	}
	return strings.Join(parts, profileAccountSplit)
}

// profileInitial is the letter an avatar carries.
func profileInitial(name string) string {
	for _, r := range name {
		return strings.ToUpper(string(r))
	}
	return "?"
}

// profileUsage is the five-hour reading a Continue in row shows: its text, and
// whether the window is spent. "" for a plan that gave no reading.
func profileUsage(plan lichclient.QuotaPlan, ok bool) (text string, limited bool) {
	if !ok || plan.Status != "ok" {
		return "", false
	}
	i := slices.IndexFunc(plan.Windows, func(w lichclient.QuotaWindow) bool { return w.Seconds == sessionWindowSeconds })
	w, found := lichclient.QuotaWindow{}, i >= 0
	if found {
		w = plan.Windows[i]
	} else if w, found = hottestWindow(plan); !found {
		return "", false
	}
	if w.Percent >= 100 || w.LockedReason != "" {
		return profileUsageLimit, true
	}
	return fmt.Sprintf(providersPercentUsed, w.Percent), false
}

// setProfile records the profile a session now runs in, from a
// session-profile event.
func (m *model) setProfile(id, profile string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for pi := range m.projects {
		for si := range m.projects[pi].Sessions {
			if m.projects[pi].Sessions[si].ID == id {
				m.projects[pi].Sessions[si].Profile = profile
			}
		}
	}
}
