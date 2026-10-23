package api

var DropAttach = define[AttachDropOptions, DropAttachment]("drop.Attach")

// AttachDropOptions asks for a file to attach to a session: SessionID's
// checkout is Root, and a Confined session reads only what lies inside it.
type AttachDropOptions struct {
	SessionID string `json:"sessionId"`
	Root      string `json:"root"`
	Confined  bool   `json:"confined"`
}

// DropAttachment is what the picker produced: the path to write at the prompt, and
// the line that goes under it when that path is a copy's (copyNotice), empty
// when the session opens the file where it lies. Both empty for a cancelled
// dialog.
type DropAttachment struct {
	Path   string `json:"path"`
	Notice string `json:"notice"`
}
