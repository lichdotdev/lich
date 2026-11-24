package main

import (
	"image"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// chatPreviewShare is how much of the card the option list takes beside a
// preview, the mockup's 1fr 1fr.
const chatPreviewShare = 0.5

// chatAnswerJoin joins the options picked for one ask in the answered card.
const chatAnswerJoin = ", "

// chatRecommended ends the label of the option the agent recommends: Claude
// Code's AskUserQuestion prompt tells the model to add it.
const chatRecommended = "(Recommended)"

// The "Other…" field is laid out over its row's line: chatOtherPadX is
// ui.Input's own inner padding, pulled back so its text starts where the
// labels do, and chatOtherHeight its height, chatOtherPadY of it above and
// under the line.
var (
	chatOtherPadX   = ui.Space(2.5)
	chatOtherPadY   = ui.Space(1)
	chatOtherHeight = ui.Space(7)
)

// chatRowLine is a label's line in an option row, TextSM's leading
// (text-sm/5): Continue's arrow centres on it.
const chatRowLine unit.Sp = 20

var (
	icQuestionDecline  = icons.Lucide("x")
	icQuestionContinue = icons.Lucide("arrow-right")
)

// chatAsking is the waiting question's card, over the composer, a gap under
// it; nothing while none waits.
func (u *window) chatAsking(gtx C, th *ui.Theme, p *chatPane, sv sessionView) D {
	waiting := questionWaiting(u.model.chat.of(sv.ID))
	if waiting == nil {
		return D{}
	}
	it := *waiting
	if it.Question.Proposal != nil {
		it = proposalShown(it)
	}
	q := p.quiz(it.ID, it.Question.Questions)
	q.proposal = it.Question.Proposal != nil
	return layout.Inset{Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return chatQuestionCard(gtx, th, q, it.Question.Questions)
	})
}

// asksInComposer is whether it is a question the composer holds while it
// waits, instead of the conversation: all but a plan, which stays in it.
func asksInComposer(it backend.ChatItem) bool {
	return it.Kind == api.ItemQuestion && it.Question != nil && it.Question.Reply == nil && it.Question.Proposal == nil
}

// chatQuestion is a question in the conversation: once answered, the answer
// as the person's own message; nothing while it waits over the composer.
func chatQuestion(gtx C, th *ui.Theme, it backend.ChatItem) D {
	if it.Question.Reply == nil {
		return D{}
	}
	if it.Question.Proposal != nil {
		it = proposalShown(it)
	}
	return chatQuestionAnswered(gtx, th, it.Question.Questions, it.Question.Reply)
}

// chatQuestionCard is the box of a question waiting, its card q.
func chatQuestionCard(gtx C, th *ui.Theme, q *chatQuiz, asks []api.ChatAsk) D {
	box := th.On(th.Sidebar)
	d := layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, th.Sidebar, ui.RadiusLG) },
		func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5), Left: ui.Space(1.5), Right: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return chatQuestionHead(gtx, box, q, asks) }),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Bottom: ui.Space(1.5)}.Layout(gtx,
							box.Text(ui.TextSM, asks[q.step].Text).Weight(font.Medium).Wrapped().Layout)
					}),
					layout.Rigid(func(gtx C) D { return chatQuestionBody(gtx, box, q, asks) }),
				)
			})
		})
	ui.InsetBorder(gtx, d.Size, gtx.Dp(ui.RadiusLG), th.Border)
	return d
}

// chatQuestionHead is the ask's header, or a tab per ask of several, and the
// ✕ that declines.
func chatQuestionHead(gtx C, th *ui.Theme, q *chatQuiz, asks []api.ChatAsk) D {
	tabs := make([]layout.FlexChild, 0, 2*len(asks)+2)
	if len(asks) == 1 {
		tabs = append(tabs, layout.Rigid(th.Text(ui.TextXS, asks[0].Header).Muted().Layout))
	} else {
		for i, a := range asks {
			tabs = append(tabs, layout.Rigid(func(gtx C) D {
				return q.steps[i].Layout(gtx, func(gtx C) D { return chatAskTab(gtx, th, a.Header, i, q.done[i], i == q.step) })
			}), ui.Gap(1))
		}
	}
	decline := th.Button(&q.decline, "")
	decline.Icon, decline.Variant, decline.Size = icQuestionDecline, ui.VariantGhost, ui.SizeIconXS
	tabs = append(tabs, layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }), layout.Rigid(decline.Layout))
	// A pill's own padding lines its text up with the question's.
	left := ui.Space(2)
	if len(asks) > 1 {
		left = 0
	}
	return layout.Inset{Left: left, Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, tabs...)
	})
}

// chatAskTab is ask i's tab among several, a pill as a project tab is: its
// number, a check in its place once answered, and its header; the one on show
// is filled in Accent.
func chatAskTab(gtx C, th *ui.Theme, label string, i int, done, shown bool) D {
	mark := th.Text(ui.TextXS, strconv.Itoa(i+1)).Muted()
	if done {
		mark = th.Text(ui.TextXS, "✓").In(th.TonePass)
	}
	text := th.Text(ui.TextXS, label).Muted()
	if done || shown {
		text = text.In(th.Foreground)
	}
	return layout.Background{}.Layout(gtx,
		func(gtx C) D {
			if !shown {
				return D{Size: gtx.Constraints.Min}
			}
			return ui.Fill(gtx, th.Accent, gtx.Metric.PxToDp(gtx.Constraints.Min.Y/2))
		},
		func(gtx C) D {
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(0.75), Bottom: ui.Space(0.75)}.Layout(gtx, func(gtx C) D {
				row := []layout.FlexChild{layout.Rigid(mark.Layout)}
				if label != "" {
					row = append(row, ui.Gap(1.5), layout.Rigid(text.Layout))
				}
				return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, row...)
			})
		})
}

// chatQuestionBody is the ask on show: its options, or a field when it has
// none.
func chatQuestionBody(gtx C, th *ui.Theme, q *chatQuiz, asks []api.ChatAsk) D {
	a := asks[q.step]
	if len(a.Options) == 0 {
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(th.Textarea(&q.others[q.step], "").Layout),
				ui.VGap(2),
				layout.Rigid(th.Text(ui.TextXS, i18n.T("chat.question.freeHint")).Muted().Layout),
			)
		})
	}
	if hasPreview(a) {
		return chatOptionsWithPreview(gtx, th, q, a)
	}
	return chatOffers(gtx, th, q, a)
}

func hasPreview(a api.ChatAsk) bool {
	for _, o := range a.Options {
		if o.Preview != "" {
			return true
		}
	}
	return false
}

// chatOffers is an ask's options, a row each, then its "Other…" field and,
// on one that takes several, Continue.
func chatOffers(gtx C, th *ui.Theme, q *chatQuiz, a api.ChatAsk) D {
	rows := make([]layout.FlexChild, 0, len(a.Options)+2)
	for j, o := range a.Options {
		rows = append(rows, layout.Rigid(func(gtx C) D { return chatOptionRow(gtx, th, q, a, j, o, !hasPreview(a)) }))
	}
	rows = append(rows, layout.Rigid(func(gtx C) D { return chatOtherRow(gtx, th, q, a) }))
	if a.Multi {
		rows = append(rows, layout.Rigid(func(gtx C) D { return chatContinueRow(gtx, th, q, a) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

// chatOptionsWithPreview is the options at the left and the preview of the
// one the keys stand on at the right, in mono.
func chatOptionsWithPreview(gtx C, th *ui.Theme, q *chatQuiz, a api.ChatAsk) D {
	preview := ""
	if q.hot < len(a.Options) {
		preview = a.Options[q.hot].Preview
	}
	return layout.Flex{}.Layout(gtx,
		layout.Flexed(chatPreviewShare, func(gtx C) D { return chatOffers(gtx, th, q, a) }),
		ui.Gap(2),
		layout.Flexed(1-chatPreviewShare, func(gtx C) D {
			return layout.Background{}.Layout(gtx,
				func(gtx C) D { return ui.Fill(gtx, th.Background, ui.RadiusSM) },
				func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.UniformInset(ui.Space(2)).Layout(gtx, th.On(th.Background).Text(ui.TextXS, expandTabs(preview)).Mono().Wrapped().Layout)
				})
		}),
	)
}

// recommended is label without the mark the agent ends it with to recommend
// it, and whether it had it.
func recommended(label string) (string, bool) {
	rest, ok := strings.CutSuffix(strings.TrimSpace(label), chatRecommended)
	return strings.TrimSpace(rest), ok
}

// chatOptionRow is option j: its key (a box to mark, for several), its label,
// the recommendation and its description; the one the keys stand on is lit,
// with Enter's cap when Enter answers with it.
func chatOptionRow(gtx C, th *ui.Theme, q *chatQuiz, a api.ChatAsk, j int, o api.ChatOffer, described bool) D {
	click := &q.rows[q.step][j]
	return click.Layout(gtx, func(gtx C) D {
		lead := func(gtx C) D { return th.KeyCap(strconv.Itoa(j + 1)).Layout(gtx) }
		if a.Multi {
			// The row is the one target: a box taking the click too would mark
			// the option and the row's click unmark it, in the same press.
			lead = func(gtx C) D { return th.Checkbox(&q.marks[q.step][j]).Layout(gtx.Disabled()) }
		}
		lit := q.hot == j
		label, rec := recommended(o.Label)
		return chatQuizRow(gtx, th, lit || click.Hovered(), lit && !a.Multi, lead, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
						layout.Rigid(th.Text(ui.TextSM, label).Layout),
						layout.Rigid(func(gtx C) D {
							if !rec {
								return D{}
							}
							return layout.Inset{Left: ui.Space(2)}.Layout(gtx, th.Text(ui.TextXS, i18n.T("chat.question.recommended")).In(th.Primary).Layout)
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					if !described || o.Description == "" {
						return D{}
					}
					return th.Text(ui.TextXS, o.Description).Muted().Wrapped().Layout(gtx)
				}),
			)
		})
	})
}

// chatOtherRow is the person's own answer, a bare field after the options
// whose placeholder reads as the row's label.
func chatOtherRow(gtx C, th *ui.Theme, q *chatQuiz, a api.ChatAsk) D {
	n := len(a.Options)
	field := &q.others[q.step]
	lead := func(gtx C) D { return th.KeyCap(strconv.Itoa(n + 1)).Layout(gtx) }
	if a.Multi {
		lead = func(gtx C) D { return D{Size: image.Pt(gtx.Dp(ui.Space(4)), 0)} }
	}
	hint := i18n.T("chat.question.write")
	if q.proposal {
		hint = i18n.T("chat.proposal.change")
	}
	lit := q.hot == n || gtx.Focused(field)
	return chatQuizRow(gtx, th, lit, lit && field.Len() > 0, lead, func(gtx C) D {
		in := th.Input(field, hint)
		in.Bare, in.Height = true, chatOtherHeight
		return layout.Inset{Left: -chatOtherPadX, Top: -chatOtherPadY, Bottom: -chatOtherPadY}.Layout(gtx, in.Layout)
	})
}

// chatContinueRow sends the marks of an ask that takes several, with their
// count; muted while nothing is marked or written.
func chatContinueRow(gtx C, th *ui.Theme, q *chatQuiz, a api.ChatAsk) D {
	_, ready := q.draft(a)
	marked := 0
	for _, m := range q.marks[q.step] {
		if m.Value {
			marked++
		}
	}
	lit := q.hot == len(a.Options)+1
	return q.send.Layout(gtx, func(gtx C) D {
		ink := th.MutedForeground
		if ready {
			ink = th.Foreground
		}
		lead := func(gtx C) D {
			gtx.Constraints.Min = image.Pt(gtx.Dp(ui.Space(4)), gtx.Sp(chatRowLine))
			return ui.CenteredIcon(gtx, icQuestionContinue, ui.Space(4), ink)
		}
		return chatQuizRow(gtx, th, lit || q.send.Hovered(), lit && ready, lead, func(gtx C) D {
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextSM, i18n.T("chat.question.continue")).In(ink).Layout),
				ui.Gap(2),
				layout.Rigid(th.Text(ui.TextXS, i18n.T("chat.question.marked", i18n.Args{"count": marked})).Muted().Layout),
			)
		})
	})
}

// chatQuizRow is one row of options: lead, then content, on Accent while lit,
// and Enter's cap at its end when enter says Enter answers with it.
func chatQuizRow(gtx C, th *ui.Theme, lit, enter bool, lead, content layout.Widget) D {
	return layout.Background{}.Layout(gtx,
		func(gtx C) D {
			if !lit {
				return D{Size: gtx.Constraints.Min}
			}
			return ui.Fill(gtx, th.Over(th.Accent), ui.RadiusMD)
		},
		func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Start}.Layout(gtx,
					layout.Rigid(lead),
					ui.Gap(2.5),
					layout.Flexed(1, content),
					layout.Rigid(func(gtx C) D {
						if !enter {
							return D{}
						}
						return layout.Inset{Left: ui.Space(2)}.Layout(gtx, th.KeyCap("↵").Layout)
					}),
				)
			})
		})
}

// chatQuestionAnswered is a question once answered, as the person's message
// at the column's right: a line per ask, its header and its answer; declined,
// a muted word there.
func chatQuestionAnswered(gtx C, th *ui.Theme, asks []api.ChatAsk, r *api.ChatQReplied) D {
	if r.Declined {
		return layout.E.Layout(gtx, th.Text(ui.TextXS, i18n.T("chat.question.declined")).Muted().Layout)
	}
	maxW := int(float32(gtx.Constraints.Max.X) * chatUserShare)
	on := th.On(th.Accent)
	return layout.E.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = image.Point{}
		gtx.Constraints.Max.X = maxW
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, th.Accent, ui.RadiusLG) },
			func(gtx C) D {
				return layout.Inset{Top: ui.Space(2), Bottom: ui.Space(2), Left: ui.Space(3), Right: ui.Space(3)}.Layout(gtx, func(gtx C) D {
					return chatAnswerLines(gtx, on, asks, r.Answers)
				})
			})
	})
}

// chatAnswerLines is a muted header and its answer, a line per ask.
func chatAnswerLines(gtx C, th *ui.Theme, asks []api.ChatAsk, answers []api.ChatAskAnswer) D {
	lines := make([]layout.FlexChild, 0, len(asks))
	for i, a := range asks {
		if i >= len(answers) {
			break
		}
		label := a.Header
		if label == "" {
			label = a.Text
		}
		lines = append(lines, layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextSM, i18n.T("chat.question.label", i18n.Args{"header": label})).Muted().Layout),
				ui.Gap(1.5),
				layout.Rigid(th.Text(ui.TextSM, answerText(answers[i])).Wrapped().Layout),
			)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, lines...)
}

// answerText is an ask's answer as the card reads it: the options picked, and
// the person's own words in quotes.
func answerText(a api.ChatAskAnswer) string {
	parts := append([]string(nil), a.Picked...)
	if a.Other != "" {
		parts = append(parts, i18n.T("chat.question.own", i18n.Args{"text": a.Other}))
	}
	return strings.Join(parts, chatAnswerJoin)
}
