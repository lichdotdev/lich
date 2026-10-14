package xdgprofile

import "testing"

func TestStatusesReadWhatEachCLIPrints(t *testing.T) {
	cases := []struct {
		name   string
		status statusCommand
		out    string
		want   login
	}{
		{"opencode logged out", opencodeStatus, "┌  Credentials ~/.local/share/opencode/auth.json\n│\n└  0 credentials\n", login{}},
		{"opencode logged in", opencodeStatus, "┌  Credentials\n│\n●  Anthropic oauth\n│\n└  1 credential\n", login{loggedIn: true}},
		{"cursor logged out", cursorStatus, "{\n  \"status\": \"unauthenticated\",\n  \"isAuthenticated\": false,\n  \"hasAccessToken\": false,\n  \"hasRefreshToken\": false,\n  \"message\": \"Not logged in\"\n}\n", login{}},
		{"cursor logged in", cursorStatus, "Update available\n" + `{"status":"authenticated","isAuthenticated":true,"userInfo":{"email":"me@work.example"}}`, login{loggedIn: true, email: "me@work.example"}},
		{"cursor logged in without details", cursorStatus, `{"status":"authenticated","isAuthenticated":true}`, login{loggedIn: true}},
		{"kiro logged out", kiroStatus, `{"account":null}`, login{}},
		{"kiro logged in", kiroStatus, `{"account":{"email":"me@work.example","type":"social"}}`, login{loggedIn: true, email: "me@work.example"}},
	}
	for _, c := range cases {
		got, err := c.status.read(c.out)
		if err != nil || got != c.want {
			t.Errorf("%s: read = %+v, %v; want %+v", c.name, got, err, c.want)
		}
	}
	for _, s := range []statusCommand{opencodeStatus, cursorStatus, kiroStatus} {
		if _, err := s.read("error: something went wrong\n"); err == nil {
			t.Errorf("%v read an error message as an answer", s.args)
		}
	}
}
