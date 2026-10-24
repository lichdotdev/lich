//go:build linux || windows

package tray

import (
	"log/slog"
	"runtime"
	"time"

	"fyne.io/systray"
)

// startNative runs the platform's tray loop on a thread of its own: Windows
// pumps its notification-area messages on the thread that created the icon,
// and Linux talks StatusNotifierItem over D-Bus. Neither needs main's thread.
func startNative(t *Tray) {
	go func() {
		runtime.LockOSThread()
		systray.Run(func() { t.ready() }, func() {})
	}()
}

func (t *Tray) ready() {
	systray.SetIcon(t.icon())
	systray.SetTooltip("lich")
	labels := t.currentLabels()
	m := menu{show: systray.AddMenuItem(labels.Show, "")}
	m.running = systray.AddMenuItem("", "")
	m.running.Disable()
	m.services = systray.AddMenuItem("", "")
	m.services.Disable()
	for i := range m.slots {
		item := systray.AddMenuItem("", "")
		m.slots[i] = slot{
			item:    item,
			show:    item.AddSubMenuItem(labels.ShowOutput, ""),
			restart: item.AddSubMenuItem(labels.Restart, ""),
			stop:    item.AddSubMenuItem(labels.Stop, ""),
		}
	}
	m.stopAll = systray.AddMenuItem(labels.StopAll, "")
	t.refresh(m, labels)
	systray.AddSeparator()
	m.quit = systray.AddMenuItem(labels.Quit, "")
	t.setRelabel(func(l Labels) {
		m.show.SetTitle(l.Show)
		m.quit.SetTitle(l.Quit)
		m.stopAll.SetTitle(l.StopAll)
		for _, sl := range m.slots {
			sl.show.SetTitle(l.ShowOutput)
			sl.restart.SetTitle(l.Restart)
			sl.stop.SetTitle(l.Stop)
		}
		t.refresh(m, l)
	})
	for i, sl := range m.slots {
		go t.serveSlot(i, sl)
	}
	go t.serve(m)
}

// menu is the tray's items.
type menu struct {
	show, running, quit *systray.MenuItem
	services, stopAll   *systray.MenuItem
	slots               [serviceSlots]slot
}

// slot is one running service's item and its submenu.
type slot struct {
	item, show, restart, stop *systray.MenuItem
}

// refresh rewrites what changes without a new language: the session count and
// the running services. A count it cannot read leaves the title as it was
// rather than show a wrong one.
func (t *Tray) refresh(m menu, l Labels) {
	t.refreshServices(m, l)
	live, err := t.live()
	if err != nil {
		slog.Warn("tray: count the running sessions", "err", err)
		return
	}
	m.running.SetTitle(l.running(live))
}

func (t *Tray) refreshServices(m menu, l Labels) {
	shown, total := t.runningServices()
	visible(m.services, total > 0)
	visible(m.stopAll, total > 0)
	m.services.SetTitle(l.servicesRunning(total))
	for i, sl := range m.slots {
		visible(sl.item, i < len(shown))
		if i < len(shown) {
			sl.item.SetTitle(serviceTitle(shown[i]))
		}
	}
}

func visible(item *systray.MenuItem, on bool) {
	if on {
		item.Show()
		return
	}
	item.Hide()
}

func (t *Tray) serveSlot(i int, sl slot) {
	for {
		select {
		case <-sl.show.ClickedCh:
			t.show()
			t.slotAction(i, func(s Services, id string) { s.Reveal(id) })
		case <-sl.restart.ClickedCh:
			t.slotAction(i, func(s Services, id string) { s.Restart(id) })
		case <-sl.stop.ClickedCh:
			t.slotAction(i, func(s Services, id string) { s.Stop(id) })
		}
	}
}

func (t *Tray) serve(m menu) {
	ticker := time.NewTicker(refreshEvery)
	defer ticker.Stop()
	for {
		select {
		case <-m.show.ClickedCh:
			t.show()
		case <-m.stopAll.ClickedCh:
			t.stopAllServices()
		case <-m.quit.ClickedCh:
			if err := t.quit(); err != nil {
				slog.Warn("tray: quit", "err", err)
			}
		case <-ticker.C:
			t.refresh(m, t.currentLabels())
		}
	}
}
