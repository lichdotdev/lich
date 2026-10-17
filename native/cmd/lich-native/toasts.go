package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
)

var (
	flagToasts         = flag.String("toasts", "", "comma-separated toasts to raise at start, the last in front, for -shot: default, success, info, warning, error, action, attention, undo, update, progress")
	flagToastsExpanded = flag.Bool("toasts-expanded", false, "draw the toast stack expanded, as a hover would, for -shot")
)

// failure is the web's toast.error with a formatted title.
func failure(format string, args ...any) ui.Toast {
	for i, arg := range args {
		if err, ok := arg.(error); ok {
			args[i] = lichclient.ErrorText(err)
		}
	}
	return ui.Toast{Variant: ui.ToastError, Title: fmt.Sprintf(format, args...)}
}

// demoToasts are what -toasts raises, by name: one of each variant; action,
// the shape of the web's unclean-exit warning; attention and undo, the
// needs-your-input and closed-session toasts; update and progress, the app
// update's prompt and its download. A function, so the texts that come from
// the catalog are read in the language of the moment.
func demoToasts() map[string]ui.Toast {
	return map[string]ui.Toast{
		"default": {Title: i18n.T("shell.projects.closed", i18n.Args{"label": "Claude 3"})},
		"success": {Variant: ui.ToastSuccess, Title: i18n.T("shell.projects.relocated", i18n.Args{"name": "lich", "path": "~/try/skipo"})},
		"info":    {Variant: ui.ToastInfo, Title: i18n.T("terminal.host.noRoom")},
		"warning": {Variant: ui.ToastWarning, Title: i18n.T("shell.uncleanExitGate.title")},
		"error":   {Variant: ui.ToastError, Title: i18n.T("sidebar.sessionCard.openFolderFailed", i18n.Args{"error": "no file manager found"})},
		"action":  uncleanExitToast(func() {}),
		"attention": needsInputToast(statusReport{label: "Claude 3", project: "skipo",
			reason: "Claude needs your permission to use Bash"}, func(string, string) {}),
		"undo": {Title: "Closed Claude 3", Action: &ui.ToastButton{Label: "Undo"}},
		"update": {Title: i18n.T("shell.appUpdateGate.available", i18n.Args{"version": "1.4.0"}), Cancel: &ui.ToastButton{Label: i18n.T("shell.appUpdateGate.later")},
			Secondary: &ui.ToastButton{Label: i18n.T("shell.appUpdateGate.viewRelease")}, Action: &ui.ToastButton{Label: i18n.T("shell.appUpdateGate.updateAndInstall")}},
		"progress": updateProgressToast("1.4.0", &lichclient.AppUpdateProgress{Phase: lichclient.UpdatePhaseDownload, Received: 12_300_000, Total: 41_000_000}),
	}
}

// raiseDemoToasts is -toasts. They stay up, so the shot finds them.
func (u *window) raiseDemoToasts(names string) error {
	for name := range strings.SplitSeq(names, ",") {
		t, ok := demoToasts()[name]
		if !ok {
			return fmt.Errorf("-toasts: no demo toast %q", name)
		}
		t.Duration = ui.ToastForever
		u.toaster.Show(t)
	}
	return nil
}
