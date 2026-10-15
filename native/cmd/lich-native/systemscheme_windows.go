package main

import (
	"context"
	"log"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	personalizeKey    = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	appsUseLightTheme = "AppsUseLightTheme"
)

// watchSystemDark reads the apps' color mode from the registry and hands
// every later change to onChange. Without the value (before Windows 10 1809)
// it reports dark, the scheme the window opened in before System was followed.
func watchSystemDark(ctx context.Context, onChange func(dark bool)) bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, personalizeKey, registry.QUERY_VALUE|registry.NOTIFY)
	if err != nil {
		log.Printf("appearance: no Personalize key, System draws dark: %v", err)
		return true
	}
	dark, err := readAppsDark(key)
	if err != nil {
		key.Close()
		log.Printf("appearance: no %s, System draws dark: %v", appsUseLightTheme, err)
		return true
	}
	go followRegistry(ctx, key, onChange)
	return dark
}

func readAppsDark(key registry.Key) (bool, error) {
	value, _, err := key.GetIntegerValue(appsUseLightTheme)
	return appsUseLightThemeDark(value), err
}

// followRegistry re-arms the notification after every change: one
// RegNotifyChangeKeyValue reports one change. THREAD_AGNOSTIC keeps the
// registration alive when the goroutine moves to another OS thread.
func followRegistry(ctx context.Context, key registry.Key, onChange func(dark bool)) {
	defer key.Close()
	changed, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		log.Printf("appearance: can't follow the color scheme: %v", err)
		return
	}
	defer windows.CloseHandle(changed)
	stop, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		log.Printf("appearance: can't follow the color scheme: %v", err)
		return
	}
	defer windows.CloseHandle(stop)
	go func() {
		<-ctx.Done()
		windows.SetEvent(stop)
	}()
	filter := uint32(windows.REG_NOTIFY_CHANGE_LAST_SET | windows.REG_NOTIFY_THREAD_AGNOSTIC)
	for {
		if err := windows.RegNotifyChangeKeyValue(windows.Handle(key), false, filter, changed, true); err != nil {
			log.Printf("appearance: can't follow the color scheme: %v", err)
			return
		}
		fired, err := windows.WaitForMultipleObjects([]windows.Handle{changed, stop}, false, windows.INFINITE)
		if err != nil || fired != windows.WAIT_OBJECT_0 {
			return
		}
		if dark, err := readAppsDark(key); err == nil {
			onChange(dark)
		}
	}
}
