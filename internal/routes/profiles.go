package routes

import (
	"context"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/quota"
	"github.com/lichdotdev/lich/internal/rpc"
)

// ProfileService is what every provider's profile service answers
// (claudeprofile, codexprofile, ompprofile, xdgprofile).
type ProfileService interface {
	Profiles() ([]api.ProfileInfo, error)
	CreateProfile(name string) (api.Profile, error)
	UpdateProfile(slug string, u api.ProfileUpdate) (api.Profile, error)
	DeleteProfile(slug string) error
	LoginCommand(slug string) (string, error)
}

// ProfileLogins is a profile service that runs its provider's login itself.
type ProfileLogins interface {
	Login(slug string) error
	CancelLogin(slug string)
}

// ProfilePlugins is a profile service that carries plugins between profiles.
type ProfilePlugins interface {
	Plugins(slug string) ([]api.ProfilePlugin, error)
	InstallPlugins(slug string, keys []string) ([]api.ProfilePluginResult, error)
}

// Profiles exposes s as the profile methods m.
func Profiles(d *rpc.Handler, m api.ProfileMethods, s ProfileService) {
	rpc.Handle(d, m.Profiles, func(context.Context, api.None) ([]api.ProfileInfo, error) { return s.Profiles() })
	rpc.Handle(d, m.CreateProfile, func(_ context.Context, o api.CreateProfileOptions) (api.Profile, error) {
		return s.CreateProfile(o.Name)
	})
	rpc.Handle(d, m.UpdateProfile, func(_ context.Context, o api.UpdateProfileOptions) (api.Profile, error) {
		return s.UpdateProfile(o.Slug, o.Update)
	})
	rpc.Handle(d, m.DeleteProfile, func(_ context.Context, o api.ProfileSlugOptions) (api.None, error) {
		return api.None{}, s.DeleteProfile(o.Slug)
	})
	rpc.Handle(d, m.LoginCommand, func(_ context.Context, o api.ProfileSlugOptions) (string, error) {
		return s.LoginCommand(o.Slug)
	})
}

// ProfileLogin exposes s's login as the methods m.
func ProfileLogin(d *rpc.Handler, m api.ProfileLoginMethods, s ProfileLogins) {
	rpc.Handle(d, m.Login, func(_ context.Context, o api.ProfileSlugOptions) (api.None, error) {
		return api.None{}, s.Login(o.Slug)
	})
	rpc.Handle(d, m.CancelLogin, func(_ context.Context, o api.ProfileSlugOptions) (api.None, error) {
		s.CancelLogin(o.Slug)
		return api.None{}, nil
	})
}

// ProfilePluginCalls exposes s's plugin calls as the methods m.
func ProfilePluginCalls(d *rpc.Handler, m api.ProfilePluginMethods, s ProfilePlugins) {
	rpc.Handle(d, m.Plugins, func(_ context.Context, o api.ProfileSlugOptions) ([]api.ProfilePlugin, error) {
		return s.Plugins(o.Slug)
	})
	rpc.Handle(d, m.InstallPlugins, func(_ context.Context, o api.InstallPluginsOptions) ([]api.ProfilePluginResult, error) {
		return s.InstallPlugins(o.Slug, o.Keys)
	})
}

// ClaudeLoginCode hands a running Claude Code login the code its page showed.
func ClaudeLoginCode(d *rpc.Handler, loginCode func(slug, code string) error) {
	rpc.Handle(d, api.ClaudeProfileLoginCode, func(_ context.Context, o api.LoginCodeOptions) (api.None, error) {
		return api.None{}, loginCode(o.Slug, o.Code)
	})
}

// Quota exposes the plan readings as api declares them.
func Quota(d *rpc.Handler, s *quota.Service) {
	rpc.Handle(d, api.QuotaPlans, func(_ context.Context, o api.QuotaPlansOptions) ([]api.QuotaPlan, error) {
		return s.Plans(o.SessionID), nil
	})
	rpc.Handle(d, api.QuotaProfilePlan, func(_ context.Context, o api.ProfilePlanOptions) (api.QuotaPlan, error) {
		return s.ProfilePlan(o.Profile)
	})
	rpc.Handle(d, api.QuotaCodexProfilePlan, func(_ context.Context, o api.ProfilePlanOptions) (api.QuotaPlan, error) {
		return s.CodexProfilePlan(o.Profile)
	})
}
