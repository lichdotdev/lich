//go:build linux

package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/sandbox"
)

// The whole fix from inside a real sandbox: the process holds only its scoped
// token, the launch token's record is not on its disk, the RPC refuses it the
// escape and takes its own calls.
func TestConfinedProcessHoldsOnlyItsScopedToken(t *testing.T) {
	if !sandbox.Available() {
		t.Skip("bubblewrap is not installed")
	}
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl is not installed")
	}
	tr, token, served := scopedTransport(t)
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	cwd := filepath.Join(home, "work")
	for _, dir := range []string{".config/lich", "work"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, ".config", "lich", "runtime.json"), []byte(`{"token":"`+tr.token+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	spec := ptySpec{bin: "/bin/sh", dir: cwd, env: []string{"LICH_TOKEN=" + tr.token, "LICH_PORT=" + fmt.Sprint(tr.port), "PATH=" + os.Getenv("PATH")}}
	spec, _ = wrapSandbox(spec, providers.Claude, home, nil, nil, "", true, sandboxCreds{})
	spec.env = replaceEnv(spec.env, "LICH_TOKEN="+token)
	rpc := `curl -s -o /dev/null -w "%{http_code}" -X POST "http://127.0.0.1:$LICH_PORT/rpc/$1?token=$LICH_TOKEN" -d "$2"`
	script := strings.Join([]string{
		`call() { ` + rpc + `; }`,
		`echo "record: $(test -e "$HOME/.config/lich/runtime.json" && echo readable || echo gone)"`,
		`echo "start: $(call services.Start '[{"id":"web"}]')"`,
		`echo "setting: $(call store.SetSetting '[{"key":"sandbox","value":"off"}]')"`,
		`echo "relay: $(call relay.Send '[{"from":"mine","target":"peer","prompt":"hi"}]')"`,
		`echo "forged: $(call relay.Send '[{"from":"other","target":"peer","prompt":"hi"}]')"`,
	}, "; ")
	cmd := exec.Command(spec.bin, append(spec.args, "-c", script)...)
	cmd.Env = spec.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("confined run failed: %v\n%s", err, out)
	}
	for _, want := range []string{"record: gone", "start: 403", "setting: 403", "relay: 200", "forged: 403"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("expected %q in:\n%s", want, out)
		}
	}
	if len(*served) != 1 || !strings.Contains((*served)[0], "/rpc/relay.Send as mine") {
		t.Errorf("the RPC served %v, want the one relay call as the session", *served)
	}
}
