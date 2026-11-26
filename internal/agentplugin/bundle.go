package agentplugin

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lichdotdev/lich/internal/semver"
)

// lich's own copy of the plugin, which Claude Code, Cursor CLI, opencode and
// oh-my-pi load per session from lich's spawn (internal/terminal) instead of from
// an install in their own config: `--plugin-dir` for the first two, the inline
// config for opencode and `-e` for oh-my-pi (measured on 2.1.332, 2026.11.19,
// 1.19.4 and 19.1.1). A plugin the user turned off, or a config that lost it,
// can no longer leave a lich session without its hooks.
//
// Each release is unpacked once into a directory of its own and never written
// again, so a session keeps reading the files it started with while a newer
// release lands beside them.

const (
	// tarballBaseURL serves the repository at a tag as a gzipped tarball whose
	// entries sit under one top directory, `lich-plugin-<version>/`.
	tarballBaseURL = "https://codeload.github.com/" + marketplaceRepo + "/tar.gz/refs/tags"
	// bundleLimit caps what a release may unpack to. 0.23.0 is 632 KiB; anything
	// near this is not the plugin, and lich is about to hand it to a harness that
	// runs it.
	bundleLimit = 64 << 20
	// bundleTmpPrefix names a release still being unpacked, which bundleVersion
	// never picks.
	bundleTmpPrefix = ".tmp-"
)

// userConfigDir is os.UserConfigDir, swapped by tests: the per-OS variables
// behind it are not all ones a test can redirect without moving the home other
// paths under test read.
var userConfigDir = os.UserConfigDir

// BundleDir is the plugin release lich hands its sessions: the newest compatible
// one unpacked, or ok false when none is.
func BundleDir() (dir, version string, ok bool) {
	root, err := bundleRoot()
	if err != nil {
		return "", "", false
	}
	version, ok = bundleVersionIn(root)
	if !ok {
		return "", "", false
	}
	return filepath.Join(root, version), version, true
}

// bundleRoot holds one directory per unpacked release, beside the hook scripts
// lich keeps for Crush and Kiro (pluginScriptDir).
func bundleRoot() (string, error) {
	dir, err := userConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	return filepath.Join(dir, "lich", "plugin", "releases"), nil
}

// bundleVersionIn is the newest compatible release unpacked under root.
func bundleVersionIn(root string) (string, bool) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", false
	}
	best := ""
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || strings.HasPrefix(name, bundleTmpPrefix) || !Compatible(name) {
			continue
		}
		if best == "" || semver.Less(best, name) {
			best = name
		}
	}
	return best, best != ""
}

// bundleVersion is the version BundleDir hands sessions.
func bundleVersion() (string, bool) {
	_, version, ok := BundleDir()
	return version, ok
}

// installBundle unpacks the newest compatible release, unless it already is.
func (s *Service) installBundle() error {
	version, err := s.releaseVersion()
	if err != nil {
		return err
	}
	return s.installRelease(version)
}

// installRelease unpacks version, unless it already is.
func (s *Service) installRelease(version string) error {
	root, err := bundleRoot()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, version)); err == nil {
		return nil
	}
	return s.unpackRelease(root, version)
}

// unpackRelease downloads version's tarball and unpacks it into root/version
// through a temporary directory, renamed only once whole.
func (s *Service) unpackRelease(root, version string) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", root, err)
	}
	tmp, err := os.MkdirTemp(root, bundleTmpPrefix+version+"-")
	if err != nil {
		return fmt.Errorf("create a directory under %s: %w", root, err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := s.downloadRelease(version, tmp); err != nil {
		return err
	}
	target := filepath.Join(root, version)
	if err := os.Rename(tmp, target); err != nil {
		// Another unpack of the same release won the rename.
		if _, statErr := os.Stat(target); statErr == nil {
			return nil
		}
		return fmt.Errorf("unpack the plugin %s: %w", version, err)
	}
	return nil
}

func (s *Service) downloadRelease(version, dest string) error {
	url := fmt.Sprintf("%s/v%s", strings.TrimSuffix(s.tarballBase, "/"), version)
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("request %s: %w", url, err)
	}
	req.Header.Set("User-Agent", "lich")
	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	if err := untar(resp.Body, dest); err != nil {
		return fmt.Errorf("unpack %s: %w", url, err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".claude-plugin", "plugin.json")); err != nil {
		return fmt.Errorf("unpack %s: no .claude-plugin/plugin.json in it", url)
	}
	return nil
}

// untar writes a gzipped tarball's directories and regular files under dest,
// without its top directory. Anything else (links, devices, git's pax header)
// is skipped, and a name that would land outside dest is refused.
func untar(r io.Reader, dest string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	var total int64
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		rel, ok := entryPath(hdr.Name)
		if !ok {
			return fmt.Errorf("entry %q leaves the archive", hdr.Name)
		}
		if rel == "" {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			total += hdr.Size
			if total > bundleLimit {
				return fmt.Errorf("larger than %d bytes", bundleLimit)
			}
			if err := writeEntry(tr, target, hdr.Mode); err != nil {
				return err
			}
		}
	}
}

// entryPath is a tar entry's path with its top directory cut, or ok false for
// one that climbs out of the archive.
func entryPath(name string) (string, bool) {
	clean := path.Clean(strings.TrimPrefix(name, "./"))
	if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	_, rest, _ := strings.Cut(clean, "/")
	return rest, true
}

func writeEntry(r io.Reader, target string, mode int64) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	perm := os.FileMode(0o644)
	if mode&0o111 != 0 {
		perm = 0o755
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, io.LimitReader(r, bundleLimit)); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// migrateInstalls takes the plugin a lich of the past installed out of the
// providers whose sessions now load lich's own copy, once that copy is there:
// a module left in opencode's or omp's directory would run beside it, and an
// install left in Claude Code would run twice in every Cursor session.
func (s *Service) migrateInstalls() {
	if _, ok := bundleVersion(); !ok {
		return
	}
	for name, remove := range map[string]func() error{
		"opencode": removeOpencodeModules,
		"oh-my-pi": removeOmpExtension,
	} {
		if err := remove(); err != nil {
			slog.Warn("agentplugin: remove the install of the past", "provider", name, "err", err)
		}
	}
	s.migrateClaude()
}
