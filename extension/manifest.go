package extension

// ManifestFile is the name of the manifest in an extension's directory.
const ManifestFile = "manifest.json"

// DataDirEnv names the environment variable holding the absolute path of the
// extension's data directory, which lich creates before starting it and keeps
// across a reinstall. The install directory is replaced by one, so nothing the
// extension saves belongs there.
const DataDirEnv = "LICH_EXTENSION_DATA"

// ManifestFormat is the newest manifest format this package reads.
const ManifestFormat = 1

// Manifest declares an extension to lich. Executable is relative to the
// manifest's directory and must stay inside it.
type Manifest struct {
	FormatVersion  int       `json:"formatVersion"`
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Version        string    `json:"version"`
	MinLichVersion string    `json:"minLichVersion,omitempty"`
	Executable     string    `json:"executable"`
	Surfaces       []Surface `json:"surfaces,omitempty"`
	// Permissions is every Permission the extension's calls and
	// subscriptions need; lich refuses any other.
	Permissions []Permission `json:"permissions,omitempty"`
}

// Surface is a screen the extension draws, living where Place says.
// IconFile is a PNG relative to the manifest's directory, drawn in place of
// Icon in its own colors, or in the theme's when every visible pixel is
// black; Icon is still what a lich older than IconFile draws.
type Surface struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Icon     Icon   `json:"icon,omitempty"`
	IconFile string `json:"iconFile,omitempty"`
	Place    Place  `json:"place,omitempty"`
}

// Place is where a surface lives. Empty is a full screen opened from the top
// bar. A dock surface is a tab of the dock beside the terminal and a panel
// one rises from the footer; both open from an item of the footer and follow
// the active session (see ContextData). A lich older than Place shows every
// surface as a full screen.
type Place string

const (
	PlaceScreen Place = ""
	PlaceDock   Place = "dock"
	PlacePanel  Place = "panel"
)
