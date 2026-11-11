package api

var (
	// ProjectShowCode checks a file and lines of session ID's own checkout and
	// asks the window to show them. A confined token may call it for its own
	// session only (internal/scope).
	ProjectShowCode = define[ShowCodeOptions, ShowCode]("project.ShowCode")

	// ShowCodeEvent carries what ProjectShowCode checked. It is global: the
	// session may not be the one on screen, and the window words that itself.
	ShowCodeEvent = defineEvent[ShowCode]("show-code")
)

// ShowCodeOptions names a file relative to the session's checkout, or absolute
// inside it. Line and EndLine are 1-based and inclusive; 0 leaves them out.
// Comment is plain text the agent adds under the lines.
type ShowCodeOptions struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	EndLine int    `json:"endLine,omitempty"`
	Comment string `json:"comment,omitempty"`
}

// ShowCode is a checked request: Rel is relative to the session's checkout.
type ShowCode struct {
	SessionID string `json:"sessionId"`
	Rel       string `json:"rel"`
	Line      int    `json:"line,omitempty"`
	EndLine   int    `json:"endLine,omitempty"`
	Comment   string `json:"comment,omitempty"`
}
