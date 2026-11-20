package providers

import (
	"bufio"
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

// installHelperVar makes the test binary act as an installer: the subprocess
// at the boundary, with the output and the exit each case needs.
const installHelperVar = "LICH_TEST_INSTALLER"

func TestInstallHelperProcess(t *testing.T) {
	switch os.Getenv(installHelperVar) {
	case "":
		return
	case "ok":
		fmt.Println("Downloading the CLI")
		fmt.Print("10%\r50%\r100%\r\n")
		fmt.Println("Installed")
		os.Exit(0)
	case "fail":
		fmt.Println("Downloading the CLI")
		fmt.Fprintln(os.Stderr, "curl: (6) Could not resolve host: downloads.example.com")
		fmt.Println()
		os.Exit(1)
	case "silent-fail":
		os.Exit(3)
	case "slow":
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}
}

// fakeInstaller is a Service whose installer is this test binary in mode, and
// whose PATH answers every name but those in missing.
func fakeInstaller(t *testing.T, mode string, missing ...string) (*Service, <-chan Install) {
	t.Helper()
	t.Setenv(installHelperVar, mode)
	steps := make(chan Install, 64)
	s := &Service{
		lookPath: func(name string) (string, error) {
			if slices.Contains(missing, name) {
				return "", exec.ErrNotFound
			}
			return "/fake/bin/" + name, nil
		},
		newInstall: func(ctx context.Context, _ []string) *exec.Cmd {
			return exec.CommandContext(ctx, os.Args[0], "-test.run=^TestInstallHelperProcess$")
		},
	}
	s.SetEmit(func(name string, data any) {
		if name == api.ProviderInstallEvent.Name {
			steps <- data.(Install)
		}
	})
	return s, steps
}

// withRecipe gives id a recipe on this OS for the test's length.
func withRecipe(t *testing.T, id string, r recipe) {
	t.Helper()
	saved := recipes[hostOS]
	mine := map[string]recipe{}
	maps.Copy(mine, saved)
	mine[id] = r
	recipes[hostOS] = mine
	t.Cleanup(func() { recipes[hostOS] = saved })
}

var testRecipe = recipe{needs: []string{"bash", "curl"}, args: []string{"bash", "-c", "true"}}

// untilDone collects the steps of one install up to its last.
func untilDone(t *testing.T, steps <-chan Install) []Install {
	t.Helper()
	var got []Install
	timeout := time.After(30 * time.Second)
	for {
		select {
		case step := <-steps:
			got = append(got, step)
			if step.Done {
				return got
			}
		case <-timeout:
			t.Fatalf("no last step; got %+v", got)
		}
	}
}

func TestInstallReportsProgressThenDone(t *testing.T) {
	s, steps := fakeInstaller(t, "ok")
	withRecipe(t, Claude, testRecipe)
	if _, err := s.Install(t.Context(), api.ProviderIDOptions{ID: Claude}); err != nil {
		t.Fatal(err)
	}
	got := untilDone(t, steps)
	last := got[len(got)-1]
	if last != (Install{ID: Claude, Progress: 1, Done: true}) {
		t.Fatalf("last step = %+v, want done at 1 with no error", last)
	}
	if len(got) < 3 {
		t.Fatalf("steps = %+v, want progress before the end", got)
	}
	prev := 0.0
	for _, step := range got[:len(got)-1] {
		if step.Done || step.Progress <= prev || step.Progress >= installCeiling {
			t.Fatalf("step %+v after %v: want a rising estimate under %v", step, prev, installCeiling)
		}
		prev = step.Progress
	}
}

func TestInstallFailureEndsOnTheInstallersLastLine(t *testing.T) {
	s, steps := fakeInstaller(t, "fail")
	withRecipe(t, Claude, testRecipe)
	if _, err := s.Install(t.Context(), api.ProviderIDOptions{ID: Claude}); err != nil {
		t.Fatal(err)
	}
	got := untilDone(t, steps)
	want := "curl: (6) Could not resolve host: downloads.example.com"
	if last := got[len(got)-1]; last.Error != want {
		t.Fatalf("error = %q, want %q", last.Error, want)
	}
}

func TestInstallFailureWithNoOutputSaysHowItExited(t *testing.T) {
	s, steps := fakeInstaller(t, "silent-fail")
	withRecipe(t, Claude, testRecipe)
	if _, err := s.Install(t.Context(), api.ProviderIDOptions{ID: Claude}); err != nil {
		t.Fatal(err)
	}
	got := untilDone(t, steps)
	if last := got[len(got)-1]; last.Error != "exit status 3" {
		t.Fatalf("error = %q, want the exit status", last.Error)
	}
}

func TestInstallRetriesAfterAFailure(t *testing.T) {
	s, steps := fakeInstaller(t, "fail")
	withRecipe(t, Claude, testRecipe)
	if _, err := s.Install(t.Context(), api.ProviderIDOptions{ID: Claude}); err != nil {
		t.Fatal(err)
	}
	untilDone(t, steps)
	t.Setenv(installHelperVar, "ok")
	if _, err := s.Install(t.Context(), api.ProviderIDOptions{ID: Claude}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	got := untilDone(t, steps)
	if last := got[len(got)-1]; last.Error != "" {
		t.Fatalf("retry ended on %q", last.Error)
	}
}

func TestInstallRunsOneAtATimePerProvider(t *testing.T) {
	s, steps := fakeInstaller(t, "slow")
	withRecipe(t, Claude, testRecipe)
	if _, err := s.Install(t.Context(), api.ProviderIDOptions{ID: Claude}); err != nil {
		t.Fatal(err)
	}
	_, err := s.Install(t.Context(), api.ProviderIDOptions{ID: Claude})
	if err == nil || !strings.Contains(err.Error(), "already installing") {
		t.Fatalf("second install: %v, want already installing", err)
	}
	untilDone(t, steps)
}

func TestInstallNeedsItsPrerequisites(t *testing.T) {
	s, _ := fakeInstaller(t, "ok", "curl")
	withRecipe(t, Claude, testRecipe)
	_, err := s.Install(t.Context(), api.ProviderIDOptions{ID: Claude})
	if err == nil || err.Error() != "installing Claude Code needs curl, which is not on PATH" {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallThatLeavesTheBinaryOffPathFails(t *testing.T) {
	s, steps := fakeInstaller(t, "ok", "claude")
	withRecipe(t, Claude, testRecipe)
	if _, err := s.Install(t.Context(), api.ProviderIDOptions{ID: Claude}); err != nil {
		t.Fatal(err)
	}
	got := untilDone(t, steps)
	if last := got[len(got)-1]; last.Error != "Claude Code installed, but claude is not on PATH" {
		t.Fatalf("error = %q", last.Error)
	}
}

func TestInstallRefusesWhatItCannotRun(t *testing.T) {
	s, _ := fakeInstaller(t, "ok")
	if _, err := s.Install(t.Context(), api.ProviderIDOptions{ID: "nope"}); err == nil {
		t.Error("an unknown provider installed")
	}
	saved := recipes[hostOS]
	recipes[hostOS] = map[string]recipe{}
	t.Cleanup(func() { recipes[hostOS] = saved })
	if _, err := s.Install(t.Context(), api.ProviderIDOptions{ID: Claude}); err != errNoInstall {
		t.Errorf("no recipe: err = %v, want errNoInstall", err)
	}
}

func TestInstallEnvSilencesEveryPrompter(t *testing.T) {
	env := installEnv([]string{"HOME=/home/x"})
	for _, want := range []string{"SUDO_ASKPASS=", "SSH_ASKPASS=", "GIT_ASKPASS=", "GIT_TERMINAL_PROMPT=0"} {
		found := false
		for _, kv := range env {
			found = found || kv == want
		}
		if !found {
			t.Errorf("env %v lacks %s", env, want)
		}
	}
}

func TestInstallProgressRisesUnderTheCeiling(t *testing.T) {
	prev := 0.0
	for lines := 1; lines <= 10000; lines *= 2 {
		p := installProgress(lines)
		if p <= prev || p >= installCeiling {
			t.Fatalf("installProgress(%d) = %v after %v", lines, p, prev)
		}
		prev = p
	}
	if got := installProgress(installHalfLines); got != installCeiling/2 {
		t.Errorf("installProgress(%d) = %v, want half the ceiling", installHalfLines, got)
	}
}

func TestScanLinesEndsALineAtACarriageReturn(t *testing.T) {
	scan := bufio.NewScanner(strings.NewReader("a\r\nb\rc\nd"))
	scan.Split(scanLines)
	var got []string
	for scan.Scan() {
		got = append(got, scan.Text())
	}
	if strings.Join(got, "|") != "a||b|c|d" {
		t.Fatalf("lines = %q", got)
	}
}

func TestClipKeepsUTF8Whole(t *testing.T) {
	if got := clip("ação", 2); got != "a…" {
		t.Errorf("clip = %q", got)
	}
	if got := clip("short", 10); got != "short" {
		t.Errorf("clip = %q", got)
	}
}
