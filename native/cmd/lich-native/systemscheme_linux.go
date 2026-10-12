package main

import (
	"context"
	"log"

	"github.com/godbus/dbus/v5"
)

const (
	portalService     = "org.freedesktop.portal.Desktop"
	portalPath        = "/org/freedesktop/portal/desktop"
	portalSettings    = "org.freedesktop.portal.Settings"
	appearanceNS      = "org.freedesktop.appearance"
	colorSchemeKey    = "color-scheme"
	portalSignalQueue = 4
)

// watchSystemDark reads the desktop's color scheme from the settings portal
// and hands every later change to onChange. Without a portal it reports
// dark, the scheme the window opened in before System was followed.
func watchSystemDark(ctx context.Context, onChange func(dark bool)) bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		log.Printf("appearance: no session bus, System draws dark: %v", err)
		return true
	}
	var v dbus.Variant
	call := conn.Object(portalService, portalPath).Call(portalSettings+".ReadOne", 0, appearanceNS, colorSchemeKey)
	if err := call.Store(&v); err != nil {
		log.Printf("appearance: no appearance portal, System draws dark: %v", err)
		return true
	}
	scheme, ok := v.Value().(uint32)
	if !ok {
		log.Printf("appearance: color-scheme is %v, System draws dark", v)
		return true
	}
	go followPortal(ctx, conn, onChange)
	return portalDark(scheme)
}

func followPortal(ctx context.Context, conn *dbus.Conn, onChange func(dark bool)) {
	if err := conn.AddMatchSignal(dbus.WithMatchInterface(portalSettings), dbus.WithMatchMember("SettingChanged")); err != nil {
		log.Printf("appearance: can't follow the color scheme: %v", err)
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
			if dark, ok := schemeChange(s.Body); ok {
				onChange(dark)
			}
		}
	}
}

// schemeChange reads a SettingChanged body (namespace, key, value) and
// reports whether it is the color scheme, and whether that is dark.
func schemeChange(body []any) (dark, ok bool) {
	if len(body) != 3 || body[0] != appearanceNS || body[1] != colorSchemeKey {
		return false, false
	}
	v, isVariant := body[2].(dbus.Variant)
	if !isVariant {
		return false, false
	}
	scheme, isUint := v.Value().(uint32)
	return portalDark(scheme), isUint
}
