package themes

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
)

// SchemaPath is where the generated schema lives in this repository, and the
// tail of the raw URL a theme author points "$schema" at.
const SchemaPath = "themes/lich-theme.schema.json"

const schemaID = "https://raw.githubusercontent.com/lichdotdev/lich/main/" + SchemaPath

// Schema renders the JSON Schema a theme file is checked against in an editor.
// It is generated from the same token sets and patterns validateTheme reads, so
// a token added to the bundled theme cannot leave the schema behind; the test
// beside this file fails when the checked-in copy has drifted.
func Schema() ([]byte, error) {
	schema := map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  schemaID,
		"title":                "lich theme",
		"description":          "A color theme for lich.",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"id", "name", "scheme", "app", "terminal"},
		"properties": map[string]any{
			"$schema": map[string]any{"type": "string"},
			"formatVersion": map[string]any{
				"description": "The theme file format. Omitted means 1; lich refuses a format newer than it reads.",
				"type":        "integer",
				"minimum":     1,
				"maximum":     FormatVersion,
			},
			"id": map[string]any{
				"description": "Names the file the theme is stored as, so reserved ids and Windows device names are out.",
				"type":        "string",
				"pattern":     idPattern.String(),
				"maxLength":   themeIDMaxLength,
				"not":         map[string]any{"enum": rejectedIDs()},
			},
			"name": map[string]any{
				"type":      "string",
				"maxLength": themeNameMaxLength,
				// Anchorless, so it only asks for one non-space rune anywhere:
				// the name is rejected when it trims to nothing, not when it
				// is short.
				"pattern": `\S`,
			},
			"scheme": map[string]any{"enum": []string{SchemeLight, SchemeDark}},
			"origin": map[string]any{
				"description": "Written by lich on import; a hand-written value is ignored.",
				"enum":        []string{OriginBundled, OriginCustom},
			},
			"source": map[string]any{
				"description":          "Written by lich for a theme installed from a repository.",
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"url", "version"},
				"properties": map[string]any{
					"url":     map[string]any{"type": "string"},
					"version": map[string]any{"type": "string", "pattern": `^v?\d+\.\d+\.\d+$`},
				},
			},
			"app":      colorObject(appTokens, "#/$defs/appColor", sortedKeys(appTokens)),
			"terminal": colorObject(terminalTokens, "#/$defs/terminalColor", []string{"background", "foreground"}),
			"syntax":   syntaxObject(),
			"pair": map[string]any{
				"description": "The id of this theme's variant for the other scheme, drawn while the OS asks for that scheme and following it is on. lich lists a theme whose pair has the same scheme as broken.",
				"type":        "string",
				"pattern":     idPattern.String(),
				"maxLength":   themeIDMaxLength,
			},
		},
		"$defs": map[string]any{
			"appColor":      colorValue(appColorPattern),
			"terminalColor": colorValue(terminalColorPattern),
			"syntaxStyle":   syntaxStyle(),
		},
	}
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode theme schema: %w", err)
	}
	return append(data, '\n'), nil
}

func colorObject(tokens map[string]struct{}, ref string, required []string) map[string]any {
	properties := make(map[string]any, len(tokens))
	for token := range tokens {
		properties[token] = map[string]any{"$ref": ref}
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             required,
		"properties":           properties,
	}
}

// syntaxObject keys styles by tag. The pattern only checks a tag's spelling,
// a name under any modifiers; which names exist is godemirror's tags.Parse,
// checked on import.
func syntaxObject() map[string]any {
	return map[string]any{
		"description":          "Syntax highlighting by CodeMirror highlighting tag, as in \"keyword\" or \"function(variableName)\". The native window reads it; the web window ignores it.",
		"type":                 "object",
		"propertyNames":        map[string]any{"pattern": `^([a-z]+\()*[a-zA-Z]+\)*$`},
		"additionalProperties": map[string]any{"$ref": "#/$defs/syntaxStyle"},
	}
}

func syntaxStyle() map[string]any {
	return map[string]any{
		"oneOf": []any{
			map[string]any{"$ref": "#/$defs/appColor"},
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"minProperties":        1,
				"properties": map[string]any{
					"color":         map[string]any{"$ref": "#/$defs/appColor"},
					"bold":          map[string]any{"type": "boolean"},
					"italic":        map[string]any{"type": "boolean"},
					"underline":     map[string]any{"type": "boolean"},
					"strikethrough": map[string]any{"type": "boolean"},
				},
			},
		},
	}
}

func colorValue(pattern string) map[string]any {
	return map[string]any{
		"type":      "string",
		"pattern":   pattern,
		"maxLength": colorValueMaxLength,
	}
}

// rejectedIDs is every id validateID turns down for what it means rather than
// for its spelling.
func rejectedIDs() []string {
	all := make(map[string]struct{}, len(reserved)+len(windowsDeviceNames))
	maps.Copy(all, reserved)
	maps.Copy(all, windowsDeviceNames)
	return sortedKeys(all)
}

func sortedKeys(tokens map[string]struct{}) []string {
	return slices.Sorted(maps.Keys(tokens))
}
