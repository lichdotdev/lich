package subagent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/relay"
)

// fakeWorld is the workspace the fixture's answers are written against: the
// asker s1 at a depth and provider, and the sessions a deny opens, named and
// ticketed in order.
type fakeWorld struct {
	mu     sync.Mutex
	asker  Asker
	opened []api.OpenSubagentOptions
	closed []api.CloseSessionOptions
	sent   []relay.SendOptions
	failAt int
	// answered is the tickets whose errand no longer waits on its worker.
	answered map[string]bool
}

var fixtureSessions = []api.SubagentOpened{
	{ID: "w1", Label: "Fix #412", Ticket: "4b9bc94e"},
	{ID: "w2", Label: "Docs sweep", Ticket: "7c1d2e3f"},
}

func (w *fakeWorld) Asker(id string) (Asker, bool) { return w.asker, id == "s1" }

func (w *fakeWorld) ErrandOpen(ticket string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return !w.answered[ticket]
}

func (w *fakeWorld) OpenSubagentNow(_ context.Context, opts api.OpenSubagentOptions) (api.OpenedSession, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := len(w.opened)
	w.opened = append(w.opened, opts)
	if w.failAt > 0 && n+1 == w.failAt {
		return api.OpenedSession{}, fmt.Errorf("refused")
	}
	return api.OpenedSession{ID: fixtureSessions[n].ID, Label: fixtureSessions[n].Label}, nil
}

func (w *fakeWorld) Close(_ context.Context, opts api.CloseSessionOptions) (api.ClosedSession, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = append(w.closed, opts)
	return api.ClosedSession{ID: opts.Target}, nil
}

func (w *fakeWorld) HandSubagent(_ context.Context, opts relay.SendOptions) (relay.Result, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.sent = append(w.sent, opts)
	for _, s := range fixtureSessions {
		if s.ID == opts.Target {
			return relay.Result{Ticket: s.Ticket, Status: relay.StatusPending}, nil
		}
	}
	return relay.Result{}, fmt.Errorf("no session %q", opts.Target)
}

func newFake(asker Asker) (*Service, *fakeWorld, *[]api.SubagentVeto) {
	w := &fakeWorld{asker: asker}
	var vetoes []api.SubagentVeto
	svc := New(w, w, w, func() prompt.Lang { return prompt.English }, func(v api.SubagentVeto) { vetoes = append(vetoes, v) })
	return svc, w, &vetoes
}

type fixtureCase struct {
	Name   string            `json:"name"`
	Body   json.RawMessage   `json:"body"`
	Answer json.RawMessage   `json:"answer"`
	Before []json.RawMessage `json:"before"`
	Depth  int               `json:"depth"`
	Kind   string            `json:"kind"`
}

func loadFixture(t *testing.T) []fixtureCase {
	t.Helper()
	f, err := os.Open("../../docs/hooks/fixtures/subagent-cards.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var cases []fixtureCase
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			var c fixtureCase
			if err := json.Unmarshal([]byte(line), &c); err != nil {
				t.Fatal(err)
			}
			cases = append(cases, c)
		}
	}
	return cases
}

// decode reads a fixture body the way the endpoint does: call defaults to
// spawn and isolation is trimmed.
func decode(t *testing.T, raw json.RawMessage) api.SubagentRequest {
	t.Helper()
	var req api.SubagentRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	if req.Call == "" {
		req.Call = api.SubagentSpawn
	}
	req.Isolation = strings.TrimSpace(req.Isolation)
	return req
}

// Every answered case of the contract's fixture is lich's answer, byte for
// byte once decoded.
func TestDecisionsMatchTheFixture(t *testing.T) {
	for _, c := range loadFixture(t) {
		if c.Answer == nil {
			continue
		}
		t.Run(c.Name, func(t *testing.T) {
			kind := c.Kind
			if kind == "" {
				kind = "claude"
			}
			svc, _, _ := newFake(Asker{Kind: kind, Label: "Relay fixes", Depth: c.Depth})
			for _, before := range c.Before {
				_, commit := svc.Decide(context.Background(), decode(t, before))
				commit(true)
			}
			got, commit := svc.Decide(context.Background(), decode(t, c.Body))
			commit(true)
			var want api.SubagentAnswer
			if err := json.Unmarshal(c.Answer, &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("answer = %+v\nwant %+v", got, want)
			}
		})
	}
}
