package main

import (
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/native/i18n"
)

// notesMessage is the notes as the agent reads them: each element's label and
// page, the selector browser_click takes, the React component that drew it,
// its markup, and the person's note.
func notesMessage(notes []*browserNote) string {
	lines := []string{i18n.Prompt("prompts.browserNotes.head", i18n.Args{"count": len(notes)})}
	for i, n := range notes {
		el := n.el
		lines = append(lines, i18n.Prompt("prompts.browserNotes.item", i18n.Args{"index": strconv.Itoa(i + 1), "label": el.Label, "url": el.URL}),
			"   "+el.Selector)
		if el.Component != "" {
			react := "   React: " + el.Component
			if el.Source != "" {
				react += " (" + el.Source + ")"
			}
			lines = append(lines, react)
		}
		lines = append(lines, "   "+el.HTML)
		if text := strings.TrimSpace(n.text); text != "" {
			lines = append(lines, "   "+i18n.Prompt("prompts.browserNotes.note", i18n.Args{"note": strings.ReplaceAll(text, "\n", " ")}))
		}
	}
	lines = append(lines, i18n.Prompt("prompts.browserNotes.selectors"))
	return strings.Join(lines, "\n")
}
