//go:build !windows

package codexhooks

import (
	"context"
	"os/exec"
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
