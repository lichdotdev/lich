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
	Profiles(_ context.Context, _ api.None) ([]api.ProfileInfo, error)
	CreateProfile(_ context.Context, opts api.CreateProfileOptions) (api.Profile, error)
	UpdateProfile(_ context.Context, opts api.UpdateProfileOptions) (api.Profile, error)
	DeleteProfile(_ context.Context, opts api.ProfileSlugOptions) (api.None, error)
	LoginCommand(_ context.Context, opts api.ProfileSlugOptions) (string, error)
}

// ProfileLogins is a profile service that runs its provider's login itself.
type ProfileLogins interface {
	Login(_ context.Context, opts api.ProfileSlugOptions) (api.None, error)
	CancelLogin(_ context.Context, opts api.ProfileSlugOptions) (api.None, error)
}

// ProfilePlugins is a profile service that carries plugins between profiles.
type ProfilePlugins interface {
	Plugins(_ context.Context, opts api.ProfileSlugOptions) ([]api.ProfilePlugin, error)
	InstallPlugins(_ context.Context, opts api.InstallPluginsOptions) ([]api.ProfilePluginResult, error)
}

// Profiles exposes s as the profile methods m.
func Profiles(d *rpc.Handler, m api.ProfileMethods, s ProfileService) {
	rpc.Handle(d, m.Profiles, s.Profiles)
	rpc.Handle(d, m.CreateProfile, s.CreateProfile)
	rpc.Handle(d, m.UpdateProfile, s.UpdateProfile)
	rpc.Handle(d, m.DeleteProfile, s.DeleteProfile)
	rpc.Handle(d, m.LoginCommand, s.LoginCommand)
}

// ProfileLogin exposes s's login as the methods m.
func ProfileLogin(d *rpc.Handler, m api.ProfileLoginMethods, s ProfileLogins) {
	rpc.Handle(d, m.Login, s.Login)
	rpc.Handle(d, m.CancelLogin, s.CancelLogin)
}

// ProfilePluginCalls exposes s's plugin calls as the methods m.
func ProfilePluginCalls(d *rpc.Handler, m api.ProfilePluginMethods, s ProfilePlugins) {
	rpc.Handle(d, m.Plugins, s.Plugins)
	rpc.Handle(d, m.InstallPlugins, s.InstallPlugins)
}

// ClaudeLoginCode hands a running Claude Code login the code its page showed.
func ClaudeLoginCode(d *rpc.Handler, loginCode func(context.Context, api.LoginCodeOptions) (api.None, error)) {
	rpc.Handle(d, api.ClaudeProfileLoginCode, loginCode)
}

// Quota exposes the plan readings as api declares them.
func Quota(d *rpc.Handler, s *quota.Service) {
	rpc.Handle(d, api.QuotaPlans, s.Plans)
	rpc.Handle(d, api.QuotaProfilePlan, s.ProfilePlan)
	rpc.Handle(d, api.QuotaCodexProfilePlan, s.CodexProfilePlan)
}
