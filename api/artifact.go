package api

var (
	// ArtifactList is every open session's artifacts, for a window that has
	// just attached; ArtifactsEvent keeps it current from there.
	ArtifactList = define[None, []SessionArtifacts]("artifact.List")

	// ArtifactAttach puts one of From's artifacts where SessionID can read it
	// and answers with the path and notice to paste at its prompt: From's own
	// path and no notice when the two are the same session, a copy in
	// SessionID's artifact folder otherwise.
	ArtifactAttach = define[AttachArtifactOptions, DropAttachment]("artifact.Attach")

	// ArtifactOpen loads From's artifact Name in SessionID's browser, starting
	// one when SessionID has none. The page is served from From's own folder
	// and origin. Window only: a confined token is refused it
	// (internal/scope), since it would let one session read another's pages.
	ArtifactOpen = define[AttachArtifactOptions, None]("artifact.Open")

	// ArtifactsEvent carries one open session's whole artifact list each time
	// its folder changes, and an empty one when the session closes.
	ArtifactsEvent = defineEvent[SessionArtifacts]("artifacts")
)

// SessionArtifacts is one session's artifacts, newest first.
type SessionArtifacts struct {
	SessionID string     `json:"sessionId"`
	Files     []Artifact `json:"files"`
}

// Artifact is one page or file an agent left in its session's artifact
// folder. Name is the entry's own name; Path is the file to open, the folder's
// index.html for a folder that holds one. Modified is in unix milliseconds.
type Artifact struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Modified int64  `json:"modified"`
}

// AttachArtifactOptions names the artifact Name of session From, for session
// SessionID: to attach to its prompt, or to open in its browser.
type AttachArtifactOptions struct {
	SessionID string `json:"sessionId"`
	From      string `json:"from"`
	Name      string `json:"name"`
}
