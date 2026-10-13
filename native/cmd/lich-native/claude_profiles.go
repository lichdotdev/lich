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

// profileListMaxAge is how old the list may be before a window focus reads it
// again. A read runs one CLI per profile, so focus cannot afford one every
// time, and a login in a terminal or an editor outside lich is the case it
// exists for.
const profileListMaxAge = time.Minute

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
// logged in where. The window loads it at start and again after anything that
// changes a login; Settings and the card menu read it. Each profile's plan
// reading lives in quota, the window's one store of them. Every method is safe
// from any goroutine.
type profileBook struct {
	client     *lichclient.Client
	invalidate func()
	quota      *planQuota

	mu       sync.Mutex
	list     []lichclient.ClaudeProfile
	loaded   bool
	loadedAt time.Time
	reading  bool
	// quoted are the readings the Continue in rows quote, watched.
	quoted map[string]func()
}

func newProfileBook(c *lichclient.Client, invalidate func(), quota *planQuota) *profileBook {
	return &profileBook{client: c, invalidate: invalidate, quota: quota, quoted: map[string]func(){}}
}

// reload asks the backend for the profiles. It runs one CLI per profile, so
// callers keep it off the frame.
func (b *profileBook) reload(ctx context.Context) error {
	list, err := b.client.ClaudeProfiles(ctx)
	if err != nil {
		return err
	}
	b.mu.Lock()
	gone := slices.DeleteFunc(slices.Clone(b.list), func(old lichclient.ClaudeProfile) bool {
		return slices.ContainsFunc(list, func(p lichclient.ClaudeProfile) bool { return p.Slug == old.Slug })
	})
	b.list, b.loaded, b.loadedAt = list, true, time.Now()
	b.followQuoted()
	b.mu.Unlock()
	for _, p := range gone {
		b.planChanged(p.Slug)
	}
	b.invalidate()
	return nil
}

// loginFinished takes the end of a login: who is logged in changed, and with
// it whose plan each profile has.
func (b *profileBook) loginFinished(ctx context.Context) {
	if err := b.reload(ctx); err != nil {
		log.Printf("claude profiles after a login: %v", err)
	}
	list, _ := b.snapshot()
	for _, p := range list {
		b.planChanged(p.Slug)
	}
}

// refreshIfStale reads the list again when it is older than
// profileListMaxAge and no read is already running. A book nobody has read yet
// is left to its first read.
func (b *profileBook) refreshIfStale(ctx context.Context) {
	b.mu.Lock()
	due := b.loaded && !b.reading && time.Since(b.loadedAt) > profileListMaxAge
	b.reading = b.reading || due
	b.mu.Unlock()
	if !due {
		return
	}
	go func() {
		err := b.reload(ctx)
		b.mu.Lock()
		b.reading = false
		b.mu.Unlock()
		if err != nil {
			log.Printf("claude profiles on focus: %v", err)
		}
	}()
}

// followQuoted keeps the plan readings the Continue in rows quote current: one
// per logged-in profile while there is more than one, the only case a reading
// is quoted outside Settings. The caller holds b.mu.
func (b *profileBook) followQuoted() {
	want := map[string]bool{}
	if logged := slices.DeleteFunc(slices.Clone(b.list), func(p lichclient.ClaudeProfile) bool { return !p.LoggedIn }); len(logged) > 1 {
		for _, p := range logged {
			want[profileQuotaKey(p.Slug)] = true
		}
	}
	for key, unwatch := range b.quoted {
		if !want[key] {
			unwatch()
			delete(b.quoted, key)
		}
	}
	for key := range want {
		if b.quoted[key] == nil {
			b.quoted[key] = b.quota.watch(key)
		}
	}
}

// planChanged is a login or a deletion that changes whose plan a profile has:
// its reading is dropped and read again.
func (b *profileBook) planChanged(slug string) {
	b.quota.invalidate(profileQuotaKey(slug))
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
