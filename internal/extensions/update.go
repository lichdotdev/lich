package extensions

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/semver"
)

const (
	// updateCheckEvery paces the look for newer releases: one unauthenticated
	// GitHub API request per extension, against a limit of 60 an hour per
	// address (docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api).
	updateCheckEvery = 6 * time.Hour
	// replacedDir holds the installed version inside the staged install's work
	// directory while its replacement moves in.
	replacedDir = "replaced"
)

// WatchUpdates reads the catalog and looks for newer releases now and every
// updateCheckEvery, until Shutdown.
func (s *Service) WatchUpdates() {
	go func() {
		ticker := time.NewTicker(updateCheckEvery)
		defer ticker.Stop()
		for {
			s.refreshCatalog()
			if err := s.checkUpdates(); err != nil {
				slog.Warn("extensions: look for updates", "err", err)
			}
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// CheckUpdates looks for a newer release of every extension installed from
// GitHub, tells the window of each one whose answer changed, and returns the
// list as it then stands.
func (s *Service) CheckUpdates(ctx context.Context, _ api.None) ([]api.Extension, error) {
	if err := s.checkUpdates(); err != nil {
		return nil, err
	}
	return s.List(ctx, api.None{})
}

func (s *Service) checkUpdates() error {
	s.mu.Lock()
	all := slices.DeleteFunc(slices.Clone(s.all), func(x *installed) bool { return x.source.Revoked != "" })
	s.mu.Unlock()
	cat := s.catalog.Load()
	var errs []error
	for _, x := range all {
		owner, repo, err := parseRepo(x.source.URL)
		if err != nil || x.source.Release == "" {
			continue
		}
		if entry := listing(cat, x.source.URL); entry != nil {
			s.setLatest(x, listedNewer(x.source.Release, entry))
			continue
		}
		rel, err := s.latestRelease(owner, repo)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		s.setLatest(x, newerThan(x.source.Release, rel.Tag))
	}
	return errors.Join(errs...)
}

// newerThan is tag when it is a later version than the release installed.
// GitHub's latest release is the last one published, not the highest: an
// author who deletes a release must not have every install offered the one
// before it.
func newerThan(installed, tag string) string {
	if semver.Less(installed, tag) {
		return tag
	}
	return ""
}

// listedNewer is the catalog's tag when the version it lists is later than
// the release installed: a listed repository updates to what the catalog
// lists, never past it.
func listedNewer(installed string, entry *extension.CatalogEntry) string {
	if semver.Less(installed, entry.Version) {
		return entry.Tag
	}
	return ""
}

func (s *Service) setLatest(x *installed, latest string) {
	s.mu.Lock()
	changed := s.ctx.Err() == nil && s.find(x.id) == x && x.latest != latest
	if changed {
		x.latest = latest
	}
	s.mu.Unlock()
	if changed {
		events.Send(s.emit, api.ExtensionUpdateEvent, api.ExtensionUpdate{ID: x.id, Latest: latest})
	}
}

// PrepareUpdate stages the latest release of the repository an extension was
// installed from, for CommitInstall to put in its place. The installed
// version keeps running until then.
func (s *Service) PrepareUpdate(_ context.Context, opts api.ExtensionIDOptions) (api.ExtensionInstallPreview, error) {
	s.mu.Lock()
	x := s.find(opts.ID)
	s.mu.Unlock()
	if x == nil {
		return api.ExtensionInstallPreview{}, fmt.Errorf("no extension %q", opts.ID)
	}
	owner, repo, err := parseRepo(x.source.URL)
	if err != nil || x.source.Release == "" {
		return api.ExtensionInstallPreview{}, fmt.Errorf("extension %q was not installed from a GitHub release, so there is nowhere to update it from: remove it and install it again", opts.ID)
	}
	s.installMu.Lock()
	defer s.installMu.Unlock()
	cat := s.catalogNow()
	s.mu.Lock()
	reason := cmp.Or(x.source.Revoked, revokedReason(cat, x.source.URL))
	s.mu.Unlock()
	if reason != "" {
		return api.ExtensionInstallPreview{}, fmt.Errorf("extension %q was %s; lich does not update it", opts.ID, revokedError(reason))
	}
	entry := listing(cat, x.source.URL)
	rel, err := s.newerRelease(x, owner, repo, entry)
	if err != nil {
		return api.ExtensionInstallPreview{}, err
	}
	id, archive, err := pickAsset(rel, s.goos, s.goarch)
	if err != nil {
		return api.ExtensionInstallPreview{}, err
	}
	if id != x.id {
		return api.ExtensionInstallPreview{}, fmt.Errorf("release %s ships extension %q, not %q: remove this one and install the repository again", rel.Tag, id, x.id)
	}
	pinned, err := s.pinnedSum(entry)
	if err != nil {
		return api.ExtensionInstallPreview{}, err
	}
	src := source{URL: x.source.URL, Release: rel.Tag}
	preview, err := s.stage(id, src, x, func(stage string) error { return s.unpack(rel, archive, pinned, id, stage) })
	return published(preview, owner, entry), err
}

// newerRelease is the release an update of x installs, refused when it is no
// later than the one installed: the catalog's for a listed repository, read
// from GitHub only once it is known to be newer, and the latest for another.
func (s *Service) newerRelease(x *installed, owner, repo string, entry *extension.CatalogEntry) (release, error) {
	if entry != nil {
		latest := listedNewer(x.source.Release, entry)
		s.setLatest(x, latest)
		if latest == "" {
			return release{}, fmt.Errorf("extension %q is at %s and the lich catalog lists %s, which is not a newer version", x.id, x.source.Release, entry.Version)
		}
		return s.taggedRelease(owner, repo, entry.Tag)
	}
	rel, err := s.latestRelease(owner, repo)
	if err != nil {
		return release{}, err
	}
	latest := newerThan(x.source.Release, rel.Tag)
	s.setLatest(x, latest)
	if latest == "" {
		return release{}, fmt.Errorf("extension %q is at %s and its repository's latest release, %s, is not a newer version", x.id, x.source.Release, rel.Tag)
	}
	return rel, nil
}

// replace swaps old's directory for the staged copy in st and starts it. The
// old directory waits inside the work directory until the new one is in
// place, so a swap that fails puts it back; the data directory is not touched.
func (s *Service) replace(ctx context.Context, st stagedInstall) (api.Extension, error) {
	old, stranded := st.replaces, false
	defer func() {
		if !stranded {
			_ = afterExit(func() error { return os.RemoveAll(st.work) })
		}
	}()
	old.restartMu.Lock()
	defer old.restartMu.Unlock()
	s.mu.Lock()
	gone := s.find(old.id) != old
	stop, done := old.stop, old.done
	s.mu.Unlock()
	if gone {
		return api.Extension{}, fmt.Errorf("extension %q was removed or updated since this update was prepared: start it again", old.id)
	}
	if stop != nil {
		stop()
		select {
		case <-done:
		case <-ctx.Done():
			return api.Extension{}, ctx.Err()
		}
	}
	kept := filepath.Join(st.work, replacedDir)
	if err := afterExit(func() error { return os.Rename(old.root, kept) }); err != nil {
		s.relaunch(old)
		return api.Extension{}, fmt.Errorf("move the installed version out of %s: %w", old.root, err)
	}
	if err := os.Rename(filepath.Join(st.work, old.id), old.root); err != nil {
		if back := os.Rename(kept, old.root); back != nil {
			stranded = true
			return api.Extension{}, fmt.Errorf("move the update into %s: %w; the installed version is now in %s: move it back by hand", old.root, err, kept)
		}
		s.relaunch(old)
		return api.Extension{}, fmt.Errorf("move the update into %s: %w", old.root, err)
	}
	fresh := s.swap(old)
	// The window's offer of this release is answered.
	events.Send(s.emit, api.ExtensionUpdateEvent, api.ExtensionUpdate{ID: fresh.ID})
	return fresh, nil
}

// swap reads the directory an update just filled and starts it in old's place
// in the list.
func (s *Service) swap(old *installed) api.Extension {
	fresh := load(old.root, old.id, s.lichVersion)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revokeLoaded(fresh)
	fresh.tickets = old.tickets
	s.all[slices.Index(s.all, old)] = fresh
	if s.ctx.Err() == nil && fresh.invalid == nil {
		s.launch(fresh)
	}
	return fresh.view()
}

// relaunch starts x again after an update that did not happen stopped it.
func (s *Service) relaunch(x *installed) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() == nil && s.find(x.id) == x {
		s.launch(x)
	}
}
