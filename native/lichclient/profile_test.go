package lichclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestProfileCallsSendPositionalArgs(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name, method, args string
		result             any
		call               func(c *Client) error
	}{
		{"Profiles", "claudeprofile.Profiles", ``, []Profile{}, func(c *Client) error {
			_, err := c.ClaudeProfiles().List(ctx)
			return err
		}},
		{"Create", "claudeprofile.CreateProfile", `"Work"`, Profile{Slug: "work"}, func(c *Client) error {
			_, err := c.ClaudeProfiles().Create(ctx, "Work")
			return err
		}},
		{"Rename", "claudeprofile.RenameProfile", `"work","Day job"`, nil, func(c *Client) error {
			return c.ClaudeProfiles().Rename(ctx, "work", "Day job")
		}},
		{"Delete", "claudeprofile.DeleteProfile", `"work"`, nil, func(c *Client) error {
			return c.ClaudeProfiles().Delete(ctx, "work")
		}},
		{"Plugins", "claudeprofile.Plugins", `"personal"`, []ProfilePlugin{}, func(c *Client) error {
			_, err := c.ClaudeProfiles().Plugins(ctx, "personal")
			return err
		}},
		{"InstallPlugins", "claudeprofile.InstallPlugins", `"work",["a@m","b@m"]`, []ProfilePluginResult{}, func(c *Client) error {
			_, err := c.ClaudeProfiles().InstallPlugins(ctx, "work", []string{"a@m", "b@m"})
			return err
		}},
		{"Login", "claudeprofile.Login", `"work"`, nil, func(c *Client) error {
			return c.ClaudeProfiles().Login(ctx, "work")
		}},
		{"LoginCode", "claudeprofile.LoginCode", `"work","abc"`, nil, func(c *Client) error {
			return c.ClaudeProfiles().LoginCode(ctx, "work", "abc")
		}},
		{"CancelLogin", "claudeprofile.CancelLogin", `"work"`, nil, func(c *Client) error {
			return c.ClaudeProfiles().CancelLogin(ctx, "work")
		}},
		{"LoginCommand", "claudeprofile.LoginCommand", `"work"`, "claude auth login", func(c *Client) error {
			_, err := c.ClaudeProfiles().LoginCommand(ctx, "work")
			return err
		}},
		{"ProfilePlan", "quota.ProfilePlan", `"work"`, QuotaPlan{}, func(c *Client) error {
			_, err := c.ClaudeProfiles().Plan(ctx, "work")
			return err
		}},
		{"CodexProfiles", "codexprofile.Profiles", ``, []Profile{}, func(c *Client) error {
			_, err := c.CodexProfiles().List(ctx)
			return err
		}},
		{"CodexLogin", "codexprofile.Login", `"work"`, nil, func(c *Client) error {
			return c.CodexProfiles().Login(ctx, "work")
		}},
		{"CodexProfilePlan", "quota.CodexProfilePlan", `"work"`, QuotaPlan{}, func(c *Client) error {
			_, err := c.CodexProfiles().Plan(ctx, "work")
			return err
		}},
		{"OMPProfiles", "ompprofile.Profiles", ``, []Profile{}, func(c *Client) error {
			_, err := c.OMPProfiles().List(ctx)
			return err
		}},
		{"OMPLoginCommand", "ompprofile.LoginCommand", `"work"`, "env OMP_PROFILE='work' omp login", func(c *Client) error {
			_, err := c.OMPProfiles().LoginCommand(ctx, "work")
			return err
		}},
		{"XDGProfiles", "kiroprofile.Profiles", ``, []Profile{}, func(c *Client) error {
			_, err := c.XDGProfiles("kiro").List(ctx)
			return err
		}},
		{"XDGLoginCommand", "cursorprofile.LoginCommand", `"work"`, "cursor-agent login", func(c *Client) error {
			_, err := c.XDGProfiles("cursor").LoginCommand(ctx, "work")
			return err
		}},
		{"ContinueIn", "terminal.ContinueIn", `"s1","work"`, nil, func(c *Client) error {
			return c.ContinueIn(ctx, "s1", "work")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := fakeBackend(t, func(method string, args []json.RawMessage) (any, int) {
				if method != tc.method || rawArgs(args) != tc.args {
					t.Errorf("got %s(%s), want %s(%s)", method, rawArgs(args), tc.method, tc.args)
				}
				return tc.result, http.StatusOK
			})
			if err := tc.call(c); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// The wire names are claudeprofile.Info's: Profile is embedded, so its tags
// sit beside the account's.
func TestProfileReadsInfo(t *testing.T) {
	const wire = `{"slug":"work","name":"Work","dir":"/c/work","personal":false,"copied":["settings.json"],"loggedIn":true,"email":"a@b.c","plan":"max","plugins":3}`
	var p Profile
	if err := json.Unmarshal([]byte(wire), &p); err != nil {
		t.Fatal(err)
	}
	if p.Slug != "work" || p.Name != "Work" || !p.LoggedIn || p.Email != "a@b.c" || p.Plan != "max" || p.Plugins != 3 || len(p.Copied) != 1 {
		t.Errorf("decoded %+v", p)
	}
	const omp = `{"slug":"w","name":"W","personal":false,"loggedIn":true,"email":"a@b.c","providers":["anthropic"]}`
	if err := json.Unmarshal([]byte(omp), &p); err != nil || len(p.Providers) != 1 || p.Providers[0] != "anthropic" {
		t.Errorf("decoded %+v, %v", p.Providers, err)
	}
}

func TestProfileGoneMatchesTheBackendSentence(t *testing.T) {
	remote := &RemoteError{Method: "terminal.Start", Message: "start session: claude profile no longer exists", Status: http.StatusBadRequest}
	if !ProfileGone(remote) {
		t.Error("the backend's sentence was not matched")
	}
	codex := &RemoteError{Method: "terminal.Start", Message: "start session: codex profile no longer exists", Status: http.StatusBadRequest}
	if !ProfileGone(codex) {
		t.Error("the Codex sentence was not matched")
	}
	omp := &RemoteError{Method: "terminal.Start", Message: "start session: omp profile no longer exists", Status: http.StatusBadRequest}
	if !ProfileGone(omp) {
		t.Error("the oh-my-pi sentence was not matched")
	}
	if ProfileGone(errors.New("pty: no such file")) || ProfileGone(nil) {
		t.Error("an unrelated failure was matched")
	}
}
