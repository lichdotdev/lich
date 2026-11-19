package quota

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/claudeprofile"
	"github.com/lichdotdev/lich/internal/providers"
)

const (
	// claudeResetURL spends a reset: %s is the account's organization.
	claudeResetURL = "https://api.anthropic.com/api/organizations/%s/reset_rate_limits"
	// claudeResetProgram names the grant program a spend is made against.
	claudeResetProgram = "cedar_ember"
	// maxResetBody bounds what is read of an answer.
	maxResetBody = 1 << 20
)

// claudeResetRequest is the body of a spend. RequestID is what lets the server
// tell a retry of an attempt from a second spend.
type claudeResetRequest struct {
	Program   string `json:"program"`
	GrantID   string `json:"grant_id"`
	RequestID string `json:"request_id"`
}

// claudeResetAnswer is the part of the answer lich reads. The route is not
// documented and its shape was never measured by spending a reset, so the
// status is looked for under the names a result of this kind plausibly has.
type claudeResetAnswer struct {
	Status        string `json:"status"`
	Result        string `json:"result"`
	CooldownUntil string `json:"cooldown_until"`
}

func (a claudeResetAnswer) status() string {
	return strings.ToLower(strings.TrimSpace(cmp.Or(a.Status, a.Result)))
}

// SetBinary wires the answer to "which Claude Code binary is installed", read
// once for its version (usageAgent). Startup wiring; unset looks claude up on
// PATH.
func (s *Service) SetBinary(fn func() string) { s.claudeBin = fn }

// versions caches a binary's `--version` by path, size and modification time: a
// probe boots the CLI, and an upgrade rewrites the file, which is what
// invalidates it.
var versions sync.Map

// usageAgent is the user agent of the usage read and of a spend: what the CLI
// sends, with the version of the one installed, or claudeFallbackVersion when
// that cannot be read.
func (s *Service) usageAgent() string {
	version := s.installedVersion()
	if version == "" {
		version = claudeFallbackVersion
	}
	return fmt.Sprintf("claude-cli/%s (external, cli)", version)
}

func (s *Service) installedVersion() string {
	if s.claudeVersion != nil {
		return s.claudeVersion()
	}
	bin := providers.DefaultBinary(providers.Claude)
	if s.claudeBin != nil && s.claudeBin() != "" {
		bin = s.claudeBin()
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	key := fmt.Sprintf("%s\x00%d\x00%d", path, info.Size(), info.ModTime().UnixNano())
	if v, ok := versions.Load(key); ok {
		return v.(string)
	}
	version, err := providers.ProbeVersion(context.Background(), path)
	if err != nil {
		return ""
	}
	versions.Store(key, version)
	return version
}

// UseReset spends one reset of a grant of the profile's account, and answers
// what Claude said. Nothing about it is retried or refreshed: a spend cannot be
// undone, so a failure is reported as it is, and the token is only ever read.
//
// The request id of an attempt is kept until the provider gives an answer, so a
// retry after an error carries the id of the attempt that may have gone
// through, and the server counts the two as one.
func (s *Service) UseReset(ctx context.Context, opts api.UseResetOptions) (api.UseResetResult, error) {
	account, err := s.profileAccount(opts.Profile)
	if err != nil {
		return api.UseResetResult{}, err
	}
	if account.lookup(claudeTokenVar) != "" {
		return api.UseResetResult{}, errors.New("a login made with a long-lived token has no resets to use")
	}
	creds, status := s.claudeLogin(account)
	if status != StatusOK {
		return api.UseResetResult{}, errors.New("sign in to Claude Code again to use resets")
	}
	org, err := claudeOrganization(account)
	if err != nil {
		return api.UseResetResult{}, err
	}
	id := s.attemptID(opts.Profile, opts.GrantID)
	result, err := s.postReset(ctx, creds.OAuth.AccessToken, org, claudeResetRequest{Program: claudeResetProgram, GrantID: opts.GrantID, RequestID: id})
	if err != nil {
		return api.UseResetResult{}, err
	}
	s.mu.Lock()
	delete(s.attempts, opts.Profile+"\x00"+opts.GrantID)
	if result.Status == api.UseResetReset || result.Status == api.UseResetAlreadyUsed {
		// The limit is cleared: the next reading must see it, not the cache.
		delete(s.cache, cacheKey(account))
		delete(s.cache, providers.Claude+"\x00"+cacheKey(account))
	}
	s.mu.Unlock()
	return result, nil
}

// profileAccount is the account a Claude Code profile spends, as ProfilePlan
// reads it.
func (s *Service) profileAccount(profile string) (Account, error) {
	if claudeprofile.IsPersonal(profile) {
		return Account{Read: true}, nil
	}
	dir, err := claudeprofile.ConfigDir(profile)
	if err != nil {
		return Account{}, err
	}
	return Account{Read: true, Env: envMap(claudeprofile.Env(os.Environ(), dir))}, nil
}

// attemptID is the request id of the spend of grant by profile: made on the
// first try and kept until an answer comes.
func (s *Service) attemptID(profile, grant string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := profile + "\x00" + grant
	if id, ok := s.attempts[key]; ok {
		return id
	}
	if s.attempts == nil {
		s.attempts = map[string]string{}
	}
	id := newUUID()
	s.attempts[key] = id
	return id
}

// newUUID is a random (version 4) UUID.
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// claudeOrganization is the organization the account's login belongs to, which
// the CLI keeps in its global config next to the credentials: the config dir's
// .claude.json when the environment names one, else ~/.claude.json.
func claudeOrganization(a Account) (string, error) {
	path := ""
	if dir := a.lookup(claudeDirVar); dir != "" {
		path = filepath.Join(dir, ".claude.json")
	} else if home := a.lookup(accountHomeVar); home != "" {
		path = filepath.Join(home, ".claude.json")
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("lich cannot tell which organization this login belongs to")
	}
	var config struct {
		OAuthAccount struct {
			Organization string `json:"organizationUuid"`
		} `json:"oauthAccount"`
	}
	if !readCredentials(path, &config) || config.OAuthAccount.Organization == "" {
		return "", errors.New("the login's organization is not in " + path + "; start Claude Code once to write it")
	}
	return config.OAuthAccount.Organization, nil
}

// postReset sends the spend and reads Claude's answer. A 401 or 403 is a login
// to redo; any other status that carries no answer lich knows is an error with
// what the server said.
func (s *Service) postReset(ctx context.Context, token, org string, body claudeResetRequest) (api.UseResetResult, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return api.UseResetResult{}, err
	}
	url := fmt.Sprintf(s.resetURL, org)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payload)))
	if err != nil {
		return api.UseResetResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", claudeBetaHeader)
	req.Header.Set("User-Agent", s.usageAgent())
	req.Header.Set("content-type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return api.UseResetResult{}, fmt.Errorf("could not reach Claude: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return api.UseResetResult{}, errors.New("sign in to Claude Code again to use resets")
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, maxResetBody))
	var answer claudeResetAnswer
	_ = json.Unmarshal(data, &answer)
	switch status := answer.status(); status {
	case api.UseResetReset, api.UseResetAlreadyUsed, api.UseResetNotLimited, api.UseResetIneligible, api.UseResetCooldown:
		return api.UseResetResult{Status: status, CooldownUntil: timestamp(answer.CooldownUntil)}, nil
	case "":
	default:
		return api.UseResetResult{}, fmt.Errorf("Claude answered %q, which lich does not know", status)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return api.UseResetResult{}, errors.New("Claude is rate limiting these requests, try again soon")
	}
	return api.UseResetResult{}, fmt.Errorf("Claude answered %d and did not say whether the reset went through", resp.StatusCode)
}
