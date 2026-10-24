package tray

import (
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestRunningFillsTheCount(t *testing.T) {
	l := Labels{Running: "Sessions running: {count}"}
	if got := l.running(4); got != "Sessions running: 4" {
		t.Fatalf("running(4) = %q", got)
	}
}

func TestSetLabelsStartsOnceThenRelabels(t *testing.T) {
	starts := 0
	var relabeled []Labels
	tr := New(Icons{}, func() {}, func() error { return nil }, func() (int, error) { return 0, nil })
	tr.start = func(t *Tray) {
		starts++
		t.setRelabel(func(l Labels) { relabeled = append(relabeled, l) })
	}

	en := Labels{Show: "Show lich", Running: "Sessions running: {count}", Quit: "Quit lich"}
	pt := Labels{Show: "Mostrar o lich", Running: "Sessões rodando: {count}", Quit: "Encerrar o lich"}
	if _, err := tr.SetLabels(t.Context(), api.TrayLabels(en)); err != nil {
		t.Fatal(err)
	}
	if starts != 1 || len(relabeled) != 0 {
		t.Fatalf("first SetLabels: starts=%d relabeled=%v, want the tray put up once", starts, relabeled)
	}
	if tr.currentLabels() != en {
		t.Fatalf("labels = %+v, want %+v", tr.currentLabels(), en)
	}
	if _, err := tr.SetLabels(t.Context(), api.TrayLabels(pt)); err != nil {
		t.Fatal(err)
	}
	if starts != 1 || len(relabeled) != 1 || relabeled[0] != pt {
		t.Fatalf("second SetLabels: starts=%d relabeled=%v, want the menu rewritten in place", starts, relabeled)
	}
}

func TestServicesRunningFillsTheCount(t *testing.T) {
	l := Labels{ServicesRunning: "Services running: {count}"}
	if got := l.servicesRunning(3); got != "Services running: 3" {
		t.Fatalf("servicesRunning(3) = %q", got)
	}
}

func TestServiceTitleNamesTheProject(t *testing.T) {
	v := api.Service{ServiceConfig: api.ServiceConfig{Name: "db"}, ProjectName: "billing"}
	if got := serviceTitle(v); got != "db · billing" {
		t.Errorf("serviceTitle = %q", got)
	}
	v.ProjectName = ""
	if got := serviceTitle(v); got != "db" {
		t.Errorf("serviceTitle without a project = %q", got)
	}
}

func TestServiceSlotsActOnWhatTheyShowed(t *testing.T) {
	tr := New(Icons{}, func() {}, func() error { return nil }, func() (int, error) { return 0, nil })
	if shown, total := tr.runningServices(); shown != nil || total != 0 {
		t.Fatalf("without services wired: %v, %d", shown, total)
	}
	var running []api.Service
	for i := range serviceSlots + 2 {
		running = append(running, api.Service{ServiceConfig: api.ServiceConfig{ID: string(rune('a' + i))}})
	}
	var stopped []string
	stopAll := 0
	tr.SetServices(Services{
		Running: func() []api.Service { return running },
		Stop:    func(id string) { stopped = append(stopped, id) },
		StopAll: func() { stopAll++ },
	})
	shown, total := tr.runningServices()
	if len(shown) != serviceSlots || total != serviceSlots+2 {
		t.Fatalf("shown %d of %d, want %d of %d", len(shown), total, serviceSlots, serviceSlots+2)
	}
	tr.slotAction(1, func(s Services, id string) { s.Stop(id) })
	running = running[:1]
	tr.runningServices()
	tr.slotAction(1, func(s Services, id string) { s.Stop(id) })
	if len(stopped) != 1 || stopped[0] != "b" {
		t.Errorf("stopped %v, want only the service slot 1 showed", stopped)
	}
	tr.stopAllServices()
	if stopAll != 1 {
		t.Errorf("StopAll called %d times", stopAll)
	}
}
