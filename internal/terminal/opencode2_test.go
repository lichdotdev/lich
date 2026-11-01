package terminal

import (
	"reflect"
	"testing"
)

func TestVersionMajor(t *testing.T) {
	for version, want := range map[string]int{"2.2.5": 2, "1.19.4": 1, "10.0": 10, "": 0, "dev": 0} {
		if got := versionMajor(version); got != want {
			t.Errorf("versionMajor(%q) = %d, want %d", version, got, want)
		}
	}
}

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
