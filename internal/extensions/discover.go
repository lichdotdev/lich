package extensions

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/internal/semver"
)

const idMaxLength = 64

var idPattern = regexp.MustCompile(fmt.Sprintf(`^[a-z0-9][a-z0-9._-]{0,%d}$`, idMaxLength-1))

// Dir is where extensions are installed under configDir. A dev instance has
// its own, as it has its own themes and database: otherwise `task dev` would
// start a second copy of every extension the installed lich runs.
func Dir(configDir string) string {
	name := "extensions"
	if os.Getenv("LICH_DEV") != "" {
		name = "extensions-dev"
	}
	return filepath.Join(configDir, "lich", name)
}

// discover reads every extension directory under dir. A bad one comes back
// with its error, and the others are unaffected.
func discover(dir, lichVersion string) []*installed {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("extensions: read the directory", "dir", dir, "err", err)
		}
		return nil
	}
	var found []*installed
	for _, entry := range entries {
		root := filepath.Join(dir, entry.Name())
		// Stat follows a symlink, the way to develop an extension in place. One
		// that dangles falls through to fail on its manifest and gets listed.
		if info, err := os.Stat(root); err == nil && !info.IsDir() {
			continue
		}
		found = append(found, load(root, entry.Name(), lichVersion))
	}
	return found
}

func load(root, dirName, lichVersion string) *installed {
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	x := &installed{id: dirName, root: root}
	x.manifest, x.exe, x.invalid = readManifest(root, dirName, lichVersion)
	if x.invalid != nil {
		x.status, x.errText = statusInvalid, x.invalid.Error()
		slog.Warn("extensions: invalid", "extension", dirName, "err", x.invalid)
		return x
	}
	x.status = statusStopped
	return x
}

// readManifest also returns a manifest that failed validation, so an invalid
// extension can still be listed under the name it gave itself.
func readManifest(root, dirName, lichVersion string) (extension.Manifest, string, error) {
	var m extension.Manifest
	raw, err := os.ReadFile(filepath.Join(root, extension.ManifestFile))
	if err != nil {
		return m, "", fmt.Errorf("read %s: %w", extension.ManifestFile, err)
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, "", fmt.Errorf("parse %s: %w", extension.ManifestFile, err)
	}
	if err := validate(m, dirName, lichVersion); err != nil {
		return m, "", err
	}
	exe, err := resolveExecutable(root, m.Executable)
	return m, exe, err
}

func validate(m extension.Manifest, dirName, lichVersion string) error {
	if m.FormatVersion < 1 {
		return fmt.Errorf("formatVersion %d must be a positive integer", m.FormatVersion)
	}
	if m.FormatVersion > extension.ManifestFormat {
		return fmt.Errorf("manifest format %d is newer than this lich reads (up to %d); update lich", m.FormatVersion, extension.ManifestFormat)
	}
	if !idPattern.MatchString(m.ID) {
		return fmt.Errorf("id %q must be lowercase letters, digits, dots, underscores or dashes, at most %d long", m.ID, idMaxLength)
	}
	if m.ID != dirName {
		return fmt.Errorf("id %q must equal its directory %q", m.ID, dirName)
	}
	if err := checkMinLichVersion(m.MinLichVersion, lichVersion); err != nil {
		return err
	}
	return checkSurfaces(m.Surfaces)
}

func checkMinLichVersion(minimum, lichVersion string) error {
	if minimum == "" {
		return nil
	}
	if !semver.IsRelease(minimum) {
		return fmt.Errorf("minLichVersion %q must be MAJOR.MINOR.PATCH", minimum)
	}
	if semver.IsRelease(lichVersion) && semver.Less(lichVersion, minimum) {
		return fmt.Errorf("extension needs lich %s or newer (this is %s); update lich",
			strings.TrimPrefix(minimum, "v"), strings.TrimPrefix(lichVersion, "v"))
	}
	return nil
}

func checkSurfaces(surfaces []extension.Surface) error {
	seen := make(map[string]bool, len(surfaces))
	for _, surface := range surfaces {
		if surface.ID == "" {
			return errors.New("a surface needs an id")
		}
		if seen[surface.ID] {
			return fmt.Errorf("surface id %q is declared twice", surface.ID)
		}
		seen[surface.ID] = true
	}
	return nil
}

// resolveExecutable returns the real path of the manifest's executable. The
// lexical check alone would let a symlink inside the directory point out of it.
func resolveExecutable(root, relative string) (string, error) {
	if relative == "" {
		return "", errors.New("executable is required")
	}
	local := filepath.FromSlash(relative)
	if !filepath.IsLocal(local) {
		return "", fmt.Errorf("executable %q must stay inside the extension's directory", relative)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve the extension's directory: %w", err)
	}
	real, err := filepath.EvalSymlinks(filepath.Join(realRoot, local))
	if err != nil {
		return "", fmt.Errorf("executable %q: %w", relative, err)
	}
	if inside, err := filepath.Rel(realRoot, real); err != nil || !filepath.IsLocal(inside) {
		return "", fmt.Errorf("executable %q resolves outside the extension's directory", relative)
	}
	info, err := os.Stat(real)
	if err != nil {
		return "", fmt.Errorf("executable %q: %w", relative, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("executable %q is not a regular file", relative)
	}
	return real, nil
}
