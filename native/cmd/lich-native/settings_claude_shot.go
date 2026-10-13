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
	flagProfileDialog = flag.String("settings-profile-dialog", "", "open Settings › Providers › Claude Code's profile dialog at start, for -shot: new, rename, delete, copy, login or result")
	flagProfileUsage  = flag.String("profile-usage", "", "seed Continue in's five-hour readings as slug=percent,…, for -shot")
	flagCardMenu      = flag.String("card-menu", "", "open a card's menu at start as <session id>@<x>,<y>:<row>[,<row>…], for -shot")
)

// shotAddress is the sign-in address -settings-profile-dialog login shows,
// the shape the CLI prints (claude auth login --claudeai, 2.1.296).
const shotAddress = "https://claude.com/cai/oauth/authorize?code=true&client_id=9d1c250a-e61b-44d9-88ed-5944d1962f5e&response_type=code&redirect_uri=https%3A%2F%2Fplatform.claude.com%2Foauth%2Fcode%2Fcallback"

// showProfileDialog puts the dialog in the state a -shot asks for, with
// nothing sent to the backend beyond what opening it reads.
func (s *settingsScreen) showProfileDialog(name string) error {
	target := lichclient.ClaudeProfile{Slug: "work", Name: "Work", LoggedIn: true}
	f := &s.providers.claude.flow
	switch name {
	case "new":
		s.startNewProfile()
	case "rename":
		s.startRename(target)
	case "delete":
		s.startDelete(target)
	case "copy":
		s.startCopy(target)
	case "login":
		f.set(func(d *flowData) {
			*d = flowData{kind: flowLogin, open: true, step: stepWaiting, target: target, url: shotAddress}
		})
	case "result":
		f.set(func(d *flowData) {
			*d = flowData{kind: flowLogin, open: true, step: stepResult, target: target, account: "Logged in as work@example.com · Max",
				results: []lichclient.ClaudeProfilePluginResult{
					{Key: "frontend-design@claude-plugins-official", OK: true},
					{Key: "security-guidance@acme-internal", Message: "Plugin installation is blocked by your organization's policy."},
				}}
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
		plan := lichclient.QuotaPlan{Provider: claudeKind, Status: "ok",
			Windows: []lichclient.QuotaWindow{{Label: "5h", Seconds: sessionWindowSeconds, Percent: percent}}}
		b.quota.seed(profileQuotaKey(slug), []lichclient.QuotaPlan{plan})
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
