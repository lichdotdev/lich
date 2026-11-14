package i18n

// The keys lich reads an extension's manifest strings from, in the namespace
// "extension" (locales/<tag>/extension.json). A key a catalog lacks reads the
// manifest's own text.
const (
	ManifestNameKey        = "extension.manifest.name"
	ManifestDescriptionKey = "extension.manifest.description"
	surfacesKey            = "extension.manifest.surfaces."
)

// SurfaceTitleKey is the key of surface id's title.
func SurfaceTitleKey(id string) string { return surfacesKey + id + ".title" }
