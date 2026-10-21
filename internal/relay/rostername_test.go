package relay

import "testing"

// TestRosterNameOf pins the choice between the two names a session can be
// addressed by: what its agent has on record beats what lich can derive, every
// time. The derivation is only ever the answer when there is nothing to read —
// and a name that is only whitespace is nothing to read. A Claude Code session
// with nothing on record named itself, so there is no name to give.
func TestRosterNameOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		kind     string
		recorded string
		want     string
	}{
		{name: "the recorded name wins", kind: "claude", recorded: "reviewer", want: "reviewer"},
		{name: "trimmed on the way through", kind: "claude", recorded: "  reviewer\n", want: "reviewer"},
		{name: "nothing recorded falls back", kind: "codex", recorded: "", want: "lich-4f2a"},
		{name: "whitespace is nothing recorded", kind: "codex", recorded: "   ", want: "lich-4f2a"},
		{name: "claude with nothing recorded has no name", kind: "claude", recorded: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RosterNameOf(tt.kind, tt.recorded, "/home/me/code/lich", "4f2a1b3c")
			if got != tt.want {
				t.Errorf("RosterNameOf(%q, %q, …) = %q, want %q", tt.kind, tt.recorded, got, tt.want)
			}
		})
	}
}

// TestRosterNameDerivesTheBirthName pins the string lich gives a session that
// has nothing on record yet — the one it is spawned under, and the one the
// roster falls back to.
func TestRosterNameDerivesTheBirthName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cwd  string
		id   string
		want string
	}{
		{
			name: "directory and id tail",
			cwd:  "/home/me/code/lich",
			id:   "4f2a1b3c-0000-4000-8000-000000000000",
			want: "lich-4f2a",
		},
		{
			name: "trailing separator ignored",
			cwd:  "/home/me/code/lich/",
			id:   "4f2a1b3c",
			want: "lich-4f2a",
		},
		{
			name: "windows path",
			cwd:  `C:\Users\me\code\lich`,
			id:   "4f2a1b3c",
			want: "lich-4f2a",
		},
		{
			name: "no directory to name",
			cwd:  "",
			id:   "4f2a1b3c",
			want: "lich-4f2a",
		},
		{
			name: "root is not a name",
			cwd:  "/",
			id:   "4f2a1b3c",
			want: "lich-4f2a",
		},
		{
			name: "here is not a name either",
			cwd:  ".",
			id:   "4f2a1b3c",
			want: "lich-4f2a",
		},
		{
			name: "id carries nothing usable",
			cwd:  "/home/me/code/lich",
			id:   "---",
			want: "lich",
		},
		{
			name: "punctuation inside the id is skipped, not counted",
			cwd:  "/home/me/code/lich",
			id:   "4-f-2-a-1",
			want: "lich-4f2a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RosterName(tt.cwd, tt.id); got != tt.want {
				t.Errorf("RosterName(%q, %q) = %q, want %q", tt.cwd, tt.id, got, tt.want)
			}
		})
	}
}

func TestRosterNameSeparatesSessionsSharingACheckout(t *testing.T) {
	t.Parallel()
	const cwd = "/home/me/code/lich"
	first := RosterName(cwd, "4f2a1b3c-0000-4000-8000-000000000000")
	second := RosterName(cwd, "9d8e7f6a-0000-4000-8000-000000000000")

	if first == second {
		t.Fatalf("two sessions in one checkout share the name %q", first)
	}
}

// TestIsRosterNameRecognisesEveryBirthName pins the inverse: whatever id lich
// mints a session under, the name RosterName gives it is recognised as one, and
// a name the user typed is not.
func TestIsRosterNameRecognisesEveryBirthName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ cwd, id string }{
		{"/home/u/try/skipo", "3c5bd823-ad2f"},
		{"/home/u/try/skipo/", "a-b-c-d-e"},
		{"", "0118880a"},
		{`C:\work\alpha`, "ffff0000"},
	} {
		if name := RosterName(tc.cwd, tc.id); !IsRosterName(name, tc.cwd) {
			t.Errorf("IsRosterName(%q, %q) = false, want true for RosterName's own output", name, tc.cwd)
		}
	}
	for _, name := range []string{"", "skipo", "skipo-3c5", "skipo-3c5bd", "skipo-3c_b", "fix login", "other-3c5b"} {
		if IsRosterName(name, "/home/u/try/skipo") {
			t.Errorf("IsRosterName(%q) = true, want false", name)
		}
	}
}

// TestKnownLabelsLeavesOutAMissingName pins the refusal's list of who is
// reachable: a session with no second name is listed by its label alone, never
// with an empty pair of brackets an agent could read as a name.
func TestKnownLabelsLeavesOutAMissingName(t *testing.T) {
	t.Parallel()
	got := knownLabels([]candidate{
		{Peer: Peer{Label: "docs", Name: "lich-4f2a"}},
		{Peer: Peer{Label: "review"}},
	})
	if want := `Live sessions: "docs" (lich-4f2a), "review".`; got != want {
		t.Errorf("knownLabels = %q, want %q", got, want)
	}
}
