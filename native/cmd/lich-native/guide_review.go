package main

import driver "github.com/lichdotdev/gio-driver"

const reviewID = "review"

// The widgets the Review changes guide points at; the panel holds steps 2
// and 3, since the composer opens inside its scrolling list.
const (
	tourReviewChip  = "review.chip"
	tourReviewPanel = "review.panel"
	tourReviewBatch = "review.batch"
)

var reviewTour = []tourStep{
	{anchor: tourReviewChip, side: driver.Top, until: (*window).tourReviewOpen,
		text: tourText{"tourReview.chip.title", "tourReview.chip.body", "tourReview.chip.hint"}},
	{anchor: tourReviewPanel, side: driver.Left, text: tourText{"tourReview.panel.title", "tourReview.panel.body", ""}},
	{anchor: tourReviewPanel, side: driver.Left, until: (*window).tourReviewCommented,
		text: tourText{"tourReview.comment.title", "tourReview.comment.body", "tourReview.comment.hint"}},
	// Waits for the send rather than a Next: the strip leaves the screen once
	// the session takes the batch, which would strand the popover.
	{anchor: tourReviewBatch, side: driver.Left, until: (*window).tourReviewSent,
		text: tourText{"tourReview.send.title", "tourReview.send.body", "tourReview.send.hint"}},
}

func init() {
	registerGuide(&guide{id: reviewID, order: 20, label: "tourReview.label", steps: reviewTour,
		ready: (*window).tourReviewReady, unavailable: "tourReview.unavailable"})
}

// canReview is whether sv has changes to review and takes a batch: a plain
// terminal refuses the send.
func canReview(sv sessionView) bool { return sv.Kind != "shell" && sv.git.Files > 0 }

func (u *window) tourReviewReady() bool {
	sv, ok := u.model.activeSession()
	return ok && canReview(sv)
}

func (u *window) tourReviewOpen() bool { return u.dock.tab == dockReview }

// reviewComments is the batch of the active session's checkout, the target
// the dock's Review tab keys it by.
func (u *window) reviewComments() int {
	sv, ok := u.model.activeSession()
	if !ok {
		return 0
	}
	return len(u.comments.list(sv.shown))
}

func (u *window) tourReviewCommented() bool { return u.reviewComments() > 0 }

func (u *window) tourReviewSent() bool { return u.reviewComments() == 0 }
