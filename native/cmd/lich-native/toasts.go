package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n; the web's en catalog.
const (
	toastCheckoutGone       = "This session's worktree is gone, so the session was closed. Re-create the worktree to pick it up again."
	toastConversationGone   = "The previous conversation is no longer available — starting a new session."
	toastRememberFailed     = "Couldn't remember the choice: %v"
	toastStartFailed        = "Session failed to start: %v"
	toastNotAttached        = "Not attached: %s"
	toastOpenCheckoutFailed = "Could not open the checkout: %v"
	toastOpenFolderFailed   = "Could not open the folder: %v"
	toastRelocated          = "%s now opens %s"
	toastRelocateFailed     = "Relocate failed: %v"
	toastReadFailed         = "Couldn't read %s"
	toastRevertFailed       = "Couldn't revert lines in %s"
)

var (
	flagToasts         = flag.String("toasts", "", "comma-separated toasts to raise at start, the last in front, for -shot: default, success, info, warning, error, action, attention, undo")
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
// needs-your-input and closed-session toasts.
var demoToasts = map[string]ui.Toast{
	"default": {Title: "Closed Claude 3"},
	"success": {Variant: ui.ToastSuccess, Title: "lich now opens ~/try/skipo"},
	"info":    {Variant: ui.ToastInfo, Title: "No room to show it beside"},
	"warning": {Variant: ui.ToastWarning, Title: "lich's previous run ended unexpectedly"},
	"error":   failure(toastOpenFolderFailed, "no file manager found"),
	"action": {Variant: ui.ToastWarning, Title: "lich's previous run ended unexpectedly",
		Description: "Your sessions were restored, but whatever they were running stopped with it.",
		Action:      &ui.ToastButton{Label: "Open log folder"}, Cancel: &ui.ToastButton{Label: "Dismiss"}},
	"attention": needsInputToast(statusReport{label: "Claude 3", project: "skipo",
		reason: "Claude needs your permission to use Bash"}, func(string, string) {}),
	"undo": {Title: fmt.Sprintf(toastClosed, "Claude 3"), Action: &ui.ToastButton{Label: toastUndo}},
}

// raiseDemoToasts is -toasts. They stay up, so the shot finds them.
func (u *window) raiseDemoToasts(names string) error {
	for name := range strings.SplitSeq(names, ",") {
		t, ok := demoToasts[name]
		if !ok {
			return fmt.Errorf("-toasts: no demo toast %q", name)
		}
		t.Duration = ui.ToastForever
		u.toaster.Show(t)
	}
	return nil
}
