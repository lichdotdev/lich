package lichclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestClaudeProfileCallsSendPositionalArgs(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name, method, args string
		result             any
		call               func(c *Client) error
	}{
		{"Profiles", "claudeprofile.Profiles", ``, []ClaudeProfile{}, func(c *Client) error {
			_, err := c.ClaudeProfiles(ctx)
			return err
		}},
		{"Create", "claudeprofile.CreateProfile", `"Work"`, ClaudeProfile{Slug: "work"}, func(c *Client) error {
			_, err := c.CreateClaudeProfile(ctx, "Work")
			return err
		}},
		{"Rename", "claudeprofile.RenameProfile", `"work","Day job"`, nil, func(c *Client) error {
			return c.RenameClaudeProfile(ctx, "work", "Day job")
		}},
		{"Delete", "claudeprofile.DeleteProfile", `"work"`, nil, func(c *Client) error {
			return c.DeleteClaudeProfile(ctx, "work")
		}},
		{"Plugins", "claudeprofile.Plugins", `"personal"`, []ClaudeProfilePlugin{}, func(c *Client) error {
			_, err := c.ClaudeProfilePlugins(ctx, "personal")
			return err
		}},
		{"InstallPlugins", "claudeprofile.InstallPlugins", `"work",["a@m","b@m"]`, []ClaudeProfilePluginResult{}, func(c *Client) error {
			_, err := c.InstallClaudeProfilePlugins(ctx, "work", []string{"a@m", "b@m"})
			return err
		}},
		{"Login", "claudeprofile.Login", `"work"`, nil, func(c *Client) error {
			return c.LoginClaudeProfile(ctx, "work")
		}},
		{"LoginCode", "claudeprofile.LoginCode", `"work","abc"`, nil, func(c *Client) error {
			return c.ClaudeLoginCode(ctx, "work", "abc")
		}},
		{"CancelLogin", "claudeprofile.CancelLogin", `"work"`, nil, func(c *Client) error {
			return c.CancelClaudeLogin(ctx, "work")
		}},
		{"LoginCommand", "claudeprofile.LoginCommand", `"work"`, "claude auth login", func(c *Client) error {
			_, err := c.ClaudeLoginCommand(ctx, "work")
			return err
		}},
		{"ProfilePlan", "quota.ProfilePlan", `"work"`, QuotaPlan{}, func(c *Client) error {
			_, err := c.ClaudeProfilePlan(ctx, "work")
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
func TestClaudeProfileReadsInfo(t *testing.T) {
	const wire = `{"slug":"work","name":"Work","dir":"/c/work","personal":false,"copied":["settings.json"],"loggedIn":true,"email":"a@b.c","plan":"max","plugins":3}`
	var p ClaudeProfile
	if err := json.Unmarshal([]byte(wire), &p); err != nil {
		t.Fatal(err)
	}
	if p.Slug != "work" || p.Name != "Work" || !p.LoggedIn || p.Email != "a@b.c" || p.Plan != "max" || p.Plugins != 3 || len(p.Copied) != 1 {
		t.Errorf("decoded %+v", p)
	}
}

func TestProfileGoneMatchesTheBackendSentence(t *testing.T) {
	remote := &RemoteError{Method: "terminal.Start", Message: "start session: claude profile no longer exists", Status: http.StatusBadRequest}
	if !ProfileGone(remote) {
		t.Error("the backend's sentence was not matched")
	}
	if ProfileGone(errors.New("pty: no such file")) || ProfileGone(nil) {
		t.Error("an unrelated failure was matched")
	}
}
