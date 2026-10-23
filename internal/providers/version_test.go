package providers

import (
	"context"
	"errors"
	"github.com/lichdotdev/lich/api"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseVersionReadsEachProvidersSpelling(t *testing.T) {
	// What the eight print for --version, measured on the versions named.
	cases := map[string]string{
		"2.1.299 (Claude Code)\n": "2.1.299",
		"codex-cli 0.162.1\n":     "0.162.1",
		"1.3.5\n":                 "1.3.5",
		"1.18.35\n":               "1.18.35",
		"omp/18.8.9\n":            "18.8.9",
		"crush version v0.98.1\n": "0.98.1",
		"2026.10.01-e373342\n":    "2026.10.01-e373342",
		"kiro-cli 2.21.0\n":       "2.21.0",
		"\nWARNING: no helper under /tmp/x\ncodex-cli 0.147.0\n": "0.147.0",
		"usage: whatever\n": "",
	}
	for out, want := range cases {
		if got := parseVersion([]byte(out)); got != want {
			t.Errorf("parseVersion(%q) = %q, want %q", out, got, want)
		}
	}
}

// pathMachine is a $PATH of dirs whose files are the keys of found, each
// printing its version, so OnPath and Version run without touching the
// machine.
func pathMachine(t *testing.T, dirs []string, found map[string]string) *Service {
	t.Helper()
	t.Setenv("PATH", strings.Join(dirs, string(os.PathListSeparator)))
	return &Service{
		lookPath: func(name string) (string, error) {
			if _, ok := found[name]; ok {
				return name, nil
			}
			return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
		},
		runVersion: func(_ context.Context, path string) ([]byte, []byte, error) {
			v := found[path]
			if v == "" {
				return nil, nil, errors.New("exit status 1")
			}
			return []byte("codex-cli " + v + "\n"), nil, nil
		},
	}
}

func TestOnPathListsEveryCodexInPathOrderWithItsVersion(t *testing.T) {
	local, mise, usr := absolute("home", "u", ".local", "bin"), absolute("home", "u", "mise", "shims"), absolute("usr", "bin")
	svc := pathMachine(t, []string{local, mise, usr}, map[string]string{
		filepath.Join(local, "codex"): "0.162.1",
		filepath.Join(mise, "codex"):  "0.144.5",
		filepath.Join(usr, "codex"):   "",
	})

	got, err := svc.OnPath(context.Background(), api.ProviderIDOptions{ID: Codex})
	if err != nil {
		t.Fatal(err)
	}
	want := []Binary{
		{Path: filepath.Join(local, "codex"), Version: "0.162.1"},
		{Path: filepath.Join(mise, "codex"), Version: "0.144.5"},
		// A probe that fails keeps the binary: the spawn can still find it.
		{Path: filepath.Join(usr, "codex"), Version: ""},
	}
	if len(got) != len(want) {
		t.Fatalf("OnPath = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("OnPath[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestOnPathRefusesAnUnknownProvider(t *testing.T) {
	svc := pathMachine(t, []string{absolute("bin")}, nil)
	if _, err := svc.OnPath(context.Background(), api.ProviderIDOptions{ID: "vim"}); err == nil {
		t.Error("OnPath(unknown) = nil error")
	}
}

func TestVersionProbesWhatTheSpawnWouldRun(t *testing.T) {
	bin := absolute("opt", "codex")
	svc := pathMachine(t, []string{absolute("bin")}, map[string]string{bin: "0.150.0"})

	if v, err := svc.Version(context.Background(), api.BinaryOptions{Bin: bin}); err != nil || v != "0.150.0" {
		t.Errorf("Version(%s) = %q, %v", bin, v, err)
	}
	if _, err := svc.Version(context.Background(), api.BinaryOptions{Bin: "~/codex"}); err == nil {
		t.Error("Version of a binary the spawn cannot resolve answered")
	}
}

func TestVersionReadsStderrWhenStdoutHasNone(t *testing.T) {
	bin := absolute("opt", "codex")
	svc := pathMachine(t, nil, map[string]string{bin: "x"})
	svc.runVersion = func(context.Context, string) ([]byte, []byte, error) {
		return nil, []byte("codex-cli 0.150.0\n"), nil
	}
	if v, err := svc.Version(context.Background(), api.BinaryOptions{Bin: bin}); err != nil || v != "0.150.0" {
		t.Errorf("Version = %q, %v", v, err)
	}
}

func TestVersionSaysWhenNothingVersionShapedCameBack(t *testing.T) {
	bin := absolute("opt", "codex")
	svc := pathMachine(t, nil, map[string]string{bin: "x"})
	svc.runVersion = func(context.Context, string) ([]byte, []byte, error) {
		return []byte("usage: codex\n"), []byte("try --help\n"), nil
	}
	if _, err := svc.Version(context.Background(), api.BinaryOptions{Bin: bin}); err == nil {
		t.Error("Version = nil error for output with no version in it")
	}
}
