//go:build !windows

package codexhooks

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

// Probe sends initialize and hooks/list and reads the answer to the second,
// past whatever else the app-server writes first.
func TestProbeReadsTheHooksListAnswer(t *testing.T) {
	answer := `{"id":2,"result":{"data":[{"hooks":[{"eventName":"preToolUse","pluginId":"lich@lich-plugin","trustStatus":"modified"}]}]}}`
	cmd := exec.Command("sh", "-c", `printf '%s\n' '{"id":1,"result":{}}' '{"method":"note"}' '`+answer+`'; cat >/dev/null`)
	list, err := Probe(context.Background(), cmd, "/tmp")
	if err != nil || LichTrust(list) != api.HookModified {
		t.Fatalf("Probe = %+v, %v", list, err)
	}
}

// An app-server that ends without answering is an error, not an empty list.
func TestProbeOfAnAppServerThatEndsEarly(t *testing.T) {
	cmd := exec.Command("sh", "-c", `printf '%s\n' '{"id":1,"result":{}}'`)
	if _, err := Probe(context.Background(), cmd, "/tmp"); err == nil {
		t.Fatal("Probe of an app-server that ended early read a list")
	}
	cmd = exec.Command("sh", "-c", `printf '%s\n' '{"id":2,"error":{"message":"no"}}'; cat >/dev/null`)
	if _, err := Probe(context.Background(), cmd, "/tmp"); err == nil {
		t.Fatal("Probe took an error answer as a list")
	}
}

// TrustLich trusts lich's untrusted or modified hooks at their current hash, and
// no other plugin's.
func TestTrustLichWritesOnlyLichsHooks(t *testing.T) {
	list := `{"id":2,"result":{"data":[{"hooks":[` +
		`{"key":"lich@lich-plugin:hooks/codex-hooks.json:pre_tool_use:0:0","pluginId":"lich@lich-plugin","trustStatus":"modified","currentHash":"sha256:a"},` +
		`{"key":"lich@lich-plugin:hooks/codex-hooks.json:stop:0:0","pluginId":"lich@lich-plugin","trustStatus":"trusted","currentHash":"sha256:b"},` +
		`{"key":"other@m:hooks.json:stop:0:0","pluginId":"other@m","trustStatus":"untrusted","currentHash":"sha256:c"}]}]}}`
	got := filepath.Join(t.TempDir(), "in")
	cmd := exec.Command("sh", "-c", `printf '%s\n' '`+list+`'; while read -r line; do printf '%s\n' "$line" >> "$1"; case "$line" in *batchWrite*) printf '%s\n' '{"id":3,"result":{}}';; esac; done`, "sh", got)
	n, err := TrustLich(context.Background(), cmd, "/tmp")
	if err != nil || n != 1 {
		t.Fatalf("TrustLich = (%d, %v), want 1 hook trusted", n, err)
	}
	data, _ := os.ReadFile(got)
	if !strings.Contains(string(data), `"lich@lich-plugin:hooks/codex-hooks.json:pre_tool_use:0:0":{"trusted_hash":"sha256:a"}`) ||
		strings.Contains(string(data), "other@m") || strings.Contains(string(data), "sha256:b") {
		t.Errorf("app-server got:\n%s", data)
	}
}

// With every lich hook trusted, nothing is written.
func TestTrustLichWithNothingToTrust(t *testing.T) {
	list := `{"id":2,"result":{"data":[{"hooks":[{"key":"k","pluginId":"lich@lich-plugin","trustStatus":"trusted","currentHash":"h"}]}]}}`
	cmd := exec.Command("sh", "-c", `printf '%s\n' '`+list+`'; cat >/dev/null`)
	if n, err := TrustLich(context.Background(), cmd, "/tmp"); err != nil || n != 0 {
		t.Fatalf("TrustLich = (%d, %v), want nothing done", n, err)
	}
}
