package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unicode/utf16"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/lichdotdev/lich/native/i18n"
)

const toastJobQueue = 32

// toastNotes raises Windows toasts through Windows PowerShell, whose WinRT
// projection reaches ToastNotificationManager without COM bindings in Go.
// A click never comes back to this process: it goes through the
// lich-session protocol to `lich focus`, so it works from the Action Center
// after the toast left the screen too.
// ponytail: one powershell.exe per toast, about a second and 60 MB while it
// runs; an in-process WinRT notifier is the upgrade if that ever shows.
type toastNotes struct {
	jobs chan func()
}

// newDesktopNotes is lich's toast under its AppUserModelID once the protocol
// a click goes through is registered, else fallback.
func newDesktopNotes(ctx context.Context, fallback noteSink, _ func(projectID, sessionID, token string), _ func(string)) noteSink {
	if err := registerToastProtocol(); err != nil {
		log.Printf("notify: no toasts, using the backend: %v", err)
		return fallback
	}
	t := &toastNotes{jobs: make(chan func(), toastJobQueue)}
	go t.work(ctx)
	return t
}

func (t *toastNotes) work(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-t.jobs:
			job()
		}
	}
}

func (t *toastNotes) show(n desktopNote) {
	t.jobs <- func() {
		doc := toastXML(n, i18n.T("shell.projectEvents.open"), i18n.T("shell.uncleanExitGate.dismiss"))
		if err := runToastScript(toastShowScript, doc, n.sessionID); err != nil {
			log.Printf("notify: %v", err)
		}
	}
}

func (t *toastNotes) withdraw(sessionID string) {
	t.jobs <- func() {
		if err := runToastScript(toastRemoveScript, "", sessionID); err != nil {
			log.Printf("notify: close %s: %v", sessionID, err)
		}
	}
}

// The scripts take their values as base64 literals runToastScript fills in,
// so nothing a session says reaches PowerShell's parser.
const (
	toastLoad = `[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] > $null
function D($s) { [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($s)) }
$doc = D '%s'; $tag = D '%s'; $group = D '%s'; $app = D '%s'
`
	toastShowScript = toastLoad + `$x = New-Object Windows.Data.Xml.Dom.XmlDocument
$x.LoadXml($doc)
$t = [Windows.UI.Notifications.ToastNotification]::new($x)
$t.Tag = $tag; $t.Group = $group
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($app).Show($t)
`
	toastRemoveScript = toastLoad + `[Windows.UI.Notifications.ToastNotificationManager]::History.Remove($tag, $group, $app)
`
)

func runToastScript(script, doc, tag string) error {
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand",
		encodePowerShell(fmt.Sprintf(script, b64(doc), b64(tag), b64(toastGroup), b64(toastAppID))))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("toast: %w: %s", err, out)
	}
	return nil
}

// encodePowerShell is script as -EncodedCommand wants it: base64 of UTF-16LE.
func encodePowerShell(script string) string {
	units := utf16.Encode([]rune(script))
	raw := make([]byte, 2*len(units))
	for i, u := range units {
		raw[2*i], raw[2*i+1] = byte(u), byte(u>>8)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// registerToastProtocol points lich-session: at the lich.exe beside this
// window and names toastAppID, both per user. The name is what lets a toast
// show under the id when no shortcut carries it (Scoop's, the zip); the
// installer's uninstaller removes both keys (lich.iss).
func registerToastProtocol() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	lich := filepath.Join(filepath.Dir(exe), "lich.exe")
	if _, err := os.Stat(lich); err != nil {
		return fmt.Errorf("a click needs lich.exe beside the window: %w", err)
	}
	command := fmt.Sprintf(`"%s" focus "%%1"`, lich)
	return errors.Join(
		setRegistryValues(`Software\Classes\AppUserModelId\`+toastAppID, map[string]string{"DisplayName": "lich"}),
		setRegistryValues(`Software\Classes\`+toastSessionScheme, map[string]string{"": "URL:lich session", "URL Protocol": ""}),
		setRegistryValues(`Software\Classes\`+toastSessionScheme+`\shell\open\command`, map[string]string{"": command}),
	)
}

func setRegistryValues(path string, values map[string]string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("registry %s: %w", path, err)
	}
	defer key.Close()
	for name, value := range values {
		if err := key.SetStringValue(name, value); err != nil {
			return fmt.Errorf("registry %s\\%s: %w", path, name, err)
		}
	}
	return nil
}
