package system

import "github.com/lichdotdev/lich/api"

// The backend outlives its window: closing the window leaves every session
// running, and these two calls are how anyone else gets the window back or
// ends lich for good. A second launch of lich asks for ShowWindow, and
// `lich quit` asks for Quit.

// SetQuit wires how lich ends. Called at startup.
func (s *Service) SetQuit(quit func() error) {
	s.quit = quit
}

type ShowWindowOptions = api.ShowWindowOptions

// ShowWindow starts the native window on the running backend, or brings it
// forward when it is open. It returns as soon as the window is asked for, not
// when it is on screen; a window that cannot open is reported to
// NativeHooks.Failed, not here.
func (s *Service) ShowWindow(ShowWindowOptions) error {
	s.openNative()
	return nil
}

type QuitOptions = api.QuitOptions

// Quit ends lich: the window closes, every session's process ends with it, and
// the process exits. It returns once the exit is under way, so the reply can be
// lost to the exit it announces.
func (s *Service) Quit(QuitOptions) error {
	return s.quit()
}
