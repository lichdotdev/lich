package chat

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// permissionsDriver is a driver whose provider reports its permission rules:
// Claude Code's. Codex's /permissions is its permission profile, the mode
// the window already sets (api.OptionMode).
type permissionsDriver interface {
	permissions(ctx context.Context) (api.ChatPermissionList, error)
	// permissionAct takes an action the service checked against what
	// permissions offers.
	permissionAct(ctx context.Context, opts api.ChatPermissionActOptions) (api.ChatPermissionActResult, error)
}

// hooksDriver is a driver whose provider reports its hooks: Claude Code's,
// read only as its /hooks is, and Codex's.
type hooksDriver interface {
	hooks(ctx context.Context) (api.ChatHookList, error)
	// hookAct takes one of the actions hooks lists.
	hookAct(ctx context.Context, opts api.ChatHookActOptions) error
}

var (
	errNoPermissions = errors.New("this agent does not report its permission rules")
	errNoHooks       = errors.New("this agent does not report its hooks")
	errRuleText      = errors.New("a permission rule is one line of text")
)

func (s *Service) permissionsOf(ctx context.Context, id string) (permissionsDriver, bool, error) {
	c, err := s.ensure(ctx, id)
	if err != nil {
		return nil, false, err
	}
	d, ok := c.drv.(permissionsDriver)
	return d, ok, nil
}

func (s *Service) hooksOf(ctx context.Context, id string) (hooksDriver, bool, error) {
	c, err := s.ensure(ctx, id)
	if err != nil {
		return nil, false, err
	}
	d, ok := c.drv.(hooksDriver)
	return d, ok, nil
}

// Permissions answers api.ChatPermissions.
func (s *Service) Permissions(ctx context.Context, opts api.SessionIDOptions) (api.ChatPermissionList, error) {
	d, ok, err := s.permissionsOf(ctx, opts.ID)
	if err != nil || !ok {
		return api.ChatPermissionList{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return d.permissions(callCtx)
}

// PermissionAct answers api.ChatPermissionAct.
func (s *Service) PermissionAct(ctx context.Context, opts api.ChatPermissionActOptions) (api.ChatPermissionActResult, error) {
	d, ok, err := s.permissionsOf(ctx, opts.ID)
	if err != nil {
		return api.ChatPermissionActResult{}, err
	}
	if !ok {
		return api.ChatPermissionActResult{}, errNoPermissions
	}
	if err := checkPermissionAct(opts); err != nil {
		return api.ChatPermissionActResult{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return d.permissionAct(callCtx, opts)
}

// checkPermissionAct refuses what no settings file takes: an unknown action,
// behavior or file, and a rule that is empty or spans lines.
func checkPermissionAct(opts api.ChatPermissionActOptions) error {
	if opts.Action != api.PermissionAdd && opts.Action != api.PermissionRemove {
		return fmt.Errorf("no agent can %s a permission rule", opts.Action)
	}
	if !slices.Contains(permissionBehaviors, opts.Behavior) {
		return fmt.Errorf("a permission rule cannot %s", opts.Behavior)
	}
	if !slices.Contains(claudeRuleFiles, opts.Source) {
		return fmt.Errorf("no settings file of %q takes a rule", opts.Source)
	}
	if strings.TrimSpace(opts.Rule) == "" || strings.ContainsAny(opts.Rule, "\r\n") {
		return errRuleText
	}
	return nil
}

var permissionBehaviors = []string{api.PermissionAllow, api.PermissionAsk, api.PermissionDeny}

// Hooks answers api.ChatHooks.
func (s *Service) Hooks(ctx context.Context, opts api.SessionIDOptions) (api.ChatHookList, error) {
	d, ok, err := s.hooksOf(ctx, opts.ID)
	if err != nil || !ok {
		return api.ChatHookList{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return d.hooks(callCtx)
}

// HookAct answers api.ChatHookAct.
func (s *Service) HookAct(ctx context.Context, opts api.ChatHookActOptions) (api.None, error) {
	d, ok, err := s.hooksOf(ctx, opts.ID)
	if err != nil {
		return api.None{}, err
	}
	if !ok {
		return api.None{}, errNoHooks
	}
	if !slices.Contains([]string{api.HookTrust, api.HookEnable, api.HookDisable}, opts.Action) || opts.Key == "" {
		return api.None{}, fmt.Errorf("no agent can %s a hook", opts.Action)
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return api.None{}, d.hookAct(callCtx, opts)
}
