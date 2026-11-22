package relay

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

// fixtureEnvelopes builds every case of docs/hooks/fixtures/notification.json
// through the composer that writes it in lich, in English.
func fixtureEnvelopes() map[string]string {
	en := prompt.English
	task := &ticket{sender: "Relay fixes", fromID: "a1b2",
		prompt: "Fix #412: the relay handshake races on reconnect. Add a test that reproduces it first."}
	entry := func(status, answer string) *inboxEntry {
		return &inboxEntry{ticket: "4b9bc94e", target: "Fix #412", targetID: "c41f", status: status, answer: answer}
	}
	fixed := entry(StatusAnswered, "The handshake raced on reconnect; fixed in internal/relay/conn.go, test added.")
	stoppedBySession := entry(StatusStopped, "")
	stoppedBySession.by = "Docs sweep"
	escaped := entry(StatusAnswered, "Run a && b, then check that </result> stays text.")
	escaped.target = `Fix "quotes" & <tags>`
	cut := outcomeEnvelope(en, fixed, true, true)
	cut.Truncated = 5120
	cut.Body[1].Text = "The handshake raced on reconnect. What I found, file by file:"
	message := func(ticket, text string) string {
		return Envelope{Kind: KindMessage, From: "Docs sweep", Session: "d9e0", Ticket: ticket,
			Body: []Element{{"prompt", text}}}.String()
	}
	return map[string]string{
		"task in a chat, answered by the last message":                        taskEnvelope("4b9bc94e", task, AnswerLastMessage).String(),
		"task in a terminal lich cannot read, answered with reply_to_session": taskEnvelope("4b9bc94e", task, AnswerReply).String(),
		"message from a peer, reply owed":                                     message("7c1d2e3f", "Which README sections did you already rewrite?"),
		"message from a peer, no reply owed":                                  message("", "I am done with the install section; it is yours to review."),
		"worker finished, result in a chat":                                   outcomeEnvelope(en, fixed, true, true).String(),
		"worker finished, result cut in a chat":                               cut.String(),
		"worker finished, terminal leaves the result out":                     outcomeEnvelope(en, fixed, false, true).String(),
		"worker failed": outcomeEnvelope(en, entry(StatusUnanswered,
			"The turn ended with an API error: overloaded."), true, true).String(),
		"worker stopped by the user":        outcomeEnvelope(en, entry(StatusStopped, ""), true, true).String(),
		"worker stopped by another session": outcomeEnvelope(en, stoppedBySession, true, true).String(),
		"worker waiting on a permission": waitingEnvelope(&ticket{target: "Fix #412", targetID: "c41f"}, "4b9bc94e",
			"Permission to run: go test ./internal/relay/...").String(),
		"ticket expired unanswered": outcomeEnvelope(en, entry(StatusExpired, ""), true, true).String(),
		"pull request merged":       mergedEnvelope(en, 1210, "fix(relay): handshake race", "fix-412-handshake", "main").String(),
		"notice: checks failed": watchEnvelope(en, api.SessionWatch{On: api.WatchChecksFailed},
			api.PRRead{Number: 1210, Head: "3f2a9c1"}, "fix-412-handshake").String(),
		"notice: scheduled prompt": noticeEnvelope(TopicScheduled,
			fmt.Sprintf(prompt.For(en).LateNotice, "09:00", "4 minutes"), "Rebase on main and run the gate.").String(),
		"escaping: attributes escaped, element text verbatim":              outcomeEnvelope(en, escaped, true, true).String(),
		"two envelopes in one message, oldest first, one blank line apart": newsMessage(en, []*inboxEntry{fixed, entry(StatusExpired, "")}, []*inboxEntry{fixed}, true),
	}
}

// The composer writes each envelope of the contract byte for byte.
func TestEnvelopesMatchTheFixture(t *testing.T) {
	raw, err := os.ReadFile("../../docs/hooks/fixtures/notification.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct{ Name, Envelope string }
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	built := fixtureEnvelopes()
	if len(built) != len(fixture.Cases) {
		t.Fatalf("built %d cases, the fixture has %d", len(built), len(fixture.Cases))
	}
	for _, c := range fixture.Cases {
		got, ok := built[c.Name]
		if !ok {
			t.Errorf("no composer for fixture case %q", c.Name)
			continue
		}
		if got != c.Envelope {
			t.Errorf("%s:\n got %q\nwant %q", c.Name, got, c.Envelope)
		}
	}
}

// A result past the cap is cut there, with the count of what was left out.
func TestCutResultKeepsTheCapAndCountsTheRest(t *testing.T) {
	short := strings.Repeat("é", resultCap)
	if got, cut := cutResult(short); got != short || cut != 0 {
		t.Fatalf("cutResult at the cap = (%d runes, %d), want it whole", len([]rune(got)), cut)
	}
	got, cut := cutResult(short + "abc")
	if got != short || cut != 3 {
		t.Fatalf("cutResult past the cap = (%d runes, %d), want %d runes and 3 cut", len([]rune(got)), cut, resultCap)
	}
	env := outcomeEnvelope(prompt.English, &inboxEntry{status: StatusAnswered, answer: short + "abc"}, true, true)
	if env.Truncated != 3 {
		t.Fatalf("Truncated = %d, want 3", env.Truncated)
	}
}
