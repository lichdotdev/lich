package extensions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

func restart(s *Service, id string) error {
	_, err := s.Restart(context.Background(), api.ExtensionIDOptions{ID: id})
	return err
}

func TestRestartIsTheWayBackFromFailed(t *testing.T) {
	dir := t.TempDir()
	root := install(t, dir, "ext", modeCrash)
	s, r := boot(t, dir)
	waitStatus(t, r, api.ExtensionFailed)

	writeFile(t, filepath.Join(root, modeFile), modeEcho)
	if err := restart(s, "ext"); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the extension to run again", func() bool {
		return runCount(root) == maxQuickCrashes+1 && r.lastStatus().Status == api.ExtensionRunning
	})
	if _, err := s.Send(context.Background(), sendOptions(t, "ext", extension.SurfaceOpened.New(extension.SurfaceData{Surface: "main"}))); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the echo", func() bool { return slices.Contains(r.titles(t), "got surface.opened") })
}

func TestRestartStartsTheCrashCountOver(t *testing.T) {
	dir := t.TempDir()
	root := install(t, dir, "ext", modeCrash)
	s, r := boot(t, dir)
	waitStatus(t, r, api.ExtensionFailed)

	if err := restart(s, "ext"); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "a second full crash streak", func() bool {
		return runCount(root) == 2*maxQuickCrashes && r.lastStatus().Status == api.ExtensionFailed
	})
}

func TestRestartOfARunningExtensionStopsItFirst(t *testing.T) {
	dir := t.TempDir()
	root := install(t, dir, "ext", modeEcho)
	s, r := boot(t, dir)
	waitUntil(t, "the first run", func() bool { return runCount(root) == 1 && r.lastStatus().Status == api.ExtensionRunning })

	if err := restart(s, "ext"); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the second run", func() bool { return runCount(root) == 2 && r.lastStatus().Status == api.ExtensionRunning })
	if !slices.Contains(r.statusNames(), api.ExtensionStopped) {
		t.Errorf("statuses %v never showed the first run stopping", r.statusNames())
	}
	if _, err := os.Stat(filepath.Join(root, eofFile)); err != nil {
		t.Errorf("the first run never saw stdin close: %v", err)
	}
}

func TestRestartOfAnExtensionWaitingToRestartDoesNotWaitItOut(t *testing.T) {
	dir := t.TempDir()
	root := install(t, dir, "ext", modeCrash)
	s, r := boot(t, dir, func(c *timing) { c.firstBackoff, c.maxBackoff = time.Hour, time.Hour })
	waitStatus(t, r, api.ExtensionRestarting)

	if err := restart(s, "ext"); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the second run", func() bool { return runCount(root) == 2 })
}

func TestRestartRefusals(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, "ext", modeEcho)
	install(t, dir, "bad", modeEcho, func(m *extension.Manifest) { m.FormatVersion = 0 })
	s, r := boot(t, dir)
	waitStatus(t, r, api.ExtensionRunning)

	if err := restart(s, "nobody"); err == nil || !strings.Contains(err.Error(), "no extension") {
		t.Errorf("unknown id: %v", err)
	}
	if err := restart(s, "bad"); err == nil || !strings.Contains(err.Error(), "formatVersion 0 must be a positive integer") {
		t.Errorf("invalid extension: %v", err)
	}
	if st := listed(t, s, "bad").Status; st != api.ExtensionInvalid {
		t.Errorf("invalid extension is now %q", st)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Restart(cancelled, api.ExtensionIDOptions{ID: "ext"}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled context: %v", err)
	}

	s.Shutdown()
	if err := restart(s, "ext"); err == nil || !strings.Contains(err.Error(), "shutting down") {
		t.Errorf("after Shutdown: %v", err)
	}
}

func TestListShowsTheExtensionsAbsoluteDirectory(t *testing.T) {
	dir := t.TempDir()
	root := install(t, dir, "ext", modeEcho)
	install(t, dir, "bad", modeEcho, func(m *extension.Manifest) { m.FormatVersion = 0 })
	s, _ := boot(t, dir)

	if got := listed(t, s, "ext").Dir; got != root || !filepath.IsAbs(got) {
		t.Errorf("Dir = %q, want %q", got, root)
	}
	if got := listed(t, s, "bad").Dir; !filepath.IsAbs(got) {
		t.Errorf("an invalid extension's Dir = %q, want it absolute", got)
	}
}

func TestDirIsAbsoluteWhenTheExtensionsDirectoryIsRelative(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, "ext", modeEcho)
	t.Chdir(dir)

	found := discover(".", "1.2.3")
	if len(found) != 1 || found[0].root != filepath.Join(dir, "ext") {
		t.Errorf("root = %+v, want %q", found, filepath.Join(dir, "ext"))
	}
}

func listed(t *testing.T, s *Service, id string) api.Extension {
	t.Helper()
	list, err := s.List(context.Background(), api.None{})
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range list {
		if x.ID == id {
			return x
		}
	}
	t.Fatalf("%q is not listed", id)
	return api.Extension{}
}
