//go:build !windows

package terminal

import (
	"path/filepath"
	"testing"
)

func TestSessionCheckoutReadsTheSpawnAndItsReservation(t *testing.T) {
	t.Parallel()
	cwd := filepath.Join(t.TempDir(), "wt")
	s := &Service{store: stubBins{ports: map[string]int{cwd: 24042}}}
	if dir, port := s.SessionCheckout("s1"); dir != "" || port != 0 {
		t.Fatalf("gone session: %q %d", dir, port)
	}
	s.spawns.Store("s1", spawn{kind: KindService, cwd: cwd + string(filepath.Separator)})
	if dir, port := s.SessionCheckout("s1"); dir != cwd+string(filepath.Separator) || port != 24042 {
		t.Fatalf("SessionCheckout = %q %d, want %q 24042", dir, port, cwd)
	}
}
