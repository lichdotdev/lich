package main

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// forgeRetry is how long a checkout whose forge could not be read waits
// before it is asked again.
const forgeRetry = 10 * time.Second

// forgeStore is the window's forge of each checkout. The forge comes from the
// remote and no event announces a change to it, so each path is read once,
// the first time something asks, and kept while the window lives.
type forgeStore struct {
	client     *backend.Client
	invalidate func()

	mu     sync.Mutex
	forges askOnce[string]
}

func newForgeStore(c *backend.Client, invalidate func()) *forgeStore {
	return &forgeStore{client: c, invalidate: invalidate}
}

// of is path's forge, known false until the backend answered.
func (f *forgeStore) of(path string) (forge string, known bool) {
	return f.forges.get(&f.mu, path, f.invalidate, func() (string, bool) {
		forge, err := f.client.Forge(context.Background(), path)
		if err != nil {
			log.Printf("forge of %s: %v", path, err)
			time.AfterFunc(forgeRetry, func() {
				f.mu.Lock()
				delete(f.forges.asked, path)
				f.mu.Unlock()
				f.invalidate()
			})
			return "", false
		}
		return forge, true
	})
}

// peek is path's forge as already read, "" while it is not; it starts no
// read.
func (f *forgeStore) peek(path string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.forges.got[path]
}

// gitLab is whether path is known to be a GitLab checkout; one still being
// read is not.
func (f *forgeStore) gitLab(path string) bool {
	forge, _ := f.of(path)
	return forge == api.ForgeGitLab
}

// forgeKey is the GitHub message key, or its GitLab twin on a GitLab
// checkout. They are separate messages rather than one with the noun
// interpolated: "merge request" and "pull request" take different gender and
// agreement in some languages.
func forgeKey(forge, github, gitlab string) string {
	if forge == api.ForgeGitLab {
		return gitlab
	}
	return github
}

// changeRef is how forge writes a reference to change n: GitHub's #12,
// GitLab's !12.
func changeRef(forge string, n int) string {
	if forge == api.ForgeGitLab {
		return "!" + strconv.Itoa(n)
	}
	return "#" + strconv.Itoa(n)
}

// forgeOf is path's forge, "" while it is read.
func (m *model) forgeOf(path string) string {
	forge, _ := m.forges.of(path)
	return forge
}
