// Package i18n reads lich's translation catalogs: locales/<tag>/<namespace>.json,
// one tree per namespace, looked up by a dotted key ("settings.title"). A
// message holds {name} placeholders or is an object of CLDR plural forms
// picked by an int "count". lich's window reads its own catalogs with this
// package, and an extension reads its own with Bundle.
package i18n

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"
)

// Args are a message's placeholder values by name; "count" picks a plural's
// form and must be an int.
type Args map[string]any

// Message is a leaf of a catalog: a text, or a plural's CLDR forms.
type Message struct {
	Text   string
	Plural map[string]string
}

// Catalog is one locale's messages by dotted key, the namespace first.
type Catalog map[string]Message

// Load reads every <namespace>.json in the directory dir of fsys into one
// catalog. Any other file is left alone.
func Load(fsys fs.FS, dir string) (Catalog, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	out := Catalog{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var tree map[string]any
		if err := json.Unmarshal(data, &tree); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if err := flatten(out, strings.TrimSuffix(e.Name(), ".json"), tree); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
	}
	return out, nil
}

// flatten walks tree into out under prefix. An object holding "other" is a
// plural: every CLDR rule falls back to it.
func flatten(out Catalog, prefix string, tree map[string]any) error {
	if _, ok := tree["other"]; ok {
		forms := make(map[string]string, len(tree))
		for form, v := range tree {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("%s.%s: a plural form must be a string", prefix, form)
			}
			forms[form] = s
		}
		out[prefix] = Message{Plural: forms}
		return nil
	}
	for k, v := range tree {
		key := prefix + "." + k
		switch v := v.(type) {
		case string:
			out[key] = Message{Text: v}
		case map[string]any:
			if err := flatten(out, key, v); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s: want a string or an object, got %T", key, v)
		}
	}
	return nil
}

// ErrNoCount is a plural asked for without an int count.
var ErrNoCount = errors.New("is a plural and needs an int count")

// Template is m's text in the language tag: a plural's form for args'
// count, or "other" when the catalog lacks the form the rule picks.
func (m Message) Template(tag string, args Args) (string, error) {
	if m.Plural == nil {
		return m.Text, nil
	}
	count, ok := args["count"].(int)
	if !ok {
		return "", ErrNoCount
	}
	if form, ok := m.Plural[PluralForm(tag, count)]; ok {
		return form, nil
	}
	return m.Plural["other"], nil
}

// Placeholder matches a {name} in a message.
var Placeholder = regexp.MustCompile(`\{(\w+)\}`)

// Fill puts args' values in text's placeholders. A placeholder without a
// value is left as it is and named in the error.
func Fill(text string, args Args) (string, error) {
	var missing error
	out := Placeholder.ReplaceAllStringFunc(text, func(match string) string {
		name := match[1 : len(match)-1]
		v, ok := args[name]
		if !ok {
			if missing == nil {
				missing = fmt.Errorf("no value for {%s} in %q", name, text)
			}
			return match
		}
		return fmt.Sprint(v)
	})
	return out, missing
}

// Match is the tag of among that tag reads: an exact match, ignoring case,
// then a language match (any Portuguese reads pt-BR), then "".
func Match(tag string, among []string) string {
	for _, l := range among {
		if strings.EqualFold(l, tag) {
			return l
		}
	}
	language, _, _ := strings.Cut(tag, "-")
	for _, l := range among {
		prefix, _, _ := strings.Cut(l, "-")
		if language != "" && strings.EqualFold(prefix, language) {
			return l
		}
	}
	return ""
}

// TagFromEnv is the POSIX locale the environment names, in the order glibc
// reads it for messages (LC_ALL, LC_MESSAGES, LANG), as a BCP 47 tag:
// pt_BR.UTF-8@euro reads pt-BR. C and POSIX name no language.
func TagFromEnv(getenv func(string) string) string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := getenv(name)
		if v == "" {
			continue
		}
		v, _, _ = strings.Cut(v, ".")
		v, _, _ = strings.Cut(v, "@")
		if v == "C" || v == "POSIX" {
			return ""
		}
		return strings.ReplaceAll(v, "_", "-")
	}
	return ""
}
