package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func showCodeRig(t *testing.T) (*Service, string, *[]string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "a.go"), []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(nil)
	s.SetSessionCheckout(func(id string) string {
		if id == "s1" {
			return dir
		}
		return ""
	})
	var emitted []string
	s.SetEmit(func(name string, _ any) { emitted = append(emitted, name) })
	return s, dir, &emitted
}

func TestShowCodeResolvesAndEmits(t *testing.T) {
	s, dir, emitted := showCodeRig(t)
	for _, path := range []string{"src/a.go", "./src/../src/a.go", filepath.Join(dir, "src", "a.go")} {
		got, err := s.ShowCode(t.Context(), api.ShowCodeOptions{ID: "s1", Path: path, Line: 2, EndLine: 3})
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		want := api.ShowCode{SessionID: "s1", Rel: "src/a.go", Line: 2, EndLine: 3}
		if got != want {
			t.Errorf("%s: got %+v, want %+v", path, got, want)
		}
	}
	if len(*emitted) != 3 || (*emitted)[0] != api.ShowCodeEvent.Name {
		t.Errorf("emitted %v", *emitted)
	}
}

func TestShowCodeRefusesWhatThePreviewCouldNotShow(t *testing.T) {
	s, dir, emitted := showCodeRig(t)
	outside := filepath.Join(t.TempDir(), "x.go")
	if err := os.WriteFile(outside, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		opts api.ShowCodeOptions
		want string
	}{
		{"gone session", api.ShowCodeOptions{ID: "s2", Path: "src/a.go"}, "not running"},
		{"missing file", api.ShowCodeOptions{ID: "s1", Path: "src/b.go"}, "no such file in this checkout: src/b.go"},
		{"missing absolute", api.ShowCodeOptions{ID: "s1", Path: filepath.Join(dir, "b.go")}, "no such file"},
		{"parent", api.ShowCodeOptions{ID: "s1", Path: "../x.go"}, "outside this session's checkout"},
		{"absolute outside", api.ShowCodeOptions{ID: "s1", Path: outside}, "outside this session's checkout"},
		{"past the end", api.ShowCodeOptions{ID: "s1", Path: "src/a.go", Line: 4}, "line 4 is past the end of src/a.go (3 lines)"},
		{"end past the end", api.ShowCodeOptions{ID: "s1", Path: "src/a.go", Line: 1, EndLine: 9}, "line 9 is past the end"},
		{"inverted", api.ShowCodeOptions{ID: "s1", Path: "src/a.go", Line: 3, EndLine: 2}, "end_line 2 is before line 3"},
		{"end alone", api.ShowCodeOptions{ID: "s1", Path: "src/a.go", EndLine: 2}, "end_line needs line"},
		{"negative", api.ShowCodeOptions{ID: "s1", Path: "src/a.go", Line: -1}, "start at 1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := s.ShowCode(t.Context(), c.opts)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want %q", err, c.want)
			}
		})
	}
	if len(*emitted) != 0 {
		t.Errorf("a refused request reached the window: %v", *emitted)
	}
}

func TestShowCodeCleansAndLimitsTheComment(t *testing.T) {
	s, _, emitted := showCodeRig(t)
	cases := []struct {
		name, comment, want, err string
		line                     int
	}{
		{name: "kept", comment: "why", want: "why", line: 1},
		{name: "trimmed", comment: "  why\n", want: "why", line: 1},
		{name: "whitespace is none", comment: " \n ", want: "", line: 1},
		{name: "none without line", comment: "", want: "", line: 0},
		{name: "tabs and crlf", comment: "a\tb\r\nc", want: "a    b\nc", line: 1},
		{name: "at the limit", comment: strings.Repeat("é", 360), want: strings.Repeat("é", 360), line: 1},
		{name: "past the limit", comment: strings.Repeat("é", 361), err: "comment is 361 characters; the limit is 360", line: 1},
		{name: "needs line", comment: "why", err: "comment needs line", line: 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := s.ShowCode(t.Context(), api.ShowCodeOptions{ID: "s1", Path: "src/a.go", Line: c.line, Comment: c.comment})
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatalf("got %v, want %q", err, c.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Comment != c.want {
				t.Errorf("comment = %q, want %q", got.Comment, c.want)
			}
		})
	}
	if len(*emitted) != 6 {
		t.Errorf("emitted %d, want the 6 accepted", len(*emitted))
	}
}

func TestLineCountMatchesSliceLines(t *testing.T) {
	for _, text := range []string{"", "a", "a\n", "a\nb", "a\nb\n", "\n", "\n\n"} {
		if got, want := lineCount(text), len(sliceLines(text, 1, 400)); got != want {
			t.Errorf("%q: lineCount %d, sliceLines %d", text, got, want)
		}
	}
}
