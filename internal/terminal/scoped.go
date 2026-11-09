package terminal

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/lichdotdev/lich/internal/scope"
)

// A confined session is handed a token minted for it alone, so the launch-wide
// token never enters a sandbox: the sandbox keeps the host's loopback (the
// agent needs its API and its hooks report over it), and with the launch token
// a confined agent could ask lich to run a command on the machine for it
// (services.Start), lower the sandbox rung (store.SetSetting) or type into
// another session. A scoped token answers to internal/scope's allowlist, and
// to the hook endpoints for its own session only.
//
// It lives in memory, minted at each spawn and dropped when the process ends,
// so a respawn, a backend restart and a resumed park each issue a new one. A
// caller that outlives its session (a background agent) is refused from then
// on, and cannot fall back on runtime.json: the sandbox hides it
// (internal/sandbox). The `lich` CLI says so (internal/cli, reissued).

// caller is who a request came from: the launch token, or one session's.
type caller struct {
	// session is the confined session a scoped token was minted for, "" for the
	// launch token.
	session string
}

func (c caller) scoped() bool { return c.session != "" }

// mintScoped issues session's token, replacing the one a previous spawn held.
func (t *transport) mintScoped(session string) (string, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("failed to generate a session token: %w", err)
	}
	token := hex.EncodeToString(raw)
	t.mu.Lock()
	if t.scoped == nil {
		t.scoped = map[string]string{}
	}
	t.scoped[session] = token
	t.mu.Unlock()
	return token, nil
}

// revokeScoped drops session's token once its process is gone.
func (t *transport) revokeScoped(session string) {
	t.mu.Lock()
	delete(t.scoped, session)
	t.mu.Unlock()
}

// tokenOf is the token a request carries: `?token=` as every client has always
// sent it, or an Authorization: Bearer header, which wins when present and
// exists for callers that would otherwise put the token in a process's command
// line.
func tokenOf(r *http.Request) string {
	if bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		return bearer
	}
	return r.URL.Query().Get("token")
}

// callerOf identifies a request by its token: the launch token, one of the
// scoped ones, or nobody.
func (t *transport) callerOf(r *http.Request) (caller, bool) {
	provided := []byte(tokenOf(r))
	if subtle.ConstantTimeCompare(provided, []byte(t.token)) == 1 {
		return caller{}, true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for session, token := range t.scoped {
		if subtle.ConstantTimeCompare(provided, []byte(token)) == 1 {
			return caller{session: session}, true
		}
	}
	return caller{}, false
}

// ownSession reports whether c may act for session: the launch token may act
// for any, a scoped one for its own.
func (c caller) ownSession(session string) bool {
	return !c.scoped() || c.session == session
}

// forbid answers a request a scoped token may not make, in the RPC's own
// envelope so the `lich` CLI shows the reason instead of retrying the token.
func forbid(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// scopedRequest holds an RPC call made with a scoped token to the policy:
// the method must be allowed, and when its options name the caller, the caller
// must be the token's session. It answers the request itself when refusing, and
// otherwise returns the request marked with its caller for the service to read
// (scope.Session).
func scopedRequest(w http.ResponseWriter, r *http.Request, c caller) (*http.Request, bool) {
	method := scope.Method(r.URL.Path)
	if method == "" || !scope.Allows(method) {
		forbid(w, scope.Refusal(strings.TrimPrefix(r.URL.Path, "/")))
		return nil, false
	}
	if field := scope.CallerField(method); field != "" {
		claimed, err := claimedCaller(r, field)
		if err != nil {
			forbid(w, err.Error())
			return nil, false
		}
		if claimed != c.session {
			forbid(w, scope.Impersonation(method, claimed, c.session))
			return nil, false
		}
	}
	return r.WithContext(scope.With(r.Context(), c.session)), true
}

// rpcBodyLimit is internal/rpc's bound on one call's body, read here first.
const rpcBodyLimit = 1 << 20

// claimedCaller reads the session the options name as the caller, leaving the
// body for the RPC to decode again.
func claimedCaller(r *http.Request, field string) (string, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, rpcBodyLimit))
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	var args []map[string]json.RawMessage
	if len(body) > 0 {
		if err := json.Unmarshal(body, &args); err != nil {
			return "", fmt.Errorf("arguments must be a JSON array: %w", err)
		}
	}
	if len(args) == 0 {
		return "", nil
	}
	var claimed string
	if raw, ok := args[0][field]; ok {
		if err := json.Unmarshal(raw, &claimed); err != nil {
			return "", fmt.Errorf("options: %s: %w", field, err)
		}
	}
	return claimed, nil
}
