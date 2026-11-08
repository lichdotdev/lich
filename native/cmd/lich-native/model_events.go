package main

import (
	"context"
	"slices"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// sessionTouched reads a session's git state now rather than at the next
// tick, and tells the Review tab's diff. A session of a project that is not on
// screen has nothing drawn from git, so it waits for its project to be shown.
func (m *model) sessionTouched(ctx context.Context, ev backend.Event) {
	e, err := backend.Decode(ev, api.SessionTouchedEvent)
	if err != nil {
		return
	}
	m.mu.Lock()
	p, i := m.sessionAt(e.ID)
	if p == nil || p != m.shownProject() {
		m.mu.Unlock()
		return
	}
	shown, checkout := m.shownPath(*p, p.Sessions[i]), sessionPath(*p, p.Sessions[i])
	m.mu.Unlock()
	go m.refreshPath(ctx, shown)
	if m.diffTouched != nil {
		m.diffTouched(checkout)
	}
}

// checkoutWritten reads again every card of the project on screen that sits in
// checkout, after an edit made through lich rather than by a session.
func (m *model) checkoutWritten(ctx context.Context, checkout string) {
	m.mu.Lock()
	var paths []string
	if p := m.shownProject(); p != nil {
		for _, s := range p.Sessions {
			if path := m.shownPath(*p, s); sessionPath(*p, s) == checkout && !slices.Contains(paths, path) {
				paths = append(paths, path)
			}
		}
	}
	m.mu.Unlock()
	for _, path := range paths {
		go m.refreshPath(ctx, path)
	}
}

// scheduleEvent takes a change to what a session waits on into its row, and
// puts in front of the person what they would otherwise only find missing.
func (m *model) scheduleEvent(ev backend.Event) {
	switch ev.Name {
	case api.SessionScheduleEvent.Name:
		if e, err := backend.Decode(ev, api.SessionScheduleEvent); err == nil {
			m.editSession(e.ID, func(s *backend.Session) { s.ScheduledAt, s.ScheduledPrompt, s.ScheduledBy = e.At, e.Prompt, "" })
		}
	case api.SessionScheduleByEvent.Name:
		if e, err := backend.Decode(ev, api.SessionScheduleByEvent); err == nil {
			m.editSession(e.ID, func(s *backend.Session) { s.ScheduledBy = e.By })
		}
	case api.SessionWatchEvent.Name:
		if e, err := backend.Decode(ev, api.SessionWatchEvent); err == nil {
			m.editSession(e.ID, func(s *backend.Session) { s.Watch = e.Watch })
		}
	case api.SessionWatchStoppedEvent.Name:
		if e, err := backend.Decode(ev, api.SessionWatchStoppedEvent); err == nil && m.showToast != nil {
			m.showToast(watchStoppedToast(e))
		}
	case api.ScheduleForfeitedEvent.Name:
		if e, err := backend.Decode(ev, api.ScheduleForfeitedEvent); err == nil && m.showToast != nil {
			m.showToast(scheduleLostToast(e))
		}
	}
}

func (m *model) editSession(id string, edit func(*backend.Session)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, i := m.sessionAt(id); p != nil {
		edit(&p.Sessions[i])
	}
}

func watchStoppedToast(e api.SessionWatchStopped) ui.Toast {
	return ui.Toast{Icon: icWatch, IconColor: waitTone,
		Title:       i18n.T("shell.projectEvents.watchStopped", i18n.Args{"number": e.Number}),
		Description: i18n.T("shell.projectEvents.watchStoppedDetail", i18n.Args{"label": e.Label, "runs": e.Runs})}
}

func scheduleLostToast(e api.ScheduleForfeited) ui.Toast {
	when := time.Unix(e.At, 0).Format(scheduleClockLayout)
	return ui.Toast{Icon: icSchedule, IconColor: waitTone,
		Title:       i18n.T("shell.projectEvents.scheduleLost"),
		Description: i18n.T("shell.projectEvents.scheduleLostDetail", i18n.Args{"label": e.Label, "when": when})}
}

// sessionFocus brings the card `lich focus` named up. One whose project is not
// open here has nowhere to be shown, so it is left alone.
func (m *model) sessionFocus(ev backend.Event) {
	e, err := backend.Decode(ev, api.SessionFocusEvent)
	if err != nil || m.focusSession == nil {
		return
	}
	m.mu.Lock()
	p, _ := m.sessionAt(e.ID)
	projectID := ""
	if p != nil {
		projectID = p.ID
	}
	m.mu.Unlock()
	if projectID != "" {
		m.focusSession(projectID, e.ID)
	}
}

// profileChanged takes a profile's update into its provider's book, whichever
// window or call made it.
func (m *model) profileChanged(c backend.ProfileChanged) {
	if book := m.profileBook(c.Provider); book != nil {
		book.changed(backend.Profile{Profile: c.Profile})
	}
}

// loginStep is one step of a profile login of the provider kind names. The
// profiles are read again before the screen hears the login ended, so what it
// draws next is the account that just signed in, whichever window or terminal
// ran the login.
func (m *model) loginStep(ctx context.Context, kind string, step backend.ProfileLogin) {
	if !step.Done {
		m.forwardLogin(kind, step)
		return
	}
	go func() {
		if book := m.profileBook(kind); book != nil {
			book.loginFinished(ctx)
		}
		m.forwardLogin(kind, step)
	}()
}

// loginEventKind is the provider a login event name belongs to.
func loginEventKind(event string) string {
	if event == api.CodexProfileLoginEvent.Name {
		return codexKind
	}
	return claudeKind
}

func (m *model) forwardLogin(kind string, step backend.ProfileLogin) {
	if m.loginProgress != nil {
		m.loginProgress(kind, step)
	}
}
