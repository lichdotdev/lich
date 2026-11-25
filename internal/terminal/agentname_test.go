package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestClaudeAgentNameReadsTheLastRecord walks the shapes a Claude transcript
// actually carries (measured on 2.1.263): `custom-title` and `agent-name` are
// both written at spawn under the name lich passed, and `agent-name` is written
// again every turn and on every /rename. The name a session answers to is
// therefore the last record in the file, whichever of the two it is.
func TestClaudeAgentNameReadsTheLastRecord(t *testing.T) {
	t.Parallel()
	const (
		head = `{"type":"last-prompt","leafUuid":"a"}
{"type":"custom-title","customTitle":"lich-4f2a"}
{"type":"agent-name","agentName":"lich-4f2a"}
{"type":"mode","mode":"normal"}`
		turn = `{"type":"assistant","message":{"content":[{"type":"text","text":"done"}]}}`
	)
	tests := []struct {
		name string
		tail string
		want string
	}{
		{
			name: "named at birth and never renamed",
			tail: head + "\n" + turn,
			want: "lich-4f2a",
		},
		{
			name: "one rename",
			tail: head + "\n" + turn + "\n" + `{"type":"agent-name","agentName":"reviewer"}`,
			want: "reviewer",
		},
		{
			name: "several renames, the last one wins",
			tail: strings.Join([]string{
				head,
				`{"type":"agent-name","agentName":"reviewer"}`,
				turn,
				`{"type":"agent-name","agentName":"packager"}`,
				turn,
				`{"type":"agent-name","agentName":"shipper"}`,
			}, "\n"),
			want: "shipper",
		},
		{
			name: "a fork's copied history loses to the name written behind it",
			tail: strings.Join([]string{
				`{"type":"custom-title","customTitle":"parent-9c1b"}`,
				`{"type":"agent-name","agentName":"parent-9c1b"}`,
				turn,
				`{"type":"custom-title","customTitle":"lich-4f2a"}`,
				`{"type":"agent-name","agentName":"lich-4f2a"}`,
			}, "\n"),
			want: "lich-4f2a",
		},
		{
			name: "nothing on record",
			tail: turn + "\n" + turn,
			want: "",
		},
		{
			name: "an empty name is not a name",
			tail: `{"type":"agent-name","agentName":"reviewer"}` + "\n" + `{"type":"agent-name","agentName":""}`,
			want: "reviewer",
		},
		{
			name: "a tail cut mid-line reads like any malformed one",
			tail: `,"agentName":"half-a-line"}` + "\n" + `{"type":"agent-name","agentName":"reviewer"}`,
			want: "reviewer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := claudeAgentName([]byte(tt.tail + "\n")); got != tt.want {
				t.Errorf("claudeAgentName = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestAgentNameKeepsTheNameAcrossAppendsWithoutOne walks one transcript the way
// the roster reads it, call after call: each read sees only what was appended
// since, so a turn that writes no name record keeps the one on record, a
// /rename replaces it, and a turn larger than the tail bound loses nothing.
func TestAgentNameKeepsTheNameAcrossAppendsWithoutOne(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "conversation.jsonl")
	write := func(text string) {
		t.Helper()
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.WriteString(text); err != nil {
			t.Fatal(err)
		}
	}
	turn := `{"type":"assistant","message":{"content":[{"type":"text","text":"done"}]}}`
	var names nameCursors
	steps := []struct {
		name, append, want string
	}{
		{"named at birth", `{"type":"agent-name","agentName":"lich-4f2a"}` + "\n" + turn + "\n", "lich-4f2a"},
		{"a turn with no record", turn + "\n" + turn + "\n", "lich-4f2a"},
		{"a rename", `{"type":"agent-name","agentName":"reviewer"}` + "\n", "reviewer"},
		{"a turn past the tail bound", `{"type":"user","text":"` + strings.Repeat("x", searchTailBytes) + `"}` + "\n" + turn + "\n", "reviewer"},
		{"half a record still being written", `{"type":"agent-name","agentName":"shi`, "reviewer"},
		{"the rest of that record", `pper"}` + "\n", "shipper"},
	}
	for _, step := range steps {
		write(step.append)
		if got := names.agentName("s1", path); got != step.want {
			t.Fatalf("after %s: agentName = %q, want %q", step.name, got, step.want)
		}
	}
}
