//go:build !windows

package terminal

import (
	"sync"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// The first report of a run is the one told, and only when Codex skips the
// hook; a chat and a terminal session racing to report tell the window once.
func TestReportCodexHookTellsTheWindowOnce(t *testing.T) {
	for _, c := range []struct {
		trust string
		want  int
	}{{"", 0}, {api.HookModified, 1}} {
		hub := events.New()
		var mu sync.Mutex
		told := 0
		hub.Observe(func(name string, _ any) {
			if name == api.CodexHookUntrustedEvent.Name {
				mu.Lock()
				told++
				mu.Unlock()
			}
		})
		svc := New(stubBins{}, nil, hub)
		svc.ReportCodexHook("s1", c.trust)
		svc.ReportCodexHook("s2", api.HookUntrusted)
		if !svc.CodexHookChecked() || told != c.want {
			t.Errorf("first trust %q: checked %v, told %d, want %d", c.trust, svc.CodexHookChecked(), told, c.want)
		}
	}
}
