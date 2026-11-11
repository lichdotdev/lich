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
}

// Surface is a full screen the extension draws, opened from the top bar.
type Surface struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Icon  Icon   `json:"icon,omitempty"`
}
