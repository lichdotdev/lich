package extension

// CatalogFormat is the newest catalog.json format this package describes.
const CatalogFormat = 1

// ListingFile is where an extension's repository keeps what the catalog shows
// beside its manifest, read at the commit its release tag points to.
const ListingFile = "listing/listing.json"

// The limits the catalog holds a listing to (docs/extensions.md, Listing in
// the catalog).
const (
	MaxDescription     = 160
	MaxCategories      = 2
	MaxScreenshots     = 6
	MaxAlt             = 200
	MaxCoverBytes      = 1 << 20
	MaxScreenshotBytes = 2 << 20
)

// Catalog is catalog.json, the index github.com/lichdotdev/extensions builds
// from the repositories listed there. lich and the site read it; nothing
// writes it but that repository's build.
type Catalog struct {
	FormatVersion int               `json:"formatVersion"`
	Generated     string            `json:"generated"`
	Categories    []CatalogCategory `json:"categories"`
	Extensions    []CatalogEntry    `json:"extensions"`
	// Revoked is every repository taken down: lich stops an installed
	// extension that came from one and never starts it again.
	Revoked []Revocation `json:"revoked,omitempty"`
}

// CatalogCategory is one category a listing can name. Names is keyed by
// language tag; "en" is always there and is the fallback.
type CatalogCategory struct {
	ID    string            `json:"id"`
	Names map[string]string `json:"names"`
}

// CatalogEntry is one listed extension at the version the catalog offers.
// Repo is https://github.com/<owner>/<repo>; Publisher is its owner.
// Builds maps <goos>_<goarch> to the lowercase hex sha256 of that release
// archive, recorded the first time the catalog saw the version: lich installs
// a listed repository at Tag only, and only an archive with that hash.
// Held, when set, says why a newer release is not the one listed.
type CatalogEntry struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Translations   map[string]CatalogText `json:"translations,omitempty"`
	Publisher      string                 `json:"publisher"`
	Official       bool                   `json:"official,omitempty"`
	Stars          int                    `json:"stars"`
	Repo           string                 `json:"repo"`
	RepoID         int64                  `json:"repoId"`
	License        string                 `json:"license,omitempty"`
	Version        string                 `json:"version"`
	Tag            string                 `json:"tag"`
	Commit         string                 `json:"commit"`
	Released       string                 `json:"released"`
	Categories     []string               `json:"categories"`
	MinLichVersion string                 `json:"minLichVersion,omitempty"`
	Permissions    []Permission           `json:"permissions,omitempty"`
	Glyph          Icon                   `json:"glyph,omitempty"`
	Icon           string                 `json:"icon,omitempty"`
	Cover          string                 `json:"cover,omitempty"`
	Screenshots    []CatalogImage         `json:"screenshots,omitempty"`
	Builds         map[string]string      `json:"builds"`
	Held           string                 `json:"held,omitempty"`
}

// CatalogText is an entry's name and description in one language, from the
// extension's own catalogs (Translating an extension).
type CatalogText struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// CatalogImage is a cover or screenshot: an absolute URL pinned to the
// release's commit, and the text a screen reader reads for it.
type CatalogImage struct {
	URL string `json:"url"`
	Alt string `json:"alt"`
}

// Revocation takes a repository down. RepoID is GitHub's numeric id, so a
// repository renamed or recreated under the same name is told apart.
type Revocation struct {
	Repo   string `json:"repo"`
	RepoID int64  `json:"repoId"`
	Reason string `json:"reason"`
}

// Listing is listing/listing.json in an extension's repository. Cover and
// each screenshot's File are relative to listing/.
type Listing struct {
	Categories  []string       `json:"categories"`
	Cover       string         `json:"cover,omitempty"`
	Screenshots []ListingImage `json:"screenshots,omitempty"`
}

// ListingImage is one screenshot and its alt text, which is required.
type ListingImage struct {
	File string `json:"file"`
	Alt  string `json:"alt"`
}
