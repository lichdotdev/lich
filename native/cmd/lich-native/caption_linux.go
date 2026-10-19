package main

import (
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
	style := captionStyleGNOME
	if strings.Contains(os.Getenv("XDG_CURRENT_DESKTOP"), "KDE") {
		style = captionStyleKDE
	}
	left, right := parseButtonLayout(buttonLayout())
	return captionLayout{left: left, right: right, style: style}
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
