package main

import (
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lichdotdev/lich/native/backend"
)

// timelineItem is one entry of the Conversation tab: exactly one of review,
// comment and thread is set.
type timelineItem struct {
	at      time.Time
	review  *backend.PRReview
	comment *backend.PRComment
	thread  *backend.PRReviewThread
}

// conversationTimeline orders reviews, comments and the
// open threads oldest first, the resolved threads apart, newest first.
func conversationTimeline(c *backend.PRConversation) (items []timelineItem, resolved []backend.PRReviewThread) {
	if c == nil {
		return nil, nil
	}
	for i := range c.Reviews {
		items = append(items, timelineItem{at: parsedTime(c.Reviews[i].Date), review: &c.Reviews[i]})
	}
	for i := range c.Comments {
		items = append(items, timelineItem{at: parsedTime(c.Comments[i].Date), comment: &c.Comments[i]})
	}
	for i := range c.Threads {
		t := &c.Threads[i]
		if t.IsResolved {
			resolved = append(resolved, *t)
			continue
		}
		items = append(items, timelineItem{at: threadStart(*t), thread: t})
	}
	slices.SortStableFunc(items, func(a, b timelineItem) int { return a.at.Compare(b.at) })
	slices.SortStableFunc(resolved, func(a, b backend.PRReviewThread) int { return threadStart(b).Compare(threadStart(a)) })
	return items, resolved
}

func parsedTime(iso string) time.Time {
	t, _ := time.Parse(time.RFC3339, iso)
	return t
}

func threadStart(t backend.PRReviewThread) time.Time {
	if len(t.Comments) == 0 {
		return time.Time{}
	}
	return parsedTime(t.Comments[0].Date)
}

// threadTally is how the threads stand: open against all. An outdated thread
// nobody resolved is open: rewriting the line is not answering the remark.
func threadTally(c *backend.PRConversation) (open, total int) {
	if c == nil {
		return 0, 0
	}
	for _, t := range c.Threads {
		if !t.IsResolved {
			open++
		}
	}
	return open, len(c.Threads)
}

// lineSpan reads a composer's lines ("19", "12-30") as a review comment's
// startLine and line; startLine is 0 for one line.
func lineSpan(lines string) (start, line int) {
	from, to, ranged := strings.Cut(lines, "-")
	first, _ := strconv.Atoi(from)
	if !ranged {
		return 0, first
	}
	last, _ := strconv.Atoi(to)
	return first, last
}

// pendingReview holds, in memory, a review's line comments, sent together
// when the review is submitted.
type pendingReview struct {
	mu       sync.Mutex
	comments []backend.ReviewComment
}

func (r *pendingReview) add(path, lines, body string) {
	start, line := lineSpan(lines)
	r.mu.Lock()
	r.comments = append(r.comments, backend.ReviewComment{Path: path, Line: line, StartLine: start, Side: "RIGHT", Body: body})
	r.mu.Unlock()
}

func (r *pendingReview) remove(i int) {
	r.mu.Lock()
	if i >= 0 && i < len(r.comments) {
		r.comments = slices.Delete(r.comments, i, i+1)
	}
	r.mu.Unlock()
}

// edit rewords comment i, keeping where it points.
func (r *pendingReview) edit(i int, body string) {
	r.mu.Lock()
	if i >= 0 && i < len(r.comments) {
		r.comments[i].Body = body
	}
	r.mu.Unlock()
}

func (r *pendingReview) clear() {
	r.mu.Lock()
	r.comments = nil
	r.mu.Unlock()
}

// drop removes the comments sent, one for each, and keeps any added since.
func (r *pendingReview) drop(sent []backend.ReviewComment) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range sent {
		if i := slices.Index(r.comments, c); i >= 0 {
			r.comments = slices.Delete(r.comments, i, i+1)
		}
	}
}

func (r *pendingReview) snapshot() []backend.ReviewComment {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.comments)
}
