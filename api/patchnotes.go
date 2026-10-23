package api

var PatchNotesCurrent = define[PatchNotesOptions, PatchNotes]("patchnotes.Current")

// PatchNotesOptions names the interface language to read the release in.
type PatchNotesOptions struct {
	Locale string `json:"locale"`
}

// PatchNotes is a release's changelog section, ready for the window to render.
type PatchNotes struct {
	Version    string                `json:"version"`
	Highlights []PatchNotesHighlight `json:"highlights"`
	Groups     []PatchNotesGroup     `json:"groups"`
}

// PatchNotesGroup is one "### Added / Changed / Fixed" block of a release's notes.
type PatchNotesGroup struct {
	Label string `json:"label"`
	// Items keep their markdown bold/code markers; the window renders them.
	Items []string `json:"items"`
}

// PatchNotesHighlight is one GitHub alert block ("> [!IMPORTANT]") written between a
// release's heading and its first group. Kind is the alert type, lowercased.
type PatchNotesHighlight struct {
	Kind string `json:"kind"`
	// Text keeps its markdown bold/code markers; the window renders them.
	Text string `json:"text"`
}
