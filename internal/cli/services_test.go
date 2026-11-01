package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/routes"
	"github.com/lichdotdev/lich/internal/rpc"
	"github.com/lichdotdev/lich/internal/spawn"
)

// fakeServices stands in for internal/services behind its RPC methods, the
// boundary the CLI talks to; the project is resolved by the real spawn.Project.
type fakeServices struct {
	mu    sync.Mutex
	saved []api.Service
	saves []api.SaveServiceOptions
	calls []string
}

func (f *fakeServices) list(context.Context, api.None) ([]api.Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]api.Service(nil), f.saved...), nil
}

func (f *fakeServices) save(_ context.Context, o api.SaveServiceOptions) (api.Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saves = append(f.saves, o)
	if o.ID == "" {
		o.ID = "new"
	}
	s := api.Service{ServiceConfig: api.ServiceConfig(o), State: api.ServiceStopped}
	return s, nil
}

func (f *fakeServices) by(verb, state string) func(context.Context, api.ServiceIDOptions) (api.None, error) {
	return func(_ context.Context, o api.ServiceIDOptions) (api.None, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.calls = append(f.calls, verb+" "+o.ID)
		for i := range f.saved {
			if f.saved[i].ID == o.ID {
				f.saved[i].State = state
			}
		}
		return api.None{}, nil
	}
}

func (f *fakeServices) all(verb, state string) func(context.Context, api.ProjectIDOptions) (api.None, error) {
	return func(_ context.Context, o api.ProjectIDOptions) (api.None, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.calls = append(f.calls, verb+" "+o.ID)
		for i := range f.saved {
			if f.saved[i].ProjectID == o.ID {
				f.saved[i].State = state
			}
		}
		return api.None{}, nil
	}
}

func (f *fakeServices) output(_ context.Context, o api.ServiceIDOptions) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "output "+o.ID)
	return []byte("listening on :3000\r\n"), nil
}

func (f *fakeServices) live(_ context.Context, o api.ServiceIDOptions) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "live "+o.ID)
	return []byte("building\r\n"), nil
}

func wiredServices(t *testing.T) (func(string) string, *fakeServices) {
	t.Helper()
	f := &fakeServices{saved: []api.Service{
		{ServiceConfig: api.ServiceConfig{ID: "w1", ProjectID: "p1", Name: "Web", Command: "pnpm dev", StartWithLich: true}, State: api.ServiceStopped},
		{ServiceConfig: api.ServiceConfig{ID: "o1", ProjectID: "p9", Name: "web", Command: "other"}, State: api.ServiceStopped},
	}}
	d := rpc.New()
	routes.Spawn(d, spawn.New(&spawnStore{}, &spawnGit{}, &spawnTerminal{}, nil))
	rpc.Handle(d, api.ServicesList, f.list)
	rpc.Handle(d, api.ServicesSave, f.save)
	rpc.Handle(d, api.ServicesDelete, f.by("delete", api.ServiceStopped))
	rpc.Handle(d, api.ServicesStart, f.by("start", api.ServiceRunning))
	rpc.Handle(d, api.ServicesStop, f.by("stop", api.ServiceStopping))
	rpc.Handle(d, api.ServicesForceStop, f.by("force-stop", api.ServiceStopped))
	rpc.Handle(d, api.ServicesRestart, f.by("restart", api.ServiceRunning))
	rpc.Handle(d, api.ServicesStartAll, f.all("start-all", api.ServiceRunning))
	rpc.Handle(d, api.ServicesStopAll, f.all("stop-all", api.ServiceStopping))
	rpc.Handle(d, api.ServicesOutput, f.output)
	rpc.Handle(d, api.ServicesLiveOutput, f.live)
	server := httptest.NewServer(d)
	t.Cleanup(server.Close)
	return sessionEnv(strconv.Itoa(server.Listener.Addr().(*net.TCPAddr).Port)), f
}

func runService(t *testing.T, env func(string) string, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(append([]string{"service"}, args...), "test", env, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

// A list shows the caller's own project only: the other project's "web" is
// not one an unqualified name may reach.
func TestServiceListShowsTheCallersProject(t *testing.T) {
	t.Parallel()
	env, _ := wiredServices(t)

	out, stderr, code := runService(t, env, "list")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(out, "Web\tstopped\tpnpm dev") || strings.Contains(out, "other") {
		t.Errorf("stdout = %q, want lich's service and nothing else", out)
	}
}

func TestServiceAddSavesANewOneInTheProject(t *testing.T) {
	t.Parallel()
	env, f := wiredServices(t)

	if _, stderr, code := runService(t, env, "--start-with-lich", "add", "db", "docker compose up"); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	want := api.SaveServiceOptions{ProjectID: "p1", Name: "db", Command: "docker compose up", StartWithLich: true}
	if len(f.saves) != 1 || f.saves[0] != want {
		t.Errorf("saves = %+v, want %+v", f.saves, want)
	}
}

// An edit changes the flags given and keeps the rest, start-with-lich among
// them: a bool flag left out must not read as false.
func TestServiceEditKeepsWhatItWasNotGiven(t *testing.T) {
	t.Parallel()
	env, f := wiredServices(t)

	if _, stderr, code := runService(t, env, "--command", "pnpm start", "edit", "web"); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	want := api.SaveServiceOptions{ID: "w1", ProjectID: "p1", Name: "Web", Command: "pnpm start", StartWithLich: true}
	if len(f.saves) != 1 || f.saves[0] != want {
		t.Errorf("saves = %+v, want %+v", f.saves, want)
	}
}

func TestServiceStartStopAndDeleteReachTheNamedOne(t *testing.T) {
	t.Parallel()
	env, f := wiredServices(t)

	out, stderr, code := runService(t, env, "start", "WEB")
	if code != 0 || !strings.Contains(out, "Web\trunning") {
		t.Fatalf("start: exit = %d, stdout = %q, stderr = %q", code, out, stderr)
	}
	for _, verb := range []string{"stop", "delete"} {
		if _, stderr, code := runService(t, env, verb, "web"); code != 0 {
			t.Fatalf("%s: exit = %d, stderr = %q", verb, code, stderr)
		}
	}
	if strings.Join(f.calls, ",") != "start w1,stop w1,delete w1" {
		t.Errorf("calls = %v, want each verb on lich's Web", f.calls)
	}
}

func TestServiceRefusesANameTheProjectLacks(t *testing.T) {
	t.Parallel()
	env, f := wiredServices(t)

	_, stderr, code := runService(t, env, "start", "api")
	if code == 0 || !strings.Contains(stderr, `no service named "api" in lich`) || !strings.Contains(stderr, `"Web"`) {
		t.Errorf("exit = %d, stderr = %q; want the miss refused with the names there are", code, stderr)
	}
	if len(f.calls) != 0 {
		t.Errorf("calls = %v, want none", f.calls)
	}
	if _, stderr, code := runService(t, env, "start"); code == 0 || !strings.Contains(stderr, "usage: lich service") {
		t.Errorf("exit = %d, stderr = %q; want the usage", code, stderr)
	}
}

func serviceTool(t *testing.T, name string) mcpTool {
	t.Helper()
	for _, tool := range serviceTools {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("no tool %q", name)
	return mcpTool{}
}

func TestEditServiceToolKeepsWhatItWasNotGiven(t *testing.T) {
	t.Parallel()
	env, f := wiredServices(t)
	c := &client{env: env, running: noInstance}

	out, err := serviceTool(t, "edit_service").Run(t.Context(), c, mcpArgs{"service": "web", "start_with_lich": false})
	if err != nil {
		t.Fatalf("edit_service: %v", err)
	}
	want := api.SaveServiceOptions{ID: "w1", ProjectID: "p1", Name: "Web", Command: "pnpm dev"}
	if len(f.saves) != 1 || f.saves[0] != want {
		t.Errorf("saves = %+v, want %+v", f.saves, want)
	}
	var s api.Service
	if err := json.Unmarshal([]byte(out), &s); err != nil || s.ID != "w1" {
		t.Errorf("result = %q (%v), want the service as JSON", out, err)
	}
	if _, err := serviceTool(t, "edit_service").Run(t.Context(), c, mcpArgs{"service": "web"}); err == nil {
		t.Error("an edit that names nothing to change was accepted")
	}
}

func TestStartServiceToolAnswersWithTheStateAfter(t *testing.T) {
	t.Parallel()
	env, _ := wiredServices(t)
	c := &client{env: env, running: noInstance}

	out, err := serviceTool(t, "start_service").Run(t.Context(), c, mcpArgs{"service": "Web"})
	if err != nil || !strings.Contains(out, `"state":"running"`) {
		t.Errorf("start_service = %q, %v; want the service running", out, err)
	}
}

func TestServiceRestartAndForceStopReachTheNamedOne(t *testing.T) {
	t.Parallel()
	env, f := wiredServices(t)

	for _, verb := range []string{"restart", "force-stop"} {
		if _, stderr, code := runService(t, env, verb, "web"); code != 0 {
			t.Fatalf("%s: exit = %d, stderr = %q", verb, code, stderr)
		}
	}
	if strings.Join(f.calls, ",") != "restart w1,force-stop w1" {
		t.Errorf("calls = %v, want each verb on lich's Web", f.calls)
	}
}

// start-all and stop-all reach the caller's project and print its services
// as they are afterwards.
func TestServiceStartAllAndStopAllReachTheProject(t *testing.T) {
	t.Parallel()
	env, f := wiredServices(t)

	out, stderr, code := runService(t, env, "start-all")
	if code != 0 || !strings.Contains(out, "Web\trunning") {
		t.Fatalf("start-all: exit = %d, stdout = %q, stderr = %q", code, out, stderr)
	}
	if _, stderr, code := runService(t, env, "stop-all"); code != 0 {
		t.Fatalf("stop-all: exit = %d, stderr = %q", code, stderr)
	}
	if strings.Join(f.calls, ",") != "start-all p1,stop-all p1" {
		t.Errorf("calls = %v, want both on lich", f.calls)
	}
}

// output reads the last run of a stopped service and the console so far of a
// running one, each from the method that holds it.
func TestServiceOutputReadsTheConsoleAsItIsNow(t *testing.T) {
	t.Parallel()
	env, f := wiredServices(t)

	out, stderr, code := runService(t, env, "output", "web")
	if code != 0 || out != "listening on :3000\r\n" {
		t.Fatalf("output: exit = %d, stdout = %q, stderr = %q", code, out, stderr)
	}
	f.mu.Lock()
	f.saved[0].State = api.ServiceRunning
	f.mu.Unlock()
	if out, stderr, code := runService(t, env, "output", "web"); code != 0 || out != "building\r\n" {
		t.Errorf("exit = %d, stdout = %q, stderr = %q; want the running console", code, out, stderr)
	}
	if strings.Join(f.calls, ",") != "output w1,live w1" {
		t.Errorf("calls = %v, want the last run, then the live one", f.calls)
	}
}

func TestStopAllServicesToolAnswersWithTheProjectsServices(t *testing.T) {
	t.Parallel()
	env, f := wiredServices(t)
	c := &client{env: env, running: noInstance}

	out, err := serviceTool(t, "stop_all_services").Run(t.Context(), c, mcpArgs{})
	if err != nil || !strings.Contains(out, `"state":"stopping"`) || strings.Contains(out, "other") {
		t.Errorf("stop_all_services = %q, %v; want lich's services stopping", out, err)
	}
	if strings.Join(f.calls, ",") != "stop-all p1" {
		t.Errorf("calls = %v", f.calls)
	}
}

func TestReadServiceOutputToolReturnsTheText(t *testing.T) {
	t.Parallel()
	env, _ := wiredServices(t)
	c := &client{env: env, running: noInstance}

	out, err := serviceTool(t, "read_service_output").Run(t.Context(), c, mcpArgs{"service": "web"})
	if err != nil || out != "listening on :3000\r\n" {
		t.Errorf("read_service_output = %q, %v", out, err)
	}
}
