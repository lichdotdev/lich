package terminal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/internal/scope"
)

// scopedTransport is a transport with one confined session's token minted,
// and a recording RPC mounted the way main.go mounts the dispatcher.
func scopedTransport(t *testing.T) (*transport, string, *[]string) {
	t.Helper()
	tr, err := newTransport(func(string, []byte) {}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	token, err := tr.mintScoped("mine")
	if err != nil {
		t.Fatalf("mintScoped: %v", err)
	}
	var served []string
	tr.mount("/rpc/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		served = append(served, r.URL.Path+" as "+scope.Session(r.Context())+" "+string(body))
		w.WriteHeader(http.StatusOK)
	}))
	tr.mount("/events", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = append(served, r.URL.Path)
	}))
	return tr, token, &served
}

func post(t *testing.T, tr *transport, path, token, body string) (int, string) {
	t.Helper()
	url := fmt.Sprintf("http://127.0.0.1:%d%s?token=%s", tr.port, path, token)
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post %s: %v", path, err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(payload)
}

func TestScopedTokenRefusedOnDeniedMethods(t *testing.T) {
	t.Parallel()
	tr, token, served := scopedTransport(t)
	for _, method := range []string{"services.Start", "store.SetSetting", "terminal.Write"} {
		status, body := post(t, tr, "/rpc/"+method, token, `[{"id":"svc"}]`)
		if status != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403", method, status)
		}
		var envelope struct{ Error string }
		if err := json.Unmarshal([]byte(body), &envelope); err != nil || !strings.Contains(envelope.Error, method) {
			t.Errorf("%s: refusal is not the RPC envelope naming the method: %s", method, body)
		}
	}
	if len(*served) != 0 {
		t.Errorf("denied calls reached the RPC: %v", *served)
	}
}

func TestScopedTokenReachesAllowedMethodsAsItsSession(t *testing.T) {
	t.Parallel()
	tr, token, served := scopedTransport(t)
	for _, call := range []struct{ method, body string }{
		{"relay.Send", `[{"from":"mine","target":"peer","prompt":"hi"}]`},
		{"relay.Wait", `[{"ticket":"t1"}]`},
		{"spawn.Open", `[{"from":"mine","project":"p"}]`},
		{"browser.Open", `[{"id":"mine","url":"https://example.com"}]`},
	} {
		if status, body := post(t, tr, "/rpc/"+call.method, token, call.body); status != http.StatusOK {
			t.Errorf("%s: status = %d, body %s", call.method, status, body)
		}
	}
	for _, line := range *served {
		if !strings.Contains(line, " as mine ") {
			t.Errorf("the RPC did not learn the caller: %s", line)
		}
	}
	// The body reaches the RPC whole after the policy read it.
	if !slices.ContainsFunc(*served, func(s string) bool { return strings.HasSuffix(s, `[{"ticket":"t1"}]`) }) {
		t.Errorf("relay.Wait's body was consumed: %v", *served)
	}
}

func TestScopedTokenCannotSpeakForAnotherSession(t *testing.T) {
	t.Parallel()
	tr, token, served := scopedTransport(t)
	for _, call := range []struct{ method, body string }{
		{"relay.Send", `[{"from":"other","target":"peer","prompt":"hi"}]`},
		{"spawn.OpenSubagent", `[{"from":"other"}]`},
		{"browser.Open", `[{"id":"other","url":"https://example.com"}]`},
		// No caller at all is not the caller either.
		{"relay.Peers", `[]`},
	} {
		status, body := post(t, tr, "/rpc/"+call.method, token, call.body)
		if status != http.StatusForbidden || !strings.Contains(body, `\"mine\"`) {
			t.Errorf("%s with %s: status = %d, body %s", call.method, call.body, status, body)
		}
	}
	if len(*served) != 0 {
		t.Errorf("impersonations reached the RPC: %v", *served)
	}
}

func TestScopedTokenReachesNoOtherSurface(t *testing.T) {
	t.Parallel()
	tr, token, served := scopedTransport(t)
	if status, _ := post(t, tr, "/events", token, ""); status != http.StatusForbidden {
		t.Errorf("/events: status = %d, want 403", status)
	}
	for _, path := range []string{"/ping", "/debug/goroutines", "/restart"} {
		if status, _ := post(t, tr, path, token, ""); status != http.StatusUnauthorized && status != http.StatusMethodNotAllowed {
			t.Errorf("%s: status = %d, want a refusal", path, status)
		}
	}
	if _, err := dial(t, tr, token); err == nil {
		t.Error("the window's socket accepted a session's token")
	}
	if len(*served) != 0 {
		t.Errorf("a scoped token reached %v", *served)
	}
}

func TestScopedTokenReportsForItsOwnSessionOnly(t *testing.T) {
	t.Parallel()
	got := make(chan hookRequest, 2)
	tr, err := newTransport(func(string, []byte) {}, func(req hookRequest) { got <- req }, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	token, _ := tr.mintScoped("mine")
	if status, _ := post(t, tr, "/hook", token, `{"session_id":"mine","state":"busy"}`); status != http.StatusNoContent {
		t.Fatalf("own report: status = %d, want 204", status)
	}
	if status, _ := post(t, tr, "/hook", token, `{"session_id":"other","state":"busy"}`); status != http.StatusForbidden {
		t.Fatalf("another session's report: status = %d, want 403", status)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/mod/commands?token=%s&session_id=other", tr.port, token)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("another session's poll: status = %d, want 403", resp.StatusCode)
	}
	select {
	case req := <-got:
		if req.SessionID != "mine" {
			t.Fatalf("forwarded a report for %q", req.SessionID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("own report never forwarded")
	}
}

func TestScopedTokenEndsWithItsProcess(t *testing.T) {
	t.Parallel()
	tr, token, _ := scopedTransport(t)
	tr.revokeScoped("mine")
	if status, body := post(t, tr, "/rpc/relay.Wait", token, `[{"ticket":"t"}]`); status != http.StatusForbidden || strings.Contains(body, "{") {
		t.Errorf("a revoked token: status = %d, body %q, want a bare 403", status, body)
	}
	// A respawn mints anew and the old one is gone with it.
	fresh, _ := tr.mintScoped("mine")
	again, _ := tr.mintScoped("mine")
	if status, _ := post(t, tr, "/rpc/relay.Wait", fresh, `[{"ticket":"t"}]`); status != http.StatusForbidden {
		t.Errorf("a replaced token still answers: %d", status)
	}
	if status, _ := post(t, tr, "/rpc/relay.Wait", again, `[{"ticket":"t"}]`); status != http.StatusOK {
		t.Errorf("the current token is refused: %d", status)
	}
}

func TestBearerHeaderCarriesTheToken(t *testing.T) {
	t.Parallel()
	tr, token, _ := scopedTransport(t)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/rpc/relay.Wait", tr.port), strings.NewReader(`[{"ticket":"t"}]`))
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

// The launch token never enters a sandbox: a confined spawn's environment
// carries its own, and an unconfined one keeps the launch token.
func TestWithScopedTokenReplacesTheLaunchToken(t *testing.T) {
	t.Parallel()
	tr, err := newTransport(func(string, []byte) {}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	s := &Service{ws: tr}
	spec := ptySpec{env: []string{"A=1", "LICH_TOKEN=" + tr.token}}

	confined, err := s.withScopedToken(spec, "sess", true)
	if err != nil {
		t.Fatalf("withScopedToken: %v", err)
	}
	if slices.Contains(confined.env, "LICH_TOKEN="+tr.token) {
		t.Error("the launch token is in a confined session's environment")
	}
	var scopedEntry string
	for _, kv := range confined.env {
		if strings.HasPrefix(kv, "LICH_TOKEN=") {
			scopedEntry = kv
		}
	}
	if scopedEntry == "" {
		t.Fatalf("no token in %v", confined.env)
	}
	req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1/rpc/relay.Wait", nil)
	req.Header.Set("Authorization", "Bearer "+strings.TrimPrefix(scopedEntry, "LICH_TOKEN="))
	if c, ok := tr.callerOf(req); !ok || c.session != "sess" {
		t.Errorf("the minted token is not the session's: %+v %v", c, ok)
	}

	open, _ := s.withScopedToken(spec, "sess", false)
	if !slices.Contains(open.env, "LICH_TOKEN="+tr.token) {
		t.Error("an unconfined session lost the launch token")
	}
}
