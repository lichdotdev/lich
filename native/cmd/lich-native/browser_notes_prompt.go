package main

import (
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/native/i18n"
)

// notesMessage is the notes as the agent reads them: each element's label and
// page, the selector browser_click takes, the React component that drew it,
// its markup, and the person's note; closing, the note about all of them, ends
// it.
func notesMessage(notes []*browserNote, closing string) string {
	return endedWith(append(noteLines(notes), i18n.Prompt("prompts.browserNotes.selectors")), closing)
}

// sketchedMessage is notesMessage with the sketch: its picture at path,
// after the notes when there are any, and alone, naming the page at url, when
// there are none.
func sketchedMessage(notes []*browserNote, path, url, closing string) string {
	if len(notes) == 0 {
		return endedWith([]string{i18n.Prompt("prompts.browserNotes.sketchOnly", i18n.Args{"url": url, "path": path})}, closing)
	}
	lines := append(noteLines(notes), i18n.Prompt("prompts.browserNotes.sketch", i18n.Args{"path": path}), i18n.Prompt("prompts.browserNotes.selectors"))
	return endedWith(lines, closing)
}

func noteLines(notes []*browserNote) []string {
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
	return lines
}

func endedWith(lines []string, closing string) string {
	if closing = strings.TrimSpace(closing); closing != "" {
		lines = append(lines, "", closing)
	}
	return strings.Join(lines, "\n")
}
