package main

import (
	"image/color"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

func svc(id, project, state string) backend.Service {
	return backend.Service{ID: id, ProjectID: project, Name: id, State: state, Terminal: "service-" + id}
}

func TestServicesStoreReadsWhatTheEventSent(t *testing.T) {
	var s servicesStore
	s.set([]backend.Service{svc("a", "p1", api.ServiceRunning), svc("b", "p2", api.ServiceStopping), svc("c", "p1", api.ServiceFailed)})
	if got := s.of("p1"); len(got) != 2 || got[0].ID != "a" || got[1].ID != "c" {
		t.Errorf("of(p1) = %+v", got)
	}
	if v, ok := s.byID("b"); !ok || v.ProjectID != "p2" {
		t.Errorf("byID(b) = %+v, %v", v, ok)
	}
	if _, ok := s.byID("nope"); ok {
		t.Error("byID found a service nobody sent")
	}
	if n := s.running(); n != 2 {
		t.Errorf("running() = %d, want the running and the stopping one", n)
	}
}

func TestTallyCountsUpAndFailed(t *testing.T) {
	got := tallyServices([]backend.Service{svc("a", "p", api.ServiceRunning), svc("b", "p", api.ServiceFailed), svc("c", "p", api.ServiceStopped)})
	// failed counts them, the head saying "1 failed" (contract changed: a bool)
	if got != (servicesTally{up: 1, total: 3, failed: 1}) {
		t.Errorf("tally = %+v", got)
	}
	if op := allServicesOp(servicesTally{up: 3, total: 3}); op != serviceStopAll {
		t.Errorf("all up: op %d, want stop all", op)
	}
	if op := allServicesOp(servicesTally{up: 1, total: 3}); op != serviceStartAll {
		t.Errorf("some down: op %d, want start all", op)
	}
	if op := allServicesOp(servicesTally{}); op != serviceStartAll {
		t.Errorf("none: op %d", op)
	}
}

func TestServiceStateTextSaysHowItRunsOrEnded(t *testing.T) {
	now := time.Date(2026, 10, 23, 9, 40, 0, 0, time.Local)
	running := svc("a", "p", api.ServiceRunning)
	running.StartedAt = now.Add(-2*time.Hour - 14*time.Minute).UnixMilli()
	if got := serviceStateText(running, now); got != formatAge(2*time.Hour+14*time.Minute) {
		t.Errorf("running: %q", got)
	}
	code := 255
	failed := svc("a", "p", api.ServiceFailed)
	failed.ExitCode = &code
	if got := serviceStateText(failed, now); got != "exit 255" {
		t.Errorf("failed: %q", got)
	}
	if got := consoleStateText(failed, now); got != "Failed · exit 255" {
		t.Errorf("console failed: %q", got)
	}
	failed.ExitCode = nil
	if got := serviceStateText(failed, now); got != "failed" {
		t.Errorf("failed without a code: %q", got)
	}
	stopped := svc("a", "p", api.ServiceStopped)
	if got := consoleStateText(stopped, now); got != "Not started yet" {
		t.Errorf("never ran: %q", got)
	}
	stopped.StoppedAt = now.UnixMilli()
	if got := consoleStateText(stopped, now); got != "Stopped · 09:40" {
		t.Errorf("stopped: %q", got)
	}
}

func TestServiceGlyphWearsItsStateTone(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark}
	for state, want := range map[string]color.NRGBA{
		api.ServiceRunning:  th.TonePass,
		api.ServiceStopping: th.TonePass,
		api.ServiceFailed:   th.Destructive,
		api.ServiceStopped:  th.MutedForeground,
	} {
		if got := serviceGlyphInk(th, svc("a", "p", state)); got != want {
			t.Errorf("%s: %v, want %v", state, got, want)
		}
	}
}

func TestServiceMenuOffersWhatTheStateAllows(t *testing.T) {
	labels := func(v backend.Service) []string {
		var act sidebarAction
		var out []string
		for _, e := range serviceMenu(v, &act) {
			if e.Kind == ui.MenuSeparator {
				continue
			}
			if e.Disabled {
				out = append(out, "("+e.Label+")")
				continue
			}
			out = append(out, e.Label)
		}
		return out
	}
	if got := labels(svc("a", "p", api.ServiceRunning)); !slices.Equal(got, []string{"Stop", "Restart", "Show output", "Edit service…", "(Delete service)"}) {
		t.Errorf("running: %v", got)
	}
	if got := labels(svc("a", "p", api.ServiceStopping)); !slices.Equal(got, []string{"Force stop", "Show output", "Edit service…", "(Delete service)"}) {
		t.Errorf("stopping: %v", got)
	}
	if got := labels(svc("a", "p", api.ServiceFailed)); !slices.Equal(got, []string{"Start", "Show output", "Edit service…", "Delete service"}) {
		t.Errorf("failed: %v", got)
	}
	var act sidebarAction
	serviceMenu(svc("a", "p", api.ServiceFailed), &act)[0].OnClick()
	if act.service != (serviceRequest{op: serviceStart, id: "a"}) {
		t.Errorf("Start asked for %+v", act.service)
	}
}

func TestCloseDialogNamesTheServicesItWouldStop(t *testing.T) {
	if got := closeRunning(2, 0); !strings.HasPrefix(got, "2 sessions are running.") || strings.Contains(got, "service") {
		t.Errorf("sessions only: %q", got)
	}
	if got := closeRunning(0, 1); !strings.HasPrefix(got, "1 service is running.") {
		t.Errorf("a service only: %q", got)
	}
	got := closeRunning(1, 3)
	if !strings.HasPrefix(got, "1 session is running.") || !strings.Contains(got, "3 services are running too") {
		t.Errorf("both: %q", got)
	}
	if closeQuits(closeAsk, 0+1) {
		t.Error("a window with only a service running quits without asking")
	}
}

func TestTrayLabelsCarryTheServices(t *testing.T) {
	got := trayLabels()
	if got.ServicesRunning != "Services running: {count}" || got.StopAll != "Stop all services" || got.ShowOutput == "" || got.Restart == "" || got.Stop == "" {
		t.Errorf("service labels: %+v", got)
	}
	inPortuguese(t)
	if got := trayLabels(); got.ServicesRunning != "Serviços rodando: {count}" {
		t.Errorf("pt-BR: %+v", got)
	}
}

func TestGlobalServicesAreHomes(t *testing.T) {
	p := servicesSettings{home: func() string { return "home-id" }}
	if got := p.projectOf(""); got != "home-id" {
		t.Errorf("Global edits %q, want Home's project", got)
	}
	if got := p.projectOf("p1"); got != "p1" {
		t.Errorf("a project's scope edits %q", got)
	}
	if got := (&servicesSettings{}).projectOf(""); got != "" {
		t.Errorf("before Home is known Global edits %q", got)
	}
}

func TestTheServicesHeaderOffersWhatTheTallyAllows(t *testing.T) {
	labels := func(tally servicesTally) []string {
		var out []string
		for _, e := range servicesEntries(tally, "p", new(sidebarAction)) {
			if e.Kind == ui.MenuSeparator {
				out = append(out, "-")
				continue
			}
			out = append(out, e.Label)
		}
		return out
	}
	start, stop, add := i18n.T("sidebar.services.startAll"), i18n.T("sidebar.services.stopAll"), i18n.T("sidebar.services.newService")
	cases := []struct {
		tally servicesTally
		want  []string
	}{
		{servicesTally{}, []string{add}},
		{servicesTally{up: 0, total: 2}, []string{start, "-", add}},
		{servicesTally{up: 1, total: 2}, []string{start, stop, "-", add}},
		{servicesTally{up: 2, total: 2}, []string{stop, "-", add}},
	}
	for _, c := range cases {
		if got := labels(c.tally); !slices.Equal(got, c.want) {
			t.Errorf("tally %+v: menu %v, want %v", c.tally, got, c.want)
		}
	}
	var act sidebarAction
	servicesEntries(servicesTally{total: 1}, "p", &act)[0].OnClick()
	if act.service != (serviceRequest{op: serviceStartAll, project: "p"}) {
		t.Errorf("Start all asked %+v", act.service)
	}
}

func TestTheServiceDialogOpensOnTheSavedServiceOrANewOne(t *testing.T) {
	var s servicesStore
	saved := svc("a", "p1", api.ServiceStopped)
	saved.Command, saved.StartWithLich = "pnpm dev", true
	s.set([]backend.Service{saved})
	if v := serviceDialogOf(&s, "p1", "a"); v.ID != "a" || v.Command != "pnpm dev" {
		t.Fatalf("saved: %+v", v)
	}
	if v := serviceDialogOf(&s, "p2", ""); v.ID != "" || v.ProjectID != "p2" || v.Name != "" {
		t.Fatalf("new: %+v, want an empty service of p2", v)
	}
	var p servicesSettings
	p.begin(saved, "lichtrack")
	if !p.open || p.projectName != "lichtrack" || p.name.Text() != "a" || p.command.Text() != "pnpm dev" || !p.withLich.Value {
		t.Fatalf("dialog opened as open=%v project=%q name=%q command=%q withLich=%v", p.open, p.projectName, p.name.Text(), p.command.Text(), p.withLich.Value)
	}
}
