//go:build !darwin

package main

// nativePageInput is what the pane takes over a page the system shows: the
// page takes the rest from the display server itself.
func (u *window) nativePageInput(gtx C, p *browserPaneUI, session string, scale float32, _ bool) {
	u.nativeNotesInput(gtx, p, session, scale)
}
