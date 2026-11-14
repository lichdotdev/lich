package extensions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lichdotdev/lich/extension"
)

// Check runs an install's checks on an extension's folder or on a release
// archive named <id>_<goos>_<goarch>.tar.gz, on a copy it then deletes, and
// returns the manifest that passed. It needs no lich running and no network.
// minLichVersion is checked for its form only: the lich that checks is not
// the one that installs.
func Check(path string) (extension.Manifest, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return extension.Manifest{}, err
	}
	work, err := os.MkdirTemp("", "lich-extension-check-")
	if err != nil {
		return extension.Manifest{}, fmt.Errorf("create a work directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(work) }()
	switch {
	case info.IsDir():
		return checkFolder(path, work)
	case info.Mode().IsRegular():
		return checkArchive(path, info.Size(), work)
	}
	return extension.Manifest{}, fmt.Errorf("%s is neither a folder nor a file; links are not accepted", path)
}

func checkFolder(dir, work string) (extension.Manifest, error) {
	id, err := manifestID(dir)
	if err != nil {
		return extension.Manifest{}, err
	}
	stage := filepath.Join(work, id)
	if err := copyTree(dir, stage); err != nil {
		return extension.Manifest{}, fmt.Errorf("copy %s: %w", dir, err)
	}
	return checkStage(stage, id, "")
}

func checkArchive(path string, size int64, work string) (extension.Manifest, error) {
	name := filepath.Base(path)
	id, err := archiveID(name)
	if err != nil {
		return extension.Manifest{}, err
	}
	if size > maxArchiveBytes {
		return extension.Manifest{}, fmt.Errorf("%s is larger than the %d MiB lich downloads", name, maxArchiveBytes>>20)
	}
	file, err := os.Open(path)
	if err != nil {
		return extension.Manifest{}, err
	}
	defer func() { _ = file.Close() }()
	stage := filepath.Join(work, id)
	if err := extract(file, stage); err != nil {
		return extension.Manifest{}, fmt.Errorf("unpack %s: %w", name, err)
	}
	return checkStage(stage, id, "")
}

// archiveID is the id a release archive's name gives. Go's OS and
// architecture names have no underscore, so they are the last two fields.
func archiveID(name string) (string, error) {
	base, ok := strings.CutSuffix(name, assetSuffix)
	fields := strings.Split(base, "_")
	if !ok || len(fields) < 3 || fields[len(fields)-1] == "" || fields[len(fields)-2] == "" {
		return "", fmt.Errorf("%s is not named <id>_<goos>_<goarch>%s", name, assetSuffix)
	}
	id := strings.Join(fields[:len(fields)-2], "_")
	if !idPattern.MatchString(id) {
		return "", fmt.Errorf("%s does not start with a valid extension id", name)
	}
	return id, nil
}
