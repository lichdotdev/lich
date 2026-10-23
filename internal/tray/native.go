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
	t.refresh(m, labels)
	systray.AddSeparator()
	m.quit = systray.AddMenuItem(labels.Quit, "")
	t.setRelabel(func(l Labels) {
		m.show.SetTitle(l.Show)
		m.quit.SetTitle(l.Quit)
		t.refresh(m, l)
	})
	go t.serve(m)
}

// menu is the tray's items.
type menu struct {
	show, running, quit *systray.MenuItem
}

// refresh rewrites what changes without a new language: the session count.
// A count it cannot read leaves the title as it was rather than show a wrong one.
func (t *Tray) refresh(m menu, l Labels) {
	live, err := t.live()
	if err != nil {
		slog.Warn("tray: count the running sessions", "err", err)
		return
	}
	m.running.SetTitle(l.running(live))
}

func (t *Tray) serve(m menu) {
	ticker := time.NewTicker(refreshEvery)
	defer ticker.Stop()
	for {
		select {
		case <-m.show.ClickedCh:
			t.show()
		case <-m.quit.ClickedCh:
			if err := t.quit(); err != nil {
				slog.Warn("tray: quit", "err", err)
			}
		case <-ticker.C:
			t.refresh(m, t.currentLabels())
		}
	}
}
