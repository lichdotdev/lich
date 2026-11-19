//go:build !windows

package chat

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// gitCheckout makes the rig's project a repository with a.txt committed.
func (r *rig) gitCheckout() string {
	r.t.Helper()
	dir := filepath.Join(r.home, "proj")
	git := func(args ...string) {
		r.t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			r.t.Skipf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x\n"), 0o644); err != nil {
		r.t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-q", "-m", "first")
	return dir
}

// TestTurnEndCarriesWhatTheTurnChanged: the count is the checkout's own, so a
// file the agent wrote behind the chat's back counts, an edit the person made
// before the turn does not, and a turn that touched nothing says so.
func TestTurnEndCarriesWhatTheTurnChanged(t *testing.T) {
	r := newRig(t, providers.Claude)
	dir := r.gitCheckout()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x\nmine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}

	r.prompt("write")
	got := r.turnEnd(0).Changes
	if got == nil || *got != (api.ChatTurnChanges{Files: 2, Added: 4}) {
		t.Errorf("a turn that edited a.txt and wrote b.txt changed %+v, want 2 files +4", got)
	}
	r.prompt("hi")
	got = r.turnEnd(1).Changes
	if got == nil || *got != (api.ChatTurnChanges{}) {
		t.Errorf("a turn that touched nothing changed %+v, want a count of none", got)
	}
}

// TestTurnThatCommitsHasNoCount: a diff against HEAD cannot compare two
// commits, so the turn says nothing rather than "no changes".
func TestTurnThatCommitsHasNoCount(t *testing.T) {
	r := newRig(t, providers.Claude)
	r.gitCheckout()
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("commit")
	if got := r.turnEnd(0).Changes; got != nil {
		t.Errorf("a turn that committed changed %+v, want no count", got)
	}
}

// TestTurnOutsideARepositoryHasNoCount: the turn ends all the same.
func TestTurnOutsideARepositoryHasNoCount(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("hi")
	turn := r.turnEnd(0)
	if turn.Changes != nil || turn.StopReason != stopEndTurn {
		t.Errorf("a turn outside a repository ended %+v, want end_turn with no count", turn)
	}
}
