package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/omartelo/lich/native/lichclient"
)

func TestFormatCost(t *testing.T) {
	cases := map[float64]string{
		12.476: "$12.48", 1: "$1.00", // cents once there are dollars
		0.42: "$0.420", 0.004: "$0.004", // three decimals below a dollar
		0.0004: "<$0.001", 0: "$0",
		-1: "", math.NaN(): "", math.Inf(1): "",
	}
	for usd, want := range cases {
		if got := formatCost(usd); got != want {
			t.Errorf("formatCost(%v) = %q, want %q", usd, got, want)
		}
	}
}

func TestBudgetShare(t *testing.T) {
	cases := []struct{ usd, budget, want float64 }{
		{5, 10, 50}, {0.25, 1, 25}, {8, 10, 80}, {9.5, 10, 95},
		{25, 10, 100}, // capped: no colour louder than red
		{12, 0, 0}, {12, -5, 0}, {math.NaN(), 10, 0}, {-1, 10, 0}, {1, math.Inf(1), 0},
	}
	for _, c := range cases {
		if got := budgetShare(c.usd, c.budget); got != c.want {
			t.Errorf("budgetShare(%v, %v) = %v, want %v", c.usd, c.budget, got, c.want)
		}
	}
	if got := budgetShare(7.9, 10); math.Abs(got-79) > 1e-9 {
		t.Errorf("budgetShare(7.9, 10) = %v, want ~79", got)
	}
}

func TestEveryCostMissHasASentence(t *testing.T) {
	for _, miss := range costMisses {
		if costMissReason[miss] == "" {
			t.Errorf("no sentence for %q", miss)
		}
	}
}

func TestToneFollowsTheUsageScale(t *testing.T) {
	cases := map[float64]usageTone{0: toneMuted, 79.9: toneMuted, 80: toneWait, 94: toneWait, 95: toneDestructive, 100: toneDestructive}
	for p, want := range cases {
		if got := toneOf(p); got != want {
			t.Errorf("toneOf(%v) = %v, want %v", p, got, want)
		}
	}
}

func cost(usd float64) *float64 { return &usd }

func TestCostReadout(t *testing.T) {
	if _, ok := costReadout(sessionUsage{}, 0); ok {
		t.Error("no figure and no reason drew a readout")
	}
	miss, ok := costReadout(sessionUsage{CostMiss: "mixed-models"}, 10)
	if !ok || miss.Text != "$—" || miss.Label != "Cost unavailable" || miss.Detail[0] != costMissReason["mixed-models"] {
		t.Errorf("cost miss: %+v, %v", miss, ok)
	}
	r, _ := costReadout(sessionUsage{CostUSD: cost(9)}, 10)
	if r.Text != "$9.00" || r.Tone != toneWait || r.Detail[0] != "$9.00 of $10.00 budget" || len(r.Detail) != 2 {
		t.Errorf("against a budget: %+v", r)
	}
	if r, _ := costReadout(sessionUsage{CostUSD: cost(9)}, 0); r.Tone != toneMuted || len(r.Detail) != 1 {
		t.Errorf("no budget: %+v", r)
	}
}

func TestContextReadout(t *testing.T) {
	if _, ok := contextReadout(sessionUsage{Percent: 0, CostUSD: cost(1)}); ok {
		t.Error("a cost-only report drew a context ring")
	}
	r, ok := contextReadout(sessionUsage{Percent: 96, Tokens: 192000, Window: 200000})
	if !ok || r.Text != "96%" || r.Tone != toneDestructive || r.Detail[0] != "192,000 / 200,000 tokens" {
		t.Errorf("got %+v, %v", r, ok)
	}
}

func TestGroupThousands(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 1000000: "1,000,000", 84000: "84,000"} {
		if got := groupThousands(n); got != want {
			t.Errorf("groupThousands(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestFormatModel(t *testing.T) {
	cases := map[string]string{
		"claude-opus-4-8":           "opus 4.8",
		"claude-haiku-4-5-20251001": "haiku 4.5",
		"claude-fable-5":            "fable 5",
		"claude-opus":               "opus",
		"gpt-5-codex":               "gpt-5-codex",
	}
	for id, want := range cases {
		if got := formatModel(id); got != want {
			t.Errorf("formatModel(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestSessionModelText(t *testing.T) {
	if _, _, ok := sessionModelText(sessionUsage{CostUSD: cost(1)}); ok {
		t.Error("a report with no model filled the slot")
	}
	text, hint, ok := sessionModelText(sessionUsage{Model: "claude-opus-4-8", Effort: "high"})
	if !ok || text != "opus 4.8 · high" || hint != "claude-opus-4-8 · high" {
		t.Errorf("got %q, %q, %v", text, hint, ok)
	}
}

func TestClockText(t *testing.T) {
	at := time.Date(2026, 10, 11, 14, 30, 59, 0, time.UTC)
	if got := clockText(at); got != "Sun, Oct 11, 2026 · 02:30 PM" {
		t.Errorf("got %q", got)
	}
	if got := untilNextMinute(at); got != time.Second {
		t.Errorf("untilNextMinute = %v, want 1s", got)
	}
}

func TestHandsOnReadoutFigure(t *testing.T) {
	cases := map[int64]string{
		0: "", 59: "", -60: "", // under a minute is no number
		60: "1m", 119: "1m", 48 * 60: "48m", 59*60 + 59: "59m",
		3600: "1h00m", 3600 + 5*60: "1h05m", 3600 + 12*60: "1h12m",
		14*3600 + 5*60: "14h05m", 100 * 3600: "100h00m",
	}
	for seconds, want := range cases {
		r, ok := handsOnReadout(seconds, nil, "claude")
		if ok != (want != "") || r.Text != want {
			t.Errorf("handsOnReadout(%d) = %q, %v; want %q", seconds, r.Text, ok, want)
		}
	}
	r, _ := handsOnReadout(3600+12*60, nil, "crush")
	if r.Detail[0] != "1h 12m" || r.Detail[1] != handsOnDetail("crush") {
		t.Errorf("tooltip: %q", r.Detail)
	}
	if r, ok := handsOnReadout(0, errors.New("down"), ""); !ok || r.Text != "—" {
		t.Errorf("a failed read: %+v, %v", r, ok)
	}
}

func TestHandsOnDetailByRung(t *testing.T) {
	turn := handsOnDetail("claude")
	for _, kind := range []string{"", "shell", "codex", "kiro"} {
		if handsOnDetail(kind) != turn {
			t.Errorf("%q does not use the turn wording", kind)
		}
	}
	if handsOnDetail("crush") == turn {
		t.Error("crush, a tool-rung provider, uses the turn wording")
	}
}

func quotaPlan(windows ...lichclient.QuotaWindow) lichclient.QuotaPlan {
	return lichclient.QuotaPlan{Provider: "claude", Name: "Claude", Status: "ok", Windows: windows}
}

func TestHottestWindow(t *testing.T) {
	session := lichclient.QuotaWindow{Label: "Session", Percent: 40}
	weekly := lichclient.QuotaWindow{Label: "Weekly", Percent: 80}
	cases := []struct {
		name string
		plan lichclient.QuotaPlan
		want string
	}{
		{"closest to spent, whatever its position", quotaPlan(session, weekly), "Weekly"},
		{"provider's order on a tie", quotaPlan(session, lichclient.QuotaWindow{Label: "Weekly", Percent: 40}), "Session"},
		{"the active verdict wins", quotaPlan(lichclient.QuotaWindow{Label: "Session", Percent: 10, Active: true}, weekly), "Session"},
	}
	for _, c := range cases {
		if got, _ := hottestWindow(c.plan); got.Label != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got.Label, c.want)
		}
	}
	if _, ok := hottestWindow(quotaPlan()); ok {
		t.Error("a plan with no windows has a hottest one")
	}
}

func TestPlanReadout(t *testing.T) {
	plan := quotaPlan(
		lichclient.QuotaWindow{Label: "Session", Seconds: 18000, Percent: 96},
		lichclient.QuotaWindow{Label: "Weekly", Seconds: 604800, Percent: 20, LockedReason: "Out of credits"},
	)
	plan.Plan = "Max"
	r, ok := planReadout(plan)
	if !ok || r.Text != "wk Locked" || r.Tone != toneDestructive || r.Title != "Claude · Max" {
		t.Errorf("a locked window must stay visible: %+v", r)
	}
	r, _ = planReadout(quotaPlan(lichclient.QuotaWindow{Label: "Session", Seconds: 18000, Percent: 82}))
	if r.Text != "5h 82%" || r.Tone != toneWait || r.Title != "Claude" {
		t.Errorf("got %+v", r)
	}
	signedOut := quotaPlan(lichclient.QuotaWindow{Label: "Session", Percent: 1})
	signedOut.Status = "signed-out"
	if _, ok := planReadout(signedOut); ok {
		t.Error("a reading that is not ok drew a readout")
	}
}

func TestPlanFor(t *testing.T) {
	plans := []lichclient.QuotaPlan{{Provider: "codex"}, {Provider: "claude"}}
	if p, ok := planFor(plans, "claude"); !ok || p.Provider != "claude" {
		t.Errorf("got %+v, %v", p, ok)
	}
	for _, provider := range []string{"", "crush"} {
		if _, ok := planFor(plans, provider); ok {
			t.Errorf("planFor(%q) found a plan", provider)
		}
	}
}

func TestQuotaGauges(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	plan := quotaPlan(
		lichclient.QuotaWindow{Label: "Session", Seconds: 18000, Percent: 42, ResetsAt: "2026-08-17T16:12:00Z"},
		lichclient.QuotaWindow{Label: "Weekly", Seconds: 604800, Percent: 85, Ahead: true, LockedReason: "x"},
	)
	want := []quotaGauge{
		{Label: "Session · 5h", Reading: "42%", Left: "4h 12m", Percent: 42},
		{Label: "Weekly · 7d", Reading: "Locked", Percent: 85, Locked: "x", Ahead: true, Tone: toneWait},
	}
	if got := quotaGauges(plan, now); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestAccountLine(t *testing.T) {
	cases := []struct {
		plan lichclient.QuotaPlan
		want string
	}{
		{lichclient.QuotaPlan{Account: "dev@example.com"}, "dev@example.com"},
		{lichclient.QuotaPlan{NoAccount: "token-login"}, "Token login"},
		{lichclient.QuotaPlan{}, ""},
	}
	for _, c := range cases {
		if got := accountLine(c.plan); got != c.want {
			t.Errorf("accountLine(%+v) = %q, want %q", c.plan, got, c.want)
		}
	}
}

func TestWindowLengths(t *testing.T) {
	short := map[int]string{18000: "5h", 604800: "wk", 2592000: "30d", 0: "", -1: ""}
	for s, want := range short {
		if got := shortWindow(s); got != want {
			t.Errorf("shortWindow(%d) = %q, want %q", s, got, want)
		}
	}
	full := map[int]string{18000: "5h", 604800: "7d", 2592000: "30d", 0: ""}
	for s, want := range full {
		if got := formatWindow(s); got != want {
			t.Errorf("formatWindow(%d) = %q, want %q", s, got, want)
		}
	}
}

func TestTimeLeft(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	cases := map[string]string{
		"2026-08-24T08:00:00Z": "6d 20h", "2026-08-17T16:12:00Z": "4h 12m", "2026-08-17T12:41:00Z": "41m",
		"2026-08-17T11:59:00Z": "", "2026-08-17T12:00:00Z": "", // never counts down to the past
		"2026-08-17T12:00:30Z": "1m", // the last minute rounds up
		"":                     "", "whenever": "",
	}
	for at, want := range cases {
		if got := timeLeft(at, now); got != want {
			t.Errorf("timeLeft(%q) = %q, want %q", at, got, want)
		}
	}
}

func TestComposeDroppedPaths(t *testing.T) {
	const start, end = "\x1b[200~", "\x1b[201~"
	cases := []struct {
		name    string
		paths   []string
		windows bool
		notices []string
		want    string
	}{
		{"an empty drop writes nothing", nil, false, nil, ""},
		{"one path, room for what is typed next", []string{"/home/u/a.ts"}, false, nil, start + "/home/u/a.ts " + end},
		{"a batch", []string{"/home/u/a.ts", "/home/u/b.ts"}, false, nil, start + "/home/u/a.ts /home/u/b.ts " + end},
		{"a notice goes under the path", []string{"/cfg/dropped/b.png"}, false, []string{"[lich] copy of b.png; …"},
			start + "/cfg/dropped/b.png\n[lich] copy of b.png; …\n" + end},
		{"an empty notice adds no line", []string{"/home/u/a.ts"}, false, []string{""}, start + "/home/u/a.ts " + end},
		{"a space is quoted", []string{"/home/u/my notes.md"}, false, nil, start + "'/home/u/my notes.md' " + end},
		{"POSIX escapes a quote", []string{"/u/it's"}, false, nil, start + `'/u/it'\''s' ` + end},
		{"PowerShell doubles it", []string{`C:\u\it's`}, true, nil, start + `'C:\u\it''s' ` + end},
		{"a Windows path is bare", []string{`C:\u\a.ts`}, true, nil, start + `C:\u\a.ts ` + end},
		{"ESC cannot close the paste early", []string{"/u/a\x1b[201~\rb"}, false, nil, start + "'/u/a[201~b' " + end},
	}
	for _, c := range cases {
		if got := composeDroppedPaths(c.paths, c.windows, c.notices); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// attachBackend answers drop.Attach with picked and records terminal.Write.
func attachBackend(t *testing.T, picked lichclient.Attachment, writes *[][]string) *lichclient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var args []json.RawMessage
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &args)
		switch r.URL.Path {
		case "/rpc/drop.Attach":
			if string(args[1]) != `"/repo"` || string(args[2]) != "true" {
				t.Errorf("drop.Attach args %s", body)
			}
			_ = json.NewEncoder(w).Encode(picked)
		case "/rpc/terminal.Write":
			var id, data string
			_ = json.Unmarshal(args[0], &id)
			_ = json.Unmarshal(args[1], &data)
			*writes = append(*writes, []string{id, data})
			_ = json.NewEncoder(w).Encode(nil)
		default:
			t.Errorf("unexpected call %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return lichclient.New(lichclient.Runtime{Port: port, Token: "tok"})
}

func TestAttachFilePastesThePickedPathUnsent(t *testing.T) {
	var writes [][]string
	c := attachBackend(t, lichclient.Attachment{Path: "/cfg/dropped/b.png", Notice: "copy of b"}, &writes)
	if err := attachFile(context.Background(), c, "s1", "/repo", true, false); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"s1", "\x1b[200~/cfg/dropped/b.png\ncopy of b\n\x1b[201~"}}
	if !reflect.DeepEqual(writes, want) {
		t.Errorf("wrote %q, want %q", writes, want)
	}
}

func TestAttachFileCancelledWritesNothing(t *testing.T) {
	var writes [][]string
	c := attachBackend(t, lichclient.Attachment{}, &writes)
	if err := attachFile(context.Background(), c, "s1", "/repo", true, false); err != nil || len(writes) != 0 {
		t.Errorf("cancelled dialog: err %v, writes %q", err, writes)
	}
}
