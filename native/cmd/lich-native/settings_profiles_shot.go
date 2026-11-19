package main

import (
	"flag"
	"fmt"
	"image"
	"strconv"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

var (
	flagProfileDialog = flag.String("settings-profile-dialog", "", "open the profile dialog of the provider -settings-provider names (Claude Code by default) at start, for -shot: new, rename, delete, copy, login or result")
	flagProfileUsage  = flag.String("profile-usage", "", "seed Continue in's five-hour readings as slug=percent,…, for -shot; slug=full seeds a whole plan for the usage tooltip and holds it against the backend's own reading")
	flagCardMenu      = flag.String("card-menu", "", "open a card's menu at start as <session id>@<x>,<y>:<row>[,<row>…], for -shot")
)

// shotAddress and shotCodexAddress are the sign-in address
// -settings-profile-dialog login shows, the shape each CLI prints
// (claude auth login --claudeai, 2.1.296; codex login, 0.162.1).
const (
	shotAddress      = "https://claude.com/cai/oauth/authorize?code=true&client_id=9d1c250a-e61b-44d9-88ed-5944d1962f5e&response_type=code&redirect_uri=https%3A%2F%2Fplatform.claude.com%2Foauth%2Fcode%2Fcallback"
	shotCodexAddress = "https://auth.openai.com/oauth/authorize?response_type=code&client_id=app_EMoamEEZ73f0CkXaXp7hrann&redirect_uri=http%3A%2F%2Flocalhost%3A1455%2Fauth%2Fcallback&scope=openid%20profile%20email%20offline_access"
)

// showProfileDialog puts the dialog in the state a -shot asks for, with
// nothing sent to the backend beyond what opening it reads.
func (s *settingsScreen) showProfileDialog(name string) error {
	target := backend.Profile{Slug: "work", Name: "Work", LoggedIn: true}
	c := s.providers.profiles(*flagSettingsProvider)
	if c == nil {
		c = &s.providers.claude
	}
	f := &c.flow
	switch name {
	case "new":
		s.startNewProfile(c)
	case "rename":
		s.startRename(c, target)
	case "delete":
		s.startDelete(c, target)
	case "copy":
		if !c.provider.plugins {
			return fmt.Errorf("-settings-profile-dialog: %s profiles have no plugins to copy", c.provider.label)
		}
		s.startCopy(c, target)
	case "login":
		url := shotAddress
		if c.provider.kind == codexKind {
			url = shotCodexAddress
		}
		f.set(func(d *flowData) {
			*d = flowData{kind: flowLogin, open: true, step: stepWaiting, target: target, url: url}
		})
	case "result":
		var results []backend.ProfilePluginResult
		if c.provider.plugins {
			results = []backend.ProfilePluginResult{
				{Key: "frontend-design@claude-plugins-official", OK: true},
				{Key: "security-guidance@acme-internal", Message: "Plugin installation is blocked by your organization's policy."},
			}
		}
		f.set(func(d *flowData) {
			*d = flowData{kind: flowLogin, open: true, step: stepResult, target: target, account: "Logged in as work@example.com · Max", results: results}
		})
	default:
		return fmt.Errorf("-settings-profile-dialog: no dialog %q", name)
	}
	return nil
}

// seedProfilePlans gives the store the five-hour readings -profile-usage names,
// as if the backend had answered them just now.
func (b *profileBook) seedProfilePlans(spec string) error {
	for pair := range strings.SplitSeq(spec, ",") {
		slug, raw, ok := strings.Cut(pair, "=")
		if raw == "full" {
			b.quota.pin(profileQuotaKey(b.provider, slug), []backend.QuotaPlan{shotUsagePlan(b.provider.kind)})
			continue
		}
		percent, err := strconv.Atoi(raw)
		if !ok || err != nil {
			return fmt.Errorf("-profile-usage: want slug=percent, got %q", pair)
		}
		plan := backend.QuotaPlan{Provider: b.provider.kind, Status: "ok",
			Windows: []backend.QuotaWindow{{Label: "5h", Seconds: sessionWindowSeconds, Percent: percent}}}
		b.quota.seed(profileQuotaKey(b.provider, slug), []backend.QuotaPlan{plan})
	}
	return nil
}

// shotUsagePlan is a plan with everything the usage tooltip can show: three
// windows out of order, a weekly one on course to run out, and the one extra
// each provider alone reports.
func shotUsagePlan(kind string) backend.QuotaPlan {
	in := func(d time.Duration) string { return time.Now().Add(d).UTC().Format(time.RFC3339) }
	weekly := 3*24*time.Hour + 14*time.Hour
	plan := backend.QuotaPlan{Provider: kind, Name: "Claude Code", Plan: "Max 5x", Account: "dev@example.com", Status: "ok",
		Windows: []backend.QuotaWindow{
			{Label: "Fable", Seconds: 7 * day, Percent: 6, ResetsAt: in(weekly)},
			{Kind: api.QuotaWindowWeekly, Seconds: 7 * day, Percent: 62, ResetsAt: in(weekly), RunsOutAt: in(33 * time.Hour)},
			{Kind: api.QuotaWindowSession, Seconds: 5 * hour, Percent: 25, ResetsAt: in(4*time.Hour + 12*time.Minute)},
		},
		Extra: &backend.QuotaExtra{Used: 1250, Limit: new(int64(5000)), Currency: "USD", Decimals: 2},
		Resets: []backend.QuotaReset{
			{Kind: api.QuotaResetFull, Left: 1, EndsAt: in(21 * 24 * time.Hour)},
			{Label: "Fable", Left: 1, EndsAt: in(110 * 24 * time.Hour)},
			{Label: "Fable", Left: 1, EndsAt: in(110 * 24 * time.Hour), Held: true},
		},
	}
	if kind == codexKind {
		plan.Name, plan.Plan, plan.Extra, plan.Resets, plan.ResetCredits = "Codex", "Plus", nil, nil, 1
	}
	return plan
}

// openShotMenu opens the card menu -card-menu names, once its session is on
// screen.
func (u *window) openShotMenu() {
	spec := *flagCardMenu
	id, rest, _ := strings.Cut(spec, "@")
	at, path, _ := strings.Cut(rest, ":")
	xs, ys, _ := strings.Cut(at, ",")
	x, errX := strconv.Atoi(xs)
	y, errY := strconv.Atoi(ys)
	if errX != nil || errY != nil {
		return
	}
	if _, _, found := u.model.slotOf(id); !found {
		return
	}
	var rows []int
	for raw := range strings.SplitSeq(path, ",") {
		if n, err := strconv.Atoi(raw); err == nil {
			rows = append(rows, n)
		}
	}
	menu, ok := u.sidebar.menus[id]
	if !ok {
		menu = new(ui.MenuState)
		u.sidebar.menus[id] = menu
	}
	menu.OpenAt(image.Pt(x, y), rows...)
	*flagCardMenu = ""
}

// pin is seed that the poll cannot replace: key answers plans for good.
func (q *planQuota) pin(key string, plans []backend.QuotaPlan) {
	q.seed(key, plans)
	fetch := q.fetch
	q.fetch = func(k string) ([]backend.QuotaPlan, error) {
		if k == key {
			return plans, nil
		}
		return fetch(k)
	}
}

// seed puts a reading under key as if a read had landed, for the rigs that
// draw a window without a backend answering.
func (q *planQuota) seed(key string, plans []backend.QuotaPlan) {
	q.mu.Lock()
	defer q.mu.Unlock()
	e := q.entries[key]
	if e == nil {
		e = &quotaEntry{}
		q.entries[key] = e
	}
	e.plans = plans
}
