package themes

import (
	"fmt"
	"strings"

	"github.com/lichdotdev/godemirror/tags"
	"github.com/lichdotdev/lich/api"
)

type (
	SyntaxStyle = api.SyntaxStyle
	Syntax      = api.Syntax
)

func setsFontStyle(s SyntaxStyle) bool {
	return s.Bold || s.Italic || s.Underline || s.Strikethrough
}

func validateSyntax(syntax Syntax) error {
	for tag, style := range syntax {
		if _, err := tags.Parse(tag); err != nil {
			return fmt.Errorf("syntax: %w", err)
		}
		if strings.TrimSpace(style.Color) == "" {
			if !setsFontStyle(style) {
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
