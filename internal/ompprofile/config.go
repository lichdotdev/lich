package ompprofile

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lichdotdev/lich/internal/profilestore"
)

// configFiles are the names omp reads its settings from, the first that
// exists winning (read in its source, 18.8.9).
var configFiles = []string{"config.yml", "config.yaml"}

// profileKeys are the top-level keys of config.yml a profile keeps as its own,
// since they belong to an account: the model `/model` saves for each role, the
// default one with its thinking level (measured on 18.8.9), and the default
// thinking level (read in omp's source).
var profileKeys = []string{"modelRoles", "defaultThinkingLevel"}

// syncConfig rebuilds each config file of a profile from Personal's, keeping
// the profile's own profileKeys; the Sync of the profile store.
func syncConfig(personal, dir string) error {
	for _, name := range configFiles {
		if err := syncConfigFile(filepath.Join(personal, name), filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

// syncConfigFile rebuilds the profile's file at path from Personal's at
// shared. A profile with no file of its own yet, or one still linked to
// Personal's from when the whole file was shared, starts on Personal's keys.
// With no such file in Personal the profile's is left as it is.
func syncConfigFile(shared, path string) error {
	personal, err := os.ReadFile(shared)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", shared, err)
	}
	var own []byte
	if !profilestore.IsLink(path) {
		if own, err = os.ReadFile(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("read %s: %w", path, err)
		}
	}
	from := own
	if from == nil {
		from = personal
	}
	out := composeConfig(personal, from)
	if own != nil && bytes.Equal(out, own) {
		return nil
	}
	return profilestore.WriteFileAtomic(path, out, 0o600)
}

// composeConfig is a profile's config: Personal's top-level blocks but its
// profileKeys, then the profile's own, each as written.
func composeConfig(personal, profile []byte) []byte {
	shared, _ := splitBlocks(personal)
	_, owned := splitBlocks(profile)
	if owned != "" && shared != "" && !strings.HasSuffix(shared, "\n") {
		shared += "\n"
	}
	if owned != "" && !strings.HasSuffix(owned, "\n") {
		owned += "\n"
	}
	return []byte(shared + owned)
}

// splitBlocks cuts a YAML document into the top-level blocks of profileKeys
// and the rest. A block is a key at the start of a line and every line under
// it, blank lines and comments at the start of a line included, which is all
// the parsing this needs: omp writes block style, and the blocks are copied,
// never read.
func splitBlocks(doc []byte) (shared, owned string) {
	var rest, own strings.Builder
	inOwn := false
	for _, line := range strings.SplitAfter(string(doc), "\n") {
		if startsBlock(line) {
			inOwn = isProfileKey(line)
		}
		if inOwn {
			own.WriteString(line)
		} else {
			rest.WriteString(line)
		}
	}
	return rest.String(), own.String()
}

func startsBlock(line string) bool {
	return line != "" && !strings.ContainsRune(" \t\r\n#", rune(line[0]))
}

func isProfileKey(line string) bool {
	for _, key := range profileKeys {
		rest, ok := strings.CutPrefix(line, key+":")
		if ok && (rest == "" || strings.ContainsRune(" \t\r\n", rune(rest[0]))) {
			return true
		}
	}
	return false
}
