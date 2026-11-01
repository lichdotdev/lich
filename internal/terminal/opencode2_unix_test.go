// Fake opencode binaries are shell scripts.
//go:build !windows

package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// fakeOpencode writes an opencode that answers --version with version, a fork
// with forkAnswer, and otherwise records its argv and environment in argvFile
// and stays up like a session.
func fakeOpencode(t *testing.T, version, forkAnswer string) (bin, argvFile string) {
	t.Helper()
	dir := t.TempDir()
	bin, argvFile = filepath.Join(dir, "opencode"), filepath.Join(dir, "argv")
	script := `#!/bin/sh
case "$1" in
  --version) echo '` + version + `'; exit 0 ;;
  api) echo "$@ $PWD" > '` + argvFile + `.fork'; echo '` + forkAnswer + `'; exit 0 ;;
esac
{ echo "$@"; echo "$OPENCODE_CONFIG_CONTENT"; } > '` + argvFile + `.tmp'
mv '` + argvFile + `.tmp' '` + argvFile + `'
sleep 30
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, argvFile
}

func readSpawn(t *testing.T, path string) (argv, config string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if data, err := os.ReadFile(path); err == nil {
			argv, config, _ = strings.Cut(strings.TrimSpace(string(data)), "\n")
			return argv, config
		}
		if time.Now().After(deadline) {
			t.Fatalf("the fake opencode never recorded its spawn at %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestIsOpencode2ReadsTheVersion(t *testing.T) {
	v2, _ := fakeOpencode(t, "opencode v2.2.5", "")
	v1, _ := fakeOpencode(t, "1.19.4", "")
	if !isOpencode2(v2) || isOpencode2(v1) {
		t.Errorf("isOpencode2: 2.2.5 = %v, 1.19.4 = %v", isOpencode2(v2), isOpencode2(v1))
	}
	if isOpencode2(filepath.Join(t.TempDir(), "absent")) {
		t.Error("a binary that is not there read as opencode 2")
	}
}

func startOpencode(t *testing.T, bins stubBins, opts api.StartOptions) {
	t.Helper()
	svc := New(bins, nil, events.New())
	t.Cleanup(func() { _, _ = svc.Close(t.Context(), api.SessionIDOptions{ID: opts.ID}) })
	opts.ProjectID, opts.Kind, opts.Cols, opts.Rows = "p1", "opencode", 80, 24
	if _, err := svc.Start(t.Context(), opts); err != nil {
		t.Fatalf("Start = %v", err)
	}
}

func TestOpencode2SpawnsStandaloneWithTheModelInItsConfig(t *testing.T) {
	bin, argv := fakeOpencode(t, "opencode v2.2.5", "")
	startOpencode(t, stubBins{bin: bin, model: "opencode/glm-5.3-flash-free"}, api.StartOptions{ID: "s1", Cwd: t.TempDir()})
	args, config := readSpawn(t, argv)
	if !strings.HasPrefix(args, "--standalone") || strings.Contains(args, "--model") {
		t.Errorf("argv = %q, want --standalone first and no --model", args)
	}
	if !strings.Contains(config, `"model":"opencode/glm-5.3-flash-free"`) {
		t.Errorf("OPENCODE_CONFIG_CONTENT = %q, want the model in it", config)
	}
}

func TestOpencode1SpawnKeepsItsFlags(t *testing.T) {
	bin, argv := fakeOpencode(t, "1.19.4", "")
	startOpencode(t, stubBins{bin: bin, model: "opencode/glm-5.3-flash-free"},
		api.StartOptions{ID: "s1", Cwd: t.TempDir(), Resume: "ses_parent", Fork: true})
	args, _ := readSpawn(t, argv)
	if args != "--session ses_parent --fork" {
		t.Errorf("argv = %q, want the 1.x fork", args)
	}
}

func TestOpencode2ForksThroughItsOwnRouteThenResumesTheCopy(t *testing.T) {
	bin, argv := fakeOpencode(t, "opencode v2.2.5", `{"data":{"id":"ses_copy"}}`)
	cwd := t.TempDir()
	startOpencode(t, stubBins{bin: bin}, api.StartOptions{ID: "s1", Cwd: cwd, Resume: "ses_parent", Fork: true})
	fork, err := os.ReadFile(argv + ".fork")
	if err != nil {
		t.Fatal(err)
	}
	if want := "api --standalone POST /api/session/ses_parent/fork --data {} " + cwd; strings.TrimSpace(string(fork)) != want {
		t.Errorf("fork call = %q, want %q", fork, want)
	}
	if args, _ := readSpawn(t, argv); args != "--standalone --session ses_copy" {
		t.Errorf("argv = %q, want a resume of the copy", args)
	}
}

func TestOpencode2ForkThatFailsStopsTheSpawn(t *testing.T) {
	bin, argv := fakeOpencode(t, "opencode v2.2.5", `{"error":"SessionNotFoundError"}`)
	svc := New(stubBins{bin: bin}, nil, events.New())
	_, err := svc.Start(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Kind: "opencode",
		Cwd: t.TempDir(), Resume: "ses_gone", Fork: true, Cols: 80, Rows: 24})
	if err == nil {
		t.Fatal("Start = nil, want the fork failure: resuming the parent would put two cards on it")
	}
	if _, statErr := os.Stat(argv); statErr == nil {
		t.Error("the session spawned after its fork failed")
	}
}
