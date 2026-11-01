package terminal

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lichdotdev/lich/internal/providers"
)

// opencodeConfigContentVar is opencode's inline config, merged over every config
// file it reads as the last local-scope layer (measured in opencode 1.18.35).
const opencodeConfigContentVar = "OPENCODE_CONFIG_CONTENT"

// opencodeMCPKey is the key opencode holds its MCP servers under.
const opencodeMCPKey = "mcp"

// pasteUnfoldEnv turns off opencode's paste summary, the "[Pasted ~N lines]"
// placeholder, when the user asked for long pastes to land unfolded. Claude Code
// and Kiro CLI are answered in the window instead
// (native/cmd/lich-native/paste_unfold.go), and no other provider has a
// switch for its fold.
//
// An OPENCODE_CONFIG_CONTENT the user already exports is extended rather than
// replaced (extendInlineConfig).
func pasteUnfoldEnv(env []string, kind string, on bool) ([]string, error) {
	if !on || kind != providers.OpenCode {
		return env, nil
	}
	env, err := extendInlineConfig(env, opencodeConfigContentVar, func(config map[string]any) error {
		experimental, err := objectAt(config, "experimental")
		if err == nil {
			experimental["disable_paste_summary"] = true
		}
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("turn off opencode's paste summary: %w", err)
	}
	return env, nil
}

// extendInlineConfig appends name=<config> to env, where config is the JSON
// object the child would otherwise see under name with set applied to it. The
// user's own value is extended rather than replaced: the child keeps only the
// last value of a repeated key, so appending a second one would drop theirs
// without a word. A value that is not a JSON object is refused for the same
// reason.
func extendInlineConfig(env []string, name string, set func(config map[string]any) error) ([]string, error) {
	config := map[string]any{}
	if own := lastEnvValue(env, name); own != "" {
		if err := json.Unmarshal([]byte(own), &config); err != nil {
			return nil, fmt.Errorf("%s must be a JSON object: %w", name, err)
		}
	}
	if err := set(config); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	content, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to encode %s: %w", name, err)
	}
	return append(env, name+"="+string(content)), nil
}

// objectAt is the object under key in config, put there empty when it is
// missing, and an error when key holds something else.
func objectAt(config map[string]any, key string) (map[string]any, error) {
	if config[key] == nil {
		object := map[string]any{}
		config[key] = object
		return object, nil
	}
	object, isObject := config[key].(map[string]any)
	if !isObject {
		return nil, fmt.Errorf("%q is not an object", key)
	}
	return object, nil
}

// lastEnvValue is the value a child process sees for key: the last entry wins.
func lastEnvValue(env []string, key string) string {
	value := ""
	for _, entry := range env {
		if k, v, ok := strings.Cut(entry, "="); ok && k == key {
			value = v
		}
	}
	return value
}
