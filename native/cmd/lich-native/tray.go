package main

import (
	"log"
	"os"

	"github.com/omartelo/lich/native/lichclient"
)

// ponytail: English until the native window has i18n (shell.tray).
var trayLabels = lichclient.TrayLabels{
	Show:    "Show lich",
	Running: "Sessions running: {count}",
	Quit:    "Quit lich",
}

// attachTray makes this window the one the tray and a second launch bring
// back, then puts the tray up: the backend leaves it down until a window
// hands it its words.
func (u *window) attachTray() {
	executable, err := os.Executable()
	if err != nil {
		log.Printf("tray: find this window's binary: %v", err)
		return
	}
	if err := u.client.AttachNative(u.ctx, executable); err != nil {
		log.Printf("system.AttachNative: %v", err)
		return
	}
	if err := u.client.SetTrayLabels(u.ctx, trayLabels); err != nil {
		log.Printf("tray.SetLabels: %v", err)
	}
}
