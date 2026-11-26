package main

// nativePageInput is all of the pane's input over a page the system shows:
// a page's layer takes none of its own on macOS, so the pane sends the
// pointer, keys and text over the wire, as it does for the screencast.
func (u *window) nativePageInput(gtx C, p *browserPaneUI, session string, scale float32, focused bool) {
	u.browserInput(gtx, p, session, scale, focused)
}
