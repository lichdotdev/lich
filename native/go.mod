module github.com/omartelo/lich/native

go 1.27.0

require (
	gioui.org v0.10.3
	gioui.org/x v0.10.3
	github.com/coder/websocket v1.8.14
	github.com/creack/pty v1.1.24
	github.com/godbus/dbus/v5 v5.2.2
	github.com/lichdotdev/godemirror v0.6.0
	golang.org/x/exp/shiny v0.0.0-20261009195045-ca0d7ba23607
	golang.org/x/image v0.47.0
	golang.org/x/sys v0.49.0
)

require (
	gioui.org/shader v1.0.9 // indirect
	github.com/alecthomas/chroma/v2 v2.27.0 // indirect
	github.com/dlclark/regexp2/v2 v2.2.1 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	github.com/yuin/goldmark v1.4.13 // indirect
	golang.org/x/exp v0.0.0-20250408133849-7e4ce0ab07d0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)

// app.DropEvent (external file drop) and Window.Activate (raising the window on
// Wayland) live in lichdotdev/gio, branch drop-event; gioui.org has neither.
replace gioui.org => github.com/lichdotdev/gio v0.0.0-20261015151446-c609b88c89a0
