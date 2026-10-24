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

func (t *Tray) icon() []byte { return t.icons.PNG }

// menu is the item's menu entries.
type menu struct {
	show, running, quit objc.ID
}

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
	button := item.Send(sel("button"))
	button.Send(sel("setImage:"), statusImage(t.icon()))
	button.Send(sel("setToolTip:"), nsString("lich"))

	m := menu{
		show:    menuItem(labels.Show, sel("show:"), target),
		running: menuItem("", 0, 0),
		quit:    menuItem(labels.Quit, sel("quit:"), target),
	}
	nsMenu := objc.ID(objc.GetClass("NSMenu")).Send(sel("new"))
	nsMenu.Send(sel("addItem:"), m.show)
	nsMenu.Send(sel("addItem:"), m.running)
	nsMenu.Send(sel("addItem:"), objc.ID(objc.GetClass("NSMenuItem")).Send(sel("separatorItem")))
	nsMenu.Send(sel("addItem:"), m.quit)
	item.Send(sel("setMenu:"), nsMenu)

	t.setRelabel(func(l Labels) {
		appkit.Do(func() {
			m.show.Send(sel("setTitle:"), nsString(l.Show))
			m.quit.Send(sel("setTitle:"), nsString(l.Quit))
		})
		t.refresh(m, l)
	})
	t.refresh(m, labels)
	go t.serve(m)
}

// target registers the object the menu's entries call, which runs the tray's
// show and quit off the main thread.
func (t *Tray) target() (objc.ID, error) {
	class, err := objc.RegisterClass("LichTrayTarget", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
		{Cmd: sel("show:"), Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID) { go t.show() }},
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

// refresh rewrites the session count. A count it cannot read leaves the title
// as it was rather than show a wrong one.
func (t *Tray) refresh(m menu, l Labels) {
	live, err := t.live()
	if err != nil {
		slog.Warn("tray: count the running sessions", "err", err)
		return
	}
	title := l.running(live)
	appkit.Do(func() { m.running.Send(sel("setTitle:"), nsString(title)) })
}

func (t *Tray) serve(m menu) {
	ticker := time.NewTicker(refreshEvery)
	defer ticker.Stop()
	for range ticker.C {
		t.refresh(m, t.currentLabels())
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
