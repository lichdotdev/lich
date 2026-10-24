package relay

import (
	"errors"
	"slices"
	"testing"
)

func TestAgentCheckoutsAreTheOnesALiveAgentRunsIn(t *testing.T) {
	t.Parallel()
	// "shell" and "parked" are in /wt/feat with "agent"; "root" runs in the
	// project's own directory; "other" makes /wt/fix live.
	svc := newRelay(merging(), newFakeTerminal("root", "agent", "shell", "other"), nil)

	got := svc.AgentCheckouts()
	if want := []string{"/src/lich", "/wt/feat", "/wt/fix"}; !slices.Equal(got, want) {
		t.Fatalf("AgentCheckouts = %v, want %v", got, want)
	}

	svc = newRelay(merging(), newFakeTerminal("shell"), nil)
	if got := svc.AgentCheckouts(); len(got) != 0 {
		t.Fatalf("AgentCheckouts with only a shell live = %v, want none", got)
	}
}

func TestAgentCheckoutsFollowNoneWhenTheWorkspaceFailsToLoad(t *testing.T) {
	t.Parallel()
	svc := newRelay(fakeSessions{err: errors.New("disk gone")}, newFakeTerminal(), nil)
	if got := svc.AgentCheckouts(); got != nil {
		t.Fatalf("AgentCheckouts = %v, want nil", got)
	}
}
