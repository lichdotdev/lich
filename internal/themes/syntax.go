package themes

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lichdotdev/godemirror/tags"
)

// SyntaxStyle is how the code under one highlighting tag is drawn: a
// CodeMirror HighlightStyle spec. A theme file writes it as a bare color or
// as an object; Color empty leaves the code its foreground color.
type SyntaxStyle struct {
	Color         string `json:"color,omitempty"`
	Bold          bool   `json:"bold,omitempty"`
	Italic        bool   `json:"italic,omitempty"`
	Underline     bool   `json:"underline,omitempty"`
	Strikethrough bool   `json:"strikethrough,omitempty"`
}

// Syntax is a theme's syntax highlighting, keyed by the tag each style is
// for, as @lezer/highlight prints it: "keyword", "function(variableName)".
type Syntax map[string]SyntaxStyle

// UnmarshalJSON reads each entry as a bare color or a style object, naming
// the entry that is neither.
func (s *Syntax) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("syntax must be an object of tag styles: %w", err)
	}
	out := make(Syntax, len(raw))
	for tag, entry := range raw {
		var style SyntaxStyle
		if err := json.Unmarshal(entry, &style.Color); err != nil {
			if err := json.Unmarshal(entry, &style); err != nil {
				return fmt.Errorf("syntax.%s must be a color or a style object", tag)
			}
		}
		out[tag] = style
	}
	*s = out
	return nil
}

func (s SyntaxStyle) setsFontStyle() bool {
	return s.Bold || s.Italic || s.Underline || s.Strikethrough
}

func validateSyntax(syntax Syntax) error {
	for tag, style := range syntax {
		if _, err := tags.Parse(tag); err != nil {
			return fmt.Errorf("syntax: %w", err)
		}
		if strings.TrimSpace(style.Color) == "" {
			if !style.setsFontStyle() {
				return fmt.Errorf("syntax.%s sets no color and no font style", tag)
			}
			continue
		}
		if err := validateColorValue("syntax", tag, style.Color, appColorValue); err != nil {
			return err
		}
	}
	return nil
}
