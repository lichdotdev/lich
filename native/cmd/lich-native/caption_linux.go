package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/godbus/dbus/v5"
)

const (
	wmPreferencesNS = "org.gnome.desktop.wm.preferences"
	buttonLayoutKey = "button-layout"
)

// platformCaption reads the desktop's button order from the settings
// portal, and draws the buttons in KDE's look under Plasma, GNOME's anywhere
// else.
func platformCaption() captionLayout {
	return captionFor(buttonLayout())
}

func captionFor(buttonLayout string) captionLayout {
	style := captionStyleGNOME
	if strings.Contains(os.Getenv("XDG_CURRENT_DESKTOP"), "KDE") {
		style = captionStyleKDE
	}
	left, right := parseButtonLayout(buttonLayout)
	return captionLayout{left: left, right: right, style: style}
}

// followCaption hands every later change of the desktop's button layout to
// onChange, as the settings portal announces it.
func followCaption(ctx context.Context, onChange func(captionLayout)) {
	conn, err := dbus.SessionBus()
	if err != nil {
		log.Printf("caption: no session bus, the button layout stays as it opened: %v", err)
		return
	}
	if err := conn.AddMatchSignal(dbus.WithMatchInterface(portalSettings), dbus.WithMatchMember("SettingChanged")); err != nil {
		log.Printf("caption: can't follow the button layout: %v", err)
		return
	}
	signals := make(chan *dbus.Signal, portalSignalQueue)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)
	for {
		select {
		case <-ctx.Done():
			return
		case s := <-signals:
			if layout, ok := buttonLayoutChange(s.Body); ok {
				onChange(captionFor(layout))
			}
		}
	}
}

// buttonLayoutChange reads a SettingChanged body (namespace, key, value) and
// reports whether it is the button layout, and what that is now.
func buttonLayoutChange(body []any) (string, bool) {
	if len(body) != 3 || body[0] != wmPreferencesNS || body[1] != buttonLayoutKey {
		return "", false
	}
	v, isVariant := body[2].(dbus.Variant)
	if !isVariant {
		return "", false
	}
	layout, isString := v.Value().(string)
	return layout, isString
}

func buttonLayout() string {
	conn, err := dbus.SessionBus()
	if err != nil {
		log.Printf("caption: no session bus, buttons on the right: %v", err)
		return defaultButtonLayout
	}
	var v dbus.Variant
	call := conn.Object(portalService, portalPath).Call(portalSettings+".ReadOne", 0, wmPreferencesNS, buttonLayoutKey)
	if err := call.Store(&v); err != nil {
		log.Printf("caption: no button-layout in the settings portal, buttons on the right: %v", err)
		return defaultButtonLayout
	}
	s, ok := v.Value().(string)
	if !ok {
		log.Printf("caption: button-layout is %v, buttons on the right", v)
		return defaultButtonLayout
	}
	return s
}
