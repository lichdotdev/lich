package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

const subagentBody = `{"session_id":"s1","tool":"Agent","tool_use_id":"toolu_1","tasks":[{"prompt":"Fix #412."}]}`

func subagentTransport(t *testing.T) *transport {
	t.Helper()
	tr, err := newTransport(func(string, []byte) {}, func(hookRequest) {},
		func(string, string, string) error { return nil }, func(string, string) error { return nil }, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tr.close() })
	return tr
}

// The decision is the 200 body, and a deny is committed as written once it
// was.
func TestSubagentAnswersTheDecisionAndCommitsIt(t *testing.T) {
	tr := subagentTransport(t)
	committed := make(chan bool, 1)
	tr.decideSubagent = func(_ context.Context, req api.SubagentRequest) (api.SubagentAnswer, func(bool)) {
		if req.Call != api.SubagentSpawn || req.Tasks[0].Prompt != "Fix #412." {
			t.Errorf("decided on %+v", req)
		}
		return api.SubagentAnswer{Decision: api.SubagentDeny, Reason: "opened"}, func(w bool) { committed <- w }
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/subagent?token=%s", tr.port, tr.token)
	resp, err := http.Post(url, "application/json", strings.NewReader(subagentBody))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var got api.SubagentAnswer
	raw, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(raw, &got); err != nil || resp.StatusCode != http.StatusOK || got.Decision != api.SubagentDeny {
		t.Fatalf("status %d body %s", resp.StatusCode, raw)
	}
	if !<-committed {
		t.Error("a written deny was committed as unwritten")
	}
}

// A request that ended before its answer was written commits the deny as
// unwritten, which closes what it opened.
func TestSubagentCommitsUnwrittenWhenTheClientLeft(t *testing.T) {
	tr := subagentTransport(t)
	ctx, cancel := context.WithCancel(context.Background())
	var written *bool
	tr.decideSubagent = func(context.Context, api.SubagentRequest) (api.SubagentAnswer, func(bool)) {
		cancel()
		return api.SubagentAnswer{Decision: api.SubagentDeny}, func(w bool) { written = &w }
	}
	req := httptest.NewRequest(http.MethodPost, "/subagent?token="+tr.token, strings.NewReader(subagentBody)).WithContext(ctx)
	tr.subagentHook(httptest.NewRecorder(), req)
	if written == nil || *written {
		t.Fatalf("written = %v, want committed as unwritten", written)
	}
}

// With nothing wired every call runs natively.
func TestSubagentWithNoDeciderAllows(t *testing.T) {
	tr := subagentTransport(t)
	rec := httptest.NewRecorder()
	tr.subagentHook(rec, httptest.NewRequest(http.MethodPost, "/subagent?token="+tr.token, strings.NewReader(subagentBody)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"decision":"allow"`) {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	tr.subagentHook(rec, httptest.NewRequest(http.MethodGet, "/subagent?token="+tr.token, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET = %d, want 405", rec.Code)
	}
}

// A mod answers a worker's errand from the release that answers until the one
// whose /subagent hook retired that answer; past it lich reads the turn's end.
func TestModAnswersUntilTheHookRelease(t *testing.T) {
	for version, want := range map[string]bool{"0.16.9": false, "0.17.0": true, "0.21.5": true, "0.22.0": false, "": false} {
		if got := modAnswersRelease(version); got != want {
			t.Errorf("modAnswersRelease(%q) = %v, want %v", version, got, want)
		}
	}
}
