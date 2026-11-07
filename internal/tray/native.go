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
	systray.SetIcon(t.icon(badgeNone))
	systray.SetTooltip("lich")
	labels := t.currentLabels()
	m := &menu{show: systray.AddMenuItem(labels.Show, "")}
	systray.AddSeparator()
	m.none = disabledItem()
	m.needs = addGroup()
	m.working = addGroup()
	m.idle = disabledItem()
	m.services = systray.AddMenuItem("", "")
	for i := range m.slots {
		item := m.services.AddSubMenuItem("", "")
		m.slots[i] = slot{
			item:    item,
			show:    item.AddSubMenuItem(labels.ShowOutput, ""),
			restart: item.AddSubMenuItem(labels.Restart, ""),
			stop:    item.AddSubMenuItem(labels.Stop, ""),
		}
	}
	m.services.AddSeparator()
	m.stopAll = m.services.AddSubMenuItem(labels.StopAll, "")
	systray.AddSeparator()
	m.quit = systray.AddMenuItem(labels.Quit, "")
	t.refresh(m, labels)
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
	for i := range sessionSlots {
		go t.serveSession(m.needs.rows[i], &t.needIDs, i)
		go t.serveSession(m.working.rows[i], &t.workIDs, i)
	}
	go t.serve(m)
}

// menu is the tray's items.
type menu struct {
	show, quit, none, idle *systray.MenuItem
	needs, working         group
	services, stopAll      *systray.MenuItem
	slots                  [serviceSlots]slot
}

// group is one group of sessions: its heading, a slot per session and the line
// counting the ones past the slots.
type group struct {
	heading, more *systray.MenuItem
	rows          [sessionSlots]*systray.MenuItem
}

// slot is one running service's item and its submenu.
type slot struct {
	item, show, restart, stop *systray.MenuItem
}

func addGroup() group {
	g := group{heading: disabledItem()}
	for i := range g.rows {
		g.rows[i] = systray.AddMenuItem("", "")
	}
	g.more = disabledItem()
	return g
}

func disabledItem() *systray.MenuItem {
	item := systray.AddMenuItem("", "")
	item.Disable()
	return item
}

// refresh rewrites what changes without a new language: the sessions, the
// icon's badge and the running services. Sessions it cannot read leave the
// menu as it was rather than show a wrong one.
func (t *Tray) refresh(m *menu, l Labels) {
	t.refreshServices(m, l)
	v, err := t.readSessions(l)
	if err != nil {
		slog.Warn("tray: read the sessions", "err", err)
		return
	}
	setGroup(m.needs, l.NeedsYou, v.needs, v.needsMore, l)
	setGroup(m.working, l.Working, v.working, v.workingMore, l)
	setLine(m.idle, withCount(l.Idle, v.idle), v.idle > 0)
	setLine(m.none, l.NoSessions, !v.any())
	systray.SetTooltip(v.tooltip)
	if t.swapBadge(v.badge) {
		systray.SetIcon(t.icon(v.badge))
	}
}

func setGroup(g group, heading string, rows []sessionRow, more int, l Labels) {
	setLine(g.heading, heading, len(rows) > 0)
	for i, item := range g.rows {
		if i < len(rows) {
			item.SetTitle(rows[i].title)
		}
		visible(item, i < len(rows))
	}
	setLine(g.more, withCount(l.More, more), more > 0)
}

func setLine(item *systray.MenuItem, title string, on bool) {
	item.SetTitle(title)
	visible(item, on)
}

func (t *Tray) refreshServices(m *menu, l Labels) {
	shown, total := t.runningServices()
	setLine(m.services, l.servicesRunning(total), total > 0)
	for i, sl := range m.slots {
		if i < len(shown) {
			sl.item.SetTitle(serviceTitle(shown[i]))
		}
		visible(sl.item, i < len(shown))
	}
}

func visible(item *systray.MenuItem, on bool) {
	if on {
		item.Show()
		return
	}
	item.Hide()
}

func (t *Tray) serveSession(item *systray.MenuItem, ids *[sessionSlots]string, i int) {
	for range item.ClickedCh {
		t.focusSlot(ids, i)
	}
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

func (t *Tray) serve(m *menu) {
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
