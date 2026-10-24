//go:build !windows

package terminal

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

type serviceRun struct {
	code        chan int
	tail        chan []byte
	interrupted atomic.Bool
}

func startTestService(t *testing.T, svc *Service, id, line string) *serviceRun {
	t.Helper()
	run := &serviceRun{code: make(chan int, 1), tail: make(chan []byte, 1)}
	err := svc.StartService(ServiceCommand{
		ID: id, Cwd: t.TempDir(), Line: line,
		Interrupted: func() { run.interrupted.Store(true) },
		Exited: func(code int, tail []byte) {
			run.code <- code
			run.tail <- tail
		},
	})
	if err != nil {
		t.Fatalf("StartService: %v", err)
	}
	return run
}

func (r *serviceRun) wait(t *testing.T) (int, string) {
	t.Helper()
	select {
	case code := <-r.code:
		return code, string(<-r.tail)
	case <-time.After(10 * time.Second):
		t.Fatal("the service never exited")
		return 0, ""
	}
}

func TestServiceExitReportsItsCodeAndOutput(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	svc := New(stubBins{}, nil, events.New())
	code, tail := startTestService(t, svc, "svc-1", "echo up; exit 3").wait(t)
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
	if !strings.Contains(tail, "up") {
		t.Errorf("tail = %q, want the command's output", tail)
	}
	waitFor(t, func() bool { return svc.ptyOf("svc-1") == nil }, "the service PTY to be dropped")
}

func TestStartServiceRefusesARunningID(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	svc := New(stubBins{}, nil, events.New())
	run := startTestService(t, svc, "svc-1", "sleep 30")
	if err := svc.StartService(ServiceCommand{ID: "svc-1", Cwd: t.TempDir(), Line: "true"}); err == nil {
		t.Error("a second start of a running service succeeded")
	}
	svc.KillService("svc-1")
	run.wait(t)
}

func TestInterruptServiceStopsTheCommand(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	svc := New(stubBins{}, nil, events.New())
	run := startTestService(t, svc, "svc-1", "sleep 30")
	waitFor(t, func() bool { return svc.ptyOf("svc-1") != nil }, "the service to run")
	time.Sleep(200 * time.Millisecond)
	if err := svc.InterruptService("svc-1"); err != nil {
		t.Fatal(err)
	}
	run.wait(t)
	if run.interrupted.Load() {
		t.Error("an interrupt lich sent was reported as one the user typed")
	}
}

func TestCtrlCTypedAtTheConsoleIsReported(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	svc := New(stubBins{}, nil, events.New())
	run := startTestService(t, svc, "svc-1", "sleep 30")
	time.Sleep(200 * time.Millisecond)
	svc.onInput("svc-1", []byte{interruptByte})
	run.wait(t)
	if !run.interrupted.Load() {
		t.Error("Ctrl+C at the console was not reported")
	}
}

func TestKillServiceEndsACommandThatIgnoresInterrupt(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	svc := New(stubBins{}, nil, events.New())
	run := startTestService(t, svc, "svc-1", "trap '' INT TERM; while :; do sleep 1; done")
	time.Sleep(200 * time.Millisecond)
	_ = svc.InterruptService("svc-1")
	svc.KillService("svc-1")
	code, _ := run.wait(t)
	if code >= 0 {
		t.Errorf("exit code = %d, want none for a killed process", code)
	}
}

func TestServicesAreNotCountedAsSessions(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	svc := New(stubBins{}, nil, events.New())
	run := startTestService(t, svc, "svc-1", "sleep 30")
	if n, _ := svc.LiveCount(t.Context(), api.None{}); n != 0 {
		t.Errorf("LiveCount = %d with only a service running", n)
	}
	for _, ts := range svc.turnStates() {
		if ts.ID == "svc-1" {
			t.Error("a service is in the turn-state catch-up")
		}
	}
	svc.KillService("svc-1")
	run.wait(t)
}
