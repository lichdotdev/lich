//go:build !windows

package restart

import (
	"strings"
	"testing"
)

// TestWatchNativeNamesTheSignalThatKilledTheWindow proves a window killed from
// outside is logged with the signal, not only a meaningless exit code.
func TestWatchNativeNamesTheSignalThatKilledTheWindow(t *testing.T) {
	logged, _, cmd, watched := startHelper(t, "hang")
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	<-watched

	if line := logged.String(); !strings.Contains(line, `state="signal: killed"`) {
		t.Errorf("backend log = %q, want the signal", line)
	}
}
