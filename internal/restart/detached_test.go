package restart

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// helperEnv turns the test binary into the native window a test launches.
const helperEnv = "LICH_RESTART_TEST_HELPER"

// helperOutput is what the helper window prints before it exits.
const helperOutput = "event loop stalled for 4s"

// helperExitCode is the code the helper window exits with.
const helperExitCode = 3

func TestMain(m *testing.M) {
	switch os.Getenv(helperEnv) {
	case "exit":
		fmt.Fprintln(os.Stderr, helperOutput)
		os.Exit(helperExitCode)
	case "hang":
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// captureLog points slog's default logger at a buffer for the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	return &buf
}

// startHelper launches the helper window in mode and watches it; watched
// closes once WatchNative logged its exit.
func startHelper(t *testing.T, mode string) (logged *bytes.Buffer, dir string, cmd *exec.Cmd, watched <-chan struct{}) {
	t.Helper()
	logged = captureLog(t)
	dir = t.TempDir()
	cmd, err := StartNative(os.Args[0], append(os.Environ(), helperEnv+"="+mode), dir)
	if err != nil {
		t.Fatalf("StartNative: %v", err)
	}
	done := make(chan struct{})
	go func() {
		WatchNative(cmd)
		close(done)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-done
	})
	return logged, dir, cmd, done
}

// TestStartNativeLogsWhatTheWindowPrintsAndHowItExited proves a native window
// started detached prints into the native log, and its exit code reaches the
// backend's log once it exits.
func TestStartNativeLogsWhatTheWindowPrintsAndHowItExited(t *testing.T) {
	t.Setenv("LICH_DEV", "")
	logged, dir, _, watched := startHelper(t, "exit")
	<-watched

	printed, err := os.ReadFile(filepath.Join(dir, "lich-native.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(printed), helperOutput) {
		t.Errorf("native log = %q, want what the window printed", printed)
	}
	line := logged.String()
	for _, want := range []string{"level=WARN", "native window exited", fmt.Sprintf("code=%d", helperExitCode)} {
		if !strings.Contains(line, want) {
			t.Errorf("backend log = %q, want %q", line, want)
		}
	}
}

// TestStartNativeFailsWithoutItsLog proves a native log that cannot be opened
// stops the launch with the reason, rather than a window that logs nowhere.
func TestStartNativeFailsWithoutItsLog(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := StartNative(os.Args[0], os.Environ(), notADir); err == nil {
		t.Fatal("StartNative under a file instead of a directory succeeded")
	}
}
