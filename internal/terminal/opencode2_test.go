package terminal

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestOpencode2ArgsRunsAPrivateServer(t *testing.T) {
	args := []string{"--session", "ses_1"}
	if got := opencode2Args(false, args); !reflect.DeepEqual(got, args) {
		t.Errorf("1.x args = %q, want them untouched", got)
	}
	want := []string{"--standalone", "--session", "ses_1"}
	if got := opencode2Args(true, args); !reflect.DeepEqual(got, want) {
		t.Errorf("2.x args = %q, want %q", got, want)
	}
}

func TestOpencode2ModelEnv(t *testing.T) {
	own := opencodeConfigContentVar + `={"mcp":{"mine":{"type":"remote"}}}`
	env, err := opencode2ModelEnv([]string{own}, "opencode/glm-5.3-flash-free", "")
	if err != nil {
		t.Fatal(err)
	}
	config := opencodeConfig(t, env)
	if config["model"] != "opencode/glm-5.3-flash-free" || config["mcp"] == nil {
		t.Errorf("config = %v, want the model beside the user's own mcp", config)
	}
	for _, tc := range []struct{ model, resume string }{{"", ""}, {"-x", ""}, {"m", "ses_1"}} {
		env, err := opencode2ModelEnv([]string{own}, tc.model, tc.resume)
		if err != nil || len(env) != 1 {
			t.Errorf("model %q resume %q: env = %q, %v, want it untouched", tc.model, tc.resume, env, err)
		}
	}
	if _, err := opencode2ModelEnv([]string{opencodeConfigContentVar + "=[1]"}, "m", ""); err == nil {
		t.Error("an inline config that is not an object: want an error, not the user's config dropped")
	}
}

func TestParseOpencode2Fork(t *testing.T) {
	// The answer 2.2.5 gave, trimmed.
	id, err := parseOpencode2Fork([]byte(`{"data":{"id":"ses_e67c07ef6ffem3mjH2rhmratNf",` +
		`"fork":{"sessionID":"ses_e67c64cb6ffeZxKHYpx89EhpZT"},"title":"Reply ok (fork #1)"}}`))
	if err != nil || id != "ses_e67c07ef6ffem3mjH2rhmratNf" {
		t.Errorf("fork id = %q, %v", id, err)
	}
	for _, out := range []string{"", "not json", `{"data":{}}`, `{"error":"SessionNotFoundError"}`} {
		if id, err := parseOpencode2Fork([]byte(out)); err == nil {
			t.Errorf("%q: fork id = %q, want an error", out, id)
		}
	}
}

func TestOpencode2PasteUnfoldEnv(t *testing.T) {
	own := opencode2CLIConfigContentVar + `={"prompt":{"editor":true},"theme":"x"}`
	env, err := opencode2PasteUnfoldEnv([]string{own}, "opencode", true)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(lastEnvValue(env, opencode2CLIConfigContentVar)), &config); err != nil {
		t.Fatal(err)
	}
	prompt, _ := config["prompt"].(map[string]any)
	if prompt["paste"] != "full" || prompt["editor"] != true || config["theme"] != "x" {
		t.Errorf("cli config = %v, want paste full beside the user's own keys", config)
	}
	for _, tc := range []struct {
		kind string
		on   bool
	}{{"opencode", false}, {"claude", true}} {
		if env, err := opencode2PasteUnfoldEnv([]string{own}, tc.kind, tc.on); err != nil || len(env) != 1 {
			t.Errorf("%s on=%v: env = %q, %v, want it untouched", tc.kind, tc.on, env, err)
		}
	}
	for _, bad := range []string{"[1]", `{"prompt":1}`} {
		if _, err := opencode2PasteUnfoldEnv([]string{opencode2CLIConfigContentVar + "=" + bad}, "opencode", true); err == nil {
			t.Errorf("%s: want an error, not the user's config dropped", bad)
		}
	}
}
