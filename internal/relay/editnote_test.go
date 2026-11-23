package relay

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/store"
)

// touchedFixtures is the session-touched contract as bytes
// (docs/hooks/fixtures/README.md, lich's decisions).
const touchedFixtures = "../../docs/hooks/fixtures/session-touched.jsonl"

type touchedCase struct {
	Name    string            `json:"name"`
	Body    json.RawMessage   `json:"body"`
	Accept  map[string]any    `json:"accept"`
	Answer  json.RawMessage   `json:"answer"`
	Before  []json.RawMessage `json:"before"`
	Minutes int               `json:"minutes"`
}

// toolsBut answers that every provider's sessions have lich's tools but one.
type toolsBut string

func (toolsBut) Installed(string) bool       { return true }
func (t toolsBut) HasTools(kind string) bool { return kind != string(t) }

// touchedRelay is the workspace the fixtures README names: one checkout, s1
// "Relay fixes", s2 "Docs sweep" (Codex, without lich's tools) and s3 "Auth
// fix" running, s4 "Old branch" closed, on a clock that moves only when told.
func touchedRelay(now *time.Time) *Service {
	sessions := fakeSessions{projects: []store.Project{{Path: "/repo", Sessions: []store.Session{
		{ID: "s1", Label: "Relay fixes", Kind: "claude"},
		{ID: "s2", Label: "Docs sweep", Kind: "codex"},
		{ID: "s3", Label: "Auth fix", Kind: "opencode"},
		{ID: "s4", Label: "Old branch", Kind: "claude"},
	}}}}
	svc := newRelay(sessions, newFakeTerminal("s1", "s2", "s3"), &fakeEvents{})
	svc.SetPlugins(toolsBut("codex"))
	svc.now = func() time.Time { return *now }
	return svc
}

func loadTouchedCases(t *testing.T) []touchedCase {
	t.Helper()
	f, err := os.Open(touchedFixtures)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()
	var cases []touchedCase
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var c touchedCase
		if err := json.Unmarshal(scanner.Bytes(), &c); err != nil {
			t.Fatalf("decode a case: %v", err)
		}
		if c.Accept != nil {
			cases = append(cases, c)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return cases
}

func touch(t *testing.T, svc *Service, body json.RawMessage) string {
	t.Helper()
	var req api.TouchedRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("decode body %s: %v", body, err)
	}
	return svc.Touched(req)
}

// TestTouchedFixtureAnswers replays every accepted case: its before requests,
// the clock moved by its minutes, then its body, whose note must be its answer
// whole, and nothing (204) when it names none.
func TestTouchedFixtureAnswers(t *testing.T) {
	for _, c := range loadTouchedCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			now := time.Unix(1_000_000, 0)
			svc := touchedRelay(&now)
			for _, before := range c.Before {
				touch(t, svc, before)
			}
			now = now.Add(time.Duration(c.Minutes) * time.Minute)
			note := touch(t, svc, c.Body)
			if c.Answer == nil {
				if note != "" {
					t.Fatalf("answered %q, want nothing", note)
				}
				return
			}
			var want api.TouchedAnswer
			if err := json.Unmarshal(c.Answer, &want); err != nil {
				t.Fatalf("decode answer: %v", err)
			}
			if note != want.Note {
				got, _ := json.Marshal(api.TouchedAnswer{Note: note})
				t.Fatalf("answer\n got %s\nwant %s", got, c.Answer)
			}
		})
	}
}

// The note is in the prompt language, its tag and attributes in English.
func TestEditNoteSpeaksThePromptLanguage(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	svc := touchedRelay(&now)
	svc.SetPromptLanguage(func() prompt.Lang { return prompt.PortugueseBR })
	svc.Touched(api.TouchedRequest{SessionID: "s2", Paths: []string{"/repo/a.go"}})
	now = now.Add(3 * time.Minute)
	note := svc.Touched(api.TouchedRequest{SessionID: "s1", Paths: []string{"/repo/a.go"}})
	want := `<lich-notification version="1" kind="notice" topic="edit-overlap" from="Docs sweep" session="s2">`
	if !strings.HasPrefix(note, want) || !strings.Contains(note, "há 3 minutos") {
		t.Fatalf("note = %q", note)
	}
}

// Expired edits leave memory with the next touch, whoever makes it.
func TestEditsForgetWhatLeftTheWindow(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	svc := touchedRelay(&now)
	svc.Touched(api.TouchedRequest{SessionID: "s2", Paths: []string{"/repo/a.go"}})
	svc.Touched(api.TouchedRequest{SessionID: "s1", Paths: []string{"/repo/a.go"}, Hold: true})
	now = now.Add(editWindow + time.Second)
	svc.Touched(api.TouchedRequest{SessionID: "s3", Paths: []string{"/repo/z.go"}})
	if len(svc.edits.last) != 1 || len(svc.edits.held) != 0 {
		t.Fatalf("last = %v, held = %v; want only z.go", svc.edits.last, svc.edits.held)
	}
}
