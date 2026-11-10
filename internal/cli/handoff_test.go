package cli

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/relay"
)

const successorBody = `{"id":"9f8e","projectId":"p1","project":"lich","label":"Session 5",
	"name":"lich-9f8e","kind":"codex","path":"/wt/auth-fix","nextSeq":6,"originLabel":"auth-fix"}`

func successorLich(t *testing.T) *fakeLich {
	t.Helper()
	f := newFakeLich(t, successorBody)
	f.answers = map[string]answer{
		"terminal.Ready": {status: 200, body: `true`},
		"relay.Submit":   {status: 200, body: `{}`},
	}
	return f
}

// The successor is opened for the caller and its note is submitted at its
// prompt with no ticket: nothing is owed back to a session done with the work.
func TestMCPHandoffOpensTheSuccessorAndSubmitsTheNote(t *testing.T) {
	t.Parallel()
	f := successorLich(t)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"handoff_session","arguments":{"done":"parser ported","next_steps":"wire the CLI",
		"branch":"feat/parser","commits":"abc123 port the parser","gotchas":"the golden moves",
		"kind":"codex","model":"gpt-5.5","effort":"high"}}}`)

	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	if len(f.calls) != 3 || f.calls[0].method != "spawn.Handoff" || f.calls[1].method != "terminal.Ready" || f.calls[2].method != "relay.Submit" {
		t.Fatalf("calls = %+v, want spawn.Handoff, terminal.Ready, relay.Submit", f.calls)
	}
	want := api.HandoffSessionOptions{From: "s1", Kind: "codex", Model: "gpt-5.5", Effort: "high"}
	if got := optionsOf[api.HandoffSessionOptions](t, f.calls[0]); got != want {
		t.Errorf("options = %+v, want %+v", got, want)
	}
	submitted := optionsOf[api.SubmitOptions](t, f.calls[2])
	if submitted.SessionID != "9f8e" {
		t.Errorf("submitted to %q, want the successor", submitted.SessionID)
	}
	for _, phrase := range []string{"[lich]", `"auth-fix"`, "parser ported", "wire the CLI", "feat/parser", "abc123 port the parser", "the golden moves"} {
		if !strings.Contains(submitted.Message, phrase) {
			t.Errorf("note is missing %q:\n%s", phrase, submitted.Message)
		}
	}
	for _, phrase := range []string{`"Session 5"`, `"lich-9f8e"`, "stays open"} {
		if !strings.Contains(text, phrase) {
			t.Errorf("result is missing %q:\n%s", phrase, text)
		}
	}
}

func TestMCPHandoffNamesTheFieldsLeftOut(t *testing.T) {
	t.Parallel()
	f := successorLich(t)

	speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"handoff_session","arguments":{"done":"parser ported","next_steps":"wire the CLI"}}}`)

	if got := optionsOf[api.HandoffSessionOptions](t, f.calls[0]); got != (api.HandoffSessionOptions{From: "s1"}) {
		t.Errorf("options = %+v, want the caller's own for everything", got)
	}
	message := optionsOf[api.SubmitOptions](t, f.calls[2]).Message
	if strings.Count(message, "(none)") != 3 {
		t.Errorf("want branch, commits and gotchas marked as none:\n%s", message)
	}
}

// Refused before anything opens: a successor with nothing to start from is a
// card nobody asked for.
func TestMCPHandoffRefusesANoteWithoutDoneOrNextSteps(t *testing.T) {
	t.Parallel()
	for _, args := range []string{`{"done":"parser ported"}`, `{"next_steps":"wire the CLI"}`, `{"done":" ","next_steps":"x"}`} {
		f := successorLich(t)
		replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
			{"name":"handoff_session","arguments":`+args+`}}`)
		if text, failed := textOf(t, replies[0]); !failed || !strings.Contains(text, "next steps") {
			t.Errorf("%s: failed %v, text %q; want the note refused", args, failed, text)
		}
		if len(f.calls) != 0 {
			t.Errorf("%s: calls = %+v, want none", args, f.calls)
		}
	}
}

// The note is typed at a prompt, so it holds to send_to_session's limit: exactly
// the limit passes, one byte more is refused, counted across all five fields.
func TestHandoffNoteHoldsToThePromptLimit(t *testing.T) {
	t.Parallel()
	fill := relay.PromptLimit - len("parser ported") - len("wire the CLI")
	at := handoffNote{done: "parser ported", nextSteps: "wire the CLI", gotchas: strings.Repeat("x", fill)}
	if err := at.check(); err != nil {
		t.Errorf("a note at the limit: %v", err)
	}
	over := at
	over.branch = "b"
	if err := over.check(); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("a note one byte over: %v, want it refused", err)
	}
}

func TestMCPHandoffOpensNothingForAnOversizedNote(t *testing.T) {
	t.Parallel()
	f := successorLich(t)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"handoff_session","arguments":{"done":"`+strings.Repeat("x", relay.PromptLimit+1)+`","next_steps":"y"}}}`)

	if text, failed := textOf(t, replies[0]); !failed || !strings.Contains(text, "limit") {
		t.Errorf("failed %v, text %q; want the note refused", failed, text)
	}
	if len(f.calls) != 0 {
		t.Errorf("calls = %+v, want nothing opened", f.calls)
	}
}

// The successor exists whether or not the note landed, so the answer says
// both rather than reporting a failed call.
func TestMCPHandoffKeepsTheSuccessorWhenTheNoteDoesNotLand(t *testing.T) {
	t.Parallel()
	f := successorLich(t)
	f.answers["relay.Submit"] = answer{status: 500, body: `{"error":"\"Session 5\" is not running"}`}

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"handoff_session","arguments":{"done":"parser ported","next_steps":"wire the CLI"}}}`)

	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("the handoff was reported as a failure, but the session exists: %s", text)
	}
	for _, phrase := range []string{`"Session 5"`, "is not running", "send_to_session"} {
		if !strings.Contains(text, phrase) {
			t.Errorf("result is missing %q:\n%s", phrase, text)
		}
	}
}

func TestMCPHandoffSubmitsNothingWhenThePromptCannotBeRead(t *testing.T) {
	t.Parallel()
	f := successorLich(t)
	f.answers["terminal.Ready"] = answer{status: 500, body: `{"error":"no session \"9f8e\""}`}

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"handoff_session","arguments":{"done":"parser ported","next_steps":"wire the CLI"}}}`)

	text, _ := textOf(t, replies[0])
	if !strings.Contains(text, "no session") {
		t.Errorf("result is missing the readiness failure:\n%s", text)
	}
	for _, call := range f.calls {
		if call.method == "relay.Submit" {
			t.Fatalf("submitted before the prompt was up: %+v", f.calls)
		}
	}
}

func TestHandoffCommandPostsTheFlagsItWasGiven(t *testing.T) {
	t.Parallel()
	f := successorLich(t)

	code, stdout, stderr := run(t, f, "handoff", "--done", "parser ported", "--next", "wire the CLI",
		"--gotchas", "the golden moves", "--kind", "claude", "--model", "opus", "--effort", "max")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	want := api.HandoffSessionOptions{From: "s1", Kind: "claude", Model: "opus", Effort: "max"}
	if got := optionsOf[api.HandoffSessionOptions](t, f.calls[0]); got != want {
		t.Errorf("options = %+v, want %+v", got, want)
	}
	if message := optionsOf[api.SubmitOptions](t, f.calls[2]).Message; !strings.Contains(message, "the golden moves") {
		t.Errorf("note lost the gotchas:\n%s", message)
	}
	if !strings.Contains(stdout, `"Session 5"`) || !strings.Contains(stdout, "stays open") {
		t.Errorf("stdout = %q, want the successor and that this session stays open", stdout)
	}
}

func TestHandoffCommandFailsWhenTheNoteDoesNotLand(t *testing.T) {
	t.Parallel()
	f := successorLich(t)
	f.answers["relay.Submit"] = answer{status: 500, body: `{"error":"boom"}`}

	code, stdout, stderr := run(t, f, "handoff", "--done", "parser ported", "--next", "wire the CLI")
	if code == 0 {
		t.Fatalf("exit 0 for a note that never landed; stdout = %q", stdout)
	}
	if !strings.Contains(stdout, `"Session 5"`) || !strings.Contains(stderr, "boom") {
		t.Errorf("stdout = %q, stderr = %q; want the open session and the failure", stdout, stderr)
	}
}

func TestHandoffNoteFormatsInEveryLanguage(t *testing.T) {
	t.Parallel()
	note := handoffNote{done: "parser ported", nextSteps: "wire the CLI"}
	for _, lang := range prompt.Langs {
		text := note.text(lang, "auth-fix")
		if strings.Contains(text, "%!") || !strings.HasPrefix(text, "[lich]") {
			t.Errorf("%s: the note is malformed:\n%s", lang, text)
		}
		for _, field := range []string{`"auth-fix"`, "parser ported", "wire the CLI", "git status"} {
			if !strings.Contains(text, field) {
				t.Errorf("%s: the note lost %q:\n%s", lang, field, text)
			}
		}
	}
}

func TestMCPHandoffPassesTheProfileAsASuggestion(t *testing.T) {
	t.Parallel()
	f := successorLich(t)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"handoff_session","arguments":{"done":"x","next_steps":"y","profile":"work"}}}`)

	if got := optionsOf[api.HandoffSessionOptions](t, f.calls[0]); got.Profile != "work" {
		t.Errorf("options = %+v, want the suggestion passed on", got)
	}
	text, failed := textOf(t, replies[0])
	if failed || !strings.Contains(text, `"work"`) || !strings.Contains(text, "suggestion") {
		t.Errorf("failed %v, text %q; want the suggestion reported as one", failed, text)
	}
}

// A dropped suggestion is reported in the answer, and the handoff is no failure.
func TestMCPHandoffSaysASuggestionWasDropped(t *testing.T) {
	t.Parallel()
	f := newFakeLich(t, `{"id":"9f8e","projectId":"p1","project":"lich","label":"Session 5",
		"name":"lich-9f8e","kind":"crush","path":"/wt","nextSeq":6,"originLabel":"auth-fix",
		"profileDropped":"crush has no profiles"}`)
	f.answers = map[string]answer{"terminal.Ready": {status: 200, body: `true`}, "relay.Submit": {status: 200, body: `{}`}}

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"handoff_session","arguments":{"done":"x","next_steps":"y","profile":"work"}}}`)

	text, failed := textOf(t, replies[0])
	if failed || !strings.Contains(text, "crush has no profiles") || !strings.Contains(text, `"Session 5"`) {
		t.Errorf("failed %v, text %q; want the open session and the reason", failed, text)
	}
}

func TestHandoffCommandSuggestsTheProfileItIsGiven(t *testing.T) {
	t.Parallel()
	f := successorLich(t)

	code, stdout, stderr := run(t, f, "handoff", "--done", "x", "--next", "y", "--profile", "work")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if got := optionsOf[api.HandoffSessionOptions](t, f.calls[0]); got.Profile != "work" {
		t.Errorf("options = %+v, want the suggestion", got)
	}
	if !strings.Contains(stdout, `"work"`) {
		t.Errorf("stdout = %q, want the suggestion reported", stdout)
	}
}
