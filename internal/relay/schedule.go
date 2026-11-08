package relay

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/store"
)

// trigger is what a session is asked to wait for: a time, a condition, or
// neither to clear what it holds.
type trigger struct {
	at     int64
	on     string
	prompt string
}

func (t trigger) validate(now time.Time) error {
	if t.at != 0 && t.on != "" {
		return errors.New("a scheduled prompt waits for a time or a condition, not both")
	}
	if t.at == 0 && t.on == "" {
		return nil
	}
	if t.prompt == "" {
		return errors.New("a scheduled prompt needs the prompt to type")
	}
	if t.at != 0 && t.at <= now.Unix() {
		return errors.New("the time for a scheduled prompt has already passed")
	}
	switch t.on {
	case "", api.WatchChecksFailed, api.WatchChecksPassed, api.WatchReview:
		return nil
	}
	return fmt.Errorf("unknown condition %q: want %s, %s or %s", t.on,
		api.WatchChecksFailed, api.WatchChecksPassed, api.WatchReview)
}

// SetSchedule is the person's trigger for a session. It replaces whatever the
// session holds, whoever set it.
func (s *Service) SetSchedule(_ context.Context, opts api.SetScheduleOptions) (api.None, error) {
	sess, ok := s.sessionRow(opts.SessionID)
	if !ok {
		return api.None{}, fmt.Errorf("no open session %q", opts.SessionID)
	}
	return api.None{}, s.applyTrigger(sess, trigger{at: opts.At, on: opts.On, prompt: opts.Prompt}, "")
}

// ScheduleSelf is an agent's trigger for the session it runs in. What the
// person set there outranks it.
func (s *Service) ScheduleSelf(_ context.Context, opts api.ScheduleSelfOptions) (api.None, error) {
	sess, ok := s.sessionRow(opts.From)
	if !ok {
		return api.None{}, errors.New("schedule runs from inside a lich session (LICH_SESSION_ID names none that is open)")
	}
	if personArmed(sess) {
		return api.None{}, errors.New("the person at this session scheduled a prompt here, so it stays until they change it")
	}
	return api.None{}, s.applyTrigger(sess, trigger{at: opts.At, on: opts.On, prompt: opts.Prompt}, api.ScheduledByAgent)
}

func personArmed(sess store.Session) bool {
	return (sess.Watch.On != "" && sess.Watch.By == "") || (sess.ScheduledPrompt != "" && sess.ScheduledBy == "")
}

func (s *Service) applyTrigger(sess store.Session, t trigger, by string) error {
	if err := t.validate(s.now()); err != nil {
		return err
	}
	switch {
	case t.on != "":
		return s.armWatch(sess, t, by)
	case t.at != 0:
		return s.armTime(sess, t, by)
	}
	return s.clearTrigger(sess, by)
}

func (s *Service) armWatch(sess store.Session, t trigger, by string) error {
	w := keptCounters(sess.Watch, by)
	w.On, w.Prompt, w.By = t.on, t.prompt, by
	if err := s.saveWatch(sess.ID, w); err != nil {
		return err
	}
	// lich's own continuation stays: it is the turn the limit cut short, not
	// the trigger being replaced.
	if sess.ScheduledPrompt == "" || isResumePrompt(sess.ScheduledPrompt) {
		return nil
	}
	return s.writeSchedule(sess.ID, 0, "", "")
}

func (s *Service) armTime(sess store.Session, t trigger, by string) error {
	if err := s.writeSchedule(sess.ID, t.at, t.prompt, by); err != nil {
		return err
	}
	if sess.Watch.On == "" {
		return nil
	}
	return s.saveWatch(sess.ID, keptCounters(sess.Watch, by))
}

// clearTrigger removes the trigger the window shows for the session: the watch
// when it has one, the scheduled prompt otherwise.
func (s *Service) clearTrigger(sess store.Session, by string) error {
	if sess.Watch.On != "" {
		return s.saveWatch(sess.ID, keptCounters(sess.Watch, by))
	}
	if sess.ScheduledPrompt == "" {
		return nil
	}
	return s.writeSchedule(sess.ID, 0, "", "")
}

// keptCounters is what survives of a watch being replaced: an agent keeps
// counting against the same limit, and a person starts it over.
func keptCounters(w api.SessionWatch, by string) api.SessionWatch {
	if by != api.ScheduledByAgent {
		return api.SessionWatch{}
	}
	return api.SessionWatch{Runs: w.Runs, PR: w.PR, Seen: w.Seen}
}

func (s *Service) writeSchedule(id string, at int64, text, by string) error {
	write := s.sessions.SetSessionSchedule
	if by == api.ScheduledByAgent {
		write = s.sessions.SetAgentSchedule
	}
	if err := write(id, at, text); err != nil {
		return err
	}
	s.emitSchedule(ScheduleEvent{ID: id, At: at, Prompt: text}, by)
	return nil
}

func (s *Service) emitSchedule(ev ScheduleEvent, by string) {
	if s.events == nil {
		return
	}
	events.Send(s.events.Emit, api.SessionScheduleEvent, ev)
	events.Send(s.events.Emit, api.SessionScheduleByEvent, api.SessionScheduleBy{ID: ev.ID, By: by})
}

func (s *Service) saveWatch(id string, w api.SessionWatch) error {
	if err := s.sessions.SetSessionWatch(id, w); err != nil {
		return err
	}
	if s.events != nil {
		events.Send(s.events.Emit, api.SessionWatchEvent, api.SessionWatchChange{ID: id, Watch: w})
	}
	return nil
}

func logSaveWatch(label string, err error) {
	if err != nil {
		slog.Warn("relay: save a session's watch", "session", label, "err", err)
	}
}
