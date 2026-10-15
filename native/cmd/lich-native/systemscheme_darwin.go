package main

import (
	"context"
	"errors"
	"log"
	"os/exec"
	"time"
)

// ponytail: polls `defaults`, so a switch shows up to this late; the
// AppleInterfaceThemeChangedNotification observer is the upgrade, through
// an Objective-C shim no one here can compile or run without a Mac.
const interfaceStylePoll = 3 * time.Second

// watchSystemDark reads macOS's interface style and hands every later change
// to onChange. Without `defaults` it reports dark, the scheme the window
// opened in before System was followed.
func watchSystemDark(ctx context.Context, onChange func(dark bool)) bool {
	dark, err := readInterfaceDark()
	if err != nil {
		log.Printf("appearance: can't read AppleInterfaceStyle, System draws dark: %v", err)
		return true
	}
	go followInterfaceStyle(ctx, dark, onChange)
	return dark
}

// readInterfaceDark treats a non-zero exit as light: `defaults` fails when
// the key is missing, which is how macOS stores light mode.
func readInterfaceDark() (bool, error) {
	out, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
	if _, missing := errors.AsType[*exec.ExitError](err); missing {
		return false, nil
	}
	return interfaceStyleDark(string(out)), err
}

func followInterfaceStyle(ctx context.Context, dark bool, onChange func(dark bool)) {
	tick := time.NewTicker(interfaceStylePoll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			now, err := readInterfaceDark()
			if err != nil || now == dark {
				continue
			}
			dark = now
			onChange(dark)
		}
	}
}
