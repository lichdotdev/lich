package claudeprofile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/lichdotdev/lich/internal/profilestore"
)

// settingsFile is Claude Code's user settings in a config directory: `/model`
// writes the default model there and `/effort` the model's effort under
// modelSettings, `claude plugin marketplace add` and `claude plugin install` the
// plugin set, all through a link to the file's target (measured on 2.1.325).
const settingsFile = "settings.json"

// profileKeys are the top-level keys of settings.json a profile keeps as its
// own: the default model and effort, which belong to an account, and the plugin
// set, since an organization account may forbid plugins Personal runs. Every
// other key is Personal's.
var profileKeys = []string{"model", "effortLevel", "modelSettings", "enabledPlugins", "extraKnownMarketplaces"}

// syncSettings rebuilds a profile's settings.json from Personal's and the
// profile's own keys (profileKeys), part of the Sync of the profile store. A
// profile with no settings.json of its own yet, or one still linked to
// Personal's from when the whole file was shared, starts from Personal's keys.
// With no settings.json in Personal there is nothing to share, and the
// profile's is left as it is.
func syncSettings(personal, dir string) error {
	shared, err := readSettings(filepath.Join(personal, settingsFile))
	if err != nil || shared == nil {
		return err
	}
	path := filepath.Join(dir, settingsFile)
	var own map[string]json.RawMessage
	if !profilestore.IsLink(path) {
		if own, err = readSettings(path); err != nil {
			return err
		}
	}
	out, err := encodeSettings(composeSettings(shared, own))
	if err != nil {
		return err
	}
	if own != nil {
		was, err := encodeSettings(own)
		if err != nil {
			return err
		}
		if bytes.Equal(out, was) {
			return nil
		}
	}
	return profilestore.WriteFileAtomic(path, out, 0o600)
}

// composeSettings is a profile's settings: Personal's, but with the profile's
// own value, or none, for each of profileKeys. A nil profile has not chosen
// yet and takes Personal's.
func composeSettings(personal, profile map[string]json.RawMessage) map[string]json.RawMessage {
	if profile == nil {
		profile = personal
	}
	out := maps.Clone(personal)
	for _, key := range profileKeys {
		delete(out, key)
		if value, ok := profile[key]; ok {
			out[key] = value
		}
	}
	return out
}

func readSettings(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	settings := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(data)) == 0 {
		return settings, nil
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return settings, nil
}

// encodeSettings writes settings the way Claude Code does, two-space indented
// and with a hook's `&&` or `>` left as written.
func encodeSettings(settings map[string]json.RawMessage) ([]byte, error) {
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(settings); err != nil {
		return nil, fmt.Errorf("encode %s: %w", settingsFile, err)
	}
	return out.Bytes(), nil
}
