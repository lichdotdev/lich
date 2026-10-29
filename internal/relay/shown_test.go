package relay

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

// shownRelay is a relay whose window is attached while *attached holds, with
// session id on screen.
func shownRelay(t *testing.T, id string, attached *bool) *Service {
	t.Helper()
	svc := newRelay(editorWorkspace("/src/lich", "/src/lich-wt"), newFakeTerminal("s1", "s2", "s3"), nil)
	svc.SetWindowAttached(func() bool { return *attached })
	if _, err := svc.SetShown(t.Context(), api.ShownSessionOptions{SessionID: id}); err != nil {
		t.Fatalf("SetShown = %v, want nil", err)
	}
	return svc
}

func focusedLabels(peers []Peer) []string {
	var labels []string
	for _, p := range peers {
		if p.Focused {
			labels = append(labels, p.Label)
		}
	}
	return labels
}

func TestPeersMarkTheSessionOnScreen(t *testing.T) {
	t.Parallel()
	attached := true
	svc := shownRelay(t, "s2", &attached)

	peers, err := svc.Peers(t.Context(), PeersOptions{From: "s1"})
	if err != nil {
		t.Fatalf("Peers = %v, want nil", err)
	}
	if got := focusedLabels(peers); len(got) != 1 || got[0] != "feature" {
		t.Errorf("focused = %v, want only feature", got)
	}
}

func TestPeersMarkNothingWithoutAWindow(t *testing.T) {
	t.Parallel()
	attached := false
	svc := shownRelay(t, "s2", &attached)

	peers, err := svc.Peers(t.Context(), PeersOptions{})
	if err != nil {
		t.Fatalf("Peers = %v, want nil", err)
	}
	if got := focusedLabels(peers); len(got) != 0 {
		t.Errorf("focused = %v, want none: a closed window shows no card", got)
	}
}

func TestPeersMarkNothingWhenNoWindowWasWired(t *testing.T) {
	t.Parallel()
	svc := newRelay(editorWorkspace("/src/lich", "/src/lich-wt"), newFakeTerminal("s1", "s2", "s3"), nil)
	if _, err := svc.SetShown(t.Context(), api.ShownSessionOptions{SessionID: "s2"}); err != nil {
		t.Fatalf("SetShown = %v, want nil", err)
	}

	peers, err := svc.Peers(t.Context(), PeersOptions{})
	if err != nil {
		t.Fatalf("Peers = %v, want nil", err)
	}
	if got := focusedLabels(peers); len(got) != 0 {
		t.Errorf("focused = %v, want none", got)
	}
}

func TestSelfSaysWhetherItIsOnScreen(t *testing.T) {
	t.Parallel()
	attached := true
	svc := shownRelay(t, "s3", &attached)

	for id, want := range map[string]bool{"s3": true, "s1": false} {
		self, err := svc.Self(t.Context(), SelfOptions{From: id})
		if err != nil {
			t.Fatalf("Self(%s) = %v, want nil", id, err)
		}
		if self.Focused != want {
			t.Errorf("Self(%s).Focused = %v, want %v", id, self.Focused, want)
		}
	}
}

func TestFocusedNamesTheSessionOnScreen(t *testing.T) {
	t.Parallel()
	attached := true
	svc := shownRelay(t, "s3", &attached)

	got, err := svc.Focused(t.Context(), api.None{})
	if err != nil {
		t.Fatalf("Focused = %v, want nil", err)
	}
	if got != (Focused{ID: "s3", Label: "solo", Project: "revu"}) {
		t.Errorf("Focused = %+v, want solo in revu", got)
	}
}

func TestFocusedFailsWhenNoCardIsOnScreen(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		shown    string
		attached bool
	}{
		"no window":          {shown: "s3", attached: false},
		"nothing shown":      {shown: "", attached: true},
		"shown session gone": {shown: "s9", attached: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			attached := tc.attached
			svc := shownRelay(t, tc.shown, &attached)

			_, err := svc.Focused(t.Context(), api.None{})
			if err == nil || !strings.Contains(err.Error(), "window") {
				t.Errorf("Focused = %v, want an error naming the window", err)
			}
		})
	}
}
