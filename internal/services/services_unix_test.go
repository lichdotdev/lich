//go:build !windows

package services

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/terminal"
)

// memStore is the services' store in memory, over projects that exist only
// when listed.
type memStore struct {
	mu       sync.Mutex
	projects map[string]string
	configs  []api.ServiceConfig
}

func (m *memStore) Services() ([]api.ServiceConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.configs), nil
}

func (m *memStore) SaveService(c api.ServiceConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.configs {
		if m.configs[i].ID == c.ID {
			m.configs[i] = c
			return nil
		}
	}
	m.configs = append(m.configs, c)
	return nil
}

func (m *memStore) DeleteService(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs = slices.DeleteFunc(m.configs, func(c api.ServiceConfig) bool { return c.ID == id })
	return nil
}

func (m *memStore) ProjectName(id string) string {
	if m.projects[id] == "" {
		return ""
	}
	return id + "-name"
}

func (m *memStore) ProjectPath(id string) string { return m.projects[id] }

// termStore is the terminal's store for a PTY that only runs services: the
// project path is all StartService reads, and any other call panics on the
// nil interface, which is the test failing loudly.
type termStore struct{ terminal.Store }

func (termStore) ProjectPath(string) string { return "" }

type emitted struct {
	mu   sync.Mutex
	last []api.Service
	n    int
}

func (e *emitted) emit(name string, data any) {
	if name != api.ServicesEvent.Name {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.last, _ = data.([]api.Service)
	e.n++
}

func newTestServices(t *testing.T) (*Service, *memStore, *emitted) {
	t.Helper()
	t.Setenv("SHELL", "/bin/sh")
	st := &memStore{projects: map[string]string{"p1": t.TempDir(), "p2": t.TempDir()}}
	em := &emitted{}
	svc := New(st, terminal.New(termStore{}, nil, events.New()), em.emit)
	t.Cleanup(func() { svc.Shutdown(time.Second) })
	return svc, st, em
}

func save(t *testing.T, svc *Service, project, name, command string, withLich bool) api.Service {
	t.Helper()
	v, err := svc.Save(context.Background(), api.SaveServiceOptions{ProjectID: project, Name: name, Command: command, StartWithLich: withLich})
	if err != nil {
		t.Fatalf("Save %s: %v", name, err)
	}
	return v
}

func stateOf(t *testing.T, svc *Service, id string) api.Service {
	t.Helper()
	list, err := svc.List(context.Background(), api.None{})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range list {
		if v.ID == id {
			return v
		}
	}
	t.Fatalf("service %s is not listed", id)
	return api.Service{}
}

func waitState(t *testing.T, svc *Service, id, state string) api.Service {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if v := stateOf(t, svc, id); v.State == state {
			return v
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("service %s never became %s, is %s", id, state, stateOf(t, svc, id).State)
	return api.Service{}
}

func TestSaveValidatesAndAnnounces(t *testing.T) {
	svc, _, em := newTestServices(t)
	v := save(t, svc, "p1", "  tailscale ", " docker compose up tailscale ", true)
	if v.ID == "" || v.Name != "tailscale" || v.Command != "docker compose up tailscale" || v.State != api.ServiceStopped {
		t.Errorf("saved = %+v", v)
	}
	if v.ProjectName != "p1-name" || v.Terminal != "service-"+v.ID {
		t.Errorf("saved view = %+v", v)
	}
	if em.n != 1 || len(em.last) != 1 {
		t.Errorf("announced %d times with %d services", em.n, len(em.last))
	}
	bad := []api.SaveServiceOptions{
		{ProjectID: "p1", Name: " ", Command: "x"},
		{ProjectID: "p1", Name: "web", Command: " "},
		{ProjectID: "nope", Name: "web", Command: "x"},
		{ProjectID: "p1", Name: "TailScale", Command: "x"},
		{ID: v.ID, ProjectID: "p2", Name: "moved", Command: "x"},
	}
	for _, opts := range bad {
		if _, err := svc.Save(context.Background(), opts); err == nil {
			t.Errorf("Save(%+v) succeeded", opts)
		}
	}
	if _, err := svc.Save(context.Background(), api.SaveServiceOptions{ProjectID: "p2", Name: "tailscale", Command: "x"}); err != nil {
		t.Errorf("the same name in another project: %v", err)
	}
	if _, err := svc.Save(context.Background(), api.SaveServiceOptions{ID: v.ID, ProjectID: "p1", Name: "tailscale", Command: "y"}); err != nil {
		t.Errorf("an edit keeping its own name: %v", err)
	}
}

func TestAServiceThatExitsStopsOrFails(t *testing.T) {
	svc, _, _ := newTestServices(t)
	ok := save(t, svc, "p1", "ok", "echo done", false)
	bad := save(t, svc, "p1", "bad", "echo broken; exit 3", false)
	for _, id := range []string{ok.ID, bad.ID} {
		if _, err := svc.Start(context.Background(), api.ServiceIDOptions{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	got := waitState(t, svc, ok.ID, api.ServiceStopped)
	if got.ExitCode == nil || *got.ExitCode != 0 || got.StartedAt == 0 || got.StoppedAt < got.StartedAt {
		t.Errorf("clean exit = %+v", got)
	}
	failed := waitState(t, svc, bad.ID, api.ServiceFailed)
	if failed.ExitCode == nil || *failed.ExitCode != 3 {
		t.Errorf("failed exit = %+v", failed)
	}
	out, _ := svc.Output(context.Background(), api.ServiceIDOptions{ID: bad.ID})
	if !strings.Contains(string(out), "broken") {
		t.Errorf("Output = %q, want what the run printed", out)
	}
}

func TestStopIsAStopWhateverTheExitCode(t *testing.T) {
	svc, _, _ := newTestServices(t)
	v := save(t, svc, "p1", "sleeper", "sleep 30", false)
	if _, err := svc.Start(context.Background(), api.ServiceIDOptions{ID: v.ID}); err != nil {
		t.Fatal(err)
	}
	waitState(t, svc, v.ID, api.ServiceRunning)
	if out, _ := svc.Output(context.Background(), api.ServiceIDOptions{ID: v.ID}); out != nil {
		t.Errorf("Output while running = %q", out)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := svc.Stop(context.Background(), api.ServiceIDOptions{ID: v.ID}); err != nil {
		t.Fatal(err)
	}
	waitState(t, svc, v.ID, api.ServiceStopped)
}

func TestForceStopEndsACommandThatIgnoresStop(t *testing.T) {
	svc, _, _ := newTestServices(t)
	v := save(t, svc, "p1", "stubborn", "trap '' INT TERM; while :; do sleep 1; done", false)
	if _, err := svc.Start(context.Background(), api.ServiceIDOptions{ID: v.ID}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	_, _ = svc.Stop(context.Background(), api.ServiceIDOptions{ID: v.ID})
	waitState(t, svc, v.ID, api.ServiceStopping)
	if _, err := svc.ForceStop(context.Background(), api.ServiceIDOptions{ID: v.ID}); err != nil {
		t.Fatal(err)
	}
	if got := waitState(t, svc, v.ID, api.ServiceStopped); got.ExitCode != nil {
		t.Errorf("a killed run has exit code %d", *got.ExitCode)
	}
}

func TestRestartRunsAgain(t *testing.T) {
	svc, _, _ := newTestServices(t)
	v := save(t, svc, "p1", "sleeper", "sleep 30", false)
	if _, err := svc.Start(context.Background(), api.ServiceIDOptions{ID: v.ID}); err != nil {
		t.Fatal(err)
	}
	first := waitState(t, svc, v.ID, api.ServiceRunning)
	time.Sleep(200 * time.Millisecond)
	if _, err := svc.Restart(context.Background(), api.ServiceIDOptions{ID: v.ID}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if got := stateOf(t, svc, v.ID); got.State == api.ServiceRunning && got.StartedAt > first.StartedAt {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("never ran again: %+v", stateOf(t, svc, v.ID))
}

func TestDeleteRefusesARunningService(t *testing.T) {
	svc, st, _ := newTestServices(t)
	v := save(t, svc, "p1", "sleeper", "sleep 30", false)
	if _, err := svc.Start(context.Background(), api.ServiceIDOptions{ID: v.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Delete(context.Background(), api.ServiceIDOptions{ID: v.ID}); err == nil {
		t.Error("a running service was deleted")
	}
	time.Sleep(200 * time.Millisecond)
	_, _ = svc.Stop(context.Background(), api.ServiceIDOptions{ID: v.ID})
	waitState(t, svc, v.ID, api.ServiceStopped)
	if _, err := svc.Delete(context.Background(), api.ServiceIDOptions{ID: v.ID}); err != nil {
		t.Fatal(err)
	}
	if configs, _ := st.Services(); len(configs) != 0 {
		t.Errorf("left %+v", configs)
	}
}

func TestBootStartsOnlyThoseSavedToStartWithLich(t *testing.T) {
	svc, _, _ := newTestServices(t)
	auto := save(t, svc, "p1", "auto", "sleep 30", true)
	manual := save(t, svc, "p1", "manual", "sleep 30", false)
	svc.Boot()
	waitState(t, svc, auto.ID, api.ServiceRunning)
	if got := stateOf(t, svc, manual.ID).State; got != api.ServiceStopped {
		t.Errorf("manual service is %s after boot", got)
	}
	if running := svc.Running(); len(running) != 1 || running[0].ID != auto.ID {
		t.Errorf("Running() = %+v", running)
	}
}

func TestStartAllAndStopAllStayInTheirProject(t *testing.T) {
	svc, _, _ := newTestServices(t)
	a := save(t, svc, "p1", "a", "sleep 30", false)
	b := save(t, svc, "p1", "b", "sleep 30", false)
	other := save(t, svc, "p2", "c", "sleep 30", false)
	if _, err := svc.StartAll(context.Background(), api.ProjectIDOptions{ID: "p1"}); err != nil {
		t.Fatal(err)
	}
	waitState(t, svc, a.ID, api.ServiceRunning)
	waitState(t, svc, b.ID, api.ServiceRunning)
	if got := stateOf(t, svc, other.ID).State; got != api.ServiceStopped {
		t.Errorf("p2's service is %s", got)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := svc.StopAll(context.Background(), api.ProjectIDOptions{ID: "p1"}); err != nil {
		t.Fatal(err)
	}
	waitState(t, svc, a.ID, api.ServiceStopped)
	waitState(t, svc, b.ID, api.ServiceStopped)
}

func TestShutdownKillsWhatOutstaysTheGrace(t *testing.T) {
	svc, _, _ := newTestServices(t)
	v := save(t, svc, "p1", "stubborn", "trap '' INT TERM; while :; do sleep 1; done", false)
	if _, err := svc.Start(context.Background(), api.ServiceIDOptions{ID: v.ID}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	svc.Shutdown(300 * time.Millisecond)
	if running := svc.Running(); len(running) != 0 {
		t.Errorf("still running after Shutdown: %+v", running)
	}
}

func TestAStartThatCannotSpawnFailsWithTheReason(t *testing.T) {
	svc, st, _ := newTestServices(t)
	v := save(t, svc, "p1", "web", "true", false)
	st.projects["p1"] = "/nonexistent/lich-test-dir"
	if _, err := svc.Start(context.Background(), api.ServiceIDOptions{ID: v.ID}); err == nil {
		t.Fatal("a start in a missing directory succeeded")
	}
	got := stateOf(t, svc, v.ID)
	if got.State != api.ServiceFailed {
		t.Errorf("state = %s", got.State)
	}
	if out, _ := svc.Output(context.Background(), api.ServiceIDOptions{ID: v.ID}); len(out) == 0 {
		t.Error("the failed start left nothing on the console")
	}
}
