package relay

import (
	"context"
	"fmt"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

// A request is what the person asked for from the window (resolve a pull
// request's conflicts, fix its checks, address review threads, send review
// comments or browser notes, start on an issue), handed to a session as the
// person's own turn: no ticket, nothing owed back
// (docs/hooks/notification.md, kind request).

// namedChecks is how many failing checks a checks request names; past it the
// request counts them.
const namedChecks = 8

// ghostAuthor stands for a review comment whose author GitHub no longer has.
const ghostAuthor = "ghost"

// Request words opts as a request envelope in the prompt language and submits
// it to the session as Submit does.
func (s *Service) Request(ctx context.Context, opts api.RequestOptions) (api.None, error) {
	env, err := requestEnvelope(s.lang(), opts)
	if err != nil {
		return api.None{}, err
	}
	return s.Submit(ctx, api.SubmitOptions{SessionID: opts.SessionID, Message: env.String()})
}

// requestEnvelope is opts as the envelope the session reads.
func requestEnvelope(lang prompt.Lang, opts api.RequestOptions) (Envelope, error) {
	text := prompt.For(lang)
	env := Envelope{Kind: KindRequest, Topic: opts.Topic}
	var summary, task string
	switch opts.Topic {
	case api.RequestConflicts:
		env.PR, env.Branch, env.Base = opts.PR, opts.Branch, opts.Base
		summary, task = text.RequestConflictsSummary, conflictsText(text, opts)
		env.Items = itemsOf(opts.Items, func(i api.RequestItem) Item { return Item{Attrs: []Attr{{"path", i.Path}}} })
	case api.RequestChecks:
		env.PR, env.Branch = opts.PR, opts.Branch
		named := opts.Items[:min(len(opts.Items), namedChecks)]
		summary, task = text.RequestChecksSummary, checksText(text, opts, named)
		if omitted := len(opts.Items) - len(named); omitted > 0 {
			summary = countf(text, text.RequestChecksMoreSummary, omitted)
		}
		env.Items = itemsOf(named, func(i api.RequestItem) Item { return Item{Attrs: []Attr{{"check", i.Check}}} })
	case api.RequestReviewThread:
		env.PR, env.Branch = opts.PR, opts.Branch
		summary, task = countf(text, text.RequestThreadsSummary, len(opts.Items)), threadsText(text, opts)
		env.Items = itemsOf(opts.Items, threadItem)
	case api.RequestReviewComments:
		summary, task = countf(text, text.RequestCommentsSummary, len(opts.Items)), commentsText(text, opts)
		env.Items = itemsOf(opts.Items, func(i api.RequestItem) Item {
			return Item{Attrs: []Attr{{"path", i.Path}, {"line", i.Lines}}, Text: i.Text}
		})
	case api.RequestBrowserNotes:
		env.URL, env.Image = opts.URL, opts.Image
		summary, task = countf(text, text.RequestNotesSummary, len(opts.Items)), notesText(text, opts)
		if len(opts.Items) == 0 {
			summary = text.RequestSketchSummary
		}
		env.Items = itemsOf(opts.Items, func(i api.RequestItem) Item {
			return Item{Attrs: []Attr{{"label", i.Label}, {"url", i.URL}, {"selector", i.Selector}}, Text: strings.TrimSpace(i.Text)}
		})
	case api.RequestIssue:
		env.Issue, env.URL, env.Title = opts.Issue, opts.URL, opts.Title
		summary, task = fmt.Sprintf(text.RequestIssueSummary, opts.Issue), issueText(text, opts)
	default:
		return Envelope{}, fmt.Errorf("unknown request topic %q", opts.Topic)
	}
	env.Body = []Element{{"summary", summary}, {"prompt", task}}
	return env, nil
}

func itemsOf(items []api.RequestItem, item func(api.RequestItem) Item) []Item {
	out := make([]Item, len(items))
	for i, it := range items {
		out[i] = item(it)
	}
	return out
}

// threadItem is a review thread as a row: where it sits and how it opens.
func threadItem(i api.RequestItem) Item {
	outdated := ""
	if i.Outdated {
		outdated = "true"
	}
	first := ""
	if len(i.Comments) > 0 {
		first = strings.TrimSpace(i.Comments[0].Body)
	}
	return Item{Attrs: []Attr{{"path", i.Path}, {"lines", i.Lines}, {"id", i.ID}, {"outdated", outdated}}, Text: first}
}

// countf is the form of p for n, with n in it where the form prints it: a
// language's one form may leave the count out.
func countf(text *prompt.Catalog, p prompt.Plural, n int) string {
	form := text.Pick(n, p)
	if !strings.Contains(form, "%") {
		return form
	}
	return fmt.Sprintf(form, n)
}

func conflictsText(text *prompt.Catalog, opts api.RequestOptions) string {
	if len(opts.Items) == 0 {
		return fmt.Sprintf(forgeText(opts.Forge, text.PRConflicts, text.PRConflictsGitLab), opts.PR, opts.Branch, opts.Base)
	}
	files := make([]string, len(opts.Items))
	for i, it := range opts.Items {
		files[i] = "- " + it.Path
	}
	return fmt.Sprintf(forgeText(opts.Forge, text.PRConflictsListed, text.PRConflictsListedGitLab),
		opts.PR, opts.Branch, opts.Base, strings.Join(files, "\n"))
}

func checksText(text *prompt.Catalog, opts api.RequestOptions, named []api.RequestItem) string {
	if len(named) == 0 {
		unnamed := text.Pick(opts.Failed, forgePlural(opts.Forge, text.PRChecksUnnamed, text.PRChecksUnnamedGitLab))
		return fmt.Sprintf(unnamed, opts.PR, opts.Branch, opts.Failed)
	}
	lines := make([]string, len(named))
	for i, c := range named {
		lines[i] = "- " + c.Check
		if c.URL != "" {
			lines[i] += ": " + c.URL
		}
	}
	out := fmt.Sprintf(forgeText(opts.Forge, text.PRChecksListed, text.PRChecksListedGitLab), opts.PR, opts.Branch, strings.Join(lines, "\n"))
	if omitted := len(opts.Items) - len(named); omitted > 0 {
		out += countf(text, text.PRChecksOmitted, omitted)
	}
	return out
}

// threadsText is each thread where it is and what was said in it, then the
// ask.
func threadsText(text *prompt.Catalog, opts api.RequestOptions) string {
	parts := make([]string, 0, len(opts.Items)+1)
	for _, t := range opts.Items {
		head := text.ReviewThreadHead
		if t.Outdated {
			head = text.ReviewThreadHeadOutdated
		}
		lines := []string{fmt.Sprintf(head, opts.PR, opts.Branch, t.Path, t.Lines), fmt.Sprintf(text.ReviewThreadID, t.ID)}
		if t.URL != "" {
			lines = append(lines, t.URL)
		}
		lines = append(lines, "")
		for _, c := range t.Comments {
			author := c.Author
			if author == "" {
				author = ghostAuthor
			}
			lines = append(lines, "- @"+author+": "+continuation(strings.TrimSpace(c.Body)))
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}
	parts = append(parts, text.Pick(len(opts.Items), text.ReviewThreadAsk))
	return strings.Join(parts, "\n\n")
}

// commentsText is the batch as a markdown list, each item anchored the way an
// inject writes a line reference, the person's closing note last.
func commentsText(text *prompt.Catalog, opts api.RequestOptions) string {
	items := make([]string, 0, len(opts.Items)+1)
	for _, c := range opts.Items {
		if c.Path == "" {
			items = append(items, "- "+continuation(c.Text))
			continue
		}
		items = append(items, fmt.Sprintf("- %s:%s: %s", c.Path, c.Lines, continuation(c.Text)))
	}
	if note := strings.TrimSpace(opts.Note); note != "" {
		items = append(items, "- "+continuation(note))
	}
	return text.ReviewCommentsHeader + "\n\n" + strings.Join(items, "\n")
}

// notesText is the notes as the agent reads them: each element's label and
// page, the selector browser_click takes, the React component that drew it,
// its markup and the person's note, then the sketch, then the person's closing
// note. A sketch with no notes names its page instead.
func notesText(text *prompt.Catalog, opts api.RequestOptions) string {
	var lines []string
	switch {
	case len(opts.Items) == 0:
		lines = []string{fmt.Sprintf(text.BrowserNotesSketchOnly, opts.URL, opts.Image)}
	default:
		lines = noteLines(text, opts.Items)
		if opts.Image != "" {
			lines = append(lines, fmt.Sprintf(text.BrowserNotesSketch, opts.Image))
		}
		lines = append(lines, text.BrowserNotesSelectors)
	}
	if note := strings.TrimSpace(opts.Note); note != "" {
		lines = append(lines, "", note)
	}
	return strings.Join(lines, "\n")
}

func noteLines(text *prompt.Catalog, notes []api.RequestItem) []string {
	lines := []string{countf(text, text.BrowserNotesHead, len(notes))}
	for i, n := range notes {
		lines = append(lines, fmt.Sprintf(text.BrowserNotesItem, i+1, n.Label, n.URL), "   "+n.Selector)
		if n.Component != "" {
			react := "   React: " + n.Component
			if n.Source != "" {
				react += " (" + n.Source + ")"
			}
			lines = append(lines, react)
		}
		lines = append(lines, "   "+n.HTML)
		if note := strings.TrimSpace(n.Text); note != "" {
			lines = append(lines, "   "+fmt.Sprintf(text.BrowserNotesNote, strings.ReplaceAll(note, "\n", " ")))
		}
	}
	return lines
}

// issueText is the issue's head, then its body when it has one.
func issueText(text *prompt.Catalog, opts api.RequestOptions) string {
	head := fmt.Sprintf(text.IssueHead, opts.Issue, opts.Title, opts.URL)
	if body := strings.TrimSpace(opts.Body); body != "" {
		return head + "\n\n" + body
	}
	return head
}

// continuation keeps a multi-line comment one list item.
func continuation(text string) string { return strings.ReplaceAll(text, "\n", "\n  ") }

// forgePlural is forgeText for a Plural.
func forgePlural(forge string, github, gitlab prompt.Plural) prompt.Plural {
	if forge == api.ForgeGitLab {
		return gitlab
	}
	return github
}
