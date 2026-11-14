package relay

import (
	"context"
	"fmt"
	"strings"
	"testing"

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

// Every message renders in every language with its arguments in place: a
// format that drops or misnumbers one shows as %!, which an agent would read as
// part of the instruction.
func TestEveryMessageRendersInEveryLanguage(t *testing.T) {
	t.Parallel()
	for _, lang := range prompt.Langs {
		messages := []string{
			compose(lang, "", "t1", "task", false),
			compose(lang, "sender", "t1", "task", true),
			composeForWorker(lang, "sender", "task"),
			pickTicketNudge(lang, 2, "  t1, t2"),
			nudgeNotice(lang, 1, []string{"docs"}, false),
			nudgeNotice(lang, 3, []string{"docs", "api"}, true),
		}
		for _, message := range messages {
			if strings.Contains(message, "%!") {
				t.Errorf("%s: a format argument is off:\n%s", lang, message)
			}
			if !strings.HasPrefix(message, "[lich] ") {
				t.Errorf("%s: the [lich] token is missing:\n%s", lang, message)
			}
		}
		for _, literal := range []string{`"$LICH_BIN" reply t1`, "`" + ToolReply + "`", "t1"} {
			if !strings.Contains(compose(lang, "", "t1", "task", true), literal) {
				t.Errorf("%s: the reply instruction lost %q", lang, literal)
			}
		}
	}
}

// The language is read when a message is composed, not when the relay starts,
// so a change in Settings reaches the next message.
func TestTheRelayTypesInThePromptLanguageReadLive(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s2")
	svc := newRelay(workspace(), term, nil)
	lang := prompt.English
	svc.SetPromptLanguage(func() prompt.Lang { return lang })
	lang = prompt.PortugueseBR

	go func() { _, _ = svc.Reply(t.Context(), ReplyOptions{Ticket: waitForTicket(svc), Answer: "ok"}) }()
	if _, err := svc.Send(context.Background(), SendOptions{Target: "docs", Prompt: "hello", WaitSeconds: 30}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	typed := term.written("s2")
	if !strings.Contains(typed, prompt.For(prompt.PortugueseBR).OriginCLI) {
		t.Errorf("the message was not typed in the language set after startup:\n%s", typed)
	}
}

// Russian's one form also covers 21, so the nudge prints the count in every
// form and the noun agrees with it.
func TestTheRussianNudgeAgreesWithItsCount(t *testing.T) {
	t.Parallel()
	for count, want := range map[int]string{
		1: "1 отправленная вами задача готова", 2: "2 отправленные вами задачи готовы",
		5: "5 отправленных вами задач готовы", 11: "11 отправленных вами задач готовы",
		21: "21 отправленная вами задача готова", 22: "22 отправленные вами задачи готовы",
	} {
		if got := nudgeNotice(prompt.Russian, count, []string{"docs"}, false); !strings.Contains(got, want) {
			t.Errorf("count %d: %q, want it to contain %q", count, got, want)
		}
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

func TestAnExtensionsMessageIsIntroducedAsRelayedByIt(t *testing.T) {
	t.Parallel()
	for _, lang := range prompt.Langs {
		term := newFakeTerminal("s2")
		svc := newRelay(workspace(), term, nil)
		svc.SetPromptLanguage(func() prompt.Lang { return lang })

		go func() { _, _ = svc.Reply(t.Context(), ReplyOptions{Ticket: waitForTicket(svc), Answer: "ok"}) }()
		if _, err := svc.Send(context.Background(), SendOptions{
			Target: "docs", Prompt: "hello", WaitSeconds: 30, ExtensionID: "notes", ExtensionName: "Lich Notes",
		}); err != nil {
			t.Fatalf("%s: Send: %v", lang, err)
		}

		typed := term.written("s2")
		if !strings.Contains(typed, `"Lich Notes"`) || !strings.Contains(typed, "notes") || strings.Contains(typed, prompt.For(lang).OriginCLI) {
			t.Errorf("%s: the message does not name the extension in place of the command line:\n%s", lang, typed)
		}
	}
}

func TestAnExtensionsMessageInEnglishReadsAsRelayedByTheExtension(t *testing.T) {
	t.Parallel()
	got := originExtension(prompt.English, "notes", "Lich Notes")
	if want := `Message relayed by the extension "Lich Notes" (notes)`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
