package system

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/lichdotdev/lich/api"
)

// NativeHooks are how lich drives its window, the native one (`lich native`),
// wired by main.
type NativeHooks struct {
	// Raise brings an open native window forward, false when none is connected.
	Raise func() bool
	// Launch starts the native window on this backend; exited hears how the
	// process ended once it does.
	Launch func(executable string, exited func(state string)) error
	// Locate is the native window's binary, shipped beside lich's.
	Locate func() (string, error)
	// Failed hears why a native window could not open.
	Failed func(reason string)
}

// nativeWindow is the native window's state this run.
type nativeWindow struct {
	mu    sync.Mutex
	hooks NativeHooks
	// executable is the binary the native window that attached ran from, the
	// one started again over Locate's: a build of one's own stays one.
	executable string
	// attached is the launch in flight having reached AttachNative; an exit
	// before that is a native window that could not start.
	attached bool
}

// SetNativeWindow wires the native window. Called at startup.
func (s *Service) SetNativeWindow(hooks NativeHooks) {
	s.native.mu.Lock()
	s.native.hooks = hooks
	s.native.mu.Unlock()
}

type AttachNativeOptions = api.AttachNativeOptions

// AttachNative answers attachNative over RPC.
func (s *Service) AttachNative(_ context.Context, opts AttachNativeOptions) (api.None, error) {
	s.attachNative(opts)
	return api.None{}, nil
}

// attachNative records the native window that started as lich's window.
func (s *Service) attachNative(opts AttachNativeOptions) {
	s.native.mu.Lock()
	defer s.native.mu.Unlock()
	s.native.executable = opts.Executable
	s.native.attached = true
	slog.Info("native window attached", "executable", opts.Executable)
}

// openNative raises the open native window, or starts one; a window that
// cannot start is reported to Failed.
func (s *Service) openNative() {
	s.native.mu.Lock()
	hooks, executable := s.native.hooks, s.native.executable
	s.native.mu.Unlock()
	if hooks.Raise() {
		return
	}
	if executable == "" {
		found, err := hooks.Locate()
		if err != nil {
			hooks.Failed(err.Error())
			return
		}
		executable = found
	}
	s.native.mu.Lock()
	s.native.attached = false
	s.native.mu.Unlock()
	if err := hooks.Launch(executable, s.nativeExited); err != nil {
		hooks.Failed(fmt.Sprintf("start %s: %v", executable, err))
	}
}

// nativeExited hears a native window lich started end: one that never
// attached could not start.
func (s *Service) nativeExited(state string) {
	s.native.mu.Lock()
	attached, failed := s.native.attached, s.native.hooks.Failed
	s.native.mu.Unlock()
	if !attached {
		failed(fmt.Sprintf("the native window ended (%s) before it opened", state))
	}
}
