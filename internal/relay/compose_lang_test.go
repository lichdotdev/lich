package relay

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

// The briefing is one argv entry that cmd.exe reads on Windows (see
// SpawnBriefing): a translation that brings a double quote or an angle bracket
// in breaks every Windows spawn of a .cmd provider in that language.
func TestSpawnBriefingIsCmdSafeInEveryLanguage(t *testing.T) {
	t.Parallel()
	for _, lang := range prompt.Langs {
		for _, route := range []SubagentRoute{RouteSessions, RouteCards, RouteNative} {
			for _, hasTools := range []bool{true, false} {
				briefing := SpawnBriefing(lang, hasTools, route)
				if strings.ContainsAny(briefing, `"<>`) {
					t.Errorf("%s route %d tools %v carries a cmd.exe metacharacter:\n%s", lang, route, hasTools, briefing)
				}
			}
		}
	}
}

// Every envelope renders in every language with its arguments in place: a
// format that drops or misnumbers one shows as %!, which an agent would read as
// part of the note.
func TestEveryEnvelopeRendersInEveryLanguage(t *testing.T) {
	t.Parallel()
	for _, lang := range prompt.Langs {
		messages := []string{
			pickTicketNudge(lang, 2, "  t1, t2"),
			compactReminder(lang, 1, "  lich reply t1"),
			compactReminder(lang, 3, "  lich reply t1"),
			outcomeEnvelope(lang, &inboxEntry{target: "docs", status: StatusAnswered}, false, true).String(),
			outcomeEnvelope(lang, &inboxEntry{target: "docs", status: StatusAnswered}, false, false).String(),
			outcomeEnvelope(lang, &inboxEntry{target: "docs", status: StatusUnread}, true, true).String(),
			outcomeEnvelope(lang, &inboxEntry{target: "docs", status: StatusUndelivered}, true, true).String(),
			outcomeEnvelope(lang, &inboxEntry{target: "docs", status: StatusUnanswered}, true, true).String(),
			loopEnvelope(lang, &ticket{sender: "api"}).String(),
		}
		for _, message := range messages {
			if strings.Contains(message, "%!") {
				t.Errorf("%s: a format argument is off:\n%s", lang, message)
			}
			if !strings.HasPrefix(message, "<lich-notification version=\"1\" kind=\"") {
				t.Errorf("%s: not an envelope:\n%s", lang, message)
			}
		}
	}
}

// The briefing is the only text every provider reads about the envelope
// (docs/ceilings.md), so it names the block, both ways a task is answered and
// the tools and command that answer and collect, in every language.
func TestSpawnBriefingExplainsTheEnvelopeInEveryLanguage(t *testing.T) {
	t.Parallel()
	for _, lang := range prompt.Langs {
		for _, route := range []SubagentRoute{RouteSessions, RouteCards, RouteNative} {
			for tools, literals := range map[bool][]string{true: {ToolReply, ToolCollect}, false: {"lich reply TICKET ANSWER", "lich wait"}} {
				briefing := SpawnBriefing(lang, tools, route)
				for _, literal := range append(literals, "lich-notification", AnswerLastMessage, AnswerReply) {
					if !strings.Contains(briefing, literal) {
						t.Errorf("%s route %d tools %v: the briefing lost %q:\n%s", lang, route, tools, literal, briefing)
					}
				}
			}
		}
		// Codex reads the briefing inside single quotes.
		if envelope := prompt.For(lang).BriefingEnvelope; strings.Contains(envelope, "'") {
			t.Errorf("%s: the envelope sentence carries a single quote:\n%s", lang, envelope)
		}
	}
}

// The language is read when a note is composed, not when the relay starts, so
// a change in Settings reaches the next one.
func TestTheRelayTypesInThePromptLanguageReadLive(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("agent")
	svc := newRelay(merging(), term, nil)
	lang := prompt.English
	svc.SetPromptLanguage(func() prompt.Lang { return lang })
	lang = prompt.PortugueseBR

	if _, err := svc.AnnounceMerge(t.Context(), api.AnnounceMergeOptions{Checkout: "/wt/feat", Number: 42, Title: "Fix", Branch: "feat/merge", Base: "main"}); err != nil {
		t.Fatalf("AnnounceMerge: %v", err)
	}

	want := fmt.Sprintf(prompt.For(prompt.PortugueseBR).MergeNotice, 42, "Fix", "main")
	if !awaitWritten(term, "agent", want) {
		t.Errorf("the note was not typed in the language set after startup:\n%s", term.written("agent"))
	}
}

// Russian's one form also covers 21, so the reminder takes a form per count
// and prints the count in each.
func TestTheRussianReminderAgreesWithItsCount(t *testing.T) {
	t.Parallel()
	ru := prompt.For(prompt.Russian)
	for count, want := range map[int]string{
		1: ru.CompactReminder.One, 2: ru.CompactReminder.Few, 5: ru.CompactReminder.Many,
		11: ru.CompactReminder.Many, 21: ru.CompactReminder.One, 22: ru.CompactReminder.Few,
	} {
		got := compactReminder(prompt.Russian, count, "  t1")
		if !strings.Contains(got, fmt.Sprintf(want, count, "  t1")) {
			t.Errorf("count %d: %q, want the form %q", count, got, want)
		}
	}
	if ru.CompactReminder.One == ru.CompactReminder.Few || ru.CompactReminder.Few == ru.CompactReminder.Many {
		t.Error("Russian's reminder forms are not told apart")
	}
}

// Every spawn is told where a page for the user goes and how it opens, in the
// harness's own system prompt, where it outranks a hosted artifact; Codex reads
// the briefing inside single quotes, so a translation must not bring one in.
func TestSpawnBriefingSendsPagesToTheArtifactFolder(t *testing.T) {
	t.Parallel()
	for _, lang := range prompt.Langs {
		for _, route := range []SubagentRoute{RouteSessions, RouteCards, RouteNative} {
			for hasTools, opener := range map[bool]string{true: "browser_open", false: "lich browser open PATH"} {
				briefing := SpawnBriefing(lang, hasTools, route)
				if !strings.Contains(briefing, "LICH_ARTIFACT_DIR") || !strings.Contains(briefing, opener) {
					t.Errorf("%s route %d tools %v does not send pages to the artifact folder with %q:\n%s", lang, route, hasTools, opener, briefing)
				}
				artifacts := fmt.Sprintf(prompt.For(lang).BriefingArtifacts, opener)
				if strings.Contains(artifacts, "'") {
					t.Errorf("%s: the artifact sentence carries a single quote:\n%s", lang, artifacts)
				}
			}
		}
	}
}

// An extension's task names the extension and no session: there is none to
// answer to but the ticket.
func TestAnExtensionsTaskIsFromTheExtension(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s2")
	svc := newRelay(workspace(), term, nil)

	go func() { _, _ = svc.Reply(t.Context(), ReplyOptions{Ticket: waitForTicket(svc), Answer: "ok"}) }()
	if _, err := svc.Send(context.Background(), SendOptions{
		Target: "docs", Prompt: "hello", WaitSeconds: 30, ExtensionID: "notes", ExtensionName: "Lich Notes",
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	typed := term.written("s2")
	if !strings.Contains(typed, `kind="task" from="Lich Notes" ticket="`) || strings.Contains(typed, "session=") {
		t.Errorf("the task does not name the extension alone:\n%s", typed)
	}
}
