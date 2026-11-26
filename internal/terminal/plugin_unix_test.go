//go:build !windows

package terminal

import (
	"slices"
	"sync"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/providers"
)

// A spawn asks for the plugin to be set up for its provider, and a report from
// a release this lich does not speak asks for a pass, without naming one.
func TestSpawnAndAnIncompatibleReportAskForThePlugin(t *testing.T) {
	svc := New(stubBins{bin: stayAliveBin(t)}, nil, events.New())
	if svc.ws == nil {
		t.Skip("no transport to report through")
	}
	var mu sync.Mutex
	var asked []string
	svc.SetPluginEnsure(func(kind string) {
		mu.Lock()
		asked = append(asked, kind)
		mu.Unlock()
	})
	t.Cleanup(func() { _, _ = svc.Close(t.Context(), api.SessionIDOptions{ID: "s1"}) })

	if _, err := svc.Start(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: t.TempDir(), Kind: providers.Claude, Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Start = %v", err)
	}
	svc.ws.plugins.note("s1", "0.1.0")

	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(asked, []string{providers.Claude, ""}) {
		t.Errorf("asked %q, want the spawn's provider then a pass for the report", asked)
	}
}
