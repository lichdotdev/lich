// Package tray keeps lich in the system tray: the one thing on screen that says
// a lich whose window is closed is still running its sessions, and the way back
// to the window or out of lich from there.
//
// The menu's words are the window's: the interface language lives in the
// window, so the window hands its translated labels over (SetLabels), and the
// tray comes up with the first of them. A lich whose window never opened shows
// no tray, and has the error that kept it from opening on screen instead.
package tray

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
)

// refreshEvery is how often the menu's session count is read again. The count
// only changes when a session is opened or closed, and nothing reads it but a
// person opening the menu.
const refreshEvery = 5 * time.Second

// Labels are the menu's words, in the interface language. Running carries a
// {count} placeholder and is written so it needs no plural form.
type Labels struct {
	Show            string `json:"show"`
	Running         string `json:"running"`
	Quit            string `json:"quit"`
	ServicesRunning string `json:"servicesRunning"`
	ShowOutput      string `json:"showOutput"`
	Restart         string `json:"restart"`
	Stop            string `json:"stop"`
	StopAll         string `json:"stopAll"`
}

func (l Labels) running(live int) string {
	return strings.ReplaceAll(l.Running, "{count}", strconv.Itoa(live))
}

func (l Labels) servicesRunning(n int) string {
	return strings.ReplaceAll(l.ServicesRunning, "{count}", strconv.Itoa(n))
}

// serviceSlots is how many running services the menu names one by one. The
// menu is built once and its items are shown and hidden, since the tray
// library cannot remove one; past this many, the count line still says how
// many run and the window lists them all.
const serviceSlots = 8

// Services is what the tray reads and does about the project services.
// Reveal opens a service's console in the window.
type Services struct {
	Running func() []api.Service
	Reveal  func(id string)
	Stop    func(id string)
	Restart func(id string)
	StopAll func()
}

// serviceTitle is how the menu names a running service: its name, and the
// project it runs for, since two projects may each have a "db".
func serviceTitle(v api.Service) string {
	if v.ProjectName == "" {
		return v.Name
	}
	return v.Name + " · " + v.ProjectName
}

// Tray is lich's tray icon and its menu.
type Tray struct {
	mu      sync.Mutex
	labels  Labels
	started bool
	// relabel is set once the menu is up, to rewrite it in new labels.
	relabel func(Labels)
	icons   Icons
	show    func()
	quit    func() error
	live    func() (int, error)
	// services is set at startup (SetServices); its zero value leaves the
	// services out of the menu.
	services Services
	// slotIDs is which service each menu slot shows, refreshed with the menu.
	slotIDs [serviceSlots]string
	// start puts the icon up; the build-tagged native one, or a test's.
	start func(t *Tray)
}

// Icons are the two encodings the platforms take: PNG for the Linux
// StatusNotifierItem and the macOS menu bar, ICO for the Windows notification
// area.
type Icons struct {
	PNG []byte
	ICO []byte
}

// New returns a tray that is not up yet. show brings the window up, quit ends
// lich, live counts the sessions with a process running.
func New(icons Icons, show func(), quit func() error, live func() (int, error)) *Tray {
	return &Tray{icons: icons, show: show, quit: quit, live: live, start: startNative}
}

// SetLabels gives the menu its words. The first call puts the tray up; a later
// one, after the language changed, rewrites the menu in place.
func (t *Tray) SetLabels(_ context.Context, opts api.TrayLabels) (api.None, error) {
	labels := Labels(opts)
	t.mu.Lock()
	t.labels = labels
	first := !t.started
	t.started = true
	relabel := t.relabel
	t.mu.Unlock()
	if first {
		t.start(t)
		return api.None{}, nil
	}
	if relabel != nil {
		relabel(labels)
	}
	return api.None{}, nil
}

// SetServices wires the project services into the menu. Startup wiring,
// called before the window can put the tray up.
func (t *Tray) SetServices(s Services) {
	t.mu.Lock()
	t.services = s
	t.mu.Unlock()
}

// runningServices is what the menu lists, at most serviceSlots of them, and
// how many run in all. Each slot's id is recorded for the click that comes
// back on it.
func (t *Tray) runningServices() ([]api.Service, int) {
	t.mu.Lock()
	running := t.services.Running
	t.mu.Unlock()
	if running == nil {
		return nil, 0
	}
	all := running()
	shown := all[:min(len(all), serviceSlots)]
	t.mu.Lock()
	t.slotIDs = [serviceSlots]string{}
	for i, v := range shown {
		t.slotIDs[i] = v.ID
	}
	t.mu.Unlock()
	return shown, len(all)
}

// slotAction runs act on the service slot i showed when the menu was last
// refreshed; a slot showing nothing does nothing.
func (t *Tray) slotAction(i int, act func(Services, string)) {
	t.mu.Lock()
	id, s := t.slotIDs[i], t.services
	t.mu.Unlock()
	if id != "" {
		act(s, id)
	}
}

func (t *Tray) stopAllServices() {
	t.mu.Lock()
	stopAll := t.services.StopAll
	t.mu.Unlock()
	if stopAll != nil {
		stopAll()
	}
}

func (t *Tray) currentLabels() Labels {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.labels
}

func (t *Tray) setRelabel(relabel func(Labels)) {
	t.mu.Lock()
	t.relabel = relabel
	t.mu.Unlock()
}
