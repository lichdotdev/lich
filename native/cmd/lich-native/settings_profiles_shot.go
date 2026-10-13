package main

import (
	"flag"
	"fmt"
	"image"
	"strconv"
	"strings"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

var (
	flagProfileDialog = flag.String("settings-profile-dialog", "", "open the profile dialog of the provider -settings-provider names (Claude Code by default) at start, for -shot: new, rename, delete, copy, login or result")
	flagProfileUsage  = flag.String("profile-usage", "", "seed Continue in's five-hour readings as slug=percent,…, for -shot")
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
	target := lichclient.Profile{Slug: "work", Name: "Work", LoggedIn: true}
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
		var results []lichclient.ProfilePluginResult
		if c.provider.plugins {
			results = []lichclient.ProfilePluginResult{
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
		percent, err := strconv.Atoi(raw)
		if !ok || err != nil {
			return fmt.Errorf("-profile-usage: want slug=percent, got %q", pair)
		}
		plan := lichclient.QuotaPlan{Provider: b.provider.kind, Status: "ok",
			Windows: []lichclient.QuotaWindow{{Label: "5h", Seconds: sessionWindowSeconds, Percent: percent}}}
		b.quota.seed(profileQuotaKey(b.provider, slug), []lichclient.QuotaPlan{plan})
	}
	return nil
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

// seed puts a reading under key as if a read had landed, for the rigs that
// draw a window without a backend answering.
func (q *planQuota) seed(key string, plans []lichclient.QuotaPlan) {
	q.mu.Lock()
	defer q.mu.Unlock()
	e := q.entries[key]
	if e == nil {
		e = &quotaEntry{}
		q.entries[key] = e
	}
	e.plans = plans
}
