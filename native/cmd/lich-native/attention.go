package main

import (
	"fmt"
	"image/color"
	"time"

	"github.com/omartelo/lich/native/ui"
)

// attentionToastDuration is project-events.tsx ATTENTION_TOAST_MS.
const attentionToastDuration = 10 * time.Second

// ponytail: English until the native window has i18n (shell.projectEvents.open).
const attentionOpen = "Open"

// attentionToast is project-events.tsx's needs-your-input toast, raised for
// every waiting report on a session other than the one on screen, whose own
// terminal shows the prompt. It keeps no state: a second prompt in one turn
// is a second question, so a repeat raises a second toast.
type attentionToast struct {
	show func(ui.Toast)
	// open puts the session on screen, the toast's Open.
	open func(projectID, sessionID string)
}

func (a attentionToast) report(r statusReport) {
	if r.status != statusWaiting || r.onScreen {
		return
	}
	a.show(needsInputToast(r, a.open))
}

// needsInputToast is the toast's body: the title, what the session is
// blocked on when the provider said, and the project it lives in.
func needsInputToast(r statusReport, open func(projectID, sessionID string)) ui.Toast {
	t := ui.Toast{Title: fmt.Sprintf(notifyNeedsInput, r.label), Icon: icBell, IconColor: waitTone, Duration: attentionToastDuration,
		Action: &ui.ToastButton{Label: attentionOpen, OnClick: func() { open(r.projectID, r.id) }}}
	if r.reason != "" {
		t.Lines = append(t.Lines, ui.ToastLine{Text: r.reason})
	}
	if r.project != "" {
		t.Lines = append(t.Lines, ui.ToastLine{Icon: icFolder, Text: r.project})
	}
	return t
}

func waitTone(th *ui.Theme) color.NRGBA { return th.ToneWait }
