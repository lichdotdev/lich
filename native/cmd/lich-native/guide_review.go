package main

import driver "github.com/lichdotdev/gio-driver"

const reviewID = "review"

// The widgets the Review changes guide points at: the changes chip, the dock
// body, the right-click menu's Comment for the session, the composer and the
// batch.
const (
	tourReviewChip     = "review.chip"
	tourReviewPanel    = "review.panel"
	tourReviewItem     = "review.item"
	tourReviewComposer = "review.composer"
	tourReviewBatch    = "review.batch"
)

// tourReviewSelect is the step that opens the right-click menu, where the
// menu's step goes back to when it closes early.
const tourReviewSelect = 2

var reviewTour = []tourStep{
	{anchor: tourReviewChip, side: driver.Top, until: (*window).tourReviewOpen,
		text: tourText{"tourReview.chip.title", "tourReview.chip.body", "tourReview.chip.hint"}},
	{anchor: tourReviewPanel, side: driver.Left, text: tourText{"tourReview.panel.title", "tourReview.panel.body", ""}},
	{anchor: tourReviewPanel, side: driver.Left, until: (*window).tourReviewMenuOpen,
		text: tourText{"tourReview.select.title", "tourReview.select.body", "tourReview.select.hint"}},
	{anchor: tourReviewItem, side: driver.Left, layer: 1, reopen: tourReviewSelect, until: (*window).tourReviewComposing,
		text: tourText{"tourReview.pick.title", "tourReview.pick.body", "tourReview.pick.hint"}},
	{anchor: tourReviewComposer, side: driver.Left, until: (*window).tourReviewCommented,
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

func (u *window) tourReviewMenuOpen() bool { return u.review.menuOpen() }

func (u *window) tourReviewComposing() bool { return u.review.composing() }

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
