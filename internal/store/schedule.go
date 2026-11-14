package store

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
)

// noteForfeitedSchedules reports every prompt still parked on a session about to
// be deleted for good. A close parks the row and the schedule comes back with
// the card (see reopen); a delete is the one exit where the prompt is lost
// rather than delayed, and the row is the only copy of what the user wrote, so
// this is the last place it exists. Read before the delete, because after it
// there is nothing left to read.
//
// It reports twice, and neither is the other's fallback: the log line is the
// record, and whatever SetScheduleForfeited wired is what puts it in front of
// the person who parked the prompt — the card's menu is the only way to park
// one, so that person is the user at the window.
//
// A failed read costs the report, never the delete: nothing here is allowed to
// stand between the user and removing a session.
func (s *Service) noteForfeitedSchedules(where string, args ...any) {
	lost, err := s.forfeitedSchedules(where, args...)
	if err != nil {
		slog.Warn("read schedules of deleted sessions", "err", err)
		return
	}
	// Reported after the rows are drained, for the reason ClosedSessions spells
	// out: the store holds a single connection, and what SetScheduleForfeited
	// wired is a websocket write plus a desktop notification, both slow enough
	// to matter and free to read the store back.
	for _, f := range lost {
		slog.Warn("scheduled prompt forfeited with deleted session",
			"session", f.Label, "due", time.Unix(f.At, 0).Format(time.RFC3339), "prompt", f.Prompt)
		if s.scheduleForfeited != nil {
			s.scheduleForfeited(f)
		}
	}
}

func (s *Service) forfeitedSchedules(where string, args ...any) ([]ForfeitedSchedule, error) {
	rows, err := s.db.Query(
		`SELECT label, scheduled_at, scheduled_prompt
		   FROM sessions WHERE scheduled_at != 0 AND `+where,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("query forfeited schedules: %w", err)
	}
	defer rows.Close()
	var lost []ForfeitedSchedule
	for rows.Next() {
		var f ForfeitedSchedule
		if err := rows.Scan(&f.Label, &f.At, &f.Prompt); err != nil {
			return nil, fmt.Errorf("scan forfeited schedule: %w", err)
		}
		lost = append(lost, f)
	}
	return lost, rows.Err()
}

// schedulePromptLimit bounds a scheduled prompt. It is typed into a TUI a
// character at a time when it comes due — the same delivery a relayed message
// gets, and the same reason that one is bounded (internal/relay, PromptLimit):
// a megabyte of it is a hang, not a prompt. Checked here rather than at
// delivery so the person writing it is told while they can still shorten it.
const schedulePromptLimit = 8192

// SetSessionSchedule parks a prompt to be typed at a session later. at is unix
// seconds; 0, or an empty prompt, clears whatever was there — there is nothing
// else to cancel, because a session holds one scheduled prompt at a time and
// scheduling again replaces it.
//
// A session whose row is gone matches nothing and is not an error: the schedule
// belongs to the card, and a card that is gone has taken its schedule with it.
func (s *Service) SetSessionSchedule(sessionID string, at int64, prompt string) error {
	return s.setSchedule(sessionID, at, prompt, "")
}

// SetLichSchedule is SetSessionSchedule for lich's own continuation of a turn a
// usage limit ended: the row records it was lich that parked it, which is how
// it is told apart from a person's prompt, whatever the words.
func (s *Service) SetLichSchedule(sessionID string, at int64, prompt string) error {
	return s.setSchedule(sessionID, at, prompt, api.ScheduledByLich)
}

// SetAgentSchedule is SetSessionSchedule for a prompt the session's own agent
// parked, which is what lets the person's outrank it.
func (s *Service) SetAgentSchedule(sessionID string, at int64, prompt string) error {
	return s.setSchedule(sessionID, at, prompt, api.ScheduledByAgent)
}

func (s *Service) setSchedule(sessionID string, at int64, prompt, by string) error {
	prompt = strings.TrimSpace(prompt)
	if err := checkPromptLimit(prompt); err != nil {
		return err
	}
	if at <= 0 || prompt == "" {
		at, prompt, by = 0, "", ""
	}
	if _, err := s.db.Exec(
		`UPDATE sessions SET scheduled_at = ?, scheduled_prompt = ?, scheduled_by = ? WHERE id = ?`,
		at, prompt, by, sessionID,
	); err != nil {
		return fmt.Errorf("set schedule on %q: %w", sessionID, err)
	}
	return nil
}

func checkPromptLimit(prompt string) error {
	if len(prompt) > schedulePromptLimit {
		return fmt.Errorf("scheduled prompt is %d bytes, over the %d limit",
			len(prompt), schedulePromptLimit)
	}
	return nil
}

// SetSessionWatch writes a session's watch as it now stands.
func (s *Service) SetSessionWatch(sessionID string, w api.SessionWatch) error {
	w.Prompt = strings.TrimSpace(w.Prompt)
	if err := checkPromptLimit(w.Prompt); err != nil {
		return err
	}
	if _, err := s.db.Exec(
		`UPDATE sessions SET watch_on = ?, watch_prompt = ?, watch_by = ?, watch_runs = ?, watch_pr = ?, watch_seen = ?
		  WHERE id = ?`,
		w.On, w.Prompt, w.By, w.Runs, w.PR, w.Seen, sessionID,
	); err != nil {
		return fmt.Errorf("set watch on %q: %w", sessionID, err)
	}
	return nil
}
