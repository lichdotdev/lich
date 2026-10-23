// The suite spawns a real PTY and reuses stubBins from terminal_test.go, which
// is Unix-tagged for the same reason; this file carries the tag so the package
// still builds on Windows.
//go:build !windows

package terminal

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/providers"
)

// The spawn is what resolves the list, so a server registered while lich was
// already open reaches the next session started after it — which is the whole
// reason this is not read once at startup.
func TestSpawnRecordsTheMCPServersItCanReach(t *testing.T) {
	t.Setenv("SHELL", "sh")
	t.Setenv("OPENCODE_CONFIG_DIR", t.TempDir())
	cwd := t.TempDir()
	config := `{"mcp":{"registered-after-launch":{}}}`
	if err := os.WriteFile(filepath.Join(cwd, "opencode.json"), []byte(config), 0o644); err != nil {
		t.Fatalf("write opencode.json: %v", err)
	}
	recorded := map[string][]string{}
	svc := New(stubBins{bin: "sh", mcpServers: recorded}, nil, events.New())
	t.Cleanup(func() { _, _ = svc.Close(t.Context(), api.SessionIDOptions{ID: "s1"}) })

	if _, err := svc.Start(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: cwd, Kind: providers.OpenCode, Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	want := []string{"lich", "registered-after-launch"}
	if got := recorded["s1"]; !slices.Equal(got, want) {
		t.Errorf("recorded %v, want %v", got, want)
	}
}
