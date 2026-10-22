package main

import (
	"encoding/xml"
	"strings"
)

// The Windows toast's protocol: a click on it, or on Open, starts
// `lich.exe focus "lich-session:<id>"` (registered in HKCU by
// registerToastProtocol), which opens the session's card the way an editor's
// `lich focus` does. lich focus reads the same prefix (internal/cli).
const (
	toastSessionScheme = "lich-session"
	// appUserModelIDPrefix predates the lichdotdev org and stays: Windows
	// keys taskbar pins and the notification settings a user chose on the
	// id, so a new one would split every pinned lich from its window and
	// reset those settings.
	appUserModelIDPrefix = "omartelo."
	// toastAppID is the AppUserModelID lich's Start Menu shortcut carries
	// (build/windows/lich.iss) and the window claims (taskbar_windows.go):
	// toasts under it are lich's in Settings › Notifications.
	toastAppID = appUserModelIDPrefix + "lich"
	// toastGroup holds every lich toast; the session id is each one's tag,
	// so a newer one replaces it and History.Remove takes it down.
	toastGroup = "lich"
)

func toastLaunch(sessionID string) string { return toastSessionScheme + ":" + sessionID }

// toastXML is n as a ToastGeneric toast. A waiting session's is a reminder,
// which stays on screen until it is acted on or lich takes it down, as the
// Linux note that never expires does.
func toastXML(n desktopNote, open, dismiss string) string {
	var b strings.Builder
	launch := escapeXML(toastLaunch(n.sessionID))
	b.WriteString(`<toast activationType="protocol" launch="` + launch + `"`)
	if n.status == statusWaiting {
		b.WriteString(` scenario="reminder"`)
	}
	b.WriteString(`><visual><binding template="ToastGeneric">`)
	for _, line := range []string{n.title, n.body, n.meta} {
		if line != "" {
			b.WriteString("<text>" + escapeXML(line) + "</text>")
		}
	}
	b.WriteString(`</binding></visual><actions>`)
	b.WriteString(`<action activationType="protocol" arguments="` + launch + `" content="` + escapeXML(open) + `"/>`)
	b.WriteString(`<action activationType="system" arguments="dismiss" content="` + escapeXML(dismiss) + `"/>`)
	b.WriteString(`</actions></toast>`)
	return b.String()
}

func escapeXML(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s)) // a strings.Builder never fails a write
	return b.String()
}
