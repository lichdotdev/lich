package terminal

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// plantCrushTitle writes Crush's checkout database with one session titled
// title and returns the checkout.
func plantCrushTitle(t *testing.T, id, title string) string {
	t.Helper()
	cwd := t.TempDir()
	path, _ := crushSessionDB(cwd)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	for _, stmt := range []string{
		`CREATE TABLE sessions (id TEXT PRIMARY KEY, title TEXT NOT NULL, cost REAL NOT NULL DEFAULT 0)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO sessions (id, title) VALUES (?, ?)`, id, title); err != nil {
		t.Fatal(err)
	}
	return cwd
}

// Crush files its title in the checkout's database, and names a session before
// its title call lands; those placeholders are not a title.
func TestCrushTitle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, title, want string
	}{
		{"a generated title", "Explain what calc.py does", "Explain what calc.py does"},
		{"trimmed like a reported one", "  Fix the parser \n", "Fix the parser"},
		{"the TUI's placeholder", "New Session", ""},
		{"the default session name", "Untitled Session", ""},
		{"a client-mode run", "non-interactive", ""},
		{"blank", "   ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cwd := plantCrushTitle(t, "crush-1", tt.title)
			got, ok := crushTitle("crush-1", cwd)
			if ok != (tt.want != "") || got != tt.want {
				t.Errorf("crushTitle = (%q, %v), want %q", got, ok, tt.want)
			}
		})
	}
	if _, ok := crushTitle("crush-1", t.TempDir()); ok {
		t.Error("a checkout without a database read as titled")
	}
	if _, ok := crushTitle("crush-1", ""); ok {
		t.Error("a session with no directory read as titled")
	}
}

// TestKiroMetadataTitle proves the name is taken from the `title` Kiro files
// beside the rest of a conversation's metadata, and that every way it can be
// absent reads as silence (a card keeping the name lich gave it) rather than as
// a title of "".
func TestKiroMetadataTitle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			"the title Kiro derived from the first prompt",
			`{"session_id":"s1","cwd":"/w","title":"Fix the flaky PTY test","session_state":{}}`,
			"Fix the flaky PTY test",
		},
		{"metadata written before the title", `{"session_id":"s1","cwd":"/w"}`, ""},
		{"a title Kiro filed empty", `{"title":""}`, ""},
		{"a title that is only spaces", `{"title":"   "}`, ""},
		{"surrounding space is trimmed", `{"title":"  Name it  "}`, "Name it"},
		// The file is written by another process, so a read can land mid-write.
		{"caught half-written", `{"session_id":"s1","tit`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "session.json")
			if err := os.WriteFile(path, []byte(tt.body), 0o600); err != nil {
				t.Fatalf("write session file: %v", err)
			}
			got, ok := metadataTitle(path)
			if ok != (tt.want != "") {
				t.Fatalf("metadataTitle = %q, %v, want ok=%v", got, ok, tt.want != "")
			}
			if got != tt.want {
				t.Errorf("metadataTitle = %q, want %q", got, tt.want)
			}
		})
	}
}

// A conversation whose metadata file is not there yet, or was pruned, is the
// common absence, and it must not reach the card as an error.
func TestKiroMetadataTitleAbsentFile(t *testing.T) {
	t.Parallel()
	if got, ok := metadataTitle(filepath.Join(t.TempDir(), "absent.json")); ok {
		t.Errorf("metadataTitle(absent) = %q, true, want a miss", got)
	}
}
