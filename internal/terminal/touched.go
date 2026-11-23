package terminal

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/lichdotdev/lich/api"
)

// touchedBodyLimit holds a /session-touched body to 64 KiB: one apply_patch
// names every file it changed, which outgrows hookBodyLimit, and this is room
// for hundreds of absolute paths.
const touchedBodyLimit = 64 << 10

// touchedRequest is a touched POST body (docs/hooks/session-touched.md).
type touchedRequest struct{ api.TouchedRequest }

func (r touchedRequest) session() string { return r.SessionID }

// editNoter is what answers a touch with its edit-overlap note
// (internal/relay).
type editNoter func(api.TouchedRequest) string

// sessionTouched receives a POST from a provider's hook when a session likely
// changed files on disk (a file-mutating tool ran). Unless it only collects a
// held note, it forwards the session id via the touched callback, which nudges
// an immediate git-status refresh. It answers 200 with the edit-overlap note
// when there is one, 204 otherwise.
func (t *transport) sessionTouched(w http.ResponseWriter, r *http.Request) {
	req, ok := readPost(t, w, r, touchedBodyLimit, parseSessionTouched)
	if !ok {
		return
	}
	if !req.Collect && t.touched != nil {
		t.touched(req.SessionID)
	}
	t.mu.Lock()
	editNote := t.editNote
	t.mu.Unlock()
	note := ""
	if editNote != nil {
		note = editNote(req.TouchedRequest)
	}
	if note == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	body, err := json.Marshal(api.TouchedAnswer{Note: note})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(body); err != nil {
		slog.Warn("hook: write the edit-overlap note", "session", req.SessionID, "err", err)
	}
}

// parseSessionTouched validates a touched POST body: a non-empty lich session
// id, no empty path, and a collect that carries nothing but the id.
func parseSessionTouched(body []byte) (touchedRequest, error) {
	var req touchedRequest
	if err := json.Unmarshal(body, &req.TouchedRequest); err != nil {
		return touchedRequest{}, fmt.Errorf("invalid session-touched body: %w", err)
	}
	if req.SessionID == "" {
		return touchedRequest{}, errors.New("session-touched missing session_id")
	}
	if slices.Contains(req.Paths, "") {
		return touchedRequest{}, errors.New("session-touched names an empty path")
	}
	if req.Collect && (len(req.Paths) > 0 || req.Hold) {
		return touchedRequest{}, errors.New("a session-touched collect carries no paths and no hold")
	}
	return req, nil
}

// SetEditNotes wires what answers a touch with its edit-overlap note:
// internal/relay's Touched. Startup wiring, before any session runs; with none
// every touch answers 204.
func (s *Service) SetEditNotes(fn func(api.TouchedRequest) string) {
	if s.ws == nil {
		return
	}
	s.ws.mu.Lock()
	s.ws.editNote = fn
	s.ws.mu.Unlock()
}
