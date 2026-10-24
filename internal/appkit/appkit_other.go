//go:build !darwin

package appkit

// Run runs app: only macOS has an application loop lich has to run beside.
func Run(app func()) { app() }

// Handle does nothing: reopen and terminate are LaunchServices' events.
func Handle(onReopen, onTerminate func()) {}
