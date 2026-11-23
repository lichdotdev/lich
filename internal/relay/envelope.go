package relay

import (
	"strconv"
	"strings"
)

// The notification envelope (docs/hooks/notification.md): the one shape every
// note lich hands an agent takes. Nothing else in lich writes
// "<lich-notification".

// The kinds an envelope is.
const (
	KindTask           = "task"
	KindWorkerFinished = "worker-finished"
	KindWorkerFailed   = "worker-failed"
	KindWorkerStopped  = "worker-stopped"
	KindWorkerWaiting  = "worker-waiting"
	KindExpired        = "expired"
	KindMerged         = "merged"
	KindNotice         = "notice"
	KindRequest        = "request"
)

// The topics a notice carries.
const (
	TopicChecksFailed = "checks-failed"
	TopicChecksPassed = "checks-passed"
	TopicReview       = "review"
	TopicScheduled    = "scheduled"
	TopicUsageReset   = "usage-reset"
	TopicCompacted    = "compacted"
	TopicUnanswered   = "unanswered"
	TopicLoop         = "loop"
	TopicHandoff      = "handoff"
	TopicEditOverlap  = "edit-overlap"
)

// How a task's answer is taken.
const (
	AnswerLastMessage = "last-message"
	AnswerReply       = "reply"
)

// resultCap is where a worker's result is cut inside the envelope, the cut
// terminal.LastTurnSaid makes (saidRuneCap); the rest waits on the ticket.
const resultCap = 4000

// envelopeOpen is how every envelope begins.
const envelopeOpen = "<lich-notification "

// envelopeVersion moves only when a kind, attribute or element is renamed or
// removed.
const envelopeVersion = "1"

// Envelope is one notification. Empty attributes are left out, and Body is
// written in order, each element's text verbatim.
type Envelope struct {
	Kind      string
	Topic     string
	From      string
	Session   string
	Ticket    string
	Answer    string
	Truncated int
	PR        int
	Issue     int
	Branch    string
	Base      string
	URL       string
	Image     string
	Title     string
	Body      []Element
	// Items are a request's rows, written before its prompt.
	Items []Item
}

// Element is one child of an envelope.
type Element struct {
	Tag  string
	Text string
}

// Item is one <item> of a request: its attributes in order, empty ones left
// out, and its text, self-closing when there is none.
type Item struct {
	Attrs []Attr
	Text  string
}

// Attr is one attribute of an item.
type Attr struct {
	Name, Value string
}

var attrEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

// String is the envelope as the agent reads it. Element text is not escaped:
// the model reads it as text, so an escaped a && b would reach it as
// a &amp;&amp; b (docs/hooks/notification.md, Text).
func (e Envelope) String() string {
	var b strings.Builder
	b.WriteString(envelopeOpen + `version="` + envelopeVersion + `"`)
	attr := func(name, value string) {
		if value != "" {
			b.WriteString(" " + name + `="` + attrEscaper.Replace(value) + `"`)
		}
	}
	number := func(n int) string {
		if n == 0 {
			return ""
		}
		return strconv.Itoa(n)
	}
	attr("kind", e.Kind)
	attr("topic", e.Topic)
	attr("from", e.From)
	attr("session", e.Session)
	attr("ticket", e.Ticket)
	attr("answer", e.Answer)
	attr("truncated", number(e.Truncated))
	attr("pr", number(e.PR))
	attr("issue", number(e.Issue))
	attr("branch", e.Branch)
	attr("base", e.Base)
	attr("url", e.URL)
	attr("image", e.Image)
	attr("title", e.Title)
	b.WriteString(">\n")
	items := false
	writeItems := func() {
		for _, it := range e.Items {
			b.WriteString("<item")
			for _, a := range it.Attrs {
				attr(a.Name, a.Value)
			}
			if it.Text == "" {
				b.WriteString("/>\n")
				continue
			}
			b.WriteString(">" + it.Text + "</item>\n")
		}
		items = true
	}
	for _, el := range e.Body {
		if el.Tag == "prompt" && !items {
			writeItems()
		}
		b.WriteString("<" + el.Tag + ">" + el.Text + "</" + el.Tag + ">\n")
	}
	if !items {
		writeItems()
	}
	b.WriteString("</lich-notification>")
	return b.String()
}

// joinEnvelopes is several envelopes in one message, in the order given,
// one blank line apart.
func joinEnvelopes(envs []Envelope) string {
	parts := make([]string, len(envs))
	for i, e := range envs {
		parts[i] = e.String()
	}
	return strings.Join(parts, "\n\n")
}

// cutResult is a worker's result as an envelope carries it, and how many
// characters were left out.
func cutResult(text string) (string, int) {
	runes := []rune(text)
	if len(runes) <= resultCap {
		return text, 0
	}
	return string(runes[:resultCap]), len(runes) - resultCap
}
