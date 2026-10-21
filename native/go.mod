module github.com/lichdotdev/lich/native

go 1.27.0

require (
	gioui.org v0.10.3
	gioui.org/x v0.10.3
	github.com/coder/websocket v1.8.14
	github.com/creack/pty v1.1.24
	github.com/go-text/typesetting v0.3.5
	github.com/godbus/dbus/v5 v5.2.2
	github.com/lichdotdev/godemirror v0.8.0
	golang.org/x/exp/shiny v0.0.0-20261009195045-ca0d7ba23607
	golang.org/x/image v0.47.0
	golang.org/x/sys v0.49.0
)

require (
	gioui.org/shader v1.0.9 // indirect
	github.com/alecthomas/chroma/v2 v2.27.0 // indirect
	github.com/dlclark/regexp2/v2 v2.2.1 // indirect
	github.com/yuin/goldmark v1.4.13 // indirect
	golang.org/x/exp v0.0.0-20250408133849-7e4ce0ab07d0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)

// lichdotdev/gio, branch lich, carries what gioui.org lacks: app.DropEvent
// (external file drop), Window.Activate (raising the window on Wayland), word
// deletion with Ctrl off macOS, Wayland fixes for EGL vsync, clipboard reads,
// pointer.Leave and the cursor (cursor-shape-v1), an undecorated window that
// moves, resizes and maximizes on Wayland and X11, app.TrafficLights (macOS's
// buttons on an undecorated window), and the Editor's scroll offset (the
// textarea's scrollbar). Every patch of ours lands on that branch.
replace gioui.org => github.com/lichdotdev/gio v0.0.0-20261021015051-4409150e2cf9
