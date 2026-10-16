package system

import (
	"errors"
	"fmt"
	"sync"

	"github.com/omartelo/lich/internal/restart"
)

// The windows lich can open, and the setting that picks the one opened when
// lich is asked for its window.
const (
	WindowWeb     = "web"
	WindowNative  = "native"
	WindowKindKey = "window.kind"
)

// NativeHooks are how lich drives the native window (`lich native`), wired by
// main.
type NativeHooks struct {
	// Raise brings an open native window forward, false when none is connected.
	Raise func() bool
	// Launch starts the native window on this backend; exited hears how the
	// process ended once it does.
	Launch func(executable string, exited func(state string)) error
	// Close asks an open native window to close.
	Close func()
	// Locate is the native window's binary, shipped beside lich's.
	Locate func() (string, error)
}

// nativeWindow is the native window's state this run.
type nativeWindow struct {
	mu    sync.Mutex
	hooks NativeHooks
	// executable is the binary the native window that attached ran from, the
	// one started again over Locate's: a build of one's own stays one.
	executable string
	// active is the native window being lich's window: it attached, and no
	// web window took its place since.
	active bool
	// attached is the launch in flight having reached AttachNative; an exit
	// before that is a native window that could not start.
	attached bool
	// fallback is why lich opened the web window in the native one's place,
	// told once (TakeWindowFallback).
	fallback string
	// chosen reads WindowKindKey.
	chosen func() string
}

// SetNativeWindow wires the native window. Called at startup.
func (s *Service) SetNativeWindow(hooks NativeHooks) {
	s.native.mu.Lock()
	s.native.hooks = hooks
	s.native.mu.Unlock()
}

// SetWindowChoice wires how the chosen window is read. Called at startup.
func (s *Service) SetWindowChoice(chosen func() string) {
	s.native.mu.Lock()
	s.native.chosen = chosen
	s.native.mu.Unlock()
}

// AttachNativeOptions is the native window announcing itself.
type AttachNativeOptions struct {
	// Executable is the native window's own binary, the one started again when
	// lich is asked for its window after that window closed.
	Executable string `json:"executable"`
}

// AttachNative makes the native window lich's window until a web window takes
// its place.
func (s *Service) AttachNative(opts AttachNativeOptions) {
	s.native.mu.Lock()
	defer s.native.mu.Unlock()
	s.native.executable = opts.Executable
	s.native.active, s.native.attached = true, true
}

// OpenWindowOptions is one OpenWindow call.
type OpenWindowOptions struct {
	// Kind is WindowWeb or WindowNative.
	Kind string `json:"kind"`
}

// OpenWindow opens the window of opts.Kind, closing the other one, without
// changing which one WindowKindKey picks: one window at a time, as both
// endpoints the windows live on take one client. A native window that cannot
// start leaves the web one open in its place, never no window.
func (s *Service) OpenWindow(opts OpenWindowOptions) error {
	switch opts.Kind {
	case WindowWeb:
		s.openWeb("")
		return nil
	case WindowNative:
		s.openNative()
		return nil
	}
	return fmt.Errorf("unknown window kind %q, want %q or %q", opts.Kind, WindowWeb, WindowNative)
}

// TakeWindowFallback is why lich opened the web window in place of the native
// one, "" when it did not; reading it clears it, so a reload does not tell it
// again.
func (s *Service) TakeWindowFallback() string {
	s.native.mu.Lock()
	defer s.native.mu.Unlock()
	reason := s.native.fallback
	s.native.fallback = ""
	return reason
}

// chosenWindow is WindowKindKey, the web window for anything else.
func (s *Service) chosenWindow() string {
	s.native.mu.Lock()
	chosen := s.native.chosen
	s.native.mu.Unlock()
	if chosen != nil && chosen() == WindowNative {
		return WindowNative
	}
	return WindowWeb
}

// openWeb shows the web window, closing an open native one first; a reason
// records why it opened in the native one's place.
func (s *Service) openWeb(reason string) {
	s.native.mu.Lock()
	closeNative := s.native.active && s.native.hooks.Close != nil
	s.native.active = false
	if reason != "" {
		s.native.fallback = reason
	}
	hooks := s.native.hooks
	s.native.mu.Unlock()
	if closeNative {
		hooks.Close()
	}
	s.showWindow()
}

// openNative raises the open native window, or closes the web one and starts
// the native one, falling back to the web window when it cannot start.
func (s *Service) openNative() {
	s.native.mu.Lock()
	hooks, active, executable := s.native.hooks, s.native.active, s.native.executable
	s.native.mu.Unlock()
	if hooks.Launch == nil {
		s.openWeb("this lich has no native window wired")
		return
	}
	if active && hooks.Raise() {
		return
	}
	if executable == "" {
		found, err := hooks.Locate()
		if err != nil {
			s.openWeb(err.Error())
			return
		}
		executable = found
	}
	if err := s.closeWindow(); err != nil && !errors.Is(err, restart.ErrNoWindow) {
		s.openWeb(fmt.Sprintf("close the web window: %v", err))
		return
	}
	s.native.mu.Lock()
	s.native.attached = false
	s.native.mu.Unlock()
	if err := hooks.Launch(executable, s.nativeExited); err != nil {
		s.openWeb(fmt.Sprintf("start %s: %v", executable, err))
	}
}

// nativeExited hears a native window lich started end: one that never
// attached could not start, and the web window opens in its place.
func (s *Service) nativeExited(state string) {
	s.native.mu.Lock()
	attached := s.native.attached
	s.native.mu.Unlock()
	if !attached {
		s.openWeb(fmt.Sprintf("the native window ended (%s) before it opened", state))
	}
}
