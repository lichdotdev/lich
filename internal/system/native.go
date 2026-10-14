package system

import (
	"fmt"
	"sync"
)

// nativeWindow is the native window (`lich native`) once it attached: from then
// on it is lich's window, the one the tray and a second launch bring back, in
// place of the Chromium one.
type nativeWindow struct {
	mu         sync.Mutex
	executable string
	// raise brings an open native window forward, false when none is connected.
	raise func() bool
	// launch starts the native window again on this backend.
	launch func(executable string) error
}

// SetNativeWindow wires how lich brings the native window forward and starts
// it again once it closed. Called at startup.
func (s *Service) SetNativeWindow(raise func() bool, launch func(executable string) error) {
	s.native.raise = raise
	s.native.launch = launch
}

// AttachNativeOptions is the native window announcing itself.
type AttachNativeOptions struct {
	// Executable is the native window's own binary, the one started again when
	// lich is asked for its window after that window closed.
	Executable string `json:"executable"`
}

// AttachNative makes the native window lich's window for the rest of this run.
func (s *Service) AttachNative(opts AttachNativeOptions) {
	s.native.mu.Lock()
	s.native.executable = opts.Executable
	s.native.mu.Unlock()
}

// showNative brings the native window back when one attached, reporting
// whether one had.
func (s *Service) showNative() (bool, error) {
	s.native.mu.Lock()
	executable := s.native.executable
	s.native.mu.Unlock()
	if executable == "" {
		return false, nil
	}
	if s.native.raise() {
		return true, nil
	}
	if err := s.native.launch(executable); err != nil {
		return true, fmt.Errorf("start the native window %s: %w", executable, err)
	}
	return true, nil
}
