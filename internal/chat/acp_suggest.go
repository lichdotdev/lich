package chat

import (
	"encoding/json"
	"strings"
)

// suggestions reads the agent's own prompt_suggestions_update, which ACP marks
// UNSTABLE (schema v1.34.1: not in the spec yet, liable to change or go). Its
// list replaces the last, most likely first, and an empty one withdraws it;
// the box shows the first suggestion's text. One heard during the turn waits
// for its end, where the same rules as every provider's decide.
func (d *acpDriver) suggestions(raw json.RawMessage) {
	var u struct {
		Suggestions []struct {
			Prompt []acpContent `json:"prompt"`
		} `json:"suggestions"`
	}
	if json.Unmarshal(raw, &u) != nil {
		return
	}
	var text strings.Builder
	if len(u.Suggestions) > 0 {
		for _, b := range u.Suggestions[0].Prompt {
			if b.Type == "text" {
				text.WriteString(b.Text)
			}
		}
	}
	if d.c.turnRunning() {
		d.mu.Lock()
		d.suggested = strings.TrimSpace(text.String())
		d.mu.Unlock()
		return
	}
	d.c.suggest(d.c.offered(), strings.TrimSpace(text.String()))
}

// suggestHeld shows the suggestion the agent sent during the turn just ended.
func (d *acpDriver) suggestHeld() {
	d.mu.Lock()
	text := d.suggested
	d.suggested = ""
	d.mu.Unlock()
	if text != "" {
		d.c.suggest(d.c.offered(), text)
	}
}
