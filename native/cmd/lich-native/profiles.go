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
// codexProfileKey and ompProfileKey are the same setting for Codex and
// oh-my-pi (store.codexProfileKey, store.ompProfileKey).
const (
	claudeProfileKey = "provider.claude.profile"
	codexProfileKey  = "provider.codex.profile"
	ompProfileKey    = "provider.omp.profile"
)

// claudeKind, codexKind and ompKind are the provider ids of the providers that
// have profiles.
const (
	claudeKind = "claude"
	codexKind  = "codex"
	ompKind    = "omp"
)

// personalProfile is claudeprofile.Personal, the slug of the default profile,
// which codexprofile and ompprofile reserve too.
const personalProfile = "personal"

// profileListMaxAge is how old the list may be before a window focus reads it
// again. A read runs one CLI per profile, so focus cannot afford one every
// time, and a login in a terminal or an editor outside lich is the case it
// exists for.
const profileListMaxAge = time.Minute

// sessionWindowSeconds is the five-hour window of a Claude Code or Codex plan,
// the one a "Continue in" row quotes.
const sessionWindowSeconds = 5 * hour

// ponytail: English until the native window has i18n (claudeProfile.*).
const (
	profileUsageLimit   = "limit"
	profileNoPlugins    = "no plugins"
	profileOnePlugin    = "1 plugin"
	profileManyPlugins  = "%d plugins"
	profileNotLoggedIn  = "Not logged in"
	profileLoggedIn     = "Logged in"
	profileAccountSplit = " · "
)

// profileProvider is what differs between the providers that have profiles: the
// profile service, the setting that names the profile new sessions are born in,
// what a profile can carry, and the words Settings uses about it.
type profileProvider struct {
	kind  string
	key   string
	label string
	api   func(c *lichclient.Client) lichclient.ProfileAPI
	// quotaPrefix starts the planQuota key of one profile's reading; "" for a
	// provider with no plan to read.
	quotaPrefix string
	// plugins is whether a profile has a plugin set of its own that plugins
	// can be brought into (Claude Code, Codex), and loginCode whether its login
	// takes the code the sign-in page shows (Claude Code).
	plugins, loginCode bool
	// terminalLogin is whether the login is only ever run in a terminal card
	// (oh-my-pi's is an interactive picker of providers), so the dialog has no
	// login of its own to run and to wait on.
	terminalLogin bool
	// noContinue is whether a conversation cannot move to another profile
	// (opencode's), so the card menu offers no Continue in.
	noContinue bool
	// words are the provider-specific sentences of the Profile and Profiles
	// rows and of the profile dialog.
	words profileWords
}

type profileWords struct {
	profileDesc, profilesDesc, copiedNote, searchAlso, profilesAlso string
	newDesc, renameDesc, deleteDesc, loginDesc, addressDesc         string
}

var claudeProfileProvider = profileProvider{
	kind: claudeKind, key: claudeProfileKey, label: "Claude Code", quotaPrefix: "profile:",
	api:     func(c *lichclient.Client) lichclient.ProfileAPI { return c.ClaudeProfiles() },
	plugins: true, loginCode: true,
	words: profileWords{
		profileDesc:  "Which Claude login new sessions use. A session keeps the profile it was born in, resumes included.",
		profilesDesc: "Each profile is its own Claude login with its own plugins and history. Settings, CLAUDE.md, agents, commands, skills and hooks are the same in all of them.",
		copiedNote:   "Settings, CLAUDE.md, agents, commands, skills and hooks are copies in this profile: editing them in another profile no longer reaches it.",
		searchAlso:   "claude account login personal work",
		profilesAlso: "claude accounts logins personal work log in plugins",
		newDesc:      "A Claude login of its own, with its own plugins and history.",
		renameDesc:   "Only its name changes: its login, plugins and history stay as they are.",
		deleteDesc:   "Its login, plugins and conversation history are removed, and sessions started in it can no longer be resumed.",
		loginDesc:    "Sign in in the browser tab Claude Code opened. This carries on by itself when you are done.",
		addressDesc:  "If the browser did not open, sign in at this address and paste the code it shows.",
	},
}

var codexProfileProvider = profileProvider{
	kind: codexKind, key: codexProfileKey, label: "Codex", quotaPrefix: "codex-profile:",
	api:     func(c *lichclient.Client) lichclient.ProfileAPI { return c.CodexProfiles() },
	plugins: true,
	words: profileWords{
		profileDesc:  "Which Codex login new sessions use. A session keeps the profile it was born in, resumes included.",
		profilesDesc: "Each profile is its own Codex login with its own plugins and history. Personal's config.toml settings, AGENTS.md, hooks, skills and rules are the same in all of them.",
		copiedNote:   "AGENTS.md and hooks.json are copies in this profile: editing them in another profile no longer reaches it.",
		searchAlso:   "codex account login personal work chatgpt",
		profilesAlso: "codex accounts logins personal work log in chatgpt plugins",
		newDesc:      "A Codex login of its own, with its own plugins and history.",
		renameDesc:   "Only its name changes: its login, plugins and history stay as they are.",
		deleteDesc:   "Its login, plugins and conversation history are removed, and sessions started in it can no longer be resumed.",
		loginDesc:    "Sign in in the browser tab Codex opened. This carries on by itself when you are done.",
		addressDesc:  "If the browser did not open, sign in at this address.",
	},
}

var ompProfileProvider = profileProvider{
	kind: ompKind, key: ompProfileKey, label: "oh-my-pi",
	api:           func(c *lichclient.Client) lichclient.ProfileAPI { return c.OMPProfiles() },
	terminalLogin: true,
	words: profileWords{
		profileDesc:  "Which oh-my-pi login new sessions use. A session keeps the profile it was born in, resumes included.",
		profilesDesc: "Each profile is its own set of oh-my-pi logins with its own history. config.yml, models.yml, mcp.json, AGENTS.md, skills, rules, commands, prompts and extensions are the same in all of them.",
		copiedNote:   "config.yml, models.yml, mcp.json and the markdown files are copies in this profile: editing them in another profile no longer reaches it.",
		searchAlso:   "omp oh-my-pi account login personal work",
		profilesAlso: "omp oh-my-pi accounts logins personal work log in",
		newDesc:      "An oh-my-pi login of its own, with its own history. You log in from a terminal once it is made.",
		renameDesc:   "Only its name changes: its logins and history stay as they are.",
		deleteDesc:   "Its logins and conversation history are removed, and sessions started in it can no longer be resumed.",
	},
}

// profileProviderOf is the profile provider for a session kind, false for a
// provider with no profiles.
func profileProviderOf(kind string) (profileProvider, bool) {
	switch kind {
	case claudeKind:
		return claudeProfileProvider, true
	case codexKind:
		return codexProfileProvider, true
	case ompKind:
		return ompProfileProvider, true
	}
	return xdgProfileProviderOf(kind)
}

// profileBook is one provider's profiles as the window knows them: who is
// logged in where. The window loads it at start and again after anything that
// changes a login; Settings and the card menu read it. Each profile's plan
// reading lives in quota, the window's one store of them. Every method is safe
// from any goroutine.
type profileBook struct {
	provider   profileProvider
	client     *lichclient.Client
	invalidate func()
	quota      *planQuota

	mu       sync.Mutex
	list     []lichclient.Profile
	loaded   bool
	loadedAt time.Time
	reading  bool
	// quoted are the readings the Continue in rows quote, watched.
	quoted map[string]func()
}

func newProfileBook(prov profileProvider, c *lichclient.Client, invalidate func(), quota *planQuota) *profileBook {
	return &profileBook{provider: prov, client: c, invalidate: invalidate, quota: quota, quoted: map[string]func(){}}
}

// reload asks the backend for the profiles. It runs one CLI per profile, so
// callers keep it off the frame.
func (b *profileBook) reload(ctx context.Context) error {
	list, err := b.provider.api(b.client).List(ctx)
	if err != nil {
		return err
	}
	b.mu.Lock()
	gone := slices.DeleteFunc(slices.Clone(b.list), func(old lichclient.Profile) bool {
		return slices.ContainsFunc(list, func(p lichclient.Profile) bool { return p.Slug == old.Slug })
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
		log.Printf("%s profiles after a login: %v", b.provider.kind, err)
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
			log.Printf("%s profiles on focus: %v", b.provider.kind, err)
		}
	}()
}

// followQuoted keeps the plan readings the Continue in rows quote current: one
// per logged-in profile while there is more than one, the only case a reading
// is quoted outside Settings. The caller holds b.mu.
func (b *profileBook) followQuoted() {
	want := map[string]bool{}
	logged := slices.DeleteFunc(slices.Clone(b.list), func(p lichclient.Profile) bool { return !p.LoggedIn })
	if b.provider.quotaPrefix != "" && len(logged) > 1 {
		for _, p := range logged {
			want[profileQuotaKey(b.provider, p.Slug)] = true
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
	if b.provider.quotaPrefix == "" {
		return
	}
	b.quota.invalidate(profileQuotaKey(b.provider, slug))
}

func (b *profileBook) snapshot() ([]lichclient.Profile, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.list), b.loaded
}

// find is the profile a stored value names; "" is Personal.
func (b *profileBook) find(stored string) (lichclient.Profile, bool) {
	slug := effectiveProfile(stored)
	b.mu.Lock()
	defer b.mu.Unlock()
	i := slices.IndexFunc(b.list, func(p lichclient.Profile) bool { return p.Slug == slug })
	if i < 0 {
		return lichclient.Profile{}, false
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
func (b *profileBook) loggedIn() []lichclient.Profile {
	list, _ := b.snapshot()
	return slices.DeleteFunc(list, func(p lichclient.Profile) bool { return !p.LoggedIn })
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

// profileAccountLine is a profile row's second line: the account, its plan or
// the providers it holds logins for and, for a provider whose profiles have a
// plugin set, how many plugins it carries. "" for a profile nobody is logged in
// to.
func profileAccountLine(prov profileProvider, p lichclient.Profile) string {
	if !p.LoggedIn {
		return ""
	}
	var parts []string
	if p.Email != "" {
		parts = append(parts, p.Email)
	}
	if p.Plan != "" {
		parts = append(parts, strings.ToUpper(p.Plan[:1])+p.Plan[1:])
	}
	if len(p.Providers) > 0 {
		parts = append(parts, strings.Join(p.Providers, ", "))
	}
	if prov.plugins {
		switch p.Plugins {
		case 0:
			parts = append(parts, profileNoPlugins)
		case 1:
			parts = append(parts, profileOnePlugin)
		default:
			parts = append(parts, fmt.Sprintf(profileManyPlugins, p.Plugins))
		}
	}
	if len(parts) == 0 {
		return profileLoggedIn
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
