// Package appkit makes lich's backend a macOS application, through purego
// rather than cgo: it runs AppKit's event loop on the main thread, which
// AppKit insists on, and the backend beside it. That loop is what hears
// LaunchServices: opening lich again while it runs reaches this process as a
// reopen, and logging out as a terminate. It is also the thread the menu-bar
// item lives on (internal/tray).
package appkit
