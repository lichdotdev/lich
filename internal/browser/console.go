package browser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/runtime"
)

// consoleErrorMax and consoleTextMax keep a page answer lean: a page that
// throws in a loop names its first errors and how many more, not every one.
const (
	consoleErrorMax = 5
	consoleTextMax  = 200
)

// consoleSink keeps each error the page's console shows until the agent's
// next page answer.
func (s *Service) consoleSink(sess *session) func(text string) {
	return func(text string) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if len(sess.consoleErrors) < consoleErrorMax {
			sess.consoleErrors = append(sess.consoleErrors, consoleLine(text))
		} else {
			sess.consoleMore++
		}
	}
}

// consoleUntold is the errors the agent has not heard, which it now has.
func (s *Service) consoleUntold(sess *session) ([]string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, more := sess.consoleErrors, sess.consoleMore
	sess.consoleErrors, sess.consoleMore = nil, 0
	return out, more
}

// consoleLine is text on one line, cut to consoleTextMax runes.
func consoleLine(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if r := []rune(text); len(r) > consoleTextMax {
		return string(r[:consoleTextMax]) + "…"
	}
	return text
}

// consoleError is the text of a CDP event that is a console error: a
// console.error call, an uncaught exception, or an error Chrome logs itself,
// such as a resource that failed to load. Warnings and the rest are not.
func consoleError(ev any) (string, bool) {
	switch e := ev.(type) {
	case *runtime.EventConsoleAPICalled:
		return consoleCallText(e), e.Type == runtime.APITypeError
	case *runtime.EventExceptionThrown:
		return exceptionText(e.ExceptionDetails), e.ExceptionDetails != nil
	case *log.EventEntryAdded:
		return logEntryText(e.Entry), e.Entry != nil && e.Entry.Level == log.LevelError
	}
	return "", false
}

func consoleCallText(e *runtime.EventConsoleAPICalled) string {
	parts := make([]string, len(e.Args))
	for i, a := range e.Args {
		parts[i] = objectText(a)
	}
	return strings.Join(parts, " ")
}

// objectText is how the console prints one argument, first line only: an
// Error's description carries its stack after it.
func objectText(o *runtime.RemoteObject) string {
	var s string
	switch {
	case len(o.Value) > 0:
		if json.Unmarshal(o.Value, &s) != nil {
			s = string(o.Value)
		}
	case o.UnserializableValue != "":
		s = string(o.UnserializableValue)
	case o.Description != "":
		s = o.Description
	default:
		s = string(o.Type)
	}
	first, _, _ := strings.Cut(s, "\n")
	return first
}

// exceptionText is "Uncaught TypeError: … (url:line)", the way the console
// shows it, the line 1-based.
func exceptionText(d *runtime.ExceptionDetails) string {
	if d == nil {
		return ""
	}
	text := d.Text
	if d.Exception != nil {
		text += " " + objectText(d.Exception)
	}
	if d.URL != "" {
		text += fmt.Sprintf(" (%s:%d)", d.URL, d.LineNumber+1)
	}
	return text
}

func logEntryText(e *log.Entry) string {
	if e == nil {
		return ""
	}
	if e.URL == "" {
		return e.Text
	}
	return e.Text + " " + e.URL
}
