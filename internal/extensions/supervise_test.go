package extensions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

func TestBackoffDoublesToItsCeiling(t *testing.T) {
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 8 * time.Second}
	for i, expected := range want {
		if got := backoff(i+1, time.Second, 8*time.Second); got != expected {
			t.Errorf("backoff(%d) = %s, want %s", i+1, got, expected)
		}
	}
}

func TestCrashLoopGivesUpAndReportsFailed(t *testing.T) {
	dir := t.TempDir()
	root := install(t, dir, "ext", modeCrash)
	s, r := boot(t, dir)

	waitStatus(t, r, api.ExtensionFailed)
	if got := runCount(root); got != maxQuickCrashes {
		t.Errorf("ran %d times, want %d", got, maxQuickCrashes)
	}
	last := r.lastStatus()
	if !strings.Contains(last.Error, fmt.Sprintf("%d times", maxQuickCrashes)) || !strings.Contains(last.Error, "exit status 3") {
		t.Errorf("failed status error = %q", last.Error)
	}
	if !slices.Contains(r.statusNames(), api.ExtensionRestarting) {
		t.Errorf("statuses %v never showed restarting", r.statusNames())
	}
	if _, err := s.Send(context.Background(), sendOptions(t, "ext", extension.SurfaceOpened.New(extension.SurfaceData{Surface: "main"}))); err == nil || !strings.Contains(err.Error(), "not running") {
		t.Errorf("Send to a failed extension: %v", err)
	}
}

func TestCrashedExtensionComesBack(t *testing.T) {
	dir := t.TempDir()
	root := install(t, dir, "ext", modeCrashTwice)
	s, r := boot(t, dir)

	waitUntil(t, "the third run", func() bool { return runCount(root) == 3 })
	waitStatus(t, r, api.ExtensionRunning)
	if _, err := s.Send(context.Background(), sendOptions(t, "ext", extension.SurfaceClosed.New(extension.SurfaceData{Surface: "main"}))); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the echo", func() bool { return slices.Contains(r.titles(t), "got surface.closed") })
}

func TestAStableRunStartsTheCrashCountOver(t *testing.T) {
	dir := t.TempDir()
	root := install(t, dir, "ext", modeCrash)
	s, r := boot(t, dir, func(c *timing) { c.stableRun = 0 })

	waitUntil(t, "more runs than the give-up count", func() bool { return runCount(root) > maxQuickCrashes+1 })
	if slices.Contains(r.statusNames(), api.ExtensionFailed) {
		t.Errorf("gave up on runs that were stable: %v", r.statusNames())
	}
	s.Shutdown()
	if st := r.lastStatus().Status; st != api.ExtensionStopped {
		t.Errorf("after Shutdown status = %q", st)
	}
}

func TestShutdownClosesStdinFirst(t *testing.T) {
	dir := t.TempDir()
	root := install(t, dir, "ext", modeEcho)
	s, r := boot(t, dir)
	waitStatus(t, r, api.ExtensionRunning)
	waitUntil(t, "the process to be up", func() bool { return runCount(root) == 1 })

	s.Shutdown()
	if st := r.lastStatus().Status; st != api.ExtensionStopped {
		t.Errorf("status = %q", st)
	}
	if _, err := os.Stat(filepath.Join(root, eofFile)); err != nil {
		t.Errorf("the extension never saw stdin close: %v", err)
	}
}

func TestShutdownKillsWhatIgnoresStdin(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, "ext", modeStubborn)
	grace := 100 * time.Millisecond
	s, r := boot(t, dir, func(c *timing) { c.stopGrace = grace })
	waitUntil(t, "the extension to be up", func() bool { return slices.Contains(r.titles(t), readyTitle) })

	started := time.Now()
	s.Shutdown()
	if elapsed := time.Since(started); elapsed < grace {
		t.Errorf("killed after %s, before its grace of %s", elapsed, grace)
	}
	if st := r.lastStatus().Status; st != api.ExtensionStopped {
		t.Errorf("status = %q; the process outlived Shutdown", st)
	}
}

func TestShutdownDuringBackoffDoesNotWaitItOut(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, "ext", modeCrash)
	s, r := boot(t, dir, func(c *timing) { c.firstBackoff, c.maxBackoff = time.Hour, time.Hour })
	waitStatus(t, r, api.ExtensionRestarting)

	s.Shutdown()
	if st := r.lastStatus().Status; st != api.ExtensionStopped {
		t.Errorf("status = %q", st)
	}
}

func TestSendRefusals(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, "ext", modeEcho)
	install(t, dir, "bad", modeEcho, func(m *extension.Manifest) { m.FormatVersion = 0 })
	s, r := boot(t, dir)
	waitStatus(t, r, api.ExtensionRunning)
	ctx := context.Background()
	opened := extension.SurfaceOpened.New(extension.SurfaceData{Surface: "main"})

	if _, err := s.Send(ctx, sendOptions(t, "nobody", opened)); err == nil || !strings.Contains(err.Error(), "no extension") {
		t.Errorf("unknown id: %v", err)
	}
	if _, err := s.Send(ctx, sendOptions(t, "bad", opened)); err == nil || !strings.Contains(err.Error(), "not running") {
		t.Errorf("invalid extension: %v", err)
	}
	toast := sendOptions(t, "ext", extension.ShowToast.New(extension.ToastData{Title: "t"}))
	if _, err := s.Send(ctx, toast); err == nil || !strings.Contains(err.Error(), "cannot be sent") {
		t.Errorf("refused kind: %v", err)
	}
}

// Send answers at once on an extension that stopped reading, rather than block
// its caller on a full pipe.
func TestSendRefusesOnceTheExtensionStopsReading(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, "ext", modeStubborn)
	s, r := boot(t, dir, func(c *timing) { c.stopGrace = 50 * time.Millisecond })
	waitUntil(t, "the extension to be up", func() bool { return slices.Contains(r.titles(t), readyTitle) })
	big := sendOptions(t, "ext", extension.FireEvent.New(extension.EventData{Handler: "x", Text: strings.Repeat("y", 64<<10)}))

	for range sendQueue + 4096 {
		if _, err := s.Send(context.Background(), big); err != nil {
			if !strings.Contains(err.Error(), "not reading") {
				t.Errorf("err = %v", err)
			}
			return
		}
	}
	t.Error("Send never noticed the extension was not reading")
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) count(sub string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Count(b.buf.String(), sub)
}

func TestStderrIsTaggedAndBounded(t *testing.T) {
	captured := &lockedBuffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(captured, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	dir := t.TempDir()
	install(t, dir, "chatty", modeStderr)
	s, _ := boot(t, dir)
	waitUntil(t, "the first lines", func() bool { return captured.count("noise") >= maxLogLines-1 })
	s.Shutdown()

	if got := captured.count("extension=chatty"); got > maxLogLines {
		t.Errorf("%d log lines from one extension, the budget is %d", got, maxLogLines)
	}
	if captured.count("extensions: stderr") == 0 {
		t.Error("stderr never reached the log")
	}
	if captured.count(fmt.Sprintf("over %d bytes", maxLogLineBytes)) != 1 {
		t.Error("the line over the limit was not reported")
	}
}

func TestLogBudgetReportsWhatItDropped(t *testing.T) {
	captured := &lockedBuffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(captured, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	b := &logBudget{}
	for range maxLogLines + 7 {
		b.note("ext", msgStderr, "line")
	}
	b.opened = time.Now().Add(-2 * logWindow)
	b.note("ext", msgStderr, strings.Repeat("z", 2*maxLogLineBytes))

	if captured.count("droppedLines=7") != 1 {
		t.Errorf("the drop was not reported")
	}
	if captured.count(strings.Repeat("z", maxLogLineBytes)) != 1 || captured.count(strings.Repeat("z", maxLogLineBytes+1)) != 0 {
		t.Errorf("a long line was not truncated")
	}
}

// The event the contract declares carries the message as raw JSON and stays
// valid however the window decodes it.
func TestStatusEventIsWhatListShows(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, "ext", modeEcho)
	s, r := boot(t, dir)
	waitStatus(t, r, api.ExtensionRunning)

	list, err := s.List(context.Background(), api.None{})
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	listed, _ := json.Marshal(list[0])
	emitted, _ := json.Marshal(r.lastStatus())
	if !bytes.Equal(listed, emitted) {
		t.Errorf("event %s, list %s", emitted, listed)
	}
}
