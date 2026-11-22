package relay

import (
	"fmt"
	"time"

	"github.com/lichdotdev/lich/internal/prompt"
)

// Each note lich hands an agent, as the envelope that carries it.

// taskEnvelope is a ticket's task as its worker reads it. An extension's
// errand names the extension and no session: there is none to answer to but
// the ticket.
func taskEnvelope(id string, t *ticket, answer string) Envelope {
	from, session := t.sender, t.fromID
	if t.extensionID != "" {
		from, session = t.extensionName, ""
	}
	return Envelope{
		Kind: KindTask, From: from, Session: session, Ticket: id, Answer: answer,
		Body: []Element{{"prompt", t.prompt}},
	}
}

// loopEnvelope rides behind a task that continues a ping-pong streak.
func loopEnvelope(lang prompt.Lang, t *ticket) Envelope {
	return Envelope{
		Kind: KindNotice, Topic: TopicLoop, From: t.sender, Session: t.fromID,
		Body: []Element{{"summary", prompt.For(lang).PingPongWarning}},
	}
}

// outcomeEnvelope is one inbox entry as its sender reads it. whole is whether
// a result goes in or only the line saying where to collect it: typed into a
// terminal, a long report is what the relay avoids.
func outcomeEnvelope(lang prompt.Lang, e *inboxEntry, whole, hasTools bool) Envelope {
	text := prompt.For(lang)
	env := Envelope{From: e.target, Session: e.targetID, Ticket: e.ticket}
	switch e.status {
	case StatusAnswered:
		env.Kind = KindWorkerFinished
		env.Body = []Element{{"status", StatusAnswered}}
		if whole {
			result, cut := cutResult(e.answer)
			env.Truncated = cut
			env.Body = append(env.Body, Element{"result", result})
			break
		}
		collect := text.CollectWithCLI
		if hasTools {
			collect = text.CollectWithTool
		}
		env.Body = append(env.Body, Element{"summary", collect})
	case StatusStopped:
		env.Kind = KindWorkerStopped
		env.Body = []Element{{"status", StatusStopped}, {"by", stoppedBy(e.by)}}
	case StatusExpired:
		env.Kind = KindExpired
		env.Body = []Element{{"status", StatusExpired}}
	default:
		env.Kind = KindWorkerFailed
		env.Body = []Element{{"status", "failed"}, {"error", failure(text, e)}}
	}
	return env
}

// stoppedByUser is who closed a worker from the window.
const stoppedByUser = "user"

func stoppedBy(label string) string {
	if label == "" {
		return stoppedByUser
	}
	return label
}

// failure is what ended an errand with no answer: the reason a worker's mod
// gave, or what the relay saw.
func failure(text *prompt.Catalog, e *inboxEntry) string {
	if e.answer != "" {
		return e.answer
	}
	switch e.status {
	case StatusUnread:
		return fmt.Sprintf(text.OutcomeUnread, e.target)
	case StatusUndelivered:
		return fmt.Sprintf(text.OutcomeUndelivered, e.target)
	}
	return fmt.Sprintf(text.OutcomeUnanswered, e.target)
}

// waitingEnvelope tells a sender its worker is blocked on request.
func waitingEnvelope(t *ticket, id, request string) Envelope {
	return Envelope{
		Kind: KindWorkerWaiting, From: t.target, Session: t.targetID, Ticket: id,
		Body: []Element{{"status", "waiting"}, {"request", request}},
	}
}

// mergedEnvelope says the work is done and asks for nothing: a notice that
// read as a task would start one (deleting the branch, pulling the base) in a
// checkout the user may be about to remove.
func mergedEnvelope(lang prompt.Lang, number int, title, branch, base string) Envelope {
	return Envelope{
		Kind: KindMerged, PR: number, Branch: branch,
		Body: []Element{{"summary", fmt.Sprintf(prompt.For(lang).MergeNotice, number, sanitize(title), base)}},
	}
}

// noticeEnvelope is a notice about this session, with the task it carries
// when there is one.
func noticeEnvelope(topic, summary, task string) Envelope {
	env := Envelope{Kind: KindNotice, Topic: topic, Body: []Element{{"summary", summary}}}
	if task != "" {
		env.Body = append(env.Body, Element{"prompt", task})
	}
	return env
}

// scheduledEnvelope is a parked prompt come due, saying when it was due and,
// when it missed that, by how much (lateNotice).
func scheduledEnvelope(lang prompt.Lang, at, now int64, task string) Envelope {
	summary := lateNotice(lang, at, now)
	if summary == "" {
		summary = fmt.Sprintf(prompt.For(lang).OnTimeNotice, time.Unix(at, 0).Format(scheduleClock))
	}
	return noticeEnvelope(TopicScheduled, summary, task)
}
