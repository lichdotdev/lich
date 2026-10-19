package project

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// helperEnv selects which half of the process tree a helper run plays.
const helperEnv = "LICH_WAITDELAY_HELPER"

// releaseEnv names the directory the grandchild watches: the test writes
// releaseFile there once it is done with the grandchild, which then exits and
// writes exitedFile. Left to sleep out its hold, the grandchild outlived a
// package that finished sooner, still running the test binary go test was
// deleting: "unlinkat ... project.test.exe: Access is denied" on Windows.
const (
	releaseEnv  = "LICH_WAITDELAY_RELEASE"
	releaseFile = "release"
	exitedFile  = "exited"
)

// grandchildHold is how long the grandchild holds the pipe at most, far past
// any budget the parent could set; pollEvery is how often it and the test look
// for each other's file.
const (
	grandchildHold = 10 * time.Second
	pollEvery      = 20 * time.Millisecond
)

// TestSubprocessHelper is not a test of anything: it is the child and the
// grandchild TestSubprocessBudgetOutlivesAGrandchild spawns, re-executing this
// binary because a shell one-liner would not run on Windows. As "child" it
// starts a grandchild on its own stdout — the pipe the parent is reading — and
// exits at once; as "grandchild" it holds that pipe open until the test
// releases it. A plain run does nothing.
func TestSubprocessHelper(t *testing.T) {
	t.Parallel()
	switch os.Getenv(helperEnv) {
	case "child":
		grandchild := exec.Command(os.Args[0], "-test.run=TestSubprocessHelper")
		grandchild.Env = append(os.Environ(), helperEnv+"=grandchild")
		grandchild.Stdout = os.Stdout
		if err := grandchild.Start(); err != nil {
			t.Fatalf("start grandchild: %v", err)
		}
		os.Exit(0)
	case "grandchild":
		dir := os.Getenv(releaseEnv)
		awaitFile(filepath.Join(dir, releaseFile), grandchildHold)
		if err := os.WriteFile(filepath.Join(dir, exitedFile), nil, 0o644); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
}

// awaitFile reports whether path appeared within limit.
func awaitFile(path string, limit time.Duration) bool {
	for deadline := time.Now().Add(limit); time.Now().Before(deadline); time.Sleep(pollEvery) {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// A budget is only real if it bounds the call rather than the child. The child
// here exits immediately and the grandchild behind it keeps stdout open until
// released: with nothing closing that pipe, Output waits for it and reports
// success, deadline or none.
func TestSubprocessBudgetOutlivesAGrandchild(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	t.Cleanup(func() {
		if err := os.WriteFile(filepath.Join(dir, releaseFile), nil, 0o644); err != nil {
			t.Fatalf("release the grandchild: %v", err)
		}
		if !awaitFile(filepath.Join(dir, exitedFile), grandchildHold) {
			t.Fatal("the grandchild did not exit once released")
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	cmd := commandContext(ctx, os.Args[0], "-test.run=TestSubprocessHelper")
	cmd.Env = append(cmd.Env, helperEnv+"=child", releaseEnv+"="+dir)

	start := time.Now()
	_, err := cmd.Output()
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("Output blocked %s on a grandchild holding the pipe, err %v: the 200ms budget bounded the child alone", elapsed, err)
	}
}
