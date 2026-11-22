package relay

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

// requestCases are the requests docs/hooks/fixtures/notification.json writes,
// by fixture case name.
func requestCases() map[string]api.RequestOptions {
	pr := func(topic string, items ...api.RequestItem) api.RequestOptions {
		return api.RequestOptions{Topic: topic, PR: 1210, Branch: "fix-412-handshake", Items: items}
	}
	conflicts := pr(api.RequestConflicts, api.RequestItem{Path: "internal/relay/conn.go"}, api.RequestItem{Path: "internal/relay/relay.go"})
	conflicts.Base = "main"
	var many []api.RequestItem
	for i := 1; i <= namedChecks+1; i++ {
		many = append(many, api.RequestItem{Check: fmt.Sprintf("test (shard %d)", i)})
	}
	return map[string]api.RequestOptions{
		"request: resolve conflicts": conflicts,
		"request: fix the failing checks": pr(api.RequestChecks,
			api.RequestItem{Check: "test (ubuntu-latest)", URL: "https://github.com/lichdotdev/lich/actions/runs/7/job/8"},
			api.RequestItem{Check: "lint"}),
		"request: checks past the ones named": pr(api.RequestChecks, many...),
		"request: review threads": pr(api.RequestReviewThread,
			api.RequestItem{Path: "internal/relay/conn.go", Lines: "88-94", ID: "PRRT_kw1",
				URL:      "https://github.com/lichdotdev/lich/pull/1210#discussion_r11",
				Comments: []api.RequestComment{{Author: "alice", Body: "This retries forever."}, {Author: "bob", Body: "Agreed, cap it."}}},
			api.RequestItem{Path: "docs/cli.md", Lines: "41", ID: "PRRT_kw2", Outdated: true,
				Comments: []api.RequestComment{{Body: "Name the flag."}}}),
		"request: review comments": {Topic: api.RequestReviewComments, Note: "Keep the tests green.", Items: []api.RequestItem{
			{Path: "internal/relay/envelope.go", Lines: "42", Text: "Name this constant."},
			{Path: "internal/relay/envelope.go", Lines: "71-73", Text: "Escape the quote too."}}},
		"request: browser notes with a sketch": {Topic: api.RequestBrowserNotes, URL: "http://localhost:5173/settings", Image: "/home/me/.config/lich/drops/sketch.png",
			Items: []api.RequestItem{
				{Label: `button "Save"`, URL: "http://localhost:5173/settings", Selector: "#save", Component: "SaveButton", Source: "src/Save.tsx:12",
					HTML: `<button id="save">Save</button>`, Text: "Too close to the edge."},
				{Label: `h2 "Profile"`, URL: "http://localhost:5173/settings", Selector: "main > h2", HTML: "<h2>Profile</h2>"}}},
		"request: start on an issue": {Topic: api.RequestIssue, Issue: 1173, Title: "Relay handshake races on reconnect",
			URL: "https://github.com/lichdotdev/lich/issues/1173", Body: "Reconnecting mid-handshake drops the session."},
		"request: escaping, attributes escaped and item text verbatim": {Topic: api.RequestReviewComments, Items: []api.RequestItem{
			{Path: `a "b" & <c>.go`, Lines: "3", Text: "Use a && b, then check that </item> stays text."}}},
	}
}

// A request reaches the session as the envelope the composer writes, by the
// path a submitted message takes.
func TestARequestIsSubmittedAsItsEnvelope(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s2")
	svc := newRelay(workspace(), term, nil)
	opts := requestCases()["request: resolve conflicts"]
	opts.SessionID = "s2"

	if _, err := svc.Request(context.Background(), opts); err != nil {
		t.Fatalf("Request: %v", err)
	}
	want, _ := requestEnvelope(prompt.English, opts)
	if typed := term.written("s2"); !strings.Contains(typed, want.String()) {
		t.Errorf("typed %q, want the envelope %q", typed, want.String())
	}
}

// A topic the contract does not name is refused before anything is typed.
func TestARequestOfAnUnknownTopicIsRefused(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s2")
	svc := newRelay(workspace(), term, nil)
	if _, err := svc.Request(context.Background(), api.RequestOptions{SessionID: "s2", Topic: "nap"}); err == nil {
		t.Fatal("Request accepted an unknown topic")
	}
	assertNotTyped(t, term, "s2")
}

// Every request renders in every language with its arguments in place.
func TestEveryRequestRendersInEveryLanguage(t *testing.T) {
	t.Parallel()
	for _, lang := range prompt.Langs {
		for name, opts := range requestCases() {
			env, err := requestEnvelope(lang, opts)
			if err != nil || strings.Contains(env.String(), "%!") {
				t.Errorf("%s %s: %v\n%s", lang, name, err, env.String())
			}
		}
		unnamed, _ := requestEnvelope(lang, api.RequestOptions{Topic: api.RequestChecks, PR: 7, Branch: "b", Failed: 3, Forge: api.ForgeGitLab})
		sketch, _ := requestEnvelope(lang, api.RequestOptions{Topic: api.RequestBrowserNotes, URL: "http://x", Image: "/s.png"})
		for _, env := range []Envelope{unnamed, sketch} {
			if strings.Contains(env.String(), "%!") {
				t.Errorf("%s: %s", lang, env.String())
			}
		}
	}
}

// The notes read as each element's label and page, its selector, the React
// component that drew it, its markup and the person's note, then the sketch,
// the selector line and the closing note.
func TestNotesTextNamesEachElementItsSelectorAndTheNote(t *testing.T) {
	t.Parallel()
	text := prompt.For(prompt.English)
	opts := api.RequestOptions{Topic: api.RequestBrowserNotes, Image: "/drops/s1/sketch.png", Note: "  before the demo\n", Items: []api.RequestItem{
		{Label: `h1 "Simple pricing"`, Selector: "section.hero > h1", Component: "Hero", Source: "src/Hero.tsx:14", HTML: "<h1>Simple pricing</h1>", URL: "http://localhost:5173/pricing", Text: "tighter,\nit wraps on mobile"},
		{Label: `button "Buy"`, Selector: "button.buy", HTML: "<button class=\"buy\">Buy</button>", URL: "http://localhost:5173/pricing"},
	}}
	want := `The person left 2 notes on the page in your browser tab:
1. h1 "Simple pricing" on http://localhost:5173/pricing
   section.hero > h1
   React: Hero (src/Hero.tsx:14)
   <h1>Simple pricing</h1>
   Note: tighter, it wraps on mobile
2. button "Buy" on http://localhost:5173/pricing
   button.buy
   <button class="buy">Buy</button>
The person also drew on the page. The screenshot with their drawing: /drops/s1/sketch.png
Each selector above works as the selector of browser_click and browser_type.

before the demo`
	if got := notesText(text, opts); got != want {
		t.Fatalf("notes:\n%s\nwant:\n%s", got, want)
	}
	alone := api.RequestOptions{Topic: api.RequestBrowserNotes, Image: "/drops/s1/sketch.png", URL: "http://localhost:5173/pricing"}
	if got, want := notesText(text, alone), "The person drew on the page in your browser tab, http://localhost:5173/pricing. The screenshot with their drawing: /drops/s1/sketch.png"; got != want {
		t.Errorf("sketch alone = %q, want %q", got, want)
	}
}

// The batch reads as a markdown list, each comment anchored where it was
// written, a multi-line one kept one item, the closing note last.
func TestCommentsTextIsAList(t *testing.T) {
	t.Parallel()
	opts := api.RequestOptions{Topic: api.RequestReviewComments, Note: "  ship after  ", Items: []api.RequestItem{
		{Path: "internal/a.go", Lines: "12-30", Text: "split this\nin two"}, {Text: "and test it"}}}
	want := "Review comments:\n\n- internal/a.go:12-30: split this\n  in two\n- and test it\n- ship after"
	if got := commentsText(prompt.For(prompt.English), opts); got != want {
		t.Fatalf("comments =\n%q\nwant\n%q", got, want)
	}
}

// Threads read as each one where it is, its id, its link and what was said in
// it, a deleted author as ghost, then the ask in the form of their number.
func TestThreadsTextNamesWhereAndWhatWasSaid(t *testing.T) {
	t.Parallel()
	opts := api.RequestOptions{Topic: api.RequestReviewThread, PR: 933, Branch: "fix/kiro-paste", Items: []api.RequestItem{
		{Path: "internal/relay/submit.go", Lines: "41-58", ID: "PRRT_1", URL: "https://github.com/o/r/pull/933#discussion_r2201",
			Comments: []api.RequestComment{{Author: "reviewer", Body: "Wait for the echo?\nOnly on Kiro."}, {Body: "Agreed."}}},
		{Path: "pulls.go", Lines: "112", ID: "PRRT_2", Outdated: true, URL: "https://github.com/o/r/pull/933#discussion_r3301",
			Comments: []api.RequestComment{{Author: "reviewer", Body: "Duplicated rule."}}},
	}}
	want := "Review thread on pull request #933 (fix/kiro-paste), internal/relay/submit.go:41-58:\n" +
		"Thread id: PRRT_1\n" +
		"https://github.com/o/r/pull/933#discussion_r2201\n\n" +
		"- @reviewer: Wait for the echo?\n  Only on Kiro.\n" +
		"- @ghost: Agreed.\n\n" +
		"Review thread on pull request #933 (fix/kiro-paste), pulls.go:112 (outdated, check the current code):\n" +
		"Thread id: PRRT_2\n" +
		"https://github.com/o/r/pull/933#discussion_r3301\n\n" +
		"- @reviewer: Duplicated rule.\n\n" +
		"Go through each thread. One that asks for a change: make it, commit it and call mark_thread_fixed with that thread's id and the commit, " +
		"and lich resolves the thread and replies on GitHub once that commit is pushed. One that asks a question or raises a point for the team: " +
		"change nothing for it, and bring it to me with your view. Never reply to or resolve a thread on GitHub yourself."
	text := prompt.For(prompt.English)
	if got := threadsText(text, opts); got != want {
		t.Errorf("threads =\n%s\nwant\n%s", got, want)
	}
	opts.Items = opts.Items[1:]
	one := threadsText(text, opts)
	if tail := "Never reply to or resolve the thread on GitHub yourself."; !strings.HasSuffix(one, tail) {
		t.Errorf("one thread's ask = %q", one)
	}
}

// A checks request names the failing checks with their runs, counts the rest,
// counts them all when none is named, and says merge request on GitLab.
func TestChecksAndConflictsText(t *testing.T) {
	t.Parallel()
	text := prompt.For(prompt.English)
	unnamed := api.RequestOptions{Topic: api.RequestChecks, PR: 7, Branch: "feat/x", Failed: 1}
	if got := checksText(text, unnamed, nil); !strings.Contains(got, "1 check is red") {
		t.Errorf("unnamed = %q", got)
	}
	gitlab := api.RequestOptions{Topic: api.RequestConflicts, Forge: api.ForgeGitLab, PR: 7, Branch: "feat/x", Base: "main"}
	if got, want := conflictsText(text, gitlab), "Merge request !7 (feat/x) has merge conflicts with main. Resolve them."; got != want {
		t.Errorf("GitLab conflicts = %q, want %q", got, want)
	}
	many := requestCases()["request: checks past the ones named"]
	env, _ := requestEnvelope(prompt.English, many)
	if !strings.HasSuffix(env.Body[1].Text, "…and 1 more failing check not listed.") || len(env.Items) != namedChecks {
		t.Errorf("past the named checks: %+v", env)
	}
}
