package tray

import (
	"log/slog"
	"runtime"
	"structs"
	"time"
	"unsafe"

	"github.com/ebitengine/purego/objc"
	"github.com/lichdotdev/lich/internal/appkit"
)

// NSVariableStatusItemLength: the item is as wide as its icon.
const variableStatusItemLength = -1.0

// iconPoints is the menu bar's icon size; the bar is 22 points tall.
const iconPoints = 18

type nsSize struct {
	_             structs.HostLayout
	Width, Height float64
}

// startNative puts the menu-bar item up on the main thread, where AppKit
// runs (internal/appkit).
func startNative(t *Tray) {
	appkit.Do(t.ready)
}

func (t *Tray) icon(b badge) []byte { return t.icons[b].PNG }

// menu is the item's menu entries, and the status item whose icon carries the
// badge.
type menu struct {
	button                 objc.ID
	show, quit, none, idle objc.ID
	needs, working         group
	services, stopAll      objc.ID
	slots                  [serviceSlots]slot
}

// group is one group of sessions: its heading, an entry per session and the
// line counting the ones past the entries.
type group struct {
	heading, more objc.ID
	rows          [sessionSlots]objc.ID
}

// slot is one running service's entry and its submenu.
type slot struct {
	item, show, restart, stop objc.ID
}

// The actions of a slot's submenu, told apart by the entry's tag:
// slot*slotActions + action.
const (
	slotShow = iota
	slotRestart
	slotStop
	slotActions
)

// A session entry's tag is its group's offset plus its slot.
const (
	tagNeeds   = 0
	tagWorking = sessionSlots
)

func (t *Tray) ready() {
	target, err := t.target()
	if err != nil {
		slog.Warn("tray: menu target", "err", err)
		return
	}
	labels := t.currentLabels()
	item := objc.ID(objc.GetClass("NSStatusBar")).Send(sel("systemStatusBar")).
		Send(sel("statusItemWithLength:"), variableStatusItemLength)
	// The status bar drops an item nothing holds.
	item.Send(sel("retain"))
	m := &menu{button: item.Send(sel("button"))}
	m.button.Send(sel("setImage:"), statusImage(t.icon(badgeNone)))
	m.button.Send(sel("setToolTip:"), nsString("lich"))

	nsMenu := objc.ID(objc.GetClass("NSMenu")).Send(sel("new"))
	add := func(entry objc.ID) objc.ID {
		nsMenu.Send(sel("addItem:"), entry)
		return entry
	}
	m.show = add(menuItem(labels.Show, sel("show:"), target))
	add(separator())
	m.none = add(menuItem("", 0, 0))
	m.needs = sessionGroup(add, tagNeeds, target)
	m.working = sessionGroup(add, tagWorking, target)
	m.idle = add(menuItem("", 0, 0))
	m.services = add(menuItem("", 0, 0))
	sub := objc.ID(objc.GetClass("NSMenu")).Send(sel("new"))
	for i := range m.slots {
		m.slots[i] = serviceSlot(i, labels, target)
		sub.Send(sel("addItem:"), m.slots[i].item)
	}
	sub.Send(sel("addItem:"), separator())
	m.stopAll = menuItem(labels.StopAll, sel("stopAllServices:"), target)
	sub.Send(sel("addItem:"), m.stopAll)
	m.services.Send(sel("setSubmenu:"), sub)
	add(separator())
	m.quit = add(menuItem(labels.Quit, sel("quit:"), target))
	item.Send(sel("setMenu:"), nsMenu)

	t.setRelabel(func(l Labels) {
		appkit.Do(func() {
			m.show.Send(sel("setTitle:"), nsString(l.Show))
			m.quit.Send(sel("setTitle:"), nsString(l.Quit))
			m.stopAll.Send(sel("setTitle:"), nsString(l.StopAll))
			for _, sl := range m.slots {
				sl.show.Send(sel("setTitle:"), nsString(l.ShowOutput))
				sl.restart.Send(sel("setTitle:"), nsString(l.Restart))
				sl.stop.Send(sel("setTitle:"), nsString(l.Stop))
			}
		})
		t.refresh(m, l)
	})
	t.refresh(m, labels)
	go t.serve(m)
}

// sessionGroup adds a group's entries, hidden until sessions fill them; the
// session entries are tagged from offset.
func sessionGroup(add func(objc.ID) objc.ID, offset int, target objc.ID) group {
	g := group{heading: add(menuItem("", 0, 0))}
	for i := range g.rows {
		g.rows[i] = add(menuItem("", sel("focusSession:"), target))
		g.rows[i].Send(sel("setTag:"), offset+i)
	}
	g.more = add(menuItem("", 0, 0))
	return g
}

// target registers the object the menu's entries call, which runs the tray's
// show and quit off the main thread.
func (t *Tray) target() (objc.ID, error) {
	class, err := objc.RegisterClass("LichTrayTarget", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
		{Cmd: sel("show:"), Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID) { go t.show() }},
		{Cmd: sel("focusSession:"), Fn: func(_ objc.ID, _ objc.SEL, sender objc.ID) {
			go t.focusTag(objc.Send[int](sender, sel("tag")))
		}},
		{Cmd: sel("serviceAction:"), Fn: func(_ objc.ID, _ objc.SEL, sender objc.ID) {
			go t.serviceAction(objc.Send[int](sender, sel("tag")))
		}},
		{Cmd: sel("stopAllServices:"), Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID) { go t.stopAllServices() }},
		{Cmd: sel("quit:"), Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID) {
			go func() {
				if err := t.quit(); err != nil {
					slog.Warn("tray: quit", "err", err)
				}
			}()
		}},
	})
	if err != nil {
		return 0, err
	}
	return objc.ID(class).Send(sel("new")), nil
}

func (t *Tray) focusTag(tag int) {
	if tag >= tagWorking {
		t.focusSlot(&t.workIDs, tag-tagWorking)
		return
	}
	t.focusSlot(&t.needIDs, tag)
}

// refresh rewrites the sessions, the icon's badge and the running services.
// Sessions it cannot read leave the menu as it was rather than show a wrong
// one.
func (t *Tray) refresh(m *menu, l Labels) {
	shown, total := t.runningServices()
	appkit.Do(func() {
		setLine(m.services, l.servicesRunning(total), total > 0)
		for i, sl := range m.slots {
			if i < len(shown) {
				sl.item.Send(sel("setTitle:"), nsString(serviceTitle(shown[i])))
			}
			sl.item.Send(sel("setHidden:"), i >= len(shown))
		}
	})
	v, err := t.readSessions(l)
	if err != nil {
		slog.Warn("tray: read the sessions", "err", err)
		return
	}
	icon := t.swapBadge(v.badge)
	appkit.Do(func() {
		setGroup(m.needs, l.NeedsYou, v.needs, v.needsMore, l)
		setGroup(m.working, l.Working, v.working, v.workingMore, l)
		setLine(m.idle, withCount(l.Idle, v.idle), v.idle > 0)
		setLine(m.none, l.NoSessions, !v.any())
		m.button.Send(sel("setToolTip:"), nsString(v.tooltip))
		if icon {
			m.button.Send(sel("setImage:"), statusImage(t.icon(v.badge)))
		}
	})
}

func setGroup(g group, heading string, rows []sessionRow, more int, l Labels) {
	setLine(g.heading, heading, len(rows) > 0)
	for i, entry := range g.rows {
		if i < len(rows) {
			entry.Send(sel("setTitle:"), nsString(rows[i].title))
			entry.Send(sel("setToolTip:"), nsString(rows[i].full))
		}
		entry.Send(sel("setHidden:"), i >= len(rows))
	}
	setLine(g.more, withCount(l.More, more), more > 0)
}

func setLine(entry objc.ID, title string, on bool) {
	entry.Send(sel("setTitle:"), nsString(title))
	entry.Send(sel("setHidden:"), !on)
}

func (t *Tray) serve(m *menu) {
	ticker := time.NewTicker(refreshEvery)
	defer ticker.Stop()
	for range ticker.C {
		t.refresh(m, t.currentLabels())
	}
}

// serviceSlot is slot i's entry, hidden until a running service fills it, and
// its submenu of Show output, Restart and Stop.
func serviceSlot(i int, l Labels, target objc.ID) slot {
	sl := slot{
		item:    menuItem("", 0, 0),
		show:    menuItem(l.ShowOutput, sel("serviceAction:"), target),
		restart: menuItem(l.Restart, sel("serviceAction:"), target),
		stop:    menuItem(l.Stop, sel("serviceAction:"), target),
	}
	sub := objc.ID(objc.GetClass("NSMenu")).Send(sel("new"))
	for action, entry := range []objc.ID{slotShow: sl.show, slotRestart: sl.restart, slotStop: sl.stop} {
		entry.Send(sel("setTag:"), i*slotActions+action)
		sub.Send(sel("addItem:"), entry)
	}
	sl.item.Send(sel("setSubmenu:"), sub)
	sl.item.Send(sel("setHidden:"), true)
	return sl
}

// serviceAction runs the submenu entry tagged tag on the service its slot
// showed.
func (t *Tray) serviceAction(tag int) {
	switch tag % slotActions {
	case slotShow:
		t.show()
		t.slotAction(tag/slotActions, func(s Services, id string) { s.Reveal(id) })
	case slotRestart:
		t.slotAction(tag/slotActions, func(s Services, id string) { s.Restart(id) })
	case slotStop:
		t.slotAction(tag/slotActions, func(s Services, id string) { s.Stop(id) })
	}
}

// menuItem is an entry calling action on target; one with no action is drawn
// disabled, which is what the session count is.
func menuItem(title string, action objc.SEL, target objc.ID) objc.ID {
	item := objc.ID(objc.GetClass("NSMenuItem")).Send(sel("alloc")).
		Send(sel("initWithTitle:action:keyEquivalent:"), nsString(title), action, nsString(""))
	if target != 0 {
		item.Send(sel("setTarget:"), target)
	}
	return item
}

func separator() objc.ID {
	return objc.ID(objc.GetClass("NSMenuItem")).Send(sel("separatorItem"))
}

func statusImage(png []byte) objc.ID {
	data := objc.ID(objc.GetClass("NSData")).Send(sel("dataWithBytes:length:"), unsafe.Pointer(&png[0]), uint(len(png)))
	runtime.KeepAlive(png)
	image := objc.ID(objc.GetClass("NSImage")).Send(sel("alloc")).Send(sel("initWithData:"), data)
	image.Send(sel("setSize:"), nsSize{Width: iconPoints, Height: iconPoints})
	return image
}

func nsString(s string) objc.ID {
	return objc.ID(objc.GetClass("NSString")).Send(sel("stringWithUTF8String:"), s)
}

func sel(name string) objc.SEL { return objc.RegisterName(name) }
