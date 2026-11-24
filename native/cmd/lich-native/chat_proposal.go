package main

import (
	"slices"

	"gioui.org/layout"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

// The agent asking to start on the plan it wrote is a question item with a
// proposal: the plan as Markdown in the conversation, and under it the
// question card, its options the modes to start in, named as Claude Code's
// TUI names them, and its field the words that keep the agent planning.

// chatProposal is the plan, then the card that asks to start on it.
func (u *window) chatProposal(gtx C, th *ui.Theme, p *chatPane, it backend.ChatItem) D {
	plan := it.Question.Proposal.Plan
	notify := func(t ui.Toast) { u.toaster.Show(t) }
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if plan == "" {
				return D{}
			}
			return layout.Inset{Bottom: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
				return p.md.LayoutLinks(gtx, th, it.ID, plan, markdown.Links{Open: func(url string) { openLink(u.client, notify, url) }, Paths: p.paths})
			})
		}),
		layout.Rigid(func(gtx C) D { return chatQuestion(gtx, th, it) }),
	)
}

// proposalShown is a proposal as the card draws it: its ask and its options
// in the window's words, and its reply read as one of them. The item's own
// asks stay as they are for the keys, which answer with the mode's id.
func proposalShown(it backend.ChatItem) backend.ChatItem {
	q := *it.Question
	asks := make([]api.ChatAsk, len(q.Questions))
	for i, a := range q.Questions {
		a.Header, a.Text = i18n.T("chat.proposal.header"), i18n.T("chat.proposal.ask")
		a.Options = slices.Clone(a.Options)
		for j := range a.Options {
			a.Options[j].Label = proposalModeLabel(a.Options[j].Label)
		}
		asks[i] = a
	}
	q.Questions = asks
	if q.Reply != nil {
		q.Reply = proposalReplyShown(q.Reply)
	}
	it.Question = &q
	return it
}

// proposalReplyShown is how the answered card reads a reply: the mode picked,
// "keep planning" with the words that came with it, or approved in a mode
// the transcript did not keep.
func proposalReplyShown(r *api.ChatQReplied) *api.ChatQReplied {
	ans := api.ChatAskAnswer{}
	switch {
	case r.Declined || len(r.Answers) == 0:
		ans.Picked = []string{i18n.T("chat.proposal.keep")}
	case len(r.Answers[0].Picked) > 0:
		ans.Picked = []string{proposalModeLabel(r.Answers[0].Picked[0])}
	case r.Answers[0].Other != "":
		ans.Picked, ans.Other = []string{i18n.T("chat.proposal.keep")}, r.Answers[0].Other
	default:
		ans.Picked = []string{i18n.T("chat.proposal.approved")}
	}
	return &api.ChatQReplied{Answers: []api.ChatAskAnswer{ans}}
}

// proposalModeLabel names a mode the work can start in: Claude Code's are
// "Yes, auto-accept edits" and "Yes, manually approve edits" (2.1.330).
func proposalModeLabel(mode string) string {
	switch mode {
	case "acceptEdits":
		return i18n.T("chat.proposal.acceptEdits")
	case "default":
		return i18n.T("chat.proposal.manual")
	}
	return mode
}
