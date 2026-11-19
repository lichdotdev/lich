package quota

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

var uuidShape = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// resetServer answers every spend with status and body, and keeps what it saw.
type resetServer struct {
	mu       sync.Mutex
	paths    []string
	bodies   []claudeResetRequest
	header   http.Header
	status   int
	response string
}

func newResetServer(t *testing.T, status int, response string) (*resetServer, *Service) {
	t.Helper()
	// The login's directory: credentials and the global config that names the
	// organization, as the CLI writes them.
	writeCreds(t, claudeCredsJSON, "")
	writeOrganization(t, os.Getenv(claudeDirVar), "org-123")
	rs := &resetServer{status: status, response: response}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var body claudeResetRequest
		_ = json.Unmarshal(data, &body)
		rs.mu.Lock()
		defer rs.mu.Unlock()
		rs.paths, rs.bodies, rs.header = append(rs.paths, r.URL.Path), append(rs.bodies, body), r.Header.Clone()
		w.WriteHeader(rs.status)
		_, _ = w.Write([]byte(rs.response))
	}))
	t.Cleanup(server.Close)
	s := newService("", "", time.Now())
	s.resetURL = server.URL + "/api/organizations/%s/reset_rate_limits"
	return rs, s
}

func writeOrganization(t *testing.T, dir, org string) {
	t.Helper()
	body := `{"oauthAccount":{"organizationUuid":"` + org + `","emailAddress":"x@example.com"}}`
	if err := os.WriteFile(filepath.Join(dir, ".claude.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func spend(s *Service) (api.UseResetResult, error) {
	return s.UseReset(context.Background(), api.UseResetOptions{Profile: "personal", GrantID: "full-reset-2611"})
}

func TestUseResetAsksTheOrganizationTheSpendTheCliWould(t *testing.T) {
	rs, s := newResetServer(t, http.StatusOK, `{"status":"reset"}`)
	got, err := spend(s)
	if err != nil || got.Status != api.UseResetReset {
		t.Fatalf("got %+v, %v", got, err)
	}
	if rs.paths[0] != "/api/organizations/org-123/reset_rate_limits" {
		t.Errorf("path = %q", rs.paths[0])
	}
	body := rs.bodies[0]
	if body.Program != "cedar_ember" || body.GrantID != "full-reset-2611" || !uuidShape.MatchString(body.RequestID) {
		t.Errorf("body = %+v", body)
	}
	for name, want := range map[string]string{
		"Authorization": "Bearer tok-claude", "anthropic-beta": "oauth-2025-04-20",
		"User-Agent": "claude-cli/" + claudeFallbackVersion + " (external, cli)", "Content-Type": "application/json",
	} {
		if got := rs.header.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestUseResetNamesEveryOutcomeTheProviderGives(t *testing.T) {
	cases := map[string]struct {
		status   int
		body     string
		want     string
		cooldown string
	}{
		"reset":                             {200, `{"status":"reset"}`, api.UseResetReset, ""},
		"already used":                      {200, `{"status":"already_used"}`, api.UseResetAlreadyUsed, ""},
		"not limited":                       {200, `{"status":"not_limited"}`, api.UseResetNotLimited, ""},
		"ineligible":                        {200, `{"status":"ineligible"}`, api.UseResetIneligible, ""},
		"cooldown":                          {200, `{"status":"cooldown","cooldown_until":"2026-11-20T10:00:00+00:00"}`, api.UseResetCooldown, "2026-11-20T10:00:00+00:00"},
		"cooldown with a bad time":          {200, `{"status":"cooldown","cooldown_until":"later"}`, api.UseResetCooldown, ""},
		"a refusal that still names itself": {409, `{"status":"not_limited"}`, api.UseResetNotLimited, ""},
		"another key for the status":        {200, `{"result":"Reset"}`, api.UseResetReset, ""},
	}
	for name, c := range cases {
		_, s := newResetServer(t, c.status, c.body)
		got, err := spend(s)
		if err != nil || got.Status != c.want || got.CooldownUntil != c.cooldown {
			t.Errorf("%s: got %+v, %v", name, got, err)
		}
	}
}

func TestUseResetReportsWhatItCannotTurnIntoAnOutcome(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
		want   string
	}{
		"unknown status": {200, `{"status":"teapot"}`, "does not know"},
		"no status":      {200, `{}`, "did not say whether"},
		"server error":   {500, `oops`, "Claude answered 500"},
		"rate limited":   {429, `{}`, "rate limiting"},
		"signed out":     {401, `{}`, "sign in to Claude Code again"},
		"forbidden":      {403, `{}`, "sign in to Claude Code again"},
	}
	for name, c := range cases {
		_, s := newResetServer(t, c.status, c.body)
		if _, err := spend(s); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want one saying %q", name, err, c.want)
		}
	}
}

func TestUseResetKeepsTheRequestIdOfAnAttemptThatFailed(t *testing.T) {
	rs, s := newResetServer(t, http.StatusInternalServerError, `oops`)
	_, _ = spend(s)
	_, _ = spend(s)
	if len(rs.bodies) != 2 || rs.bodies[0].RequestID != rs.bodies[1].RequestID {
		t.Fatalf("a retry after an error must carry the same id: %+v", rs.bodies)
	}
	rs.mu.Lock()
	rs.status, rs.response = http.StatusOK, `{"status":"reset"}`
	rs.mu.Unlock()
	if _, err := spend(s); err != nil {
		t.Fatal(err)
	}
	if rs.bodies[2].RequestID != rs.bodies[0].RequestID {
		t.Error("the retry that got through should still carry the first id")
	}
	_, _ = spend(s)
	if rs.bodies[3].RequestID == rs.bodies[0].RequestID {
		t.Error("an attempt after an answer reused the old id")
	}
	if _, err := s.UseReset(context.Background(), api.UseResetOptions{Profile: "personal", GrantID: "another"}); err != nil {
		t.Fatal(err)
	}
	if rs.bodies[4].RequestID == rs.bodies[3].RequestID {
		t.Error("two grants shared an id")
	}
}

func TestASpentResetDropsTheCachedReading(t *testing.T) {
	for status, drops := range map[string]bool{"reset": true, "already_used": true, "not_limited": false, "cooldown": false} {
		_, s := newResetServer(t, http.StatusOK, `{"status":"`+status+`"}`)
		url, calls := serve(t, http.StatusOK, claudeLimitsBody)
		s.claudeURL = url
		s.plans("")
		s.plans("")
		if *calls != 1 {
			t.Fatalf("%s: the reading was not cached: %d calls", status, *calls)
		}
		if _, err := spend(s); err != nil {
			t.Fatal(err)
		}
		s.plans("")
		if refetched := *calls == 2; refetched != drops {
			t.Errorf("%s: calls = %d, want the cache dropped = %v", status, *calls, drops)
		}
	}
}

func TestUseResetWithoutAnOrganizationSaysSo(t *testing.T) {
	_, s := newResetServer(t, http.StatusOK, `{"status":"reset"}`)
	writeOrganization(t, os.Getenv(claudeDirVar), "")
	if _, err := spend(s); err == nil || !strings.Contains(err.Error(), "organization") {
		t.Errorf("err = %v, want one naming the organization", err)
	}
	if err := os.Remove(filepath.Join(os.Getenv(claudeDirVar), ".claude.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := spend(s); err == nil || !strings.Contains(err.Error(), "organization") {
		t.Errorf("with no config at all: err = %v", err)
	}
}

func TestUseResetNeverSpendsWithoutALogin(t *testing.T) {
	rs, s := newResetServer(t, http.StatusOK, `{"status":"reset"}`)
	if err := os.Remove(filepath.Join(os.Getenv(claudeDirVar), ".credentials.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := spend(s); err == nil || !strings.Contains(err.Error(), "sign in") {
		t.Errorf("err = %v, want a sign in", err)
	}
	if len(rs.bodies) != 0 {
		t.Error("a request went out with no login")
	}
}

func TestAnOauthTokenLoginHasNoResetsToUse(t *testing.T) {
	rs, s := newResetServer(t, http.StatusOK, `{"status":"reset"}`)
	t.Setenv(claudeTokenVar, "sk-ant-oat01-x")
	if _, err := spend(s); err == nil || !strings.Contains(err.Error(), "token") {
		t.Errorf("err = %v", err)
	}
	if len(rs.bodies) != 0 {
		t.Error("a request went out for a token login")
	}
}

func TestTheUsageAgentCarriesTheInstalledVersion(t *testing.T) {
	s := newService("", "", time.Now())
	if got := s.usageAgent(); got != "claude-cli/"+claudeFallbackVersion+" (external, cli)" {
		t.Errorf("with no version readable: %q", got)
	}
	s.claudeVersion = func() string { return "2.1.400" }
	if got := s.usageAgent(); got != "claude-cli/2.1.400 (external, cli)" {
		t.Errorf("agent = %q", got)
	}
}

func TestTheInstalledVersionIsReadOnceFromTheBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the binary here is a shell script")
	}
	bin := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho '2.1.777 (Claude Code)'\n"), 0o755); err != nil {
		t.Skip("cannot write a script")
	}
	s := newService("", "", time.Now())
	s.claudeVersion, s.claudeBin = nil, func() string { return bin }
	if got := s.usageAgent(); got != "claude-cli/2.1.777 (external, cli)" {
		t.Fatalf("agent = %q", got)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho 'broken'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	s.claudeBin = func() string { return filepath.Join(t.TempDir(), "missing") }
	if got := s.usageAgent(); got != "claude-cli/"+claudeFallbackVersion+" (external, cli)" {
		t.Errorf("a binary that is gone falls back: %q", got)
	}
}
