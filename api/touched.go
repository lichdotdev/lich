package api

// TouchedRequest is one /session-touched call (docs/hooks/session-touched.md,
// Request): a file-mutating tool ran, on Paths when the provider names them.
// Hold keeps the note for a later Collect, for a provider that cannot show
// text from the hook that reports the edit.
type TouchedRequest struct {
	SessionID string   `json:"session_id"`
	Paths     []string `json:"paths"`
	Hold      bool     `json:"hold"`
	Collect   bool     `json:"collect"`
}

// TouchedAnswer is the 200 body of a touch that has something to say.
type TouchedAnswer struct {
	Note string `json:"note"`
}
