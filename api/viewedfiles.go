package api

var (
	StoreViewedFiles  = define[ViewedFilesOptions, []ViewedFile]("store.ViewedFiles")
	StoreMarkViewed   = define[MarkViewedOptions, None]("store.MarkViewed")
	StoreUnmarkViewed = define[UnmarkViewedOptions, None]("store.UnmarkViewed")
)

// ViewedFilesOptions names a pull request by its page URL, which holds the
// forge, the repository and the number.
type ViewedFilesOptions struct {
	PR string `json:"pr"`
}

// MarkViewedOptions ticks Path of pull request PR as viewed. Hash is the
// window's hash of the file's diff when it was ticked: a later diff that
// hashes differently means the file changed since the review.
type MarkViewedOptions struct {
	PR   string `json:"pr"`
	Path string `json:"path"`
	Hash string `json:"hash"`
}

// UnmarkViewedOptions unticks Path of pull request PR.
type UnmarkViewedOptions struct {
	PR   string `json:"pr"`
	Path string `json:"path"`
}

// ViewedFile is a file of a pull request ticked as viewed, ViewedAt in Unix
// seconds.
type ViewedFile struct {
	Path     string `json:"path"`
	Hash     string `json:"hash"`
	ViewedAt int64  `json:"viewedAt"`
}
